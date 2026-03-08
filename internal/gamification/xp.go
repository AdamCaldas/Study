package gamification

import (
	"fmt"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RewardInput struct {
	Action string `json:"action" binding:"required"`
	Amount int    `json:"amount"`
}

func RewardXP(c *gin.Context) {
	userIDContext, _ := c.Get("userID")
	userIDStr := fmt.Sprintf("%v", userIDContext)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "ID de usuário inválido"})
		return
	}

	var input RewardInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Ação inválida"})
		return
	}

	xpToAward := input.Amount
	// Se o front-end não mandar o valor (mandar 0), usamos um valor padrão
	if xpToAward <= 0 {
		switch input.Action {
		case "completed_pomodoro":
			xpToAward = 25
		case "created_note":
			xpToAward = 10
		default:
			xpToAward = 5
		}
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("xp", gorm.Expr("xp + ?", xpToAward)).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao atualizar XP", "detalhe": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "XP ganho!", "xp_earned": xpToAward})
}
