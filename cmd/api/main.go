package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"studfy-backend/internal/admin"
	"studfy-backend/internal/auth"
	"studfy-backend/internal/bff"
	"studfy-backend/internal/focus"
	"studfy-backend/internal/gamification"
	"studfy-backend/internal/middleware"
	"studfy-backend/internal/notebook"
	"studfy-backend/internal/space"
	"studfy-backend/internal/study"
	"studfy-backend/internal/users"
	"studfy-backend/pkg/database"

	"github.com/MicahParks/keyfunc/v2" // 👈 1. IMPORT ADICIONADO
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// 1. Carrega Variáveis de Ambiente
	err := godotenv.Load()
	if err != nil {
		log.Println("Aviso: Arquivo .env não encontrado. Usando variáveis de ambiente do sistema (Modo Produção).")
	}

	// 2. Conecta ao Banco (Agora ultra-rápido sem AutoMigrate!)
	database.ConnectDB()

	// ==========================================================
	// 🔑 KEYCLOAK OIDC / JWKS
	// ==========================================================
	keycloakURL := os.Getenv("KEYCLOAK_URL")
	if keycloakURL == "" {
		keycloakURL = "http://localhost:8080/realms/studfy"
	}
	jwksURL := keycloakURL + "/protocol/openid-connect/certs"

	// Carrega e atualiza as chaves públicas do Keycloak em background.
	// Tentamos algumas vezes porque o Keycloak pode ainda estar subindo.
	jwksOptions := keyfunc.Options{
		RefreshInterval:   time.Hour, // recarrega as chaves de hora em hora
		RefreshUnknownKID: true,      // busca a chave na hora se o `kid` for novo (rotação)
		RefreshTimeout:    10 * time.Second,
		RefreshErrorHandler: func(err error) {
			log.Printf("Aviso: falha ao atualizar as chaves do Keycloak: %v", err)
		},
	}

	var jwks *keyfunc.JWKS
	for attempt := 1; attempt <= 5; attempt++ {
		jwks, err = keyfunc.Get(jwksURL, jwksOptions)
		if err == nil {
			break
		}
		log.Printf("Tentativa %d/5: Keycloak indisponível em %s (%v)", attempt, jwksURL, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("Erro ao carregar chaves públicas do Keycloak (%s): %v", jwksURL, err)
	}
	log.Printf("Keycloak: chaves públicas carregadas de %s", jwksURL)

	// 3. Inicia o Roteador Gin em Release Mode se estiver em produção
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()

	// ==========================================================
	// 🗜️ MIDDLEWARES GLOBAIS DE SEGURANÇA E PERFORMANCE
	// ==========================================================
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.SecureCORS())
	router.Use(middleware.RateLimiter())

	// ==========================================================
	// 🔓 ROTAS PÚBLICAS
	// ==========================================================
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong! Servidor StudFy Operacional 🚀"})
	})

	router.POST("/v1/register", auth.Register)
	router.POST("/v1/verify-email", auth.VerifyEmailCode)
	router.POST("/v1/login", auth.Login)
	router.POST("/v1/forgot-password", auth.ForgotPassword)
	router.POST("/v1/reset-password", auth.ResetPassword)
	router.POST("/v1/auth/google", auth.GoogleAuth)

	// ==========================================================
	// 🛡️ ROTAS PROTEGIDAS DO USUÁRIO
	// ==========================================================
	protected := router.Group("/v1/app")
	protected.Use(auth.AuthMiddleware(jwks)) // 👈 2. PASSANDO O JWKS AQUI
	{
		// 🚀 ROTA MÁGICA DO FRONT-END (O COMBO!)
		protected.GET("/bootstrap", bff.GetAppBootstrap)

		protected.GET("/me", users.GetMyProfile)
		protected.PUT("/me", users.UpdateMyProfile)
		protected.PATCH("/me/settings", users.UpdateMySettings)
		protected.PUT("/me/password", users.UpdatePassword)
		protected.DELETE("/me", users.DeleteMyAccount)
		protected.POST("/me/become-teacher", users.BecomeTeacher)
		protected.POST("/me/availability", users.SaveAvailabilityProfile)
		protected.GET("/me/analytics", study.GetMyStudyAnalytics)
		protected.GET("/me/dashboard", study.GetPersonalDashboard)

		protected.GET("/me/analytics/heatmap", study.GetStudyHeatmap)
		protected.GET("/me/analytics/strengths", study.GetStrengthsAndWeaknesses)
		protected.GET("/me/analytics/efficiency", study.GetTimeEfficiency)

		protected.PUT("/availability/:availability_id", users.UpdateAvailabilityProfile)

		protected.GET("/questions/studfy", study.ListStudfyQuestions)

		// 🧠 Curva de esquecimento: a fila de revisões do aluno e a conclusão
		// (que agenda a próxima com intervalo maior).
		protected.GET("/reviews", study.ListMyReviews)
		protected.POST("/reviews/:review_id/complete", study.CompleteReview)

		protected.GET("/notifications", admin.GetMyNotifications)
		protected.POST("/notifications/:id/read", admin.MarkNotificationAsRead)
		protected.POST("/bugs", admin.ReportBug)

		protected.GET("/help-center", admin.GetHelpCenter)

		protected.GET("/teachers/:id", users.GetTeacherProfile)
		protected.POST("/teachers/:id/follow", users.FollowTeacher)
		protected.DELETE("/teachers/:id/follow", users.UnfollowTeacher)

		protected.POST("/gamification/reward", gamification.RewardXP)
		protected.POST("/focus/pomodoro", focus.RegisterPomodoro)
		protected.POST("/focus/mood", focus.RegisterMood)

		protected.POST("/spaces", auth.CheckSpaceLimit(), space.CreateSpace)
		protected.GET("/spaces", space.ListSpaces)
		protected.GET("/spaces/code/:code", space.GetSpaceByCode)
		protected.POST("/spaces/join", space.RequestSpaceAccess)
		protected.POST("/attendance/check-in", space.RegisterAttendance)

		protected.GET("/notebooks/:notebook_id", notebook.GetNotebookFull)
		protected.POST("/notebooks/:notebook_id/guides", notebook.CreateGuide)
		protected.PUT("/guides/:guide_id", notebook.UpdateGuide)
		protected.DELETE("/guides/:guide_id", notebook.DeleteGuide)
		protected.PATCH("/notebooks/:notebook_id/guides/reorder", notebook.ReorderGuides)

		protected.POST("/guides/:guide_id/pages", notebook.CreatePage)
		protected.GET("/guides/:guide_id/pages", notebook.ListPagesByGuide)
		protected.PATCH("/pages/reorder", notebook.ReorderPages)
		protected.PUT("/pages/:page_id", notebook.UpdatePage)
		protected.DELETE("/pages/:page_id", notebook.DeletePage)

		protected.GET("/analytics/productivity", study.GetProductivityReport)
		protected.GET("/analytics/quiz-performance", study.GetQuizPerformanceReport)
		protected.GET("/analytics/focus-mood", study.GetFocusAndMoodReport)
		protected.GET("/analytics/gamification", study.GetGamificationReport)
		protected.GET("/analytics/debts", study.GetStudyDebtReport)
		protected.GET("/analytics/engagement", study.GetSpaceEngagementReport)
		protected.GET("/analytics/reviews", study.GetPendingReviewsReport)

		// ------------------------------------------------------
		// 🏰 ROTAS INTERNAS DO SPACE (Contexto da Sala de Aula)
		// ------------------------------------------------------
		spaceRoutes := protected.Group("/spaces/:space_id")
		spaceRoutes.Use(auth.CheckSpaceAccess())
		{
			spaceRoutes.GET("", space.GetSpaceDetails)
			spaceRoutes.GET("/notebooks", notebook.ListSpaceNotebooks)
			spaceRoutes.GET("/notes", space.ListSpaceNotes)
			spaceRoutes.GET("/quizzes", study.ListSpaceQuizzes)
			spaceRoutes.GET("/dashboard", study.GetSpaceDashboard)

			spaceRoutes.PUT("", auth.RequireSpaceEditInfo(), space.UpdateSpace)
			spaceRoutes.DELETE("", auth.RequireSpaceOwner(), space.DeleteSpace)
			spaceRoutes.POST("/share", space.ShareSpace)
			spaceRoutes.GET("/history", space.GetSpaceHistory)
			spaceRoutes.GET("/requests", auth.RequireSpaceManageMembers(), space.ListSpaceRequests)
			spaceRoutes.POST("/requests/:request_id/respond", auth.RequireSpaceManageMembers(), space.RespondSpaceRequest)
			spaceRoutes.PUT("/collaborators/:user_id", auth.RequireSpaceManageMembers(), space.UpdateCollaborator)
			spaceRoutes.DELETE("/collaborators/:user_id", auth.RequireSpaceManageMembers(), space.RemoveCollaborator)
			spaceRoutes.GET("/dossier/:student_id", space.GetOrUpdateStudentDossier)
			spaceRoutes.PUT("/dossier/:student_id", space.GetOrUpdateStudentDossier)

			spaceRoutes.POST("/questions", auth.RequireSpaceCreateContent(), study.CreateSpaceQuestion)
			spaceRoutes.GET("/questions", study.ListSpaceQuestions)
			spaceRoutes.PUT("/questions/:question_id", auth.RequireSpaceEditContent(), study.UpdateSpaceQuestion)
			spaceRoutes.DELETE("/questions/:question_id", auth.RequireSpaceDeleteContent(), study.DeleteSpaceQuestion)
			spaceRoutes.POST("/questions/clone", auth.RequireSpaceCreateContent(), study.CloneStudfyQuestion)

			spaceRoutes.POST("/notebooks", auth.RequireSpaceCreateContent(), notebook.CreateNotebook)
			spaceRoutes.PUT("/notebooks/:notebook_id", notebook.UpdateNotebook)
			spaceRoutes.DELETE("/notebooks/:notebook_id", notebook.DeleteNotebook)
			spaceRoutes.POST("/notes", auth.RequireSpaceCreateContent(), space.CreateQuickNote)
			spaceRoutes.PUT("/notes/:note_id", auth.RequireSpaceEditContent(), space.UpdateQuickNote)
			spaceRoutes.DELETE("/notes/:note_id", auth.RequireSpaceDeleteContent(), space.DeleteQuickNote)

			spaceRoutes.POST("/plans/auto-generate", study.GenerateAutoPlan)
			spaceRoutes.POST("/plans/auto-fit", study.AutoFitPlanBlocks)
			spaceRoutes.GET("/plans", study.ListPlans)
			spaceRoutes.PATCH("/plans/execute", study.ExecutePlanBlock)
			spaceRoutes.PUT("/plans/full-update", study.UpdateFullPlan)
			spaceRoutes.POST("/plans", study.CreateStudyPlan)
			spaceRoutes.POST("/plans/batch", study.CreateMultipleStudyPlans)
			spaceRoutes.PUT("/plans/:plan_id", study.UpdateStudyPlan)
			spaceRoutes.DELETE("/plans/:plan_id", study.DeleteStudyPlan)

			spaceRoutes.POST("/cycles/auto-generate", study.GenerateAutoCycle)
			spaceRoutes.GET("/cycles", study.ListCycles)
			spaceRoutes.PATCH("/cycles/advance", study.AdvanceCycleStep)
			spaceRoutes.PUT("/cycles/full-update", study.UpdateFullCycle)
			spaceRoutes.POST("/cycles/blocks", study.CreateCycleBlock)
			spaceRoutes.PUT("/cycles/blocks/:block_id", study.UpdateCycleBlock)
			spaceRoutes.DELETE("/cycles/blocks/:block_id", study.DeleteCycleBlock)

			spaceRoutes.POST("/reviews", auth.RequireSpaceCreateContent(), study.CreateReview)
			spaceRoutes.POST("/quizzes", auth.RequireSpaceManageQuizzes(), study.CreateQuiz)
			spaceRoutes.POST("/quizzes/:quiz_id/submit", study.SubmitQuiz)
			spaceRoutes.POST("/quizzes/:quiz_id/cheat-alert", study.ReportCheatAttempt)
			spaceRoutes.PUT("/quizzes/results/:result_id/grade", auth.RequireSpaceManageQuizzes(), study.GradeQuizManual)
			spaceRoutes.POST("/certificate", study.ClaimCertificate)

			spaceRoutes.GET("/doubts", auth.RequireSpaceStaff(), space.ListSpaceDoubts)
			spaceRoutes.POST("/pages/:page_id/doubts", space.CreatePageDoubt)
			spaceRoutes.PUT("/doubts/:doubt_id/answer", auth.RequireSpaceStaff(), space.AnswerPageDoubt)
			spaceRoutes.POST("/megafone", space.SendMegaphoneMessage)
			spaceRoutes.POST("/attendance", space.GenerateAttendanceQR)

			spaceRoutes.GET("/missions", gamification.GetActiveMissions)
			spaceRoutes.POST("/missions", gamification.CreateFlashMission)
			spaceRoutes.POST("/missions/:mission_id/complete", gamification.CompleteFlashMission)
			spaceRoutes.POST("/badges", gamification.CreateBadge)
			spaceRoutes.POST("/badges/:badge_id/award/:student_id", gamification.AwardBadge)
			spaceRoutes.GET("/ranking", gamification.GetSpaceRanking)
			spaceRoutes.PATCH("/ranking/toggle", gamification.ToggleSpaceRanking)

			spaceRoutes.GET("/analytics/thermometer", auth.RequireSpaceStaff(), space.GetClassThermometer)
			spaceRoutes.GET("/analytics/export-diary", auth.RequireSpaceStaff(), space.ExportClassDiaryCSV)
			spaceRoutes.POST("/automation/rules", auth.RequireSpaceOwner(), space.CreateAutomationRule)
			spaceRoutes.GET("/reports/at-risk", auth.RequireSpaceStaff(), space.GetAtRiskStudents)
			spaceRoutes.GET("/reports/mortality", auth.RequireSpaceStaff(), space.GetMaterialMortalityRate)
			spaceRoutes.GET("/reports/engagement", auth.RequireSpaceStaff(), space.GetMaterialEngagement)

			spaceRoutes.POST("/flashcards", auth.RequireSpaceCreateContent(), study.CreateFlashcard)
			spaceRoutes.GET("/flashcards", study.ListFlashcards)
			spaceRoutes.PUT("/flashcards/:card_id", auth.RequireSpaceEditContent(), study.UpdateFlashcard)
			spaceRoutes.DELETE("/flashcards/:card_id", auth.RequireSpaceDeleteContent(), study.DeleteFlashcard)

			spaceRoutes.POST("/flashcard-categories", auth.RequireSpaceCreateContent(), study.CreateCategory)
			spaceRoutes.GET("/flashcard-categories", study.ListCategories)
			spaceRoutes.DELETE("/flashcard-categories/:category_id", auth.RequireSpaceDeleteContent(), study.DeleteCategory)

			spaceRoutes.POST("/flashcard-tags", auth.RequireSpaceCreateContent(), study.CreateTag)
			spaceRoutes.GET("/flashcard-tags", study.ListTags)
			spaceRoutes.DELETE("/flashcard-tags/:tag_id", auth.RequireSpaceDeleteContent(), study.DeleteTag)

			spaceRoutes.POST("/question-groups", auth.RequireSpaceCreateContent(), study.CreateQuestionGroup)
			spaceRoutes.GET("/question-groups", study.ListQuestionGroups)
			spaceRoutes.PUT("/question-groups/:group_id", auth.RequireSpaceEditContent(), study.UpdateQuestionGroup)
			spaceRoutes.DELETE("/question-groups/:group_id", auth.RequireSpaceDeleteContent(), study.DeleteQuestionGroup)
		}
	}

	// ==========================================================
	// ⚡ MODO DEUS (Painel Admin Global)
	// ==========================================================
	godMode := router.Group("/v1/admin")
	godMode.Use(auth.AuthMiddleware(jwks), auth.AdminOnly())
	{
		godMode.GET("/report", admin.GetPlatformReport)
		godMode.GET("/reports/plans", admin.GetUsersByPlan)
		godMode.GET("/reports/ranking", admin.GetTopUsersXP)
		godMode.GET("/reports/moods", admin.GetMoodStats)
		// Relatórios que existiam no código mas não tinham rota (ninguém acessava).
		godMode.GET("/reports/retention", admin.GetPlatformRetentionReport)
		godMode.GET("/reports/plan-distribution", admin.GetPlanDistributionReport)
		godMode.GET("/reports/health", admin.GetPlatformHealthStats)
		godMode.GET("/users", admin.ListAllUsers)
		godMode.PUT("/users/:id", admin.UpdateAnyUser)
		godMode.PUT("/users/:id/password", admin.ForceChangePassword)
		godMode.DELETE("/users/:id", admin.DeleteAnyUser)
		godMode.GET("/spaces", admin.ListAllSpaces)
		godMode.PUT("/spaces/:id/transfer", admin.TransferSpaceOwnership)
		godMode.DELETE("/spaces/:id/collaborators/:user_id", admin.RemoveUserFromSpace)
		godMode.DELETE("/spaces/:id", admin.DeleteAnySpace)
		godMode.PUT("/users/:id/xp", admin.UpdateUserXP)
		godMode.GET("/gamification/rules", admin.ListGamificationRules)
		godMode.POST("/gamification/rules", admin.CreateGamificationRule)
		godMode.PUT("/gamification/rules/:rule_id", admin.UpdateGamificationRule)
		godMode.GET("/notifications", admin.ListAllNotifications)
		godMode.POST("/notifications", admin.CreateNotification)
		godMode.PUT("/notifications/:id", admin.UpdateNotification)
		godMode.DELETE("/notifications/:id", admin.DeleteNotification)
		godMode.GET("/bugs", admin.ListBugs)
		godMode.PUT("/bugs/:id/status", admin.UpdateBugStatus)
		godMode.POST("/help-center/categories", admin.CreateHelpCategory)
		godMode.DELETE("/help-center/categories/:category_id", admin.DeleteHelpCategory)
		godMode.POST("/help-center/articles", admin.CreateHelpArticle)
		godMode.DELETE("/help-center/articles/:article_id", admin.DeleteHelpArticle)
		godMode.POST("/users/batch-delete", admin.MassDeleteUsers)
		godMode.POST("/questions", study.AdminCreateStudfyQuestion)
		godMode.PUT("/questions/:id", study.AdminUpdateStudfyQuestion)
		godMode.DELETE("/questions/:id", study.AdminDeleteStudfyQuestion)
	}

	// 3. ALTERAÇÃO DA PORTA PADRÃO PARA 8082
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second, // relatórios/CSV podem demorar um pouco mais
		IdleTimeout:  60 * time.Second,
	}

	// Sobe o servidor em background para podermos escutar o sinal de desligamento.
	go func() {
		log.Printf("Iniciando servidor na porta %s...", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro crítico no servidor: %v", err)
		}
	}()

	// 🛑 GRACEFUL SHUTDOWN: no deploy/CTRL+C, espera as requisições em andamento
	// terminarem (até 15s) antes de fechar, em vez de derrubar todo mundo na hora.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Desligando o servidor com elegância...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Servidor forçado a desligar: %v", err)
	}
	log.Println("Servidor desligado com sucesso.")
}
