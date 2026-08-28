package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// isEnvTrue lê uma variável de ambiente booleana de forma tolerante.
func isEnvTrue(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}

func legacyAuthEnabled() bool { return isEnvTrue("ENABLE_LEGACY_AUTH") }

// contaPeloKeycloak responde às rotas do login antigo dizendo, sem rodeios,
// onde o cadastro e o login realmente acontecem.
func contaPeloKeycloak(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error":  "Cadastro e login são feitos pelo Keycloak, não por esta API.",
		"detail": "Use o fluxo de login do Keycloak e envie o access token em Authorization: Bearer <token>.",
	})
}

// normalizaKeycloakURL aceita a URL do Keycloak em qualquer formato razoável e
// devolve sempre a URL COMPLETA DO REALM.
//
// Existe porque plataformas de nuvem entregam só "host:porta" (o Render, por
// exemplo, com fromService/hostport). Sem isto, faltava o esquema e o
// /realms/<nome>, o JWKS nunca era encontrado e a API subia sem conseguir
// validar token nenhum.
//
// Aceita:
//
//	""                                  → http://localhost:8080/realms/studfy
//	"studfy-keycloak:10000"             → http://studfy-keycloak:10000/realms/studfy
//	"https://kc.onrender.com"           → https://kc.onrender.com/realms/studfy
//	"https://kc.onrender.com/realms/x"  → mantém como está
func normalizaKeycloakURL(bruta, realm string) string {
	bruta = strings.TrimSpace(strings.TrimRight(bruta, "/"))
	if realm = strings.TrimSpace(realm); realm == "" {
		realm = "studfy"
	}
	if bruta == "" {
		return "http://localhost:8080/realms/" + realm
	}

	// Sem esquema, precisamos adivinhar — e adivinhar errado quebra tudo:
	// o `iss` do token viria com https e a API compararia com http, recusando
	// TODO token com "issuer diferente do configurado".
	//
	// Regra: domínio público (tem ponto e não é localhost) → https.
	// Nome de serviço interno ou localhost → http (rede privada, sem TLS).
	if !strings.HasPrefix(bruta, "http://") && !strings.HasPrefix(bruta, "https://") {
		semPorta := bruta
		if i := strings.Index(semPorta, ":"); i > 0 {
			semPorta = semPorta[:i]
		}
		publico := strings.Contains(semPorta, ".") &&
			!strings.HasPrefix(semPorta, "localhost") &&
			!strings.HasPrefix(semPorta, "127.")
		if publico {
			bruta = "https://" + bruta
		} else {
			bruta = "http://" + bruta
		}
	}

	// Já aponta para um realm: respeita o que foi configurado.
	if strings.Contains(bruta, "/realms/") {
		return bruta
	}
	return bruta + "/realms/" + realm
}

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
	realm := os.Getenv("KEYCLOAK_REALM")
	keycloakURL := normalizaKeycloakURL(os.Getenv("KEYCLOAK_URL"), realm)
	jwksURL := keycloakURL + "/protocol/openid-connect/certs"
	log.Printf("Keycloak: buscando as chaves em %s", keycloakURL)

	// O `iss` do token é sempre a URL PÚBLICA por onde a pessoa fez login —
	// diferente da interna usada acima para baixar as chaves. Normalizamos aqui
	// para que a nuvem possa entregar só o hostname.
	if bruto := os.Getenv("KEYCLOAK_ISSUER"); bruto != "" {
		emitente := normalizaKeycloakURL(bruto, realm)
		os.Setenv("KEYCLOAK_ISSUER", emitente)
		log.Printf("Keycloak: issuer esperado nos tokens = %s", emitente)
	}

	// Busca as chaves públicas do realm.
	//
	// ⚠️ Aqui a API NÃO morre mais se o Keycloak estiver fora.
	// Antes era log.Fatal: sem Keycloak o processo encerrava, o deploy falhava e
	// nem o /health respondia — impossível diagnosticar em nuvem.
	// Agora ela sobe do mesmo jeito e continua tentando em segundo plano.
	// Enquanto as chaves não chegam, toda rota protegida responde 503: falha
	// FECHADA, ninguém entra sem token validado.
	jwks := auth.NewJWKSProvider(jwksURL)
	if !jwks.Carregar(3, 2*time.Second) {
		log.Println("⚠️  Keycloak indisponível em " + jwksURL)
		log.Println("⚠️  A API vai subir assim mesmo, mas as rotas protegidas responderão 503")
		log.Println("⚠️  até o Keycloak responder. Confira a variável KEYCLOAK_URL.")
		jwks.CarregarEmSegundoPlano()
	}

	// 3. Inicia o Roteador Gin em Release Mode se estiver em produção
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	// gin.New() (e não gin.Default()) porque trocamos o logger e o recovery
	// padrão pelos nossos, que gravam JSON estruturado com request_id.
	router := gin.New()

	// ==========================================================
	// 🗜️ MIDDLEWARES GLOBAIS DE SEGURANÇA E PERFORMANCE
	// ==========================================================
	router.Use(middleware.RequestLogger())
	router.Use(middleware.Recovery())
	// O health check não precisa de compressão (é uma linha de texto).
	router.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/ping"})))
	router.Use(middleware.SecureCORS())
	router.Use(middleware.RateLimiter())

	// ==========================================================
	// 🔓 ROTAS PÚBLICAS
	// ==========================================================
	// Sinal de vida simples (o processo está de pé).
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong! Servidor StudFy Operacional 🚀"})
	})

	// ❤️ Health check DE VERDADE, para o balanceador e o deploy.
	// O /ping respondia "operacional" mesmo com o banco fora — e o balanceador
	// mandava aluno para uma instância que não conseguia responder nada.
	router.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		bancoOK := database.Ping(ctx) == nil
		keycloakOK := jwks.Pronto()

		estado := gin.H{
			"status":   "ok",
			"database": "ok",
			"keycloak": "ok",
		}
		if !bancoOK {
			estado["database"] = "inacessível"
		}
		if !keycloakOK {
			estado["keycloak"] = "indisponível"
			estado["keycloak_url"] = jwks.URL()
		}

		// Sem banco a API não serve para nada: 503 tira a instância do balanceador.
		// Sem Keycloak ela ainda responde rotas públicas, então segue "ok" com aviso.
		if !bancoOK {
			estado["status"] = "degradado"
			c.JSON(http.StatusServiceUnavailable, estado)
			return
		}
		if !keycloakOK {
			estado["status"] = "parcial"
		}
		c.JSON(http.StatusOK, estado)
	})

	// ----------------------------------------------------------
	// 🔑 CADASTRO E LOGIN: quem cuida disso é o KEYCLOAK.
	// ----------------------------------------------------------
	// Estas rotas são do sistema de login antigo (senha no nosso banco, token
	// HS256). Elas ficavam ABERTAS e "funcionando": o usuário se cadastrava,
	// fazia login, recebia um token — e TODAS as rotas protegidas devolviam 401,
	// porque o porteiro só aceita a assinatura do Keycloak. Uma porta de entrada
	// que não levava a lugar nenhum.
	//
	// Agora ficam desligadas por padrão e respondem 410 com a explicação, em vez
	// de sumirem sem aviso. Para reativar durante uma migração, ligue as DUAS:
	// ENABLE_LEGACY_AUTH=true e ALLOW_LEGACY_JWT=true (uma sem a outra recria
	// exatamente o beco sem saída descrito acima).
	if legacyAuthEnabled() {
		log.Println("⚠️  ENABLE_LEGACY_AUTH ativo: o cadastro/login antigo está aberto.")
		if !isEnvTrue("ALLOW_LEGACY_JWT") {
			log.Println("🚨 ...mas ALLOW_LEGACY_JWT está DESLIGADO: o token gerado pelo login antigo")
			log.Println("🚨    será recusado em todas as rotas protegidas. Ligue as duas ou nenhuma.")
		}
		router.POST("/v1/register", auth.Register)
		router.POST("/v1/verify-email", auth.VerifyEmailCode)
		router.POST("/v1/login", auth.Login)
		router.POST("/v1/forgot-password", auth.ForgotPassword)
		router.POST("/v1/reset-password", auth.ResetPassword)
		router.POST("/v1/auth/google", auth.GoogleAuth)
	} else {
		for _, rota := range []string{
			"/v1/register", "/v1/verify-email", "/v1/login",
			"/v1/forgot-password", "/v1/reset-password", "/v1/auth/google",
		} {
			router.POST(rota, contaPeloKeycloak)
		}
	}

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
		// Trocar senha também é do Keycloak: mexer no hash local não muda o
		// login de ninguém. Só fica de pé junto com o cadastro antigo.
		if legacyAuthEnabled() {
			protected.PUT("/me/password", users.UpdatePassword)
		} else {
			protected.PUT("/me/password", contaPeloKeycloak)
		}
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
