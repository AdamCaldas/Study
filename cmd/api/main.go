package main

import (
	"log"
	"os"

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
		protected.DELETE("/me", users.DeleteMyAccount)

		// ------------------------------------------------------
		// 🔔 1.5 NOTIFICAÇÕES DO ALUNO
		// ------------------------------------------------------
		protected.GET("/notifications", admin.GetMyNotifications)
		protected.POST("/notifications/:id/read", admin.MarkNotificationAsRead)

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

			// 👉 Cadernos (Apenas Ações. O 'GET' foi apagado!)
			spaceRoutes.POST("/notebooks", notebook.CreateNotebook)
			spaceRoutes.PUT("/notebooks/:notebook_id", notebook.UpdateNotebook)
			spaceRoutes.DELETE("/notebooks/:notebook_id", notebook.DeleteNotebook)

			// 👉 Notas Rápidas (Apenas Ações)
			spaceRoutes.POST("/notes", space.CreateQuickNote)
			spaceRoutes.PUT("/notes/:note_id", space.UpdateQuickNote)
			spaceRoutes.DELETE("/notes/:note_id", space.DeleteQuickNote)

			// 👉 Plano de Estudos (Apenas Ações)
			spaceRoutes.POST("/plans", study.CreateStudyPlan)
			spaceRoutes.PUT("/plans/:plan_id", study.UpdateStudyPlan)
			spaceRoutes.DELETE("/plans/:plan_id", study.DeleteStudyPlan)

			// 👉 Ciclos de Estudo (Apenas Ações)
			spaceRoutes.POST("/cycles", study.CreateStudyCycle)
			spaceRoutes.PATCH("/cycles/:cycle_id/advance", study.AdvanceCycleStep)
			spaceRoutes.PATCH("/cycles/:cycle_id/activate", study.ActivateCycle)
			spaceRoutes.DELETE("/cycles/:cycle_id", study.DeleteStudyCycle)
			spaceRoutes.POST("/cycles/simulate", study.SimulateStudyCycle)
			spaceRoutes.GET("/cycles", study.ListStudyCycles)
			spaceRoutes.PUT("/cycles/:cycle_id", study.UpdateStudyCycle)
			// 👉 Revisões e Quizzes (Apenas Ações)
			spaceRoutes.POST("/reviews", study.CreateReview)
			spaceRoutes.POST("/quizzes", study.CreateQuiz)
		}

		// ------------------------------------------------------
		// 📄 6. PÁGINAS (Apenas Ações)
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

		// 🎯 GESTÃO DE GAMIFICAÇÃO (NOVO)
		godMode.PUT("/users/:id/xp", admin.UpdateUserXP)
		godMode.GET("/gamification/rules", admin.ListGamificationRules)
		godMode.POST("/gamification/rules", admin.CreateGamificationRule)
		godMode.PUT("/gamification/rules/:rule_id", admin.UpdateGamificationRule)

		// 🎯 GESTÃO DE NOTIFICAÇÕES (ADMIN)
		godMode.POST("/notifications", admin.CreateNotification)
		godMode.PUT("/notifications/:id", admin.UpdateNotification)
		godMode.DELETE("/notifications/:id", admin.DeleteNotification)

	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Iniciando servidor na porta %s...", port)
	router.Run(":" + port)
}
