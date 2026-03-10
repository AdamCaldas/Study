package main

import (
	"log"
	"os"

	"studfy-backend/internal/auth"
	"studfy-backend/internal/focus"
	"studfy-backend/internal/gamification"
	"studfy-backend/internal/models"
	"studfy-backend/internal/notebook"
	"studfy-backend/internal/space"
	"studfy-backend/internal/study"
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

	router.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Content-Length",
			"Accept-Encoding",
			"X-CSRF-Token",
			"Authorization",
			"Accept",
			"Cache-Control",
			"X-Requested-With",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong! Servidor StudFy conectado ao banco 🚀"})
	})

	router.POST("/v1/register", auth.Register)
	router.POST("/v1/login", auth.Login)

	protected := router.Group("/v1/app")
	protected.Use(auth.AuthMiddleware())
	{
		protected.GET("/me", func(c *gin.Context) {
			userID, exists := c.Get("userID")
			if !exists {
				c.JSON(401, gin.H{"error": "Usuário não identificado"})
				return
			}

			// Busca os dados completos no banco de dados
			var user models.User
			if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
				c.JSON(404, gin.H{"error": "Usuário não encontrado no banco"})
				return
			}

			// Devolve o objeto usuário completo para o Front-end
			c.JSON(200, user)
		})

		// Rota de Gamificação
		protected.POST("/gamification/reward", gamification.RewardXP)

		// Rotas de Foco e Produtividade
		protected.POST("/focus/pomodoro", focus.RegisterPomodoro)
		protected.POST("/focus/mood", focus.RegisterMood)

		// Rotas Gerais de Spaces (Criar e Listar os meus Spaces)
		protected.POST("/spaces", auth.CheckSpaceLimit(), space.CreateSpace)
		protected.GET("/spaces", space.ListSpaces)

		// --- ROTAS DO FRONT-END (Código e Entrar) ---
		protected.GET("/spaces/code/:code", space.GetSpaceByCode)
		protected.POST("/spaces/join", space.JoinSpaceByCode)

		// --- GRUPO: CONTROLE DE ACESSO PARA AMIGOS E EDIÇÃO ---
		spaceRoutes := protected.Group("/spaces/:space_id")
		spaceRoutes.Use(auth.CheckSpaceAccess())
		{
			// Edição do Space Próprio
			spaceRoutes.PUT("", space.UpdateSpace)
			spaceRoutes.DELETE("", space.DeleteSpace)
			spaceRoutes.POST("/share", space.ShareSpace)

			// Cadernos (CRUD Completo)
			spaceRoutes.POST("/notebooks", notebook.CreateNotebook)
			spaceRoutes.GET("/notebooks", notebook.ListNotebooks)
			spaceRoutes.PUT("/notebooks/:notebook_id", notebook.UpdateNotebook)
			spaceRoutes.DELETE("/notebooks/:notebook_id", notebook.DeleteNotebook)

			// Notas Rápidas / Post-its (CRUD Completo)
			spaceRoutes.POST("/notes", space.CreateQuickNote)
			spaceRoutes.GET("/notes", space.ListQuickNotes)
			spaceRoutes.PUT("/notes/:note_id", space.UpdateQuickNote)
			spaceRoutes.DELETE("/notes/:note_id", space.DeleteQuickNote)

			// Revisões
			spaceRoutes.POST("/reviews", study.CreateReview)

			// Planos de Estudo (CRUD Completo)
			spaceRoutes.POST("/plans", study.CreateStudyPlan)
			spaceRoutes.GET("/plans", study.ListPlans)
			spaceRoutes.PUT("/plans/:plan_id", study.UpdateStudyPlan)
			spaceRoutes.DELETE("/plans/:plan_id", study.DeleteStudyPlan)

			// Ciclos de Estudo (CRUD Completo)
			spaceRoutes.POST("/cycles", study.CreateStudyCycle)
			spaceRoutes.GET("/cycles", study.ListCycles)
			spaceRoutes.PATCH("/cycles/:cycle_id/advance", study.AdvanceCycleStep)
			spaceRoutes.DELETE("/cycles/:cycle_id", study.DeleteStudyCycle)
		}

		// --- ROTAS DE PÁGINAS (CRUD Completo) ---
		protected.POST("/notebooks/:notebook_id/pages", notebook.CreatePage)
		protected.GET("/notebooks/:notebook_id/pages", notebook.ListPages)
		protected.PUT("/pages/:page_id", notebook.UpdatePage)
		protected.DELETE("/pages/:page_id", notebook.DeletePage)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Iniciando servidor na porta %s...", port)
	router.Run(":" + port)
}
