package notebook

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type GuideInput struct {
	Name          string     `json:"name" binding:"required"`
	Description   string     `json:"description"`
	Icon          string     `json:"icon"`
	ColorHex      string     `json:"color_hex"`
	Order         int        `json:"order"`
	ParentGuideID *uuid.UUID `json:"parent_guide_id"` // Manda nulo se for guia principal, manda ID se for sub-guia
}

// ==========================================================
// 📁 CRIAR UMA NOVA GUIA (Pasta)
// ==========================================================
func CreateGuide(c *gin.Context) {
	notebookID := c.Param("notebook_id")
	userIDInterface, _ := c.Get("userID")

	var req GuideInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos", "details": err.Error()})
		return
	}

	notebookUUID, err := uuid.Parse(notebookID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do caderno inválido"})
		return
	}

	// Converte o ID do usuário da sessão
	var userUUID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userUUID = v
	case string:
		userUUID, _ = uuid.Parse(v)
	}

	guide := models.Guide{
		NotebookID:    notebookUUID,
		ParentGuideID: req.ParentGuideID,
		Name:          req.Name,
		Description:   req.Description,
		Icon:          req.Icon,
		ColorHex:      req.ColorHex,
		Order:         req.Order,
		CreatedByID:   userUUID,
		UpdatedByID:   userUUID,
	}

	if err := database.DB.Create(&guide).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar a Guia"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Guia criada com sucesso!", "guide": guide})
}

// ==========================================================
// ✏️ EDITAR UMA GUIA (Nome, Cor, Ícone, etc)
// ==========================================================
func UpdateGuide(c *gin.Context) {
	guideID := c.Param("guide_id")
	userIDInterface, _ := c.Get("userID")

	var req GuideInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	var guide models.Guide
	if err := database.DB.Where("id = ?", guideID).First(&guide).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Guia não encontrada"})
		return
	}

	// Converte o ID do usuário para registrar quem editou
	var userUUID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userUUID = v
	case string:
		userUUID, _ = uuid.Parse(v)
	}

	guide.Name = req.Name
	guide.Description = req.Description
	guide.Icon = req.Icon
	guide.ColorHex = req.ColorHex
	guide.Order = req.Order
	guide.UpdatedByID = userUUID

	database.DB.Save(&guide)

	c.JSON(http.StatusOK, gin.H{"message": "Guia atualizada!", "guide": guide})
}

// ==========================================================
// 🗑️ EXCLUIR UMA GUIA (E tudo dentro dela)
// ==========================================================
func DeleteGuide(c *gin.Context) {
	guideID := c.Param("guide_id")

	// O GORM já vai apagar as páginas dentro dela automaticamente por causa do OnDelete:CASCADE no models.go!
	if err := database.DB.Where("id = ?", guideID).Delete(&models.Guide{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao excluir a Guia"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Guia excluída com sucesso!"})
}
