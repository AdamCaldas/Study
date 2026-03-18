package space

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

	// 📚 4. Cadernos COM AS PÁGINAS E GUIAS ANINHADAS
	var notebooks []models.Notebook
	database.DB.
		Preload("Pages").                  // Puxa as páginas soltas
		Preload("Guides.Pages").           // Puxa as Guias principais e suas páginas
		Preload("Guides.SubGuides.Pages"). // Puxa as Sub-Guias e suas páginas
		Where("space_id = ?", spaceID).
		Find(&notebooks)

	// ---------------------------------------------------------
	// ---------------------------------------------------------
	// 🧙‍♂️ MÁGICA DE PERFORMANCE: Mapeando Criadores e Editores
	// ---------------------------------------------------------

	// 1. Busca todos os usuários do banco de uma vez só e cria um dicionário (Map)
	var allUsers []models.User
	database.DB.Select("id, full_name").Find(&allUsers)

	userMap := make(map[uuid.UUID]string)
	for _, u := range allUsers {
		userMap[u.ID] = u.FullName
	}

	// 2. Distribui os nomes na velocidade da luz sem consultar o banco de novo!
	for i := range notebooks {
		// Rastreador do CADERNO
		notebooks[i].OwnerName = userMap[notebooks[i].CreatedByID]
		notebooks[i].UpdaterName = userMap[notebooks[i].UpdatedByID]

		// Rastreador das PÁGINAS SOLTAS
		for j := range notebooks[i].Pages {
			notebooks[i].Pages[j].OwnerName = userMap[notebooks[i].Pages[j].CreatedByID]
			notebooks[i].Pages[j].UpdaterName = userMap[notebooks[i].Pages[j].UpdatedByID]
		}

		// Rastreador das GUIAS (Pastas)
		for k := range notebooks[i].Guides {
			notebooks[i].Guides[k].OwnerName = userMap[notebooks[i].Guides[k].CreatedByID]
			notebooks[i].Guides[k].UpdaterName = userMap[notebooks[i].Guides[k].UpdatedByID]

			for l := range notebooks[i].Guides[k].Pages {
				notebooks[i].Guides[k].Pages[l].OwnerName = userMap[notebooks[i].Guides[k].Pages[l].CreatedByID]
				notebooks[i].Guides[k].Pages[l].UpdaterName = userMap[notebooks[i].Guides[k].Pages[l].UpdatedByID]
			}

			// Rastreador das SUB-GUIAS (Pastas dentro de pastas)
			for m := range notebooks[i].Guides[k].SubGuides {
				notebooks[i].Guides[k].SubGuides[m].OwnerName = userMap[notebooks[i].Guides[k].SubGuides[m].CreatedByID]
				notebooks[i].Guides[k].SubGuides[m].UpdaterName = userMap[notebooks[i].Guides[k].SubGuides[m].UpdatedByID]

				for n := range notebooks[i].Guides[k].SubGuides[m].Pages {
					notebooks[i].Guides[k].SubGuides[m].Pages[n].OwnerName = userMap[notebooks[i].Guides[k].SubGuides[m].Pages[n].CreatedByID]
					notebooks[i].Guides[k].SubGuides[m].Pages[n].UpdaterName = userMap[notebooks[i].Guides[k].SubGuides[m].Pages[n].UpdatedByID]
				}
			}
		}
	}
	// ---------------------------------------------------------
	// 🔐 4.5 NOVA TABELA: PERMISSÕES GRANULARES DOS CADERNOS
	var notebookPermissions []models.NotebookPermission
	database.DB.Joins("JOIN notebooks ON notebooks.id = notebook_permissions.notebook_id").
		Where("notebooks.space_id = ?", spaceID).
		Find(&notebookPermissions)

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
		"notebook_permissions": notebookPermissions,
		"all_cycles":           cycles,
		"active_cycle":         activeCycle,
		"study_plans":          studyPlans,
		"quick_notes":          quickNotes,
		"quizzes":              quizzes,
	})
}
