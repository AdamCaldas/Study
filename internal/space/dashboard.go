package space

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
)

// GetSpaceDashboard - Retorna o Raio-X completo do Space para o Front-end
func GetSpaceDashboard(c *gin.Context) {
	spaceID := c.Param("space_id")

	// 1. Busca os dados principais do Space
	var space models.Space
	if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado"})
		return
	}

	// 2. Conta os Colaboradores (Membros)
	var totalCollaborators int64
	database.DB.Model(&models.SpacePermission{}).Where("space_id = ?", spaceID).Count(&totalCollaborators)

	// 3. Busca os Cadernos (Apenas os campos necessários para ficar leve)
	var notebooks []models.Notebook
	database.DB.Select("id, name, color_hex").Where("space_id = ?", spaceID).Find(&notebooks)

	// Conta quantos cadernos tem
	totalNotebooks := int64(len(notebooks))

	// 4. Busca o Ciclo ATIVO (com os itens dele)
	var activeCycle models.StudyCycle
	var totalCycles int64

	// Conta todos os ciclos desse space primeiro
	database.DB.Model(&models.StudyCycle{}).Where("space_id = ?", spaceID).Count(&totalCycles)

	// Busca apenas o que está marcado como IsActive = true e traz os items (Preload)
	err := database.DB.Preload("Items").Where("space_id = ? AND is_active = ?", spaceID, true).First(&activeCycle).Error

	// Prepara a resposta do ciclo. Se não achar nenhum ativo, manda nil (nulo) pro Front-end saber que não tem.
	var cycleResponse interface{} = nil
	if err == nil {
		cycleResponse = activeCycle
	}

	// 5. Busca os Planos de Estudo (Agenda)
	var studyPlans []models.StudyPlan
	database.DB.Where("space_id = ?", spaceID).Find(&studyPlans)

	// 6. Monta o JSON Gigante e devolve com Status 200 OK!
	c.JSON(http.StatusOK, gin.H{
		"space": space,
		"stats": gin.H{
			"total_collaborators": totalCollaborators,
			"total_notebooks":     totalNotebooks,
			"total_cycles":        totalCycles,
		},
		"notebooks":    notebooks,
		"active_cycle": cycleResponse,
		"study_plans":  studyPlans,
	})
}
