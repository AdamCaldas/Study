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

	var input CreateSpaceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	if input.ColorHex == "" {
		input.ColorHex = "#FFFFFF"
	}
	if input.Visibility == "" {
		input.Visibility = "private"
	}

	randomHex := uuid.New().String()[:6]

	slugBase := strings.ToLower(strings.ReplaceAll(input.Name, " ", "-"))
	slug := fmt.Sprintf("%s-%s", slugBase, randomHex)

	shareCode := fmt.Sprintf("SPACE-%s", strings.ToUpper(randomHex))

	newSpace := models.Space{
		OwnerID:     userID.(uuid.UUID),
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

// Usamos ponteiros (*bool) para os booleanos para o Go aceitar quando o front mandar "false"
type UpdateSpaceInput struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	ColorHex           string `json:"color_hex"`
	Category           string `json:"category"`
	Visibility         string `json:"visibility"`
	Status             string `json:"status"`              // Para o toggle de Space Ativo
	AllowCollaborators *bool  `json:"allow_collaborators"` // Toggle permitir colaboradores
	AllowComments      *bool  `json:"allow_comments"`      // Toggle comentários
	MaxCollaborators   int    `json:"max_collaborators"`   // Slider de limite de membros
}

// UpdateSpace - Atualiza as configurações e permissões
func UpdateSpace(c *gin.Context) {
	spaceID := c.Param("space_id")
	var input UpdateSpaceInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Dados inválidos", "detalhe": err.Error()})
		return
	}

	// Montamos um Map apenas com os campos que o front enviou
	updates := map[string]interface{}{}
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Description != "" {
		updates["description"] = input.Description
	}
	if input.ColorHex != "" {
		updates["color_hex"] = input.ColorHex
	}
	if input.Category != "" {
		updates["category"] = input.Category
	}
	if input.Visibility != "" {
		updates["visibility"] = input.Visibility
	}
	if input.Status != "" {
		updates["status"] = input.Status
	}
	if input.MaxCollaborators > 0 {
		updates["max_collaborators"] = input.MaxCollaborators
	}
	if input.AllowCollaborators != nil {
		updates["allow_collaborators"] = *input.AllowCollaborators
	}
	if input.AllowComments != nil {
		updates["allow_comments"] = *input.AllowComments
	}

	if err := database.DB.Model(&models.Space{}).Where("id = ?", spaceID).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao atualizar Space", "detalhe": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Configurações salvas!"})
}

func DeleteSpace(c *gin.Context) {
	spaceID := c.Param("space_id")

	if err := database.DB.Delete(&models.Space{}, "id = ?", spaceID).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao apagar o Space"})
		return
	}

	c.JSON(200, gin.H{"message": "Space deletado com sucesso!"})
}

// GetSpaceByCode - Busca o Space pelo código de compartilhamento
func GetSpaceByCode(c *gin.Context) {
	code := c.Param("code")
	var space models.Space

	if err := database.DB.Where("share_code = ?", code).First(&space).Error; err != nil {
		c.JSON(404, gin.H{"error": "Space inválido ou não encontrado"})
		return
	}

	// Busca o nome do dono na tabela de Usuários
	var owner models.User
	database.DB.Select("full_name").Where("id = ?", space.OwnerID).First(&owner)

	// Devolve o Space, a data de criação já vai junto nativamente, e agora o nome do dono também!
	c.JSON(200, gin.H{
		"space":      space,
		"owner_name": owner.FullName,
	})
}

type JoinSpaceInput struct {
	Code string `json:"code" binding:"required"`
}

// JoinSpaceByCode - Adiciona o usuário ao Space via código de convite
func JoinSpaceByCode(c *gin.Context) {
	// 1. Pega o ID do usuário que está tentando entrar
	userIDContext, _ := c.Get("userID")
	userIDStr := fmt.Sprintf("%v", userIDContext)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Usuário inválido"})
		return
	}

	// 2. Valida o JSON que o front-end mandou
	var input JoinSpaceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Código não fornecido", "detalhe": err.Error()})
		return
	}

	// 3. Acha o Space pelo código no banco de dados
	var space models.Space
	if err := database.DB.Where("share_code = ?", input.Code).First(&space).Error; err != nil {
		c.JSON(404, gin.H{"error": "Space não encontrado ou código inválido"})
		return
	}

	// 4. REGRA DE NEGÓCIO: O dono não pode "entrar" no próprio space como convidado
	if space.OwnerID == userID {
		c.JSON(400, gin.H{"error": "Você já é o dono deste Space!"})
		return
	}

	// 5. REGRA DE NEGÓCIO: Verifica se o usuário já não faz parte do Space
	var existingPermission models.SpacePermission
	if err := database.DB.Where("user_id = ? AND space_id = ?", userID, space.ID).First(&existingPermission).Error; err == nil {
		c.JSON(400, gin.H{"error": "Você já faz parte deste Space!"})
		return
	}

	// 6. Inserir o amigo na tabela de convidados com permissão de visualizador (VIEWER)
	newPermission := models.SpacePermission{
		UserID:      userID,
		SpaceID:     space.ID,
		AccessLevel: "VIEWER", // Permissão padrão: só pode ler, não pode editar/apagar
	}

	if err := database.DB.Create(&newPermission).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao entrar no Space", "detalhe": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message": "Bem-vindo ao Space!",
		"space":   space,
	})
}
