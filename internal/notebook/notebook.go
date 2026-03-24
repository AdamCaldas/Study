package notebook

import (
	"net/http"
	"time"

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
	// 1. É o dono do Space?
	var space models.Space
	if err := database.DB.Select("id").Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error; err == nil {
		return true // Pode tudo
	}

	// 2. Tem permissão direta no Caderno?
	var nbPerm models.NotebookPermission
	if err := database.DB.Select("access_level").Where("notebook_id = ? AND user_id = ?", notebookID, userID).First(&nbPerm).Error; err == nil {
		return nbPerm.AccessLevel == "EDITOR"
	}

	// 3. Tem permissão geral de editor no Space?
	var spPerm models.SpacePermission
	if err := database.DB.Select("can_create_content").Where("space_id = ? AND user_id = ?", spaceID, userID).First(&spPerm).Error; err == nil {
		return spPerm.CanCreateContent // Se ele pode criar conteúdo no space, ele pode editar cadernos
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

	// Verifica a permissão granular nova ("can_create_content")
	var permission models.SpacePermission
	canCreate := database.DB.Where("space_id = ? AND user_id = ? AND can_create_content = true", parsedSpaceID, parsedUserID).First(&permission).Error == nil

	if !isOwner && !canCreate {
		c.JSON(http.StatusForbidden, gin.H{"error": "Você não tem permissão para criar cadernos neste Space."})
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

// ==========================================================
// 🗑️ DELETE NOTEBOOK
// ==========================================================
func DeleteNotebook(c *gin.Context) {
	notebookID := c.Param("notebook_id")

	// O GORM Cascata vai apagar as Guias e as Páginas sozinhas!
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

// ==========================================================
// 📚 3. GET /spaces/:space_id/notebooks (Aba de Cadernos)
// ==========================================================
func ListSpaceNotebooks(c *gin.Context) {
	spaceID := c.Param("space_id")

	var notebooks []models.Notebook
	database.DB.
		Preload("Pages").
		Preload("Guides.Pages").
		Preload("Guides.SubGuides.Pages").
		Where("space_id = ?", spaceID).
		Find(&notebooks)

	now := time.Now()
	for i := range notebooks {
		if notebooks[i].UnlockAt != nil && notebooks[i].UnlockAt.After(now) {
			notebooks[i].IsLocked = true
			notebooks[i].Pages = []models.Page{} // Esconde do aluno se tiver cadeado
			notebooks[i].Guides = []models.Guide{}
		}
	}

	c.JSON(http.StatusOK, gin.H{"notebooks": notebooks})
}
