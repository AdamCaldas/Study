package admin

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
)

// Estrutura genérica para contagens agrupadas (usada em gráficos de pizza/barras)
type GroupCountResult struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// GetUsersByPlan - Relatório de Conversão (Quantos usuários pagantes vs gratuitos)
func GetUsersByPlan(c *gin.Context) {
	var results []GroupCountResult

	// Faz um GROUP BY no banco para contar os usuários por SubscriptionType
	if err := database.DB.Model(&models.User{}).
		Select("subscription_type as label, count(*) as count").
		Group("subscription_type").
		Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao gerar relatório de planos", "detalhe": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": results})
}

// GetTopUsersXP - Relatório de Engajamento (O Ranking dos 10 alunos mais dedicados)
func GetTopUsersXP(c *gin.Context) {
	var users []models.User

	// Pega os 10 usuários com maior XP no sistema, escondendo a senha
	if err := database.DB.Order("xp desc").Limit(10).Omit("Password").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao gerar ranking de XP", "detalhe": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ranking": users})
}

// GetMoodStats - Termômetro de Saúde Mental (Métrica fortíssima para instituições)
func GetMoodStats(c *gin.Context) {
	var results []GroupCountResult

	// Conta quantas vezes cada humor (Animado, Cansado, Focado) foi registrado
	if err := database.DB.Model(&models.MoodCheckIn{}).
		Select("mood as label, count(*) as count").
		Group("mood").
		Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao gerar estatísticas de humor", "detalhe": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": results})
}
