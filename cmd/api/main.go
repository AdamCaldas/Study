package main

import (
	"log"
	"os"

	"studfy-backend/internal/auth"
	"studfy-backend/internal/focus"
	"studfy-backend/internal/gamification"
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
			userID, _ := c.Get("userID")

			c.JSON(200, gin.H{
				"message": "Acesso Autorizado! Você está logado.",
				"seu_id":  userID,
			})
		})

		// Rota de Gamificação
		protected.POST("/gamification/reward", gamification.RewardXP)

		// Rotas de Foco e Produtividade
		protected.POST("/focus/pomodoro", focus.RegisterPomodoro)
		protected.POST("/focus/mood", focus.RegisterMood)

		// Rotas Gerais de Spaces (Criar e Listar os meus Spaces)
		protected.POST("/spaces", auth.CheckSpaceLimit(), space.CreateSpace)
		protected.GET("/spaces", space.ListSpaces)

		// --- NOVO GRUPO: CONTROLE DE ACESSO PARA AMIGOS ---
		spaceRoutes := protected.Group("/spaces/:space_id")
		spaceRoutes.Use(auth.CheckSpaceAccess())
		{
			spaceRoutes.POST("/share", space.ShareSpace)

			// Notas Rápidas
			spaceRoutes.POST("/notes", space.CreateQuickNote)
			spaceRoutes.GET("/notes", space.ListQuickNotes)

			// Revisões
			spaceRoutes.POST("/reviews", study.CreateReview)

			// Planos de Estudo
			spaceRoutes.POST("/plans", study.CreateStudyPlan)
			spaceRoutes.GET("/plans", study.ListStudyPlans)

			// Ciclos de Estudo
			spaceRoutes.POST("/cycles", study.CreateStudyCycle)
			spaceRoutes.GET("/cycles", study.ListStudyCycles)
			spaceRoutes.PATCH("/cycles/:cycle_id/advance", study.AdvanceCycleStep)

			// Cadernos
			spaceRoutes.POST("/notebooks", notebook.CreateNotebook)
			spaceRoutes.GET("/notebooks", notebook.ListNotebooks)
		}

		// --- ROTAS DE PÁGINAS ---
		protected.POST("/notebooks/:notebook_id/pages", notebook.CreatePage)
		protected.GET("/notebooks/:notebook_id/pages", notebook.ListPages)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Iniciando servidor na porta %s...", port)
	router.Run(":" + port)
}
