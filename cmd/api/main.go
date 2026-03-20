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
		log.Println("Aviso: Arquivo .env não encontrado.")
	}

	database.ConnectDB()

	router := gin.Default()

	// ==========================================================
	// 🗜️ 1. GZIP GLOBAL (Comprime respostas em até 90%)
	// ==========================================================
	router.Use(gzip.Gzip(gzip.DefaultCompression))

	// ==========================================================
	// 🌐 CONFIGURAÇÃO DE CORS E HEADERS
	// ==========================================================
	router.Use(cors.New(cors.Config{
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
	// 🔓 ROTAS PÚBLICAS (Sem Autenticação)
	// ==========================================================
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong! Servidor StudFy conectado ao banco 🚀"})
	})

	router.POST("/v1/register", auth.Register)
	router.POST("/v1/login", auth.Login)

	// ==========================================================
	// 🛡️ ROTAS PROTEGIDAS DO APP (Exigem Token JWT)
	// ==========================================================
	protected := router.Group("/v1/app")
	protected.Use(auth.AuthMiddleware())
	{
		// ------------------------------------------------------
		// 👤 1. USUÁRIO E PERFIL
		// ------------------------------------------------------
		protected.GET("/me", users.GetMyProfile)
		protected.PUT("/me", users.UpdateMyProfile)
		protected.PUT("/me/password", users.UpdatePassword)
		protected.DELETE("/me", users.DeleteMyAccount)
		protected.POST("/me/become-teacher", users.BecomeTeacher)

		// 🏦 FASE 3: BANCO DE QUESTÕES
		protected.GET("/me/question-bank", study.GetMyQuestionBank)
		protected.POST("/me/question-bank", study.SaveToQuestionBank)

		// ------------------------------------------------------
		// 🔔 1.5 NOTIFICAÇÕES DO ALUNO
		// ------------------------------------------------------
		protected.GET("/notifications", admin.GetMyNotifications)
		protected.POST("/notifications/:id/read", admin.MarkNotificationAsRead)

		// ------------------------------------------------------
		// 🎓 1.8 VITRINE E PROFESSORES
		// ------------------------------------------------------
		protected.POST("/teachers/:id/follow", users.FollowTeacher)
		protected.DELETE("/teachers/:id/follow", users.UnfollowTeacher)
		protected.GET("/teachers/:id", users.GetTeacherProfile)

		// ------------------------------------------------------
		// 🎮 2. GAMIFICAÇÃO E FOCO
		// ------------------------------------------------------
		protected.POST("/gamification/reward", gamification.RewardXP)
		protected.POST("/focus/pomodoro", focus.RegisterPomodoro)
		protected.POST("/focus/mood", focus.RegisterMood)

		// ------------------------------------------------------
		// 🪐 3. SPACES - ROTAS GERAIS E CONVITES
		// ------------------------------------------------------
		protected.POST("/spaces", auth.CheckSpaceLimit(), space.CreateSpace)
		protected.GET("/spaces", space.ListSpaces)

		protected.GET("/spaces/code/:code", space.GetSpaceByCode) // Pre-view do convite
		protected.POST("/spaces/join", space.RequestSpaceAccess)  // Solicitar acesso (Sala de Espera)

		// 📷 NOVO: ALUNO BATE O PONTO LENDO O QR CODE
		protected.POST("/attendance/check-in", space.RegisterAttendance)

		// ------------------------------------------------------
		// 📊 4. SUPER DASHBOARD (O "Todas as Informações")
		// ------------------------------------------------------
		dashboardRoutes := protected.Group("/dashboard/:space_id")
		dashboardRoutes.Use(auth.CheckSpaceAccess())
		{
			// Esta rota agora entrega TUDO! Não precisamos mais das rotas "GET" separadas.
			dashboardRoutes.GET("", space.GetSpaceDashboard)
		}

		// ======================================================
		// 🛠️ 5. GESTÃO INTERNA DO SPACE E AÇÕES (CRUDs)
		// ======================================================
		spaceRoutes := protected.Group("/spaces/:space_id")
		spaceRoutes.Use(auth.CheckSpaceAccess())
		{
			// 👉 Configurações do Space Próprio
			spaceRoutes.PUT("", space.UpdateSpace)
			spaceRoutes.DELETE("", space.DeleteSpace)
			spaceRoutes.POST("/share", space.ShareSpace)
			spaceRoutes.GET("/history", space.GetSpaceHistory) // Timeline de Atividades

			// 👉 Gestão de Convites (Sala de Espera)
			spaceRoutes.GET("/requests", space.ListSpaceRequests)
			spaceRoutes.POST("/requests/:request_id/respond", space.RespondSpaceRequest)

			// 👉 Gestão de Membros Ativos
			spaceRoutes.PUT("/collaborators/:user_id", space.UpdateCollaborator)
			spaceRoutes.DELETE("/collaborators/:user_id", space.RemoveCollaborator)

			// ======================================================
			// 🕵️ DOSSIÊ DO ALUNO (Notas Privadas do Professor - FASE 2)
			// ======================================================
			spaceRoutes.GET("/dossier/:student_id", space.GetOrUpdateStudentDossier)
			spaceRoutes.PUT("/dossier/:student_id", space.GetOrUpdateStudentDossier)

			// 👉 Cadernos
			spaceRoutes.POST("/notebooks", notebook.CreateNotebook)
			spaceRoutes.PUT("/notebooks/:notebook_id", notebook.UpdateNotebook)
			spaceRoutes.DELETE("/notebooks/:notebook_id", notebook.DeleteNotebook)

			// 👉 Notas Rápidas
			spaceRoutes.POST("/notes", space.CreateQuickNote)
			spaceRoutes.PUT("/notes/:note_id", space.UpdateQuickNote)
			spaceRoutes.DELETE("/notes/:note_id", space.DeleteQuickNote)

			// 👉 Plano de Estudos
			spaceRoutes.POST("/plans", study.CreateStudyPlan)
			spaceRoutes.PUT("/plans/:plan_id", study.UpdateStudyPlan)
			spaceRoutes.DELETE("/plans/:plan_id", study.DeleteStudyPlan)

			// 👉 Ciclos de Estudo
			spaceRoutes.POST("/cycles", study.CreateStudyCycle)
			spaceRoutes.PATCH("/cycles/:cycle_id/advance", study.AdvanceCycleStep)
			spaceRoutes.PATCH("/cycles/:cycle_id/activate", study.ActivateCycle)
			spaceRoutes.DELETE("/cycles/:cycle_id", study.DeleteStudyCycle)
			spaceRoutes.POST("/cycles/simulate", study.SimulateStudyCycle)
			spaceRoutes.GET("/cycles", study.ListStudyCycles)
			spaceRoutes.PUT("/cycles/:cycle_id", study.UpdateStudyCycle)

			// 👉 Revisões e Quizzes
			spaceRoutes.POST("/reviews", study.CreateReview)
			spaceRoutes.POST("/quizzes", study.CreateQuiz)

			// ======================================================
			// ⚔️ FASE 4: AVALIAÇÃO E ANTI-COLA
			// ======================================================
			spaceRoutes.POST("/quizzes/:quiz_id/submit", study.SubmitQuiz)
			spaceRoutes.POST("/quizzes/:quiz_id/cheat-alert", study.ReportCheatAttempt)
			spaceRoutes.PUT("/quizzes/results/:result_id/grade", study.GradeQuizManual)
			spaceRoutes.POST("/certificate", study.ClaimCertificate)

			// ======================================================
			// 💬 FASE 5: FÓRUM E PLANTÃO DE DÚVIDAS
			// ======================================================
			spaceRoutes.POST("/pages/:page_id/doubts", space.CreatePageDoubt)
			spaceRoutes.GET("/doubts", space.ListSpaceDoubts)
			spaceRoutes.PUT("/doubts/:doubt_id/answer", space.AnswerPageDoubt)

			// 📢 NOVO: O PASSO 2 - MEGAFONE
			spaceRoutes.POST("/megafone", space.SendMegaphoneMessage)

			// 📷 NOVO: O PASSO 3 - GERAR QR CODE DE PRESENÇA
			spaceRoutes.POST("/attendance", space.GenerateAttendanceQR)

			// ======================================================
			// ⚡ FASE 3: MISSÕES RELÂMPAGO
			// ======================================================
			spaceRoutes.POST("/missions", gamification.CreateFlashMission)
			spaceRoutes.GET("/missions", gamification.GetActiveMissions)
			spaceRoutes.POST("/missions/:mission_id/complete", gamification.CompleteFlashMission)

			// ======================================================
			// 🏆 FASE 6: CULTURA DA TURMA E EMBLEMAS
			// ======================================================
			spaceRoutes.POST("/badges", gamification.CreateBadge)
			spaceRoutes.POST("/badges/:badge_id/award/:student_id", gamification.AwardBadge)

			// 🏆 NOVO: PASSO 2 - RANKING CONTROLADO
			spaceRoutes.PATCH("/ranking/toggle", gamification.ToggleSpaceRanking)
			spaceRoutes.GET("/ranking", gamification.GetSpaceRanking)

			// 🎮 NOVO: PASSO 3 - MODO ARENA (KAHOOT)
			spaceRoutes.GET("/arena/live", gamification.JoinArenaMode)

			// 🧠 FASE 7: PAINEL DO DIRETOR (ANALYTICS B2B)
			// ======================================================
			spaceRoutes.GET("/analytics/thermometer", space.GetClassThermometer)

			// 🎓 NOVO: O GRAND FINALE - DOWNLOAD DO DIÁRIO
			spaceRoutes.GET("/analytics/export-diary", space.ExportClassDiaryCSV)

			// 🤖 NOVO: PASSO 2 - CRIAR AUTOMAÇÃO
			spaceRoutes.POST("/automation/rules", space.CreateAutomationRule)
		}

		// ------------------------------------------------------
		// 📁 6. GUIAS / PASTAS DO CADERNO (As novas pastas!)
		// ------------------------------------------------------
		protected.POST("/notebooks/:notebook_id/guides", notebook.CreateGuide)
		protected.PUT("/guides/:guide_id", notebook.UpdateGuide)
		protected.DELETE("/guides/:guide_id", notebook.DeleteGuide)

		// ------------------------------------------------------
		// 📄 7. PÁGINAS (Apenas Ações)
		// ------------------------------------------------------
		protected.POST("/notebooks/:notebook_id/pages", notebook.CreatePage)
		protected.PATCH("/notebooks/:notebook_id/pages/reorder", notebook.ReorderPages)
		protected.PUT("/pages/:page_id", notebook.UpdatePage)
		protected.DELETE("/pages/:page_id", notebook.DeletePage)
	}

	// ==========================================================
	// ⚡ MODO DEUS (DASHBOARD ADMIN / DEV) ⚡
	// ==========================================================
	godMode := router.Group("/v1/admin")
	godMode.Use(auth.AuthMiddleware(), auth.AdminOnly())
	{
		godMode.GET("/report", admin.GetPlatformReport)

		godMode.GET("/users", admin.ListAllUsers)
		godMode.PUT("/users/:id", admin.UpdateAnyUser)
		godMode.PUT("/users/:id/password", admin.ForceChangePassword)
		godMode.DELETE("/users/:id", admin.DeleteAnyUser)

		godMode.GET("/spaces", admin.ListAllSpaces)
		godMode.PUT("/spaces/:id/transfer", admin.TransferSpaceOwnership)
		godMode.DELETE("/spaces/:id/collaborators/:user_id", admin.RemoveUserFromSpace)
		godMode.DELETE("/spaces/:id", admin.DeleteAnySpace)

		godMode.GET("/reports/plans", admin.GetUsersByPlan)
		godMode.GET("/reports/ranking", admin.GetTopUsersXP)
		godMode.GET("/reports/moods", admin.GetMoodStats)

		// 🎯 GESTÃO DE GAMIFICAÇÃO
		godMode.PUT("/users/:id/xp", admin.UpdateUserXP)
		godMode.GET("/gamification/rules", admin.ListGamificationRules)
		godMode.POST("/gamification/rules", admin.CreateGamificationRule)
		godMode.PUT("/gamification/rules/:rule_id", admin.UpdateGamificationRule)

		// 🎯 GESTÃO DE NOTIFICAÇÕES
		godMode.POST("/notifications", admin.CreateNotification)
		godMode.PUT("/notifications/:id", admin.UpdateNotification)
		godMode.DELETE("/notifications/:id", admin.DeleteNotification)
		godMode.GET("/notifications", admin.ListAllNotifications)
	}

	// ==========================================================
	// 🚀 LIGANDO O MOTOR (Com Proteção Nativa de Timeout)
	// ==========================================================
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second, // Corta se a requisição for muito lenta
		WriteTimeout: 15 * time.Second, // Corta se o servidor travar para responder
		IdleTimeout:  60 * time.Second, // Economiza memória RAM
	}

	log.Printf("Iniciando servidor na porta %s...", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Erro crítico no servidor: %v", err)
	}
}
