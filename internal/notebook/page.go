package notebook

import (
	"encoding/json"
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// O que o Frontend vai enviar para criar a página
type CreatePageInput struct {
	Title   string          `json:"title" binding:"required"`
	Content json.RawMessage `json:"content"` // Aceita qualquer JSON válido (Rich Text)
	Order   int             `json:"order"`
}

func CreatePage(c *gin.Context) {
	userID, _ := c.Get("userID")
	notebookID := c.Param("notebook_id")

	// 1. Busca o Caderno
	var notebook models.Notebook
	if err := database.DB.Where("id = ?", notebookID).First(&notebook).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Caderno não encontrado"})
		return
	}

	// 2. Segurança (Path Logic): Verifica se o usuário é o dono do Space desse caderno
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", notebook.SpaceID, userID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Você não tem permissão para adicionar páginas neste caderno"})
		return
	}

	// 3. Valida a entrada
	var input CreatePageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	// Se o conteúdo vier vazio, cria um JSON vazio padrão "{}"
	if len(input.Content) == 0 {
		input.Content = []byte("{}")
	}

	// 4. Monta e salva a Página
	parsedNotebookID, _ := uuid.Parse(notebookID)
	newPage := models.Page{
		NotebookID: parsedNotebookID,
		Title:      input.Title,
		Content:    input.Content,
		Order:      input.Order,
	}

	if err := database.DB.Create(&newPage).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar Página"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Página criada com sucesso!",
		"page":    newPage,
	})
}

// Lista todas as páginas de um caderno com suporte a Breadcrumbs
func ListPages(c *gin.Context) {
	userID, _ := c.Get("userID")
	notebookID := c.Param("notebook_id")

	// 1. Busca o Caderno
	var notebook models.Notebook
	if err := database.DB.First(&notebook, "id = ?", notebookID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Caderno não encontrado"})
		return
	}

	// 2. Segurança: valida se o usuário tem acesso ao Space
	var space models.Space
	if err := database.DB.First(&space, "id = ? AND owner_id = ?", notebook.SpaceID, userID).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Acesso negado"})
		return
	}

	// 3. Busca as páginas ordenadas pelo campo "Order"
	var pages []models.Page
	if err := database.DB.Where("notebook_id = ?", notebookID).Order("\"order\" asc").Find(&pages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar Páginas"})
		return
	}

	// 4. A Mágica dos Breadcrumbs!
	// Enviamos não só a lista de páginas, mas também os nomes das pastas pai
	c.JSON(http.StatusOK, gin.H{
		"breadcrumbs": gin.H{
			"space_name":    space.Name,
			"notebook_name": notebook.Name,
		},
		"pages": pages,
	})
}
