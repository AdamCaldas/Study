package space

import (
	"net/http"
	"time"

	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DTO para enviar os dados mastigados para o Front-end
type HistoryResponse struct {
	ID            string    `json:"id"`
	UserName      string    `json:"user_name"`
	Action        string    `json:"action"`
	CreatedAtISO  time.Time `json:"created_at_iso"`
	DateFormatted string    `json:"date_formatted"`
	TimeFormatted string    `json:"time_formatted"`
}

// GetSpaceHistory - Retorna o histórico de atividades de um Space
func GetSpaceHistory(c *gin.Context) {
	spaceID := c.Param("space_id")

	var logs []struct {
		ID        uuid.UUID
		FullName  string
		Action    string
		CreatedAt time.Time
	}

	// Busca os logs fazendo JOIN com users para pegar o nome de quem fez a ação
	err := database.DB.Table("activity_logs").
		Select("activity_logs.id, users.full_name, activity_logs.action, activity_logs.created_at").
		Joins("left join users on users.id = activity_logs.user_id").
		Where("activity_logs.space_id = ?", spaceID).
		Order("activity_logs.created_at desc").
		Scan(&logs).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar histórico"})
		return
	}

	var response []HistoryResponse
	for _, log := range logs {
		response = append(response, HistoryResponse{
			ID:            log.ID.String(),
			UserName:      log.FullName,
			Action:        log.Action,
			CreatedAtISO:  log.CreatedAt,
			DateFormatted: log.CreatedAt.Format("02/01/2006"), // DD/MM/YYYY
			TimeFormatted: log.CreatedAt.Format("15:04"),      // HH:mm
		})
	}

	if response == nil {
		response = []HistoryResponse{}
	}

	c.JSON(http.StatusOK, response)
}
