package focus

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PomodoroInput struct {
	Duration int `json:"duration_minutes" binding:"required"`
}

// RegisterPomodoro salva o tempo de estudo e recompensa o usuário com XP
func RegisterPomodoro(c *gin.Context) {
	userIDStr, _ := c.Get("userID")
	parsedUserID, _ := uuid.Parse(userIDStr.(string))

	var input PomodoroInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Duração do pomodoro é obrigatória (ex: 25)"})
		return
	}

	// 1. Salva a sessão no banco
	session := models.PomodoroSession{
		UserID:   parsedUserID,
		Duration: input.Duration,
	}

	if err := database.DB.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar sessão de foco"})
		return
	}

	// 2. Calcula e injeta o XP (2 XP por minuto de foco)
	xpGained := input.Duration * 2
	database.DB.Model(&models.User{}).Where("id = ?", parsedUserID).Update("xp", gorm.Expr("xp + ?", xpGained))

	// 3. Busca o usuário para devolver o total atualizado
	var updatedUser models.User
	database.DB.Select("xp").First(&updatedUser, "id = ?", parsedUserID)

	c.JSON(http.StatusCreated, gin.H{
		"message":   "Sessão de foco concluída! Bom trabalho.",
		"xp_gained": xpGained,
		"total_xp":  updatedUser.XP,
	})
}

type MoodInput struct {
	Mood string `json:"mood" binding:"required"`
}

// RegisterMood salva como o estudante está se sentindo
func RegisterMood(c *gin.Context) {
	userIDStr, _ := c.Get("userID")
	parsedUserID, _ := uuid.Parse(userIDStr.(string))

	var input MoodInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O humor é obrigatório"})
		return
	}

	checkIn := models.MoodCheckIn{
		UserID: parsedUserID,
		Mood:   input.Mood,
	}

	if err := database.DB.Create(&checkIn).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao registrar humor"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Check-in de humor registrado com sucesso!",
		"mood":    checkIn.Mood,
	})
}
