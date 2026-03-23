package admin

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==========================================================
// 📱 MODO APP: Listar tudo para os Alunos (GET)
// ==========================================================
func GetHelpCenter(c *gin.Context) {
	var categories []models.HelpCategory

	// O Preload puxa os artigos dentro da categoria e já traz tudo ordenado bonitinho pro Front-end
	if err := database.DB.Preload("Articles", func(db *gorm.DB) *gorm.DB {
		return db.Order("help_articles.order ASC")
	}).Order("help_categories.order ASC").Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao carregar a Central de Ajuda"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// ==========================================================
// ⚡ MODO DEUS (ADMIN): Criar Categoria e Artigo (POST)
// ==========================================================

func CreateHelpCategory(c *gin.Context) {
	var input struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		Order       int    `json:"order"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	category := models.HelpCategory{
		Title:       input.Title,
		Description: input.Description,
		Order:       input.Order,
	}

	database.DB.Create(&category)
	c.JSON(http.StatusCreated, gin.H{"message": "Categoria criada!", "category": category})
}

func CreateHelpArticle(c *gin.Context) {
	var input struct {
		CategoryID uuid.UUID `json:"category_id" binding:"required"`
		Title      string    `json:"title" binding:"required"`
		VideoURL   string    `json:"video_url"`
		Content    string    `json:"content"`
		Order      int       `json:"order"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	article := models.HelpArticle{
		CategoryID: input.CategoryID,
		Title:      input.Title,
		VideoURL:   input.VideoURL,
		Content:    input.Content,
		Order:      input.Order,
	}

	database.DB.Create(&article)
	c.JSON(http.StatusCreated, gin.H{"message": "Artigo com vídeo criado!", "article": article})
}

func DeleteHelpArticle(c *gin.Context) {
	articleID := c.Param("article_id")
	database.DB.Where("id = ?", articleID).Delete(&models.HelpArticle{})
	c.JSON(http.StatusOK, gin.H{"message": "Artigo removido com sucesso."})
}

func DeleteHelpCategory(c *gin.Context) {
	categoryID := c.Param("category_id")
	// Como colocamos OnDelete:CASCADE no models, isso apaga a categoria e todos os vídeos dentro dela!
	database.DB.Where("id = ?", categoryID).Delete(&models.HelpCategory{})
	c.JSON(http.StatusOK, gin.H{"message": "Categoria removida com sucesso."})
}
