package gamification

import (
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RewardInput struct {
	Action string `json:"action" binding:"required"`
}

func RewardXP(c *gin.Context) {
	userIDContext, _ := c.Get("userID")
	userID, err := uuid.Parse(userIDContext.(string))
	if err != nil {
		c.JSON(400, gin.H{"error": "ID de usuário inválido"})
		return
	}

	var input RewardInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Ação não informada"})
		return
	}

	// 1. Define quantos pontos a ação vale
	xpToAward := 0
	switch input.Action {
	case "completed_pomodoro":
		xpToAward = 50 // Ganha 50 XP por Pomodoro
	case "created_note":
		xpToAward = 10 // Ganha 10 XP por anotação
	default:
		xpToAward = 5 // Qualquer outra ação ganha 5 XP
	}

	// 2. Adiciona o XP direto na conta do usuário no banco usando gorm.Expr
	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("xp", gorm.Expr("xp + ?", xpToAward)).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao atualizar XP", "detalhe": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message":   "XP ganho com sucesso!",
		"xp_earned": xpToAward,
	})
}
