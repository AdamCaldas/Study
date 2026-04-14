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
// 1️⃣ CREATE PAGE (Agora dentro da Guia!)
// ==========================================================
type CreatePageInput struct {
	Title   string           `json:"title" binding:"required"`
	Content json.RawMessage  `json:"content"`
	Order   int              `json:"order"`
	Tags    []models.PageTag `json:"tags"`
}

func CreatePage(c *gin.Context) {
	parsedUserID := getUserID(c)
	guideIDStr := c.Param("guide_id") // 👈 A mágica: Lê da Guia e não do Caderno
	parsedGuideID, err := uuid.Parse(guideIDStr)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID da Guia inválido"})
		return
	}

	var guide models.Guide
	if err := database.DB.Where("id = ?", parsedGuideID).First(&guide).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Guia não encontrada"})
		return
	}

	var notebook models.Notebook
	database.DB.Where("id = ?", guide.NotebookID).First(&notebook)

	if !canEditNotebook(notebook.SpaceID, notebook.ID, parsedUserID) {
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

	if input.Tags == nil {
		input.Tags = []models.PageTag{}
	}

	newPage := models.Page{
		NotebookID:  notebook.ID,   // Mantém pra histórico
		GuideID:     parsedGuideID, // 👈 Prende a página na guia!
		Title:       input.Title,
		Content:     string(input.Content),
		Order:       input.Order,
		Tags:        input.Tags,
		CreatedByID: parsedUserID,
		UpdatedByID: parsedUserID,
	}

	if err := database.DB.Create(&newPage).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar Página"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Página criada na guia com sucesso!", "page": newPage})
}

// ==========================================================
// 📋 LISTAR PÁGINAS DA GUIA (Para o Mayan renderizar o A4)
// ==========================================================
func ListPagesByGuide(c *gin.Context) {
	guideIDStr := c.Param("guide_id")
	var pages []models.Page

	// Puxa as páginas dessa guia específica, em ordem
	database.DB.Where("guide_id = ?", guideIDStr).Order("\"order\" asc").Find(&pages)
	c.JSON(http.StatusOK, gin.H{"pages": pages})
}

// ==========================================================
// 2️⃣ UPDATE PAGE
// ==========================================================
type UpdatePageInput struct {
	Title   *string           `json:"title"`
	Content *json.RawMessage  `json:"content"`
	Order   *int              `json:"order"`
	Tags    *[]models.PageTag `json:"tags"`
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	updates := map[string]interface{}{
		"updated_by_id": parsedUserID,
	}

	if input.Title != nil {
		updates["title"] = *input.Title
	}
	if input.Order != nil {
		updates["order"] = *input.Order
	}
	if input.Tags != nil {
		updates["tags"] = *input.Tags
	}
	if input.Content != nil {
		updates["content"] = string(*input.Content)
	}

	if err := database.DB.Model(&page).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro do Banco ao atualizar página: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Página atualizada com sucesso!"})
}

// ==========================================================
// 3️⃣ REORDER E DELETE DE PÁGINAS
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
