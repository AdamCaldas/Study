package space

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CreateQuickNoteInput struct {
	Title   string `json:"title"`
	Content string `json:"content" binding:"required"`
	Color   string `json:"color"`
}

func CreateQuickNote(c *gin.Context) {
	userID, _ := c.Get("userID")
	spaceID := c.Param("space_id")

	// 1. Verifica se o usuário tem acesso a esse Space
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Space não encontrado ou acesso negado"})
		return
	}

	// 2. Valida a entrada
	var input CreateQuickNoteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O conteúdo da nota é obrigatório"})
		return
	}

	if input.Color == "" {
		input.Color = "#FFF9C4" // Cor de post-it amarelo clarinho padrão
	}

	parsedSpaceID, _ := uuid.Parse(spaceID)

	// 3. Monta e salva a nota
	newNote := models.QuickNote{
		SpaceID: parsedSpaceID,
		Title:   input.Title,
		Content: input.Content,
		Color:   input.Color,
	}

	if err := database.DB.Create(&newNote).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar Nota Rápida"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Nota rápida criada com sucesso!",
		"note":    newNote,
	})
}

func ListQuickNotes(c *gin.Context) {
	userID, _ := c.Get("userID")
	spaceID := c.Param("space_id")

	// Segurança: O usuário só pode ver notas de Spaces dele
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Acesso negado a este Space"})
		return
	}

	var notes []models.QuickNote
	if err := database.DB.Where("space_id = ?", spaceID).Find(&notes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar Notas"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"notes": notes})
}
