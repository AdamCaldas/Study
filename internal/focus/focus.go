package focus

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils" // 👈 Importamos o novo pacote

	"github.com/gin-gonic/gin"
)

type PomodoroInput struct {
	Duration int `json:"duration_minutes" binding:"required"`
}

type MoodInput struct {
	Mood string `json:"mood" binding:"required"`
}

func RegisterPomodoro(c *gin.Context) {
	// 👇 Olha como ficou limpo! Em 1 linha a gente pega o ID com segurança.
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "ID de usuário inválido ou não autenticado"})
		return
	}

	var input PomodoroInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Duração inválida", "detalhe": err.Error()})
		return
	}

	session := models.PomodoroSession{
		UserID:   userID,
		Duration: input.Duration,
	}

	if err := database.DB.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar a sessão do Pomodoro", "detalhe": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Pomodoro salvo com sucesso!", "session": session})
}

func RegisterMood(c *gin.Context) {
	// 👇 Reutilizando a função! Fim do código WET (Write Everything Twice).
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "ID de usuário inválido ou não autenticado"})
		return
	}

	var input MoodInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Humor inválido", "detalhe": err.Error()})
		return
	}

	mood := models.MoodCheckIn{
		UserID: userID,
		Mood:   input.Mood,
	}

	if err := database.DB.Create(&mood).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar humor", "detalhe": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Humor salvo com sucesso!", "mood": mood})
}
