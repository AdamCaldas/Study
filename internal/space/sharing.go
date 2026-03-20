package space

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==========================================================
// 1️⃣ Amigo solicita acesso pelo código
// ==========================================================
func RequestSpaceAccess(c *gin.Context) {
	userIDInterface, _ := c.Get("userID")

	var parsedUserID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		parsedUserID = v
	case string:
		parsedUserID, _ = uuid.Parse(v)
	}

	var input struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Código não informado"})
		return
	}

	var space models.Space
	if err := database.DB.Where("share_code = ?", input.Code).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Código de Space inválido"})
		return
	}

	if space.OwnerID == parsedUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Você já é o dono deste Space!"})
		return
	}

	var existingPermission models.SpacePermission
	if err := database.DB.Where("space_id = ? AND user_id = ?", space.ID, parsedUserID).First(&existingPermission).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Você já faz parte deste Space!"})
		return
	}

	var existingRequest models.SpaceJoinRequest
	if err := database.DB.Where("space_id = ? AND user_id = ? AND status = 'pending'", space.ID, parsedUserID).First(&existingRequest).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Você já enviou uma solicitação. Aguarde aprovação!"})
		return
	}

	newRequest := models.SpaceJoinRequest{
		SpaceID: space.ID,
		UserID:  parsedUserID,
		Status:  "pending",
	}
	if err := database.DB.Create(&newRequest).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao enviar solicitação"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Solicitação de acesso enviada com sucesso para o dono do Space!"})
}

// ==========================================================
// 2️⃣ Dono lista as solicitações pendentes (Sala de Espera)
// ==========================================================
func ListSpaceRequests(c *gin.Context) {
	spaceID := c.Param("space_id")

	var requests []struct {
		RequestID string `json:"request_id"`
		UserID    string `json:"user_id"`
		FullName  string `json:"full_name"`
		Status    string `json:"status"`
	}

	err := database.DB.Table("space_join_requests").
		Select("space_join_requests.id as request_id, users.id as user_id, users.full_name, space_join_requests.status").
		Joins("left join users on users.id = space_join_requests.user_id").
		Where("space_join_requests.space_id = ? AND space_join_requests.status = 'pending'", spaceID).
		Scan(&requests).Error

	if err != nil || len(requests) == 0 {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	c.JSON(http.StatusOK, requests)
}

// ==========================================================
// 3️⃣ Dono aceita ou rejeita a solicitação
// ==========================================================
func RespondSpaceRequest(c *gin.Context) {
	spaceID := c.Param("space_id")
	requestID := c.Param("request_id")

	var input struct {
		Action      string `json:"action" binding:"required"`
		AccessLevel string `json:"access_level"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ação inválida ou formato incorreto"})
		return
	}

	var joinRequest models.SpaceJoinRequest
	if err := database.DB.Where("id = ? AND space_id = ?", requestID, spaceID).First(&joinRequest).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Solicitação não encontrada"})
		return
	}

	tx := database.DB.Begin()

	// 🟢 SE O DONO ACEITAR
	if input.Action == "accept" {
		if err := tx.Model(&joinRequest).Update("status", "approved").Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao aprovar solicitação"})
			return
		}

		if input.AccessLevel != "EDITOR" && input.AccessLevel != "MONITOR" {
			input.AccessLevel = "VIEWER"
		}

		permission := models.SpacePermission{
			SpaceID:     joinRequest.SpaceID,
			UserID:      joinRequest.UserID,
			AccessLevel: input.AccessLevel,
		}

		if err := tx.Create(&permission).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao adicionar usuário ao Space"})
			return
		}

		tx.Commit()
		c.JSON(http.StatusOK, gin.H{"message": "Solicitação aceita! Usuário agora é " + input.AccessLevel})
		return
	}

	// 🔴 SE O DONO REJEITAR
	if input.Action == "reject" {
		if err := tx.Model(&joinRequest).Update("status", "rejected").Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao rejeitar solicitação"})
			return
		}
		tx.Commit()
		c.JSON(http.StatusOK, gin.H{"message": "Solicitação rejeitada com sucesso."})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "A 'action' deve ser obrigatoriamente 'accept' ou 'reject'"})
}

// ==========================================================
// 4️⃣ Gerar/Pegar o código de compartilhamento (🔒 COM TRAVA DE CLASSROOM)
// ==========================================================
func ShareSpace(c *gin.Context) {
	spaceID := c.Param("space_id")
	userIDInterface, _ := c.Get("userID")

	var loggedUserID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		loggedUserID = v
	case string:
		loggedUserID, _ = uuid.Parse(v)
	}

	// Busca o Space para ver se é Sala de Aula ou Normal
	var space models.Space
	if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado"})
		return
	}

	// 🔒 FASE 2: MONOPÓLIO DE CONVITES (SÓ PARA CLASSROOMS)
	if space.IsClassroom {
		// Se quem tá pedindo não for o Dono, a gente tem que checar se ele é MONITOR
		if space.OwnerID != loggedUserID {
			var perm models.SpacePermission
			database.DB.Where("space_id = ? AND user_id = ?", space.ID, loggedUserID).First(&perm)

			// Se for um Classroom e o cara não for Monitor, toma Block!
			if perm.AccessLevel != "MONITOR" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Esta é uma Sala de Aula. Apenas o Professor e os Monitores podem gerar links de convite."})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Código de compartilhamento recuperado!",
		"share_code": space.ShareCode,
	})
}

// ==========================================================
// 5️⃣ Atualiza Nível de Acesso + Checkboxes Granulares
// ==========================================================
func UpdateCollaborator(c *gin.Context) {
	spaceID := c.Param("space_id")
	userIDToUpdate := c.Param("user_id")

	var input struct {
		AccessLevel       string `json:"access_level" binding:"required"` // "VIEWER", "EDITOR", "MONITOR" ou "CUSTOM"
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

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados de permissão inválidos"})
		return
	}

	result := database.DB.Model(&models.SpacePermission{}).
		Where("space_id = ? AND user_id = ?", spaceID, userIDToUpdate).
		Updates(map[string]interface{}{
			"access_level":         input.AccessLevel,
			"can_edit_space_info":  input.CanEditSpaceInfo,
			"can_edit_space_color": input.CanEditSpaceColor,
			"can_create_content":   input.CanCreateContent,
			"can_edit_content":     input.CanEditContent,
			"can_delete_content":   input.CanDeleteContent,
			"can_manage_tags":      input.CanManageTags,
			"can_manage_members":   input.CanManageMembers,
			"can_send_invites":     input.CanSendInvites,
			"can_search_content":   input.CanSearchContent,
			"can_change_settings":  input.CanChangeSettings,
			"can_manage_plans":     input.CanManagePlans,
			"can_manage_cycles":    input.CanManageCycles,
			"can_manage_quizzes":   input.CanManageQuizzes,
		})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar permissão"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Colaborador não encontrado neste Space"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permissões atualizadas com sucesso para " + input.AccessLevel})
}

// ==========================================================
// 6️⃣ Expulsa o colaborador do Space
// ==========================================================
func RemoveCollaborator(c *gin.Context) {
	spaceID := c.Param("space_id")
	userIDToRemove := c.Param("user_id")

	result := database.DB.Where("space_id = ? AND user_id = ?", spaceID, userIDToRemove).Delete(&models.SpacePermission{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao expulsar usuário"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuário já não faz parte deste Space"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Usuário removido do Space com sucesso."})
}
