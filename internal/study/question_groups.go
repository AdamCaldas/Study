package study

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils" // 👈 Import global

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type QuestionGroupInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	ColorHex    string `json:"color_hex"`
}

// ==========================================================
// 📁 1. CRIAR UMA PASTINHA DE EDITAL (GRUPO)
// ==========================================================
func CreateQuestionGroup(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	parsedSpaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido."})
		return
	}

	// 👇 Limpeza do ID!
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado."})
		return
	}

	var input QuestionGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados do grupo inválidos."})
		return
	}

	newGroup := models.QuestionGroup{
		SpaceID:     parsedSpaceID,
		CreatedByID: userID, // 👈 Salva o dono da pasta!
		Name:        input.Name,
		Description: input.Description,
		ColorHex:    input.ColorHex,
	}

	if err := database.DB.Create(&newGroup).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar grupo."}) // 👈 Erro blindado
		return
	}

	// Puxa o criador pra devolver completinho
	database.DB.Preload("Creator").First(&newGroup, newGroup.ID)

	c.JSON(http.StatusCreated, gin.H{"message": "Pasta criada com sucesso!", "group": newGroup})
}

// ==========================================================
// 📋 2. LISTAR AS PASTINHAS DO SPACE
// ==========================================================
func ListQuestionGroups(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	parsedSpaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido."})
		return
	}

	var groups []models.QuestionGroup
	database.DB.Preload("Creator").Where("space_id = ?", parsedSpaceID).Order("created_at asc").Find(&groups)

	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

// ==========================================================
// ✏️ 3. EDITAR UMA PASTINHA
// ==========================================================
func UpdateQuestionGroup(c *gin.Context) {
	groupIDStr := c.Param("group_id")
	spaceIDStr := c.Param("space_id")

	parsedGroupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Grupo inválido."})
		return
	}

	var input QuestionGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	if err := database.DB.Model(&models.QuestionGroup{}).Where("id = ? AND space_id = ?", parsedGroupID, spaceIDStr).Updates(map[string]interface{}{
		"name":        input.Name,
		"description": input.Description,
		"color_hex":   input.ColorHex,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar pasta."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pasta atualizada!"})
}

// ==========================================================
// 🗑️ 4. APAGAR UMA PASTINHA
// ==========================================================
func DeleteQuestionGroup(c *gin.Context) {
	groupIDStr := c.Param("group_id")
	spaceIDStr := c.Param("space_id")

	parsedGroupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Grupo inválido."})
		return
	}

	if err := database.DB.Where("id = ? AND space_id = ?", parsedGroupID, spaceIDStr).Delete(&models.QuestionGroup{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao apagar pasta."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pasta removida!"})
}
