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

		// Rota de Gamificação (Agora no lugar certo!)
		protected.POST("/gamification/reward", gamification.RewardXP)

		// Rotas de Foco e Produtividade (NOVAS!)
		protected.POST("/focus/pomodoro", focus.RegisterPomodoro)
		protected.POST("/focus/mood", focus.RegisterMood)

		// Rotas Gerais de Spaces (Criar e Listar os meus Spaces)
		protected.POST("/spaces", auth.CheckSpaceLimit(), space.CreateSpace)
		protected.GET("/spaces", space.ListSpaces)

		// --- NOVO GRUPO: CONTROLE DE ACESSO PARA AMIGOS ---
		// Injetamos o CheckSpaceAccess. Tudo aqui dentro está protegido pela validação de dono/convidado!
		spaceRoutes := protected.Group("/spaces/:space_id")
		spaceRoutes.Use(auth.CheckSpaceAccess())
		{

			spaceRoutes.POST("/share", space.ShareSpace)

			// Notas Rápidas
			spaceRoutes.POST("/notes", space.CreateQuickNote)
			spaceRoutes.GET("/notes", space.ListQuickNotes)

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
		// Ficam fora do spaceRoutes porque a URL base delas é /notebooks/...
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
