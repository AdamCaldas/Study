package notebook

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==========================================================
// FUNÇÃO AUXILIAR: Extrai o ID do usuário com segurança
// ==========================================================
func getUserID(c *gin.Context) uuid.UUID {
	userIDInterface, _ := c.Get("userID")
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		return v
	case string:
		parsed, _ := uuid.Parse(v)
		return parsed
	default:
		return uuid.Nil
	}
}

// ==========================================================
// 🛡️ O LEÃO DE CHÁCARA GRANULAR
// Verifica se o usuário pode mexer neste caderno específico
// ==========================================================
func canEditNotebook(spaceID uuid.UUID, notebookID uuid.UUID, userID uuid.UUID) bool {
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error; err == nil {
		return true // É o dono do Space
	}

	var nbPerm models.NotebookPermission
	if err := database.DB.Where("notebook_id = ? AND user_id = ?", notebookID, userID).First(&nbPerm).Error; err == nil {
		return nbPerm.AccessLevel == "EDITOR" // Trava Específica
	}

	var spPerm models.SpacePermission
	if err := database.DB.Where("space_id = ? AND user_id = ?", spaceID, userID).First(&spPerm).Error; err == nil {
		return spPerm.AccessLevel == "EDITOR" // Trava Geral
	}

	return false
}

// ==========================================================
// 1️⃣ CREATE NOTEBOOK
// ==========================================================
type CreateNotebookInput struct {
	Name     string `json:"name" binding:"required"`
	ColorHex string `json:"color_hex"`
}

func CreateNotebook(c *gin.Context) {
	parsedUserID := getUserID(c)
	spaceIDStr := c.Param("space_id")
	parsedSpaceID, _ := uuid.Parse(spaceIDStr)

	var space models.Space
	isOwner := database.DB.Where("id = ? AND owner_id = ?", parsedSpaceID, parsedUserID).First(&space).Error == nil
	var permission models.SpacePermission
	isSpaceEditor := database.DB.Where("space_id = ? AND user_id = ? AND access_level = 'EDITOR'", parsedSpaceID, parsedUserID).First(&permission).Error == nil

	if !isOwner && !isSpaceEditor {
		c.JSON(http.StatusForbidden, gin.H{"error": "Acesso Negado."})
		return
	}

	var input CreateNotebookInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	if input.ColorHex == "" {
		input.ColorHex = "#E0E0E0"
	}

	newNotebook := models.Notebook{
		SpaceID:     parsedSpaceID,
		Name:        input.Name,
		ColorHex:    input.ColorHex,
		CreatedByID: parsedUserID, // ASSINATURA
		UpdatedByID: parsedUserID, // ASSINATURA
	}

	if err := database.DB.Create(&newNotebook).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar Caderno"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Caderno criado!", "notebook": newNotebook})
}

// ==========================================================
// 2️⃣ UPDATE NOTEBOOK
// ==========================================================
type UpdateNotebookInput struct {
	Name     string `json:"name"`
	ColorHex string `json:"color_hex"`
}

func UpdateNotebook(c *gin.Context) {
	parsedUserID := getUserID(c)
	notebookIDStr := c.Param("notebook_id")
	parsedNotebookID, _ := uuid.Parse(notebookIDStr)

	var notebook models.Notebook
	if err := database.DB.Where("id = ?", parsedNotebookID).First(&notebook).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Caderno não encontrado"})
		return
	}

	if !canEditNotebook(notebook.SpaceID, parsedNotebookID, parsedUserID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Você não tem permissão para editar este caderno."})
		return
	}

	var input UpdateNotebookInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	if err := database.DB.Model(&notebook).Updates(map[string]interface{}{
		"name":          input.Name,
		"color_hex":     input.ColorHex,
		"updated_by_id": parsedUserID, // ASSINATURA
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Caderno atualizado!"})
}

func DeleteNotebook(c *gin.Context) {
	notebookID := c.Param("notebook_id")
	if err := database.DB.Where("id = ?", notebookID).Delete(&models.Notebook{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao apagar caderno", "detalhe": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Caderno apagado!"})
}

// ListNotebooks mantido provisoriamente para não quebrar a Fase 4
func ListNotebooks(c *gin.Context) {
	spaceID := c.Param("space_id")
	var notebooks []models.Notebook
	database.DB.Where("space_id = ?", spaceID).Find(&notebooks)
	c.JSON(http.StatusOK, gin.H{"notebooks": notebooks})
}
