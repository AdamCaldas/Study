package notebook

import (
	"fmt"
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Estrutura que o Frontend vai enviar
type CreateNotebookInput struct {
	Name     string `json:"name" binding:"required"`
	ColorHex string `json:"color_hex"`
}

func CreateNotebook(c *gin.Context) {
	// 1. Pega o ID do usuário (do porteiro) e o ID do Space (da URL)
	userID, _ := c.Get("userID")
	spaceID := c.Param("space_id")

	// 2. Validação de Segurança: O Space existe e pertence a esse usuário?
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Space não encontrado ou acesso negado"})
		return
	}

	// 3. Valida os dados enviados (o nome do caderno é obrigatório)
	var input CreateNotebookInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	if input.ColorHex == "" {
		input.ColorHex = "#E0E0E0" // Cor padrão cinza claro
	}

	// 4. Converte a string da URL para UUID e monta o Caderno
	parsedSpaceID, _ := uuid.Parse(spaceID)
	newNotebook := models.Notebook{
		SpaceID:  parsedSpaceID,
		Name:     input.Name,
		ColorHex: input.ColorHex,
	}

	// 5. Salva no banco de dados
	if err := database.DB.Create(&newNotebook).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar Caderno"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Caderno criado com sucesso!",
		"notebook": newNotebook,
	})
}

// Exemplo de como deve ficar a sua função ListNotebooks (ou GetAll)
func ListNotebooks(c *gin.Context) {
	spaceID := c.Param("space_id")

	// Pega o ID do usuário logado (Dono ou Convidado)
	userIDContext, _ := c.Get("userID")
	userIDStr := fmt.Sprintf("%v", userIDContext)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "ID de usuário inválido"})
		return
	}

	// 1. CHECAGEM DE SEGURANÇA (O Leão de Chácara)
	// Verifica se o usuário é o DONO do Space OU se ele está na tabela de CONVIDADOS (SpacePermissions)
	var space models.Space
	var permission models.SpacePermission

	isOwner := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error == nil
	isGuest := database.DB.Where("space_id = ? AND user_id = ?", spaceID, userID).First(&permission).Error == nil

	// Se não for dono e não for convidado, BLOQUEIA!
	if !isOwner && !isGuest {
		c.JSON(403, gin.H{"error": "Acesso Negado: Você não tem permissão para ver este Space."})
		return
	}

	// 2. BUSCA OS CADERNOS (Se passou da segurança, libera a leitura)
	var notebooks []models.Notebook
	if err := database.DB.Where("space_id = ?", spaceID).Find(&notebooks).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao carregar os cadernos", "detalhe": err.Error()})
		return
	}

	c.JSON(200, gin.H{"notebooks": notebooks})
}

// DeleteNotebook - Apaga um caderno e tudo dentro dele
func DeleteNotebook(c *gin.Context) {
	notebookID := c.Param("notebook_id")
	if err := database.DB.Where("id = ?", notebookID).Delete(&models.Notebook{}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao apagar caderno", "detalhe": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Caderno apagado com sucesso!"})
}

type UpdateNotebookInput struct {
	Name     string `json:"name"`
	ColorHex string `json:"color_hex"`
}

// UpdateNotebook - Edita nome e cor do caderno
func UpdateNotebook(c *gin.Context) {
	notebookID := c.Param("notebook_id")
	var input UpdateNotebookInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Dados inválidos"})
		return
	}

	if err := database.DB.Model(&models.Notebook{}).Where("id = ?", notebookID).Updates(models.Notebook{
		Name:     input.Name,
		ColorHex: input.ColorHex,
	}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao atualizar caderno", "detalhe": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Caderno atualizado!"})
}
