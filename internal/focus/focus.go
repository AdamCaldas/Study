package focus

import (
	"fmt"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PomodoroInput struct {
	Duration int `json:"duration_minutes" binding:"required"`
}

type MoodInput struct {
	Mood string `json:"mood" binding:"required"`
}

func RegisterPomodoro(c *gin.Context) {
	userIDContext, _ := c.Get("userID")

	// TRUQUE ANTI-PANIC: Transforma em string de forma segura antes de converter
	userIDStr := fmt.Sprintf("%v", userIDContext)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "ID de usuário inválido"})
		return
	}

	var input PomodoroInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Duração inválida", "detalhe": err.Error()})
		return
	}

	session := models.PomodoroSession{
		UserID:   userID,
		Duration: input.Duration,
	}

	if err := database.DB.Create(&session).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao salvar a sessão do Pomodoro", "detalhe": err.Error()})
		return
	}

	c.JSON(201, gin.H{"message": "Pomodoro salvo com sucesso!", "session": session})
}

func RegisterMood(c *gin.Context) {
	userIDContext, _ := c.Get("userID")

	// TRUQUE ANTI-PANIC AQUI TAMBÉM
	userIDStr := fmt.Sprintf("%v", userIDContext)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "ID de usuário inválido"})
		return
	}

	var input MoodInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Humor inválido", "detalhe": err.Error()})
		return
	}

	mood := models.MoodCheckIn{
		UserID: userID,
		Mood:   input.Mood,
	}

	if err := database.DB.Create(&mood).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao salvar humor", "detalhe": err.Error()})
		return
	}

	c.JSON(201, gin.H{"message": "Humor salvo com sucesso!", "mood": mood})
}
