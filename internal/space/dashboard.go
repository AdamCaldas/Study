package space

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
)

// GetSpaceDashboard - Retorna o Raio-X ABSOLUTO do Space (TUDO) para o Front-end
func GetSpaceDashboard(c *gin.Context) {
	spaceID := c.Param("space_id")

	// 1. Dados principais do Space
	var space models.Space
	if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado"})
		return
	}

	// 🌟 2. BUSCA O DONO DO SPACE
	var owner models.User
	database.DB.Select("id, full_name, email, profile_pic").Where("id = ?", space.OwnerID).First(&owner)

	// 🌟 3. BUSCA OS COLABORADORES (Permissão Geral do Space)
	var collaborators []struct {
		UserID      string `json:"user_id"`
		FullName    string `json:"full_name"`
		Email       string `json:"email"`
		ProfilePic  string `json:"profile_picture_url"`
		AccessLevel string `json:"access_level"`
	}

	database.DB.Table("space_permissions").
		Select("users.id as user_id, users.full_name, users.email, users.profile_pic, space_permissions.access_level").
		Joins("join users on users.id = space_permissions.user_id").
		Where("space_permissions.space_id = ?", spaceID).
		Scan(&collaborators)

	if collaborators == nil {
		collaborators = []struct {
			UserID      string `json:"user_id"`
			FullName    string `json:"full_name"`
			Email       string `json:"email"`
			ProfilePic  string `json:"profile_picture_url"`
			AccessLevel string `json:"access_level"`
		}{}
	}

	// 📚 4. Cadernos COM AS PÁGINAS (Agora já vêm com as Assinaturas Digitais preenchidas!)
	var notebooks []models.Notebook
	database.DB.Preload("Pages").Where("space_id = ?", spaceID).Find(&notebooks)

	// 🔐 4.5 NOVA TABELA: PERMISSÕES GRANULARES DOS CADERNOS
	var notebookPermissions []models.NotebookPermission
	// Faz um JOIN para buscar apenas as permissões dos cadernos que estão DENTRO deste Space
	database.DB.Joins("JOIN notebooks ON notebooks.id = notebook_permissions.notebook_id").
		Where("notebooks.space_id = ?", spaceID).
		Find(&notebookPermissions)

	// Evita mandar null pro Front
	if notebookPermissions == nil {
		notebookPermissions = []models.NotebookPermission{}
	}

	// 5. Todos os Ciclos (Ativos e Inativos) COM AS MATÉRIAS
	var cycles []models.StudyCycle
	database.DB.Preload("Items").Where("space_id = ?", spaceID).Find(&cycles)

	var activeCycle interface{} = nil
	for _, cycle := range cycles {
		if cycle.IsActive {
			activeCycle = cycle
			break
		}
	}

	// 6. Planos de Estudo (Agenda Completa)
	var studyPlans []models.StudyPlan
	database.DB.Where("space_id = ?", spaceID).Find(&studyPlans)

	// 7. Notas Rápidas / Post-its
	var quickNotes []models.QuickNote
	database.DB.Where("space_id = ?", spaceID).Find(&quickNotes)

	// 8. Quizzes / Simulados COM AS PERGUNTAS
	var quizzes []models.Quiz
	database.DB.Preload("Questions").Where("space_id = ?", spaceID).Find(&quizzes)

	// 🚀 9. O JSON GIGANTE COM TUDO
	c.JSON(http.StatusOK, gin.H{
		"space":                space,
		"owner":                owner,
		"collaborators":        collaborators,
		"notebooks":            notebooks,
		"notebook_permissions": notebookPermissions, // 👈 Nova trava enviada ao Front-end!
		"all_cycles":           cycles,
		"active_cycle":         activeCycle,
		"study_plans":          studyPlans,
		"quick_notes":          quickNotes,
		"quizzes":              quizzes,
	})
}
