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
		Content:    string(input.Content),
		Order:      input.Order,
	}

	if err := database.DB.Create(&newPage).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Erro ao criar Página",
			"detalhe": err.Error(), // 👈 A MÁGICA TÁ AQUI! Isso vai mostrar o erro real no F12!
		})
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

// DeletePage - Apaga uma página
func DeletePage(c *gin.Context) {
	pageID := c.Param("page_id")
	if err := database.DB.Where("id = ?", pageID).Delete(&models.Page{}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao apagar página", "detalhe": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Página apagada com sucesso!"})
}

type UpdatePageInput struct {
	Title   string                 `json:"title"`
	Content map[string]interface{} `json:"content"` // Recebe o JSON flexível do front
	Order   int                    `json:"order"`
}

// UpdatePage - Edita título ou conteúdo da página
func UpdatePage(c *gin.Context) {
	pageID := c.Param("page_id")
	var input UpdatePageInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Dados inválidos"})
		return
	}

	// Transforma o map de volta para string para salvar no JSONB do banco sem dar erro 22P02
	contentBytes, _ := json.Marshal(input.Content)
	contentStr := string(contentBytes)

	// Se o front não mandou conteúdo, não atualizamos o Content para não apagar o que já tem
	updates := map[string]interface{}{
		"title": input.Title,
		"order": input.Order,
	}
	if len(input.Content) > 0 {
		updates["content"] = contentStr
	}

	if err := database.DB.Model(&models.Page{}).Where("id = ?", pageID).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao atualizar página", "detalhe": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Página atualizada com sucesso!"})
}

// Estrutura que o Front vai mandar no Body
type ReorderPagesRequest struct {
	Pages []struct {
		PageID string `json:"page_id"`
		Order  int    `json:"order"`
	} `json:"pages"`
}

// ReorderPages - Salva a nova ordem de várias páginas de uma vez
func ReorderPages(c *gin.Context) {
	var req ReorderPagesRequest

	// 1. Recebe o Array do Front-end
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato JSON inválido. Envie um array de pages."})
		return
	}

	// 2. Inicia uma transação no banco (Se der erro em uma página, desfaz tudo)
	tx := database.DB.Begin()

	// 3. Loop pelas páginas recebidas para atualizar a ordem de cada uma
	for _, p := range req.Pages {
		// Atualiza apenas a coluna 'order' daquela página específica
		if err := tx.Model(&models.Page{}).Where("id = ?", p.PageID).Update("order", p.Order).Error; err != nil {
			tx.Rollback() // Deu ruim? Cancela as trocas!
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao reordenar as páginas", "detalhe": err.Error()})
			return
		}
	}

	// 4. Salva todas as alterações
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{"message": "Ordem das páginas atualizada com sucesso!"})
}
