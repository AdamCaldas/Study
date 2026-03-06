package study

import (
	"net/http"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CreateReviewInput struct {
	NoteID string `json:"note_id" binding:"required"`
}

// CreateReview agenda uma revisão para uma anotação específica
func CreateReview(c *gin.Context) {
	var input CreateReviewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O ID da anotação (note_id) é obrigatório"})
		return
	}

	parsedNoteID, err := uuid.Parse(input.NoteID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID da anotação inválido"})
		return
	}

	// Cria a revisão agendada para o dia seguinte (Lógica simples de Repetição Espaçada)
	newReview := models.Review{
		NoteID:     parsedNoteID,
		ReviewDate: time.Now().AddDate(0, 0, 1), // Adiciona 1 dia
		Status:     "pendente",
	}

	if err := database.DB.Create(&newReview).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao agendar revisão"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Revisão agendada com sucesso para amanhã!",
		"review":  newReview,
	})
}
