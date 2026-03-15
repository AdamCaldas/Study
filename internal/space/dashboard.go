package space

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
)

// GetSpaceDashboard - Retorna o Raio-X ABSOLUTO do Space (TUDO) para testes no Front-end
func GetSpaceDashboard(c *gin.Context) {
	spaceID := c.Param("space_id")

	// 1. Dados principais do Space
	var space models.Space
	if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado"})
		return
	}

	// 2. Colaboradores (Permissões completas de quem tá no Space)
	var permissions []models.SpacePermission
	database.DB.Where("space_id = ?", spaceID).Find(&permissions)

	// 3. Cadernos COM AS PÁGINAS (Preload carrega todo o texto JSONB das Pages)
	var notebooks []models.Notebook
	database.DB.Preload("Pages").Where("space_id = ?", spaceID).Find(&notebooks)

	// 4. Todos os Ciclos (Ativos e Inativos) COM AS MATÉRIAS (Items)
	var cycles []models.StudyCycle
	database.DB.Preload("Items").Where("space_id = ?", spaceID).Find(&cycles)

	// Separa o ciclo ativo para facilitar a vida do Front-end
	var activeCycle interface{} = nil
	for _, cycle := range cycles {
		if cycle.IsActive {
			activeCycle = cycle
			break
		}
	}

	// 5. Planos de Estudo (Agenda Completa)
	var studyPlans []models.StudyPlan
	database.DB.Where("space_id = ?", spaceID).Find(&studyPlans)

	// 6. Notas Rápidas / Post-its
	var quickNotes []models.QuickNote
	database.DB.Where("space_id = ?", spaceID).Find(&quickNotes)

	// 7. Quizzes / Simulados COM AS PERGUNTAS (Questions)
	var quizzes []models.Quiz
	database.DB.Preload("Questions").Where("space_id = ?", spaceID).Find(&quizzes)

	// 8. O JSON GIGANTE COM TUDO
	c.JSON(http.StatusOK, gin.H{
		"space":        space,
		"permissions":  permissions,
		"notebooks":    notebooks, // 👈 Agora vai com todas as páginas e o JSONB dentro!
		"all_cycles":   cycles,    // 👈 Vai com ativos e inativos!
		"active_cycle": activeCycle,
		"study_plans":  studyPlans,
		"quick_notes":  quickNotes, // 👈 Vai com todos os post-its!
		"quizzes":      quizzes,    // 👈 Vai com todas as perguntas das provas!
	})
}
