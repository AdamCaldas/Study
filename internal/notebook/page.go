package notebook

import (
	"encoding/json"
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==========================================================
// 1️⃣ CREATE PAGE
// ==========================================================
type CreatePageInput struct {
	Title   string          `json:"title" binding:"required"`
	Content json.RawMessage `json:"content"`
	Order   int             `json:"order"`
	GuideID *uuid.UUID      `json:"guide_id"` // 👈 NOVO: Sabe se vai para dentro de uma pasta
}

func CreatePage(c *gin.Context) {
	parsedUserID := getUserID(c) // Usa a função auxiliar do notebook.go!
	notebookIDStr := c.Param("notebook_id")
	parsedNotebookID, _ := uuid.Parse(notebookIDStr)

	var notebook models.Notebook
	if err := database.DB.Where("id = ?", parsedNotebookID).First(&notebook).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Caderno não encontrado"})
		return
	}

	// Usa o Leão de Chácara que tá no notebook.go
	if !canEditNotebook(notebook.SpaceID, parsedNotebookID, parsedUserID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Você não tem permissão para adicionar páginas aqui."})
		return
	}

	var input CreatePageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	if len(input.Content) == 0 {
		input.Content = []byte("{}")
	}

	newPage := models.Page{
		NotebookID:  parsedNotebookID,
		GuideID:     input.GuideID, // 👈 Se mandar nulo, fica solta. Se mandar ID, entra na pasta.
		Title:       input.Title,
		Content:     string(input.Content),
		Order:       input.Order,
		CreatedByID: parsedUserID, // ASSINATURA
		UpdatedByID: parsedUserID, // ASSINATURA
	}

	if err := database.DB.Create(&newPage).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar Página"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Página criada com sucesso!", "page": newPage})
}

// ==========================================================
// 2️⃣ UPDATE PAGE
// ==========================================================
type UpdatePageInput struct {
	Title   string                 `json:"title"`
	Content map[string]interface{} `json:"content"`
	Order   int                    `json:"order"`
	GuideID *uuid.UUID             `json:"guide_id"` // 👈 NOVO: Permite arrastar a página de uma pasta pra outra
}

func UpdatePage(c *gin.Context) {
	parsedUserID := getUserID(c)
	pageIDStr := c.Param("page_id")

	var page models.Page
	if err := database.DB.Where("id = ?", pageIDStr).First(&page).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Página não encontrada"})
		return
	}

	var notebook models.Notebook
	database.DB.Where("id = ?", page.NotebookID).First(&notebook)

	if !canEditNotebook(notebook.SpaceID, notebook.ID, parsedUserID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Você não tem permissão para editar estas páginas."})
		return
	}

	var input UpdatePageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	updates := map[string]interface{}{
		"title":         input.Title,
		"order":         input.Order,
		"updated_by_id": parsedUserID, // ASSINATURA
	}

	// Permite mudar a página de pasta
	if input.GuideID != nil {
		updates["guide_id"] = input.GuideID
	}

	if len(input.Content) > 0 {
		contentBytes, _ := json.Marshal(input.Content)
		updates["content"] = string(contentBytes)
	}

	if err := database.DB.Model(&page).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar página"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Página atualizada!"})
}

// ==========================================================
// 3️⃣ REORDER E DELETE
// ==========================================================
func DeletePage(c *gin.Context) {
	pageID := c.Param("page_id")
	if err := database.DB.Where("id = ?", pageID).Delete(&models.Page{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao apagar página"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Página apagada!"})
}

type ReorderPagesRequest struct {
	Pages []struct {
		PageID string `json:"page_id"`
		Order  int    `json:"order"`
	} `json:"pages"`
}

func ReorderPages(c *gin.Context) {
	var req ReorderPagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato JSON inválido."})
		return
	}

	tx := database.DB.Begin()
	for _, p := range req.Pages {
		if err := tx.Model(&models.Page{}).Where("id = ?", p.PageID).Update("order", p.Order).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao reordenar"})
			return
		}
	}
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "Ordem atualizada com sucesso!"})
}

// ListPages mantido provisoriamente para não quebrar a Fase 4
func ListPages(c *gin.Context) {
	notebookID := c.Param("notebook_id")
	var pages []models.Page
	database.DB.Where("notebook_id = ?", notebookID).Order("\"order\" asc").Find(&pages)
	c.JSON(http.StatusOK, gin.H{"pages": pages})
}
