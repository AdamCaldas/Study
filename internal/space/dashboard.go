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

	// 🌟 3. BUSCA OS COLABORADORES (Agora com TODAS as permissões granulares!)
	var collaborators []struct {
		UserID            string `json:"user_id"`
		FullName          string `json:"full_name"`
		Email             string `json:"email"`
		ProfilePic        string `json:"profile_picture_url"`
		AccessLevel       string `json:"access_level"`
		CanEditSpaceInfo  bool   `json:"can_edit_space_info"`
		CanEditSpaceColor bool   `json:"can_edit_space_color"`
		CanCreateContent  bool   `json:"can_create_content"`
		CanEditContent    bool   `json:"can_edit_content"`
		CanDeleteContent  bool   `json:"can_delete_content"`
		CanManageTags     bool   `json:"can_manage_tags"`
		CanManageMembers  bool   `json:"can_manage_members"`
		CanSendInvites    bool   `json:"can_send_invites"`
		CanSearchContent  bool   `json:"can_search_content"`
		CanChangeSettings bool   `json:"can_change_settings"`
		CanManagePlans    bool   `json:"can_manage_plans"`
		CanManageCycles   bool   `json:"can_manage_cycles"`
		CanManageQuizzes  bool   `json:"can_manage_quizzes"`
	}

	database.DB.Table("space_permissions").
		Select(`users.id as user_id, users.full_name, users.email, users.profile_pic, 
            space_permissions.access_level, space_permissions.can_edit_space_info, 
            space_permissions.can_edit_space_color, space_permissions.can_create_content, 
            space_permissions.can_edit_content, space_permissions.can_delete_content, 
            space_permissions.can_manage_tags, space_permissions.can_manage_members, 
            space_permissions.can_send_invites, space_permissions.can_search_content, 
            space_permissions.can_change_settings, space_permissions.can_manage_plans, 
            space_permissions.can_manage_cycles, space_permissions.can_manage_quizzes`).
		Joins("join users on users.id = space_permissions.user_id").
		Where("space_permissions.space_id = ?", spaceID).
		Scan(&collaborators)

	if collaborators == nil {
		// Mantém o array vazio para o Front não quebrar
		collaborators = []struct {
			UserID            string `json:"user_id"`
			FullName          string `json:"full_name"`
			Email             string `json:"email"`
			ProfilePic        string `json:"profile_picture_url"`
			AccessLevel       string `json:"access_level"`
			CanEditSpaceInfo  bool   `json:"can_edit_space_info"`
			CanEditSpaceColor bool   `json:"can_edit_space_color"`
			CanCreateContent  bool   `json:"can_create_content"`
			CanEditContent    bool   `json:"can_edit_content"`
			CanDeleteContent  bool   `json:"can_delete_content"`
			CanManageTags     bool   `json:"can_manage_tags"`
			CanManageMembers  bool   `json:"can_manage_members"`
			CanSendInvites    bool   `json:"can_send_invites"`
			CanSearchContent  bool   `json:"can_search_content"`
			CanChangeSettings bool   `json:"can_change_settings"`
			CanManagePlans    bool   `json:"can_manage_plans"`
			CanManageCycles   bool   `json:"can_manage_cycles"`
			CanManageQuizzes  bool   `json:"can_manage_quizzes"`
		}{}
	}

	// 📚 4. Cadernos COM AS PÁGINAS (Agora já vêm com as Assinaturas Digitais preenchidas!)
	var notebooks []models.Notebook
	database.DB.Preload("Pages").Where("space_id = ?", spaceID).Find(&notebooks)

	// ---------------------------------------------------------
	// 🧙‍♂️ MÁGICA: Preencher o owner_name das páginas
	// ---------------------------------------------------------
	for i := range notebooks {
		for j := range notebooks[i].Pages {
			var authorName string

			// Vai na tabela de usuários e pesca só o Nome do cara pelo ID
			database.DB.Table("users").
				Select("full_name").
				Where("id = ?", notebooks[i].Pages[j].CreatedByID).
				Scan(&authorName)

			// Injeta o nome no campo virtual que criamos no models.go
			notebooks[i].Pages[j].OwnerName = authorName
		}
	}
	// ---------------------------------------------------------

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
