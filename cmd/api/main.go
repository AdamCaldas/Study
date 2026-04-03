package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"studfy-backend/internal/admin"
	"studfy-backend/internal/auth"
	"studfy-backend/internal/focus"
	"studfy-backend/internal/gamification"
	"studfy-backend/internal/notebook"
	"studfy-backend/internal/space"
	"studfy-backend/internal/study"
	"studfy-backend/internal/users"
	"studfy-backend/pkg/database"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Aviso: Arquivo .env não encontrado. Usando variáveis de ambiente do sistema.")
	}

	database.ConnectDB()

	router := gin.Default()

	// ==========================================================
	// 🗜️ MIDDLEWARES GLOBAIS
	// ==========================================================
	router.Use(gzip.Gzip(gzip.DefaultCompression)) // Compressão de payload
	router.Use(cors.New(cors.Config{               // Configuração de CORS
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Content-Length", "Accept-Encoding",
			"X-CSRF-Token", "Authorization", "Accept", "Cache-Control", "X-Requested-With",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// ==========================================================
	// 🔓 ROTAS PÚBLICAS (Autenticação e Health Check)
	// ==========================================================
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong! Servidor StudFy Operacional 🚀"})
	})

	router.POST("/v1/register", auth.Register)
	router.POST("/v1/verify-email", auth.VerifyEmailCode)
	router.POST("/v1/login", auth.Login)
	router.POST("/v1/forgot-password", auth.ForgotPassword)
	router.POST("/v1/reset-password", auth.ResetPassword)

	// ==========================================================
	// 🛡️ ROTAS PROTEGIDAS DO USUÁRIO (Requer JWT)
	// ==========================================================
	protected := router.Group("/v1/app")
	protected.Use(auth.AuthMiddleware())
	{
		// 👉 Conta e Perfil
		protected.GET("/me", users.GetMyProfile)
		protected.PUT("/me", users.UpdateMyProfile)
		protected.PATCH("/me/settings", users.UpdateMySettings)
		protected.PUT("/me/password", users.UpdatePassword)
		protected.DELETE("/me", users.DeleteMyAccount)
		protected.POST("/me/become-teacher", users.BecomeTeacher)
		protected.POST("/me/availability", users.SaveAvailabilityProfile)
		protected.GET("/me/analytics", study.GetMyStudyAnalytics)
		protected.GET("/me/dashboard", study.GetPersonalDashboard)

		// 👉 Rotas de Configuração do Usuário
		protected.PUT("/availability/:availability_id", study.UpdateAvailabilityProfile)

		// 👉 Repositório Global do Professor
		protected.GET("/me/question-bank", study.GetMyQuestionBank)
		protected.POST("/me/question-bank", study.SaveToQuestionBank)

		// 👇 AS DUAS ROTAS NOVAS AQUI!
		protected.POST("/questions/import-enem", study.ImportEnemQuestions)
		protected.GET("/questions/public", study.GetPublicQuestions)

		// 👉 Notificações e Suporte (Bugs)
		protected.GET("/notifications", admin.GetMyNotifications)
		protected.POST("/notifications/:id/read", admin.MarkNotificationAsRead)
		protected.POST("/bugs", admin.ReportBug)

		// 👉 Central de Ajuda (Visão do Aluno - StudFy Academy)
		protected.GET("/help-center", admin.GetHelpCenter)

		// 👉 Rede Social (Vitrine)
		protected.GET("/teachers/:id", users.GetTeacherProfile)
		protected.POST("/teachers/:id/follow", users.FollowTeacher)
		protected.DELETE("/teachers/:id/follow", users.UnfollowTeacher)

		// 👉 Foco e Gamificação Global
		protected.POST("/gamification/reward", gamification.RewardXP)
		protected.POST("/focus/pomodoro", focus.RegisterPomodoro)
		protected.POST("/focus/mood", focus.RegisterMood)

		// 👉 Gestão Macro de Spaces (Turmas)
		protected.POST("/spaces", auth.CheckSpaceLimit(), space.CreateSpace)
		protected.GET("/spaces", space.ListSpaces)
		protected.GET("/spaces/code/:code", space.GetSpaceByCode)
		protected.POST("/spaces/join", space.RequestSpaceAccess)
		protected.POST("/attendance/check-in", space.RegisterAttendance)

		// 👉 Organização Externa de Cadernos (Pastas e Guias)
		protected.GET("/notebooks/:notebook_id", notebook.GetNotebookFull)
		protected.POST("/notebooks/:notebook_id/guides", notebook.CreateGuide)
		protected.PUT("/guides/:guide_id", notebook.UpdateGuide)
		protected.DELETE("/guides/:guide_id", notebook.DeleteGuide)
		protected.POST("/notebooks/:notebook_id/pages", notebook.CreatePage)
		protected.PATCH("/notebooks/:notebook_id/pages/reorder", notebook.ReorderPages)
		protected.PUT("/pages/:page_id", notebook.UpdatePage)
		protected.DELETE("/pages/:page_id", notebook.DeletePage)

		// Exemplo de como deve ficar a rota lá no seu main.go / routes.go:
		protected.PUT("/spaces/:space_id/cycles/full-update", study.UpdateFullCycle)

		// ------------------------------------------------------
		// 🏰 ROTAS INTERNAS DO SPACE (Contexto da Sala de Aula)
		// ------------------------------------------------------
		spaceRoutes := protected.Group("/spaces/:space_id")
		spaceRoutes.Use(auth.CheckSpaceAccess())
		{
			// =======================================================
			// ⚡ O NOVO PADRÃO "LAZY LOADING" (Mini-Rotas)
			// =======================================================
			spaceRoutes.GET("", space.GetSpaceDetails)
			spaceRoutes.GET("/notebooks", notebook.ListSpaceNotebooks)
			spaceRoutes.GET("/notes", space.ListSpaceNotes)
			spaceRoutes.GET("/quizzes", study.ListSpaceQuizzes)
			spaceRoutes.GET("/dashboard", study.GetSpaceDashboard)

			// 👉 Setup e Moderação da Turma
			spaceRoutes.PUT("", space.UpdateSpace)
			spaceRoutes.DELETE("", space.DeleteSpace)
			spaceRoutes.POST("/share", space.ShareSpace)
			spaceRoutes.GET("/history", space.GetSpaceHistory)
			spaceRoutes.GET("/requests", space.ListSpaceRequests)
			spaceRoutes.POST("/requests/:request_id/respond", space.RespondSpaceRequest)
			spaceRoutes.PUT("/collaborators/:user_id", space.UpdateCollaborator)
			spaceRoutes.DELETE("/collaborators/:user_id", space.RemoveCollaborator)
			spaceRoutes.GET("/dossier/:student_id", space.GetOrUpdateStudentDossier)
			spaceRoutes.PUT("/dossier/:student_id", space.GetOrUpdateStudentDossier)

			// 👉 Ações de Cadernos e Notas Rápidas
			spaceRoutes.POST("/notebooks", notebook.CreateNotebook)
			spaceRoutes.PUT("/notebooks/:notebook_id", notebook.UpdateNotebook)
			spaceRoutes.DELETE("/notebooks/:notebook_id", notebook.DeleteNotebook)
			spaceRoutes.POST("/notes", space.CreateQuickNote)
			spaceRoutes.PUT("/notes/:note_id", space.UpdateQuickNote)
			spaceRoutes.DELETE("/notes/:note_id", space.DeleteQuickNote)

			// =======================================================
			// 👉 MOTOR DO CRONOGRAMA (FIXED)
			// =======================================================
			spaceRoutes.POST("/plans/auto-generate", study.GenerateAutoPlan)
			spaceRoutes.GET("/plans", study.ListPlans)
			spaceRoutes.PATCH("/plans/execute", study.ExecutePlanBlock)
			spaceRoutes.POST("/plans", study.CreateStudyPlan)
			spaceRoutes.POST("/plans/batch", study.CreateMultipleStudyPlans)
			spaceRoutes.PUT("/plans/:plan_id", study.UpdateStudyPlan)
			spaceRoutes.DELETE("/plans/:plan_id", study.DeleteStudyPlan)

			// =======================================================
			// 👉 MOTOR DO CICLO (ADAPTIVE - A Roleta)
			// =======================================================
			spaceRoutes.POST("/cycles/auto-generate", study.GenerateAutoCycle)
			spaceRoutes.GET("/cycles", study.ListCycles)
			spaceRoutes.PATCH("/cycles/advance", study.AdvanceCycleStep)
			spaceRoutes.POST("/cycles/blocks", study.CreateCycleBlock)
			spaceRoutes.PUT("/cycles/blocks/:block_id", study.UpdateCycleBlock)
			spaceRoutes.DELETE("/cycles/blocks/:block_id", study.DeleteCycleBlock)

			// 👉 Avaliações e Anti-Cola
			spaceRoutes.POST("/reviews", study.CreateReview)
			spaceRoutes.POST("/quizzes", study.CreateQuiz)
			spaceRoutes.POST("/quizzes/:quiz_id/submit", study.SubmitQuiz)
			spaceRoutes.POST("/quizzes/:quiz_id/cheat-alert", study.ReportCheatAttempt)
			spaceRoutes.PUT("/quizzes/results/:result_id/grade", study.GradeQuizManual)
			spaceRoutes.POST("/certificate", study.ClaimCertificate)

			// 👉 Comunicação Integrada
			spaceRoutes.GET("/doubts", space.ListSpaceDoubts)
			spaceRoutes.POST("/pages/:page_id/doubts", space.CreatePageDoubt)
			spaceRoutes.PUT("/doubts/:doubt_id/answer", space.AnswerPageDoubt)
			spaceRoutes.POST("/megafone", space.SendMegaphoneMessage)
			spaceRoutes.POST("/attendance", space.GenerateAttendanceQR)

			// 👉 Gamificação da Turma
			spaceRoutes.GET("/missions", gamification.GetActiveMissions)
			spaceRoutes.POST("/missions", gamification.CreateFlashMission)
			spaceRoutes.POST("/missions/:mission_id/complete", gamification.CompleteFlashMission)
			spaceRoutes.POST("/badges", gamification.CreateBadge)
			spaceRoutes.POST("/badges/:badge_id/award/:student_id", gamification.AwardBadge)
			spaceRoutes.GET("/ranking", gamification.GetSpaceRanking)
			spaceRoutes.PATCH("/ranking/toggle", gamification.ToggleSpaceRanking)
			spaceRoutes.GET("/arena/live", gamification.JoinArenaMode)

			// 👉 Analytics e Automação (Painel do Diretor)
			spaceRoutes.GET("/analytics/thermometer", space.GetClassThermometer)
			spaceRoutes.GET("/analytics/export-diary", space.ExportClassDiaryCSV)
			spaceRoutes.POST("/automation/rules", space.CreateAutomationRule)
		}
	}

	// ==========================================================
	// ⚡ MODO DEUS (Painel Admin Global)
	// ==========================================================
	godMode := router.Group("/v1/admin")
	godMode.Use(auth.AuthMiddleware(), auth.AdminOnly())
	{
		// (Rotas do Admin Global inalteradas...)
		godMode.GET("/report", admin.GetPlatformReport)
		godMode.GET("/reports/plans", admin.GetUsersByPlan)
		godMode.GET("/reports/ranking", admin.GetTopUsersXP)
		godMode.GET("/reports/moods", admin.GetMoodStats)
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
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Iniciando servidor na porta %s...", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Erro crítico no servidor: %v", err)
	}
}
