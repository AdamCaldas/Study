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

// --- ESTRUTURA PARA EDITAR SPACE ---
type UpdateSpaceInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ColorHex    string `json:"color_hex"`
	Category    string `json:"category"`
	Visibility  string `json:"visibility"`
}

// UpdateSpace - Atualiza os dados de um Space existente
func UpdateSpace(c *gin.Context) {
	spaceID := c.Param("space_id")

	var input UpdateSpaceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Dados inválidos para atualização"})
		return
	}

	// O GORM usa o .Updates para alterar apenas as colunas que vieram no JSON
	if err := database.DB.Model(&models.Space{}).Where("id = ?", spaceID).Updates(input).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao atualizar o Space no banco de dados"})
		return
	}

	c.JSON(200, gin.H{"message": "Space atualizado com sucesso!"})
}

// DeleteSpace - Apaga um Space do banco de dados
func DeleteSpace(c *gin.Context) {
	spaceID := c.Param("space_id")

	// O GORM faz um "Soft Delete" automático se você configurou o gorm.Model na struct
	if err := database.DB.Delete(&models.Space{}, "id = ?", spaceID).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao apagar o Space"})
		return
	}

	c.JSON(200, gin.H{"message": "Space deletado com sucesso!"})
}

// GetSpaceByCode - Busca um space público/compartilhado pelo código (SPACE-123)
func GetSpaceByCode(c *gin.Context) {
	code := c.Param("code")

	var space models.Space
	if err := database.DB.Where("share_code = ?", code).First(&space).Error; err != nil {
		c.JSON(404, gin.H{"error": "Space não encontrado ou código inválido"})
		return
	}

	c.JSON(200, space)
}

// JoinSpaceByCode - Permite que o usuário logado entre em um Space usando o código
func JoinSpaceByCode(c *gin.Context) {
	userIDContext, _ := c.Get("userID")

	// Converte o ID do contexto (texto) para o formato uuid.UUID do banco
	userID, err := uuid.Parse(userIDContext.(string))
	if err != nil {
		c.JSON(400, gin.H{"error": "ID de usuário inválido"})
		return
	}

	var input struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "O código do Space é obrigatório"})
		return
	}

	// 1. Acha o Space pelo código
	var space models.Space
	if err := database.DB.Where("share_code = ?", input.Code).First(&space).Error; err != nil {
		c.JSON(404, gin.H{"error": "Space inválido ou não encontrado"})
		return
	}

	// 2. Verifica se o usuário já é o dono do Space
	if space.OwnerID == userID {
		c.JSON(400, gin.H{"error": "Você já é o dono deste Space!"})
		return
	}

	// 3. Verifica se o usuário já é um convidado (membro)
	var perm models.SpacePermission
	err = database.DB.Where("space_id = ? AND user_id = ?", space.ID, userID).First(&perm).Error
	if err == nil {
		c.JSON(400, gin.H{"error": "Você já é membro deste Space!"})
		return
	}

	// 4. Cria a permissão de membro (Convidado/Viewer)
	newPerm := models.SpacePermission{
		SpaceID:     space.ID,
		UserID:      userID,   // Agora sim, os dois são uuid.UUID!
		AccessLevel: "VIEWER", // Nível padrão para quem entra por link
	}

	if err := database.DB.Create(&newPerm).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao tentar entrar no Space"})
		return
	}

	c.JSON(200, gin.H{
		"message": "Você entrou no Space com sucesso!",
		"space":   space,
	})
}
