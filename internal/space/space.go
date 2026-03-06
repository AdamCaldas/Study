package space

import (
	"fmt"
	"net/http"
	"strings"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// O que esperamos receber do Frontend para criar um Space (Atualizado MVP)
type CreateSpaceInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	ColorHex    string `json:"color_hex"`
	Category    string `json:"category"`
	Visibility  string `json:"visibility"`
}

// Cria um novo Space
func CreateSpace(c *gin.Context) {
	// 1. Pega o ID do usuário logado que o AuthMiddleware salvou no contexto
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuário não autenticado"})
		return
	}

	// 2. Valida os dados enviados (o nome é obrigatório)
	var input CreateSpaceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	// Define valores padrão caso o usuário não envie
	if input.ColorHex == "" {
		input.ColorHex = "#FFFFFF"
	}
	if input.Visibility == "" {
		input.Visibility = "private"
	}

	// 3. MÁGICA: Gera um código aleatório curto para os links
	randomHex := uuid.New().String()[:6]

	// Gera o SLUG (ex: "direito-penal-a1b2c3")
	slugBase := strings.ToLower(strings.ReplaceAll(input.Name, " ", "-"))
	slug := fmt.Sprintf("%s-%s", slugBase, randomHex)

	// Gera o SHARE CODE (ex: "SPACE-A1B2C3")
	shareCode := fmt.Sprintf("SPACE-%s", strings.ToUpper(randomHex))

	// 4. Monta o Space associando ao dono e com os novos campos
	newSpace := models.Space{
		OwnerID:     userID.(uuid.UUID), // Converte de interface para UUID
		Name:        input.Name,
		Description: input.Description,
		ColorHex:    input.ColorHex,
		Category:    input.Category,
		Visibility:  input.Visibility,
		Status:      "active",
		Slug:        slug,
		ShareCode:   shareCode,
	}

	// 5. Salva no banco de dados
	if err := database.DB.Create(&newSpace).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar Space"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Space criado com sucesso!",
		"space":   newSpace,
	})
}

// Lista os Spaces do usuário logado
func ListSpaces(c *gin.Context) {
	userID, _ := c.Get("userID")
	var spaces []models.Space

	// Busca no banco todos os Spaces onde o owner_id é igual ao ID do usuário
	if err := database.DB.Where("owner_id = ?", userID).Find(&spaces).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar Spaces"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"spaces": spaces})
}
