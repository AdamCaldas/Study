package study

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

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

	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	var input QuestionGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados do grupo inválidos."})
		return
	}

	newGroup := models.QuestionGroup{
		SpaceID:     uuid.MustParse(spaceIDStr),
		CreatedByID: userID, // 👈 Salva o dono da pasta!
		Name:        input.Name,
		Description: input.Description,
		ColorHex:    input.ColorHex,
	}

	if err := database.DB.Create(&newGroup).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar grupo."})
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

	var groups []models.QuestionGroup
	// O Preload("Creator") garante que o Front-end receba o ProfilePic do dono da pasta!
	database.DB.Preload("Creator").Where("space_id = ?", spaceIDStr).Order("created_at asc").Find(&groups)

	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

// ==========================================================
// ✏️ 3. EDITAR UMA PASTINHA
// ==========================================================
func UpdateQuestionGroup(c *gin.Context) {
	groupID := c.Param("group_id")
	spaceIDStr := c.Param("space_id")

	var input QuestionGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	if err := database.DB.Model(&models.QuestionGroup{}).Where("id = ? AND space_id = ?", groupID, spaceIDStr).Updates(map[string]interface{}{
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
	groupID := c.Param("group_id")
	spaceIDStr := c.Param("space_id")

	if err := database.DB.Where("id = ? AND space_id = ?", groupID, spaceIDStr).Delete(&models.QuestionGroup{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao apagar pasta."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pasta removida!"})
}