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

type UpdateQuickNoteInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Color   string `json:"color"`
}

func UpdateQuickNote(c *gin.Context) {
	noteID := c.Param("note_id")
	var input UpdateQuickNoteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Dados inválidos"})
		return
	}

	if err := database.DB.Model(&models.QuickNote{}).Where("id = ?", noteID).Updates(models.QuickNote{
		Title:   input.Title,
		Content: input.Content,
		Color:   input.Color,
	}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao atualizar nota", "detalhe": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Nota atualizada!"})
}

func DeleteQuickNote(c *gin.Context) {
	noteID := c.Param("note_id")
	if err := database.DB.Where("id = ?", noteID).Delete(&models.QuickNote{}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao apagar nota", "detalhe": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Nota apagada!"})
}
