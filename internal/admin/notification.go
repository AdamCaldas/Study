package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Estrutura do JSON que você vai mandar para criar/editar
type NotificationInput struct {
	Title     string     `json:"title" binding:"required"`
	Message   string     `json:"message" binding:"required"`
	Type      string     `json:"type" binding:"required"`     // POPUP, BELL, NEWS
	Audience  string     `json:"audience" binding:"required"` // GLOBAL, SPACES, USERS
	TargetIDs []string   `json:"target_ids"`                  // IDs específicos (se houver)
	ExpiresAt *time.Time `json:"expires_at"`
	IsActive  *bool      `json:"is_active"`
}

// ==========================================================
// 🛡️ MODO DEUS: CRIAR NOTIFICAÇÃO
// ==========================================================
func CreateNotification(c *gin.Context) {
	var req NotificationInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	targetJSON, _ := json.Marshal(req.TargetIDs)

	notif := models.Notification{
		Title:     req.Title,
		Message:   req.Message,
		Type:      req.Type,
		Audience:  req.Audience,
		TargetIDs: string(targetJSON),
		ExpiresAt: req.ExpiresAt,
	}

	if err := database.DB.Create(&notif).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao disparar notificação"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Aviso disparado com sucesso!", "notification": notif})
}

// ==========================================================
// 🛡️ MODO DEUS: EDITAR NOTIFICAÇÃO (O Conserto Rápido)
// ==========================================================
func UpdateNotification(c *gin.Context) {
	id := c.Param("id")
	var req NotificationInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	var notif models.Notification
	if err := database.DB.Where("id = ?", id).First(&notif).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notificação não encontrada"})
		return
	}

	targetJSON, _ := json.Marshal(req.TargetIDs)

	notif.Title = req.Title
	notif.Message = req.Message
	notif.Type = req.Type
	notif.Audience = req.Audience
	notif.TargetIDs = string(targetJSON)
	notif.ExpiresAt = req.ExpiresAt

	if req.IsActive != nil {
		notif.IsActive = *req.IsActive // Permite pausar/ocultar a notificação
	}

	database.DB.Save(&notif)
	c.JSON(http.StatusOK, gin.H{"message": "Notificação atualizada!", "notification": notif})
}

// ==========================================================
// 🛡️ MODO DEUS: DELETAR NOTIFICAÇÃO (Botão de Pânico)
// ==========================================================
func DeleteNotification(c *gin.Context) {
	id := c.Param("id")

	// Apaga a notificação e todos os registros de leitura dela em cascata
	database.DB.Where("notification_id = ?", id).Delete(&models.NotificationRead{})
	database.DB.Where("id = ?", id).Delete(&models.Notification{})

	c.JSON(http.StatusOK, gin.H{"message": "Notificação apagada do sistema!"})
}

// ==========================================================
// 📱 APP: BUSCAR MEUS AVISOS (A Mágica do Front-end)
// ==========================================================
func GetMyNotifications(c *gin.Context) {
	userIDInterface, _ := c.Get("userID")
	var userID string

	// 🛡️ Trava de segurança: Descobre se o ID veio como UUID ou String e converte certo
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v.String()
	case string:
		userID = v
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro de autenticação interno"})
		return
	}

	var notifications []models.Notification

	// A Query super otimizada
	database.DB.Raw(`
		SELECT n.* FROM notifications n
		LEFT JOIN notification_reads nr ON n.id = nr.notification_id AND nr.user_id = ?
		WHERE n.is_active = true 
		AND (n.expires_at IS NULL OR n.expires_at > NOW())
		AND nr.id IS NULL -- Só traz as que ele AINDA NÃO LEU!
		AND (
			n.audience = 'GLOBAL' 
			OR (n.audience = 'USERS' AND n.target_ids::jsonb @> ?)
		)
		ORDER BY n.created_at DESC
	`, userID, `"`+userID+`"`).Scan(&notifications)

	if notifications == nil {
		notifications = []models.Notification{} // Para não devolver null pro Front-end
	}

	c.JSON(http.StatusOK, gin.H{"notifications": notifications})
}

// ==========================================================
// 📱 APP: MARCAR COMO LIDO (Sino/Popup sumindo)
// ==========================================================
func MarkNotificationAsRead(c *gin.Context) {
	notifIDStr := c.Param("id")
	notifID, _ := uuid.Parse(notifIDStr)

	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	read := models.NotificationRead{
		NotificationID: notifID,
		UserID:         userID,
	}

	// Salva ignorando erros de duplicação (caso o cara clique 2x rápido)
	database.DB.FirstOrCreate(&read, read)

	c.JSON(http.StatusOK, gin.H{"message": "Marcado como lido!"})
}
