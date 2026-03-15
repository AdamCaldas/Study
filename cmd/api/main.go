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
	"studfy-backend/internal/users" // 👈 PACOTE NOVO IMPORTADO AQUI!
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
		// 👤 1. USUÁRIO E PERFIL (Agora limpo e sem conflitos!)
		// ------------------------------------------------------
		protected.GET("/me", users.GetMyProfile)    // 👈 Busca Perfil + Spaces Separados
		protected.PUT("/me", users.UpdateMyProfile) // 👈 Atualiza Nome e Idade

		// ------------------------------------------------------
		// 🎮 2. GAMIFICAÇÃO E FOCO (Pomodoro / Mood)
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
		// 📊 4. DASHBOARD (Raio-X Completo do Space)
		// ------------------------------------------------------
		dashboardRoutes := protected.Group("/dashboard/:space_id")
		dashboardRoutes.Use(auth.CheckSpaceAccess())
		{
			dashboardRoutes.GET("", space.GetSpaceDashboard)
		}

		// ======================================================
		// 🛠️ 5. GESTÃO INTERNA DO SPACE (Apenas quem tem acesso)
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
			spaceRoutes.GET("/requests", space.ListSpaceRequests)                        // Dono vê quem pediu
			spaceRoutes.POST("/requests/:request_id/respond", space.RespondSpaceRequest) // Dono aceita/rejeita

			// 👉 Cadernos (Notebooks)
			spaceRoutes.POST("/notebooks", notebook.CreateNotebook)
			spaceRoutes.GET("/notebooks", notebook.ListNotebooks)
			spaceRoutes.PUT("/notebooks/:notebook_id", notebook.UpdateNotebook)
			spaceRoutes.DELETE("/notebooks/:notebook_id", notebook.DeleteNotebook)

			// 👉 Notas Rápidas (Post-its)
			spaceRoutes.POST("/notes", space.CreateQuickNote)
			spaceRoutes.GET("/notes", space.ListQuickNotes)
			spaceRoutes.PUT("/notes/:note_id", space.UpdateQuickNote)
			spaceRoutes.DELETE("/notes/:note_id", space.DeleteQuickNote)

			// 👉 Plano de Estudos (Agenda Semanal)
			spaceRoutes.POST("/plans", study.CreateStudyPlan)
			spaceRoutes.GET("/plans", study.ListPlans)
			spaceRoutes.PUT("/plans/:plan_id", study.UpdateStudyPlan)
			spaceRoutes.DELETE("/plans/:plan_id", study.DeleteStudyPlan)

			// 👉 Ciclos de Estudo (A Roleta)
			spaceRoutes.POST("/cycles", study.CreateStudyCycle)
			spaceRoutes.GET("/cycles", study.ListCycles)
			spaceRoutes.PATCH("/cycles/:cycle_id/advance", study.AdvanceCycleStep) // Próxima matéria
			spaceRoutes.PATCH("/cycles/:cycle_id/activate", study.ActivateCycle)   // Favoritar Ciclo
			spaceRoutes.DELETE("/cycles/:cycle_id", study.DeleteStudyCycle)

			// 👉 Revisões e Quizzes
			spaceRoutes.POST("/reviews", study.CreateReview)
			spaceRoutes.POST("/quizzes", study.CreateQuiz)
			spaceRoutes.GET("/quizzes", study.ListQuizzes)
		}

		// ------------------------------------------------------
		// 📄 6. PÁGINAS (CRUD e Drag & Drop)
		// ------------------------------------------------------
		protected.POST("/notebooks/:notebook_id/pages", notebook.CreatePage)
		protected.GET("/notebooks/:notebook_id/pages", notebook.ListPages)
		protected.PATCH("/notebooks/:notebook_id/pages/reorder", notebook.ReorderPages) // Nova!

		protected.PUT("/pages/:page_id", notebook.UpdatePage)
		protected.DELETE("/pages/:page_id", notebook.DeletePage)
	}

	// ==========================================================
	// ⚡ MODO DEUS (DASHBOARD ADMIN / DEV) ⚡
	// ==========================================================
	godMode := router.Group("/v1/admin")
	// Usa DOIS seguranças: Tem que estar logado (Auth) E tem que ser DEV (AdminOnly)
	godMode.Use(auth.AuthMiddleware(), auth.AdminOnly())
	{
		// Relatório Geral
		godMode.GET("/report", admin.GetPlatformReport)

		// 👉 PILAR 1: Gestão de Usuários
		godMode.GET("/users", admin.ListAllUsers)
		godMode.PUT("/users/:id", admin.UpdateAnyUser)
		godMode.PUT("/users/:id/password", admin.ForceChangePassword)

		// 👉 PILAR 2: Controle de Conteúdo
		godMode.GET("/spaces", admin.ListAllSpaces)
		godMode.PUT("/spaces/:id/transfer", admin.TransferSpaceOwnership)
		godMode.DELETE("/spaces/:id/collaborators/:user_id", admin.RemoveUserFromSpace)
		godMode.DELETE("/spaces/:id", admin.DeleteAnySpace)

		// 👉 PILAR 3: Relatórios e Métricas
		godMode.GET("/reports/plans", admin.GetUsersByPlan)  // Gráfico de Conversão
		godMode.GET("/reports/ranking", admin.GetTopUsersXP) // Tabela de Engajamento
		godMode.GET("/reports/moods", admin.GetMoodStats)
	}

	// ----------------------------------------------------------
	// 🚀 INICIALIZAÇÃO DO SERVIDOR
	// ----------------------------------------------------------
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Iniciando servidor na porta %s...", port)
	router.Run(":" + port)
}
