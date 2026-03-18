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
	Title     string   `json:"title" binding:"required"`
	Message   string   `json:"message" binding:"required"`
	Type      string   `json:"type" binding:"required"`     // POPUP, BELL, NEWS
	Audience  string   `json:"audience" binding:"required"` // GLOBAL, SPACES, USERS
	TargetIDs []string `json:"target_ids"`                  // IDs específicos (se houver)

	// 👇 NOVOS CAMPOS DE DATA AQUI
	StartAt *time.Time `json:"start_at"`
	EndAt   *time.Time `json:"end_at"`

	IsActive *bool `json:"is_active"`
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
		EndAt:     req.EndAt,
	}

	// ⏳ Se mandar o StartAt, usa ele. Se não mandar, usa a data de agora!
	if req.StartAt != nil {
		notif.StartAt = *req.StartAt
	} else {
		notif.StartAt = time.Now()
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
	notif.EndAt = req.EndAt

	if req.StartAt != nil {
		notif.StartAt = *req.StartAt
	}

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

	// 🛡️ Trava de segurança
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

	// ⏳ A QUERY MÁGICA ATUALIZADA COM START_AT E END_AT
	database.DB.Raw(`
		SELECT n.* FROM notifications n
		LEFT JOIN notification_reads nr ON n.id = nr.notification_id AND nr.user_id = ?
		WHERE n.is_active = true 
		AND n.start_at <= NOW() -- 👈 Só aparece se a data inicial já chegou
		AND (n.end_at IS NULL OR n.end_at > NOW()) -- 👈 Não tem validade, ou ainda não venceu
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

// ==========================================================
// 🛡️ MODO DEUS: LISTAR TODAS AS NOTIFICAÇÕES
// ==========================================================
func ListAllNotifications(c *gin.Context) {
	var notifications []models.Notification

	// Puxa tudo, das mais novas pras mais velhas
	if err := database.DB.Order("created_at desc").Find(&notifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar notificações"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"notifications": notifications})
}
