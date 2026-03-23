package space

import (
	"net/http"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetSpaceDashboard - Retorna o Raio-X ABSOLUTO do Space (TUDO) para o Front-end
func GetSpaceDashboard(c *gin.Context) {
	spaceID := c.Param("space_id")

	// =========================================================
	// 🌟 0. PEGA O ID DO USUÁRIO LOGADO (Para métricas e travas)
	// =========================================================
	userIDInterface, _ := c.Get("userID")
	var loggedUserID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		loggedUserID = v
	case string:
		loggedUserID, _ = uuid.Parse(v)
	}

	// 1. Dados principais do Space
	var space models.Space
	if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado"})
		return
	}

	// 🌟 2. BUSCA O DONO DO SPACE
	var owner models.User
	database.DB.Select("id, full_name, email, profile_pic").Where("id = ?", space.OwnerID).First(&owner)

	// 🌟 3. BUSCA OS COLABORADORES (Com todas as permissões granulares atualizadas)
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
            space_permissions.can_manage_quizzes`).
		Joins("join users on users.id = space_permissions.user_id").
		Where("space_permissions.space_id = ?", spaceID).
		Scan(&collaborators)

	if collaborators == nil {
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
	// 🧙‍♂️ MÁGICA DE PERFORMANCE: Mapeando Criadores e Editores
	// ---------------------------------------------------------
	var allUsers []models.User
	database.DB.Select("id, full_name").Find(&allUsers)

	userMap := make(map[uuid.UUID]string)
	for _, u := range allUsers {
		userMap[u.ID] = u.FullName
	}

	for i := range notebooks {
		notebooks[i].OwnerName = userMap[notebooks[i].CreatedByID]
		notebooks[i].UpdaterName = userMap[notebooks[i].UpdatedByID]

		for j := range notebooks[i].Pages {
			notebooks[i].Pages[j].OwnerName = userMap[notebooks[i].Pages[j].CreatedByID]
			notebooks[i].Pages[j].UpdaterName = userMap[notebooks[i].Pages[j].UpdatedByID]
		}

		for k := range notebooks[i].Guides {
			notebooks[i].Guides[k].OwnerName = userMap[notebooks[i].Guides[k].CreatedByID]
			notebooks[i].Guides[k].UpdaterName = userMap[notebooks[i].Guides[k].UpdatedByID]

			for l := range notebooks[i].Guides[k].Pages {
				notebooks[i].Guides[k].Pages[l].OwnerName = userMap[notebooks[i].Guides[k].Pages[l].CreatedByID]
				notebooks[i].Guides[k].Pages[l].UpdaterName = userMap[notebooks[i].Guides[k].Pages[l].UpdatedByID]
			}

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

	// 🔐 4.5 PERMISSÕES GRANULARES DOS CADERNOS
	var notebookPermissions []models.NotebookPermission
	database.DB.Joins("JOIN notebooks ON notebooks.id = notebook_permissions.notebook_id").
		Where("notebooks.space_id = ?", spaceID).
		Find(&notebookPermissions)

	if notebookPermissions == nil {
		notebookPermissions = []models.NotebookPermission{}
	}

	// =========================================================
	// 🎯 5 E 6. O NOVO MOTOR DE ESTUDOS UNIFICADO
	// =========================================================
	var strategy models.StudyStrategy
	database.DB.Preload("Blocks").Where("space_id = ?", spaceID).First(&strategy)
	// Se a strategy não for encontrada, o GORM deixa a struct vazia, o que é seguro pro JSON.

	// 7. Notas Rápidas / Post-its
	var quickNotes []models.QuickNote
	database.DB.Where("space_id = ?", spaceID).Find(&quickNotes)

	// 8. Quizzes / Simulados COM AS PERGUNTAS
	var quizzes []models.Quiz
	database.DB.Preload("Questions").Where("space_id = ?", spaceID).Find(&quizzes)

	// =========================================================
	// ⏳ FASE 3: TIME-RELEASE (CADEADO TEMPORAL DA SALA DE AULA)
	// =========================================================
	now := time.Now()

	// 1. Descobre se quem está acessando é o Professor/Monitor ou um Aluno comum
	isTeacherOrMonitor := (space.OwnerID == loggedUserID)
	if !isTeacherOrMonitor {
		var perm models.SpacePermission
		database.DB.Where("space_id = ? AND user_id = ?", space.ID, loggedUserID).First(&perm)
		if perm.AccessLevel == "EDITOR" || perm.AccessLevel == "MONITOR" {
			isTeacherOrMonitor = true
		}
	}

	// 2. Trava os Cadernos do Futuro
	for i := range notebooks {
		if notebooks[i].UnlockAt != nil && notebooks[i].UnlockAt.After(now) {
			notebooks[i].IsLocked = true // Avisa o Front para desenhar um cadeado cinza

			// 🛡️ SEGURANÇA: Se for ALUNO, nós esvaziamos as páginas para ele não roubar pelo JSON!
			// Se for o Professor, a gente manda o conteúdo normal para ele poder revisar.
			if !isTeacherOrMonitor {
				notebooks[i].Pages = []models.Page{}
				notebooks[i].Guides = []models.Guide{}
			}
		}
	}

	// 3. Trava os Simulados/Quizzes do Futuro
	for i := range quizzes {
		if quizzes[i].UnlockAt != nil && quizzes[i].UnlockAt.After(now) {
			quizzes[i].IsLocked = true

			if !isTeacherOrMonitor {
				quizzes[i].Questions = []models.QuizQuestion{}
			}
		}
	}

	// =========================================================
	// 🌟 8.5 INJEÇÃO EXTRA: DADOS GLOBAIS DO USUÁRIO LOGADO
	// =========================================================
	var qtdNotebooks, qtdNotes, qtdStrategies int64
	database.DB.Model(&models.Notebook{}).Where("created_by_id = ?", loggedUserID).Count(&qtdNotebooks)
	database.DB.Model(&models.StudyStrategy{}).Where("created_by_id = ?", loggedUserID).Count(&qtdStrategies)

	database.DB.Table("quick_notes").
		Joins("JOIN spaces ON spaces.id = quick_notes.space_id").
		Where("spaces.owner_id = ?", loggedUserID).
		Count(&qtdNotes)

	var globalStudyStrategies []models.StudyStrategy
	database.DB.Preload("Blocks").
		Joins("JOIN spaces ON spaces.id = study_strategies.space_id").
		Joins("LEFT JOIN space_permissions ON space_permissions.space_id = spaces.id").
		Where("spaces.owner_id = ? OR space_permissions.user_id = ?", loggedUserID, loggedUserID).
		Group("study_strategies.id").
		Find(&globalStudyStrategies)

	if globalStudyStrategies == nil {
		globalStudyStrategies = []models.StudyStrategy{}
	}

	var guestSpaces []struct {
		SpaceID           string `json:"space_id"`
		Name              string `json:"name"`
		ColorHex          string `json:"color_hex"`
		AccessLevel       string `json:"access_level"`
		OwnerName         string `json:"owner_name"`
		ProfilePictureURL string `json:"profile_picture_url"`
		UpdatedAt         string `json:"updated_at"`
	}

	database.DB.Table("spaces").
		Select("spaces.id as space_id, spaces.name, spaces.color_hex, space_permissions.access_level, users.full_name as owner_name, users.profile_pic as profile_picture_url, spaces.updated_at").
		Joins("join space_permissions on space_permissions.space_id = spaces.id").
		Joins("join users on users.id = spaces.owner_id").
		Where("space_permissions.user_id = ?", loggedUserID).
		Scan(&guestSpaces)

	if guestSpaces == nil {
		guestSpaces = []struct {
			SpaceID           string `json:"space_id"`
			Name              string `json:"name"`
			ColorHex          string `json:"color_hex"`
			AccessLevel       string `json:"access_level"`
			OwnerName         string `json:"owner_name"`
			ProfilePictureURL string `json:"profile_picture_url"`
			UpdatedAt         string `json:"updated_at"`
		}{}
	}

	// 🚀 9. O JSON GIGANTE COM TUDO
	c.JSON(http.StatusOK, gin.H{
		"space":                space,
		"owner":                owner,
		"collaborators":        collaborators,
		"notebooks":            notebooks,
		"notebook_permissions": notebookPermissions,

		// 👇 A MÁGICA DA UNIFICAÇÃO (Substitui os velhos all_cycles, active_cycle e space_study_plans)
		"study_strategy": strategy,

		"quick_notes": quickNotes,
		"quizzes":     quizzes,

		// 👇 DADOS EXTRAS GLOBAIS
		"usage_stats": gin.H{
			"qtd_notebooks":  qtdNotebooks,
			"qtd_notes":      qtdNotes,
			"qtd_strategies": qtdStrategies, // Mudou o nome aqui também
		},
		"global_strategies": globalStudyStrategies, // E aqui
		"guest_spaces":      guestSpaces,
	})
}
