package admin

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
)

// GetPlatformReport - Traz um resumo de TUDO que tem no sistema (Modo Deus)
func GetPlatformReport(c *gin.Context) {
	var totalUsers int64
	var totalSpaces int64
	var totalNotebooks int64

	// Conta quantos usuários existem no banco todo
	database.DB.Model(&models.User{}).Count(&totalUsers)

	// Conta quantos spaces existem
	database.DB.Model(&models.Space{}).Count(&totalSpaces)

	// Conta quantos cadernos existem
	database.DB.Model(&models.Notebook{}).Count(&totalNotebooks)

	c.JSON(http.StatusOK, gin.H{
		"message": "Relatório do Modo Deus gerado com sucesso",
		"stats": gin.H{
			"total_users":     totalUsers,
			"total_spaces":    totalSpaces,
			"total_notebooks": totalNotebooks,
		},
	})
}
