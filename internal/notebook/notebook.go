package notebook

import (
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

// Lista todos os cadernos de um Space específico
func ListNotebooks(c *gin.Context) {
	userID, _ := c.Get("userID")
	spaceID := c.Param("space_id")

	// Garante que o usuário só liste cadernos de um Space que ele tem acesso
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Acesso negado a este Space"})
		return
	}

	var notebooks []models.Notebook
	if err := database.DB.Where("space_id = ?", spaceID).Find(&notebooks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar Cadernos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"notebooks": notebooks})
}
