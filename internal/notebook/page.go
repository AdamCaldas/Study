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

// ==========================================================
// CREATE PAGE - Permite Donos e EDITORES criarem páginas
// ==========================================================
func CreatePage(c *gin.Context) {
	// Pega o ID de forma segura
	userIDInterface, _ := c.Get("userID")
	var parsedUserID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		parsedUserID = v
	case string:
		parsedUserID, _ = uuid.Parse(v)
	}

	notebookID := c.Param("notebook_id")

	// 1. Busca o Caderno
	var notebook models.Notebook
	if err := database.DB.Where("id = ?", notebookID).First(&notebook).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Caderno não encontrado"})
		return
	}

	// 2. 🛡️ SEGURANÇA INTELIGENTE: É Dono OU Editor?
	var space models.Space
	isOwner := database.DB.Where("id = ? AND owner_id = ?", notebook.SpaceID, parsedUserID).First(&space).Error == nil

	var permission models.SpacePermission
	isEditor := database.DB.Where("space_id = ? AND user_id = ? AND access_level = 'EDITOR'", notebook.SpaceID, parsedUserID).First(&permission).Error == nil

	// Se não for dono e não for editor, BLOQUEIA! (403 Forbidden)
	if !isOwner && !isEditor {
		c.JSON(http.StatusForbidden, gin.H{"error": "Você não tem permissão de EDITOR para adicionar páginas neste caderno"})
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
			"detalhe": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Página criada com sucesso!",
		"page":    newPage,
	})
}

// ==========================================================
// LIST PAGES - Permite Donos, Viewers e Editores lerem
// ==========================================================
func ListPages(c *gin.Context) {
	// Pega o ID de forma segura
	userIDInterface, _ := c.Get("userID")
	var parsedUserID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		parsedUserID = v
	case string:
		parsedUserID, _ = uuid.Parse(v)
	}

	notebookID := c.Param("notebook_id")

	// 1. Busca o Caderno
	var notebook models.Notebook
	if err := database.DB.First(&notebook, "id = ?", notebookID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Caderno não encontrado"})
		return
	}

	// 2. 🛡️ SEGURANÇA INTELIGENTE: É Dono ou Convidado (Qualquer nível)?
	var space models.Space
	isOwner := database.DB.First(&space, "id = ? AND owner_id = ?", notebook.SpaceID, parsedUserID).Error == nil

	var permission models.SpacePermission
	isGuest := database.DB.Where("space_id = ? AND user_id = ?", notebook.SpaceID, parsedUserID).First(&permission).Error == nil

	if !isOwner && !isGuest {
		c.JSON(http.StatusForbidden, gin.H{"error": "Acesso negado: Você não faz parte deste Space"})
		return
	}

	// 3. Busca as páginas ordenadas pelo campo "Order"
	var pages []models.Page
	if err := database.DB.Where("notebook_id = ?", notebookID).Order("\"order\" asc").Find(&pages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar Páginas"})
		return
	}

	// 4. A Mágica dos Breadcrumbs!
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
