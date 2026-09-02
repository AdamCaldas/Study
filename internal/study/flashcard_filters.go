package study

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FilterInput struct {
	Name string `json:"name" binding:"required"`
}

// ==========================================================
// 📂 CATEGORIAS: Criar, Listar e Apagar
// ==========================================================
func CreateCategory(c *gin.Context) {
	parsedSpaceID, err := uuid.Parse(c.Param("space_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido."})
		return
	}

	var input FilterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nome da categoria é obrigatório."})
		return
	}

	newCat := models.FlashcardCategory{SpaceID: parsedSpaceID, Name: input.Name}
	if err := database.DB.Create(&newCat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar categoria."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Categoria criada!", "category": newCat})
}

func ListCategories(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	var categories []models.FlashcardCategory
	database.DB.Where("space_id = ?", spaceIDStr).Order("name asc").Limit(500).Find(&categories)
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

func DeleteCategory(c *gin.Context) {
	catID := c.Param("category_id")
	database.DB.Where("id = ?", catID).Delete(&models.FlashcardCategory{})
	c.JSON(http.StatusOK, gin.H{"message": "Categoria apagada!"})
}

// ==========================================================
// 🏷️ TAGS: Criar, Listar e Apagar
// ==========================================================
func CreateTag(c *gin.Context) {
	parsedSpaceID, err := uuid.Parse(c.Param("space_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido."})
		return
	}

	var input FilterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nome da tag é obrigatório."})
		return
	}

	newTag := models.FlashcardTag{SpaceID: parsedSpaceID, Name: input.Name}
	if err := database.DB.Create(&newTag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar tag."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Tag criada!", "tag": newTag})
}

func ListTags(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	var tags []models.FlashcardTag
	database.DB.Where("space_id = ?", spaceIDStr).Order("name asc").Limit(500).Find(&tags)
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

func DeleteTag(c *gin.Context) {
	tagID := c.Param("tag_id")
	database.DB.Where("id = ?", tagID).Delete(&models.FlashcardTag{})
	c.JSON(http.StatusOK, gin.H{"message": "Tag apagada!"})
}
