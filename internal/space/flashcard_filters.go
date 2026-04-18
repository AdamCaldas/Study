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
	spaceIDStr := c.Param("space_id")
	var input FilterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nome da categoria é obrigatório."})
		return
	}

	newCat := models.FlashcardCategory{
		SpaceID: uuid.MustParse(spaceIDStr),
		Name:    input.Name,
	}
	database.DB.Create(&newCat)
	c.JSON(http.StatusCreated, gin.H{"message": "Categoria criada!", "category": newCat})
}

func ListCategories(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	var categories []models.FlashcardCategory
	database.DB.Where("space_id = ?", spaceIDStr).Find(&categories)
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
	spaceIDStr := c.Param("space_id")
	var input FilterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nome da tag é obrigatório."})
		return
	}

	newTag := models.FlashcardTag{
		SpaceID: uuid.MustParse(spaceIDStr),
		Name:    input.Name,
	}
	database.DB.Create(&newTag)
	c.JSON(http.StatusCreated, gin.H{"message": "Tag criada!", "tag": newTag})
}

func ListTags(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	var tags []models.FlashcardTag
	database.DB.Where("space_id = ?", spaceIDStr).Find(&tags)
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

func DeleteTag(c *gin.Context) {
	tagID := c.Param("tag_id")
	database.DB.Where("id = ?", tagID).Delete(&models.FlashcardTag{})
	c.JSON(http.StatusOK, gin.H{"message": "Tag apagada!"})
}