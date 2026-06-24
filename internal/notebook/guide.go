package notebook

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils" // 👈 Import adicionado

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type GuideInput struct {
	Name          string     `json:"name" binding:"required"`
	Description   string     `json:"description"`
	Icon          string     `json:"icon"`
	ColorHex      string     `json:"color_hex"`
	Order         int        `json:"order"`
	ParentGuideID *uuid.UUID `json:"parent_guide_id"`

	// 🎨 NOVOS CAMPOS PARA PAGINAÇÃO DINÂMICA
	Orientation      string `json:"orientation"`
	PageSize         string `json:"page_size"`
	CustomDimensions string `json:"custom_dimensions"`
}

// ==========================================================
// 📁 CRIAR UMA NOVA GUIA (Pasta)
// ==========================================================
func CreateGuide(c *gin.Context) {
	notebookID := c.Param("notebook_id")

	// 👇 Uma única linha elegante substituindo aquele bloco switch gigante!
	userUUID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Utilizador não autenticado"})
		return
	}

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

	guide := models.Guide{
		NotebookID:       notebookUUID,
		ParentGuideID:    req.ParentGuideID,
		Name:             req.Name,
		Description:      req.Description,
		Icon:             req.Icon,
		ColorHex:         req.ColorHex,
		Order:            req.Order,
		Orientation:      req.Orientation,
		PageSize:         req.PageSize,
		CustomDimensions: req.CustomDimensions,
		CreatedByID:      userUUID,
		UpdatedByID:      userUUID,
	}

	if err := database.DB.Create(&guide).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar a Guia"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Guia criada com sucesso!", "guide": guide})
}

// ==========================================================
// ✏️ EDITAR UMA GUIA E SUAS CONFIGS DE PÁGINA
// ==========================================================
func UpdateGuide(c *gin.Context) {
	guideID := c.Param("guide_id")

	// 👇 Limpeza aqui também!
	userUUID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Utilizador não autenticado"})
		return
	}

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

	// Atualiza os dados normais e os de impressão!
	guide.Name = req.Name
	guide.Description = req.Description
	guide.Icon = req.Icon
	guide.ColorHex = req.ColorHex
	guide.Order = req.Order
	if req.Orientation != "" {
		guide.Orientation = req.Orientation
	}
	if req.PageSize != "" {
		guide.PageSize = req.PageSize
	}
	if req.CustomDimensions != "" {
		guide.CustomDimensions = req.CustomDimensions
	}
	guide.UpdatedByID = userUUID

	database.DB.Save(&guide)

	c.JSON(http.StatusOK, gin.H{"message": "Guia atualizada!", "guide": guide})
}

// ==========================================================
// 🔄 REORDENAR GUIAS (DRAG AND DROP)
// ==========================================================
type ReorderGuidesRequest struct {
	Guides []struct {
		GuideID string `json:"guide_id"`
		Order   int    `json:"order"`
	} `json:"guides"`
}

func ReorderGuides(c *gin.Context) {
	var req ReorderGuidesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato JSON inválido."})
		return
	}

	tx := database.DB.Begin()
	for _, g := range req.Guides {
		if err := tx.Model(&models.Guide{}).Where("id = ?", g.GuideID).Update("order", g.Order).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao reordenar guias"})
			return
		}
	}
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "Ordem das guias atualizada com sucesso!"})
}

// ==========================================================
// 🗑️ EXCLUIR UMA GUIA
// ==========================================================
func DeleteGuide(c *gin.Context) {
	guideID := c.Param("guide_id")

	if err := database.DB.Where("id = ?", guideID).Delete(&models.Guide{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao excluir a Guia"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Guia excluída com sucesso!"})
}
