package gamification

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// O que o Frontend vai enviar
type RewardXPInput struct {
	Action string `json:"action" binding:"required"` // Ex: "pomodoro", "page_created", "cycle_completed"
}

// RewardXP recebe a ação e injeta os pontos no perfil do usuário
func RewardXP(c *gin.Context) {
	userID, _ := c.Get("userID")

	var input RewardXPInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ação inválida"})
		return
	}

	// 1. Tabela de Pontuação (Você pode balancear esses valores depois)
	pointsToAward := 0
	switch input.Action {
	case "pomodoro":
		pointsToAward = 50 // Focar dá muito XP
	case "cycle_completed":
		pointsToAward = 100 // Fechar o ciclo é o "chefão"
	case "page_created":
		pointsToAward = 10 // Criar material dá um pouco de XP
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ação de gamificação desconhecida"})
		return
	}

	// 2. Atualiza o banco de dados magicamente (Incremento Atômico)
	// Isso evita bugs se o usuário mandar duas requisições ao mesmo tempo
	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("xp", gorm.Expr("xp + ?", pointsToAward)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar XP"})
		return
	}

	// 3. Busca o novo total para devolver pro Front (para ele fazer aquela animação da barra enchendo)
	var updatedUser models.User
	database.DB.Select("xp").First(&updatedUser, "id = ?", userID)

	c.JSON(http.StatusOK, gin.H{
		"message":  "XP ganho com sucesso!",
		"gained":   pointsToAward,
		"total_xp": updatedUser.XP,
	})
}
