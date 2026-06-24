package study

import (
	"encoding/json"
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils" // 👈 Import global adicionado

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Estrutura para receber os dados do Front-end
type CreateFlashcardInput struct {
	Title       string          `json:"title"`
	GroupID     string          `json:"group_id"`
	Category    string          `json:"category"`
	SubCategory string          `json:"sub_category"`
	Tags        json.RawMessage `json:"tags"` // 👈 Recebe as Tags como array JSON ["CESPE", "Difícil"]
	Front       string          `json:"front"`
	Back        string          `json:"back"`
	Hint        string          `json:"hint"`

	QuestionID          string `json:"question_id"`
	QuestionSource      string `json:"question_source"`
	SaveAsSpaceQuestion bool   `json:"save_as_space_question"`
}

// ==========================================================
// ➕ 1. CRIAR UM FLASHCARD (Com Tags e Filtros)
// ==========================================================
func CreateFlashcard(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	parsedSpaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido."})
		return
	}

	// 👇 Extração de ID limpa e segura!
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado."})
		return
	}

	var input CreateFlashcardInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos do Flashcard."})
		return
	}

	// Garante que as tags nunca fiquem nulas no banco
	tagsStr := string(input.Tags)
	if len(input.Tags) == 0 || tagsStr == "null" {
		tagsStr = "[]"
	}

	if input.QuestionID != "" {
		if input.QuestionSource == "STUDFY" {
			var q models.StudfyQuestion
			if err := database.DB.Where("id = ?", input.QuestionID).First(&q).Error; err == nil {
				input.Title = q.Title
				input.Category = q.Discipline
				input.Front = q.QuestionText
				input.Back = "Gabarito: " + q.CorrectAnswer
			}
		} else if input.QuestionSource == "SPACE" {
			var q models.SpaceQuestion
			if err := database.DB.Where("id = ?", input.QuestionID).First(&q).Error; err == nil {
				input.Title = q.Title
				input.GroupID = q.GroupID
				input.Category = q.Discipline
				input.Front = q.QuestionText
				input.Back = "Gabarito: " + q.CorrectAnswer
			}
		}
	} else if input.SaveAsSpaceQuestion {
		newSpaceQ := models.SpaceQuestion{
			SpaceID:       parsedSpaceID,
			CreatedByID:   userID,
			Title:         input.Title,
			Discipline:    input.Category,
			GroupID:       input.GroupID,
			QuestionText:  input.Front,
			CorrectAnswer: input.Back,
			QuestionType:  "flashcard_generated",
			Source:        "CUSTOM",
			Points:        1,
			Options:       "[]",
		}
		database.DB.Create(&newSpaceQ)
	}

	newCard := models.Flashcard{
		SpaceID:     parsedSpaceID,
		CreatedByID: userID,
		GroupID:     input.GroupID,
		Title:       input.Title,
		Category:    input.Category,
		SubCategory: input.SubCategory,
		Tags:        tagsStr, // 👈 Salva as tags!
		Front:       input.Front,
		Back:        input.Back,
		Hint:        input.Hint,
	}

	if err := database.DB.Create(&newCard).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar Flashcard."})
		return
	}

	database.DB.Preload("Creator").First(&newCard, newCard.ID)
	c.JSON(http.StatusCreated, gin.H{"message": "Flashcard gerado com sucesso!", "flashcard": newCard})
}

// ==========================================================
// 📋 2. LISTAR FLASHCARDS (O Motor de Filtros Completo)
// ==========================================================
func ListFlashcards(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	parsedSpaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido."})
		return
	}

	// Todos os Filtros que o Mayan pode usar!
	groupID := c.Query("group_id")
	category := c.Query("category")
	subCategory := c.Query("sub_category")
	tag := c.Query("tag") // 👈 Filtro por Tag (Ex: ?tag=CESPE)
	searchQuery := c.Query("search")

	query := database.DB.Preload("Creator").Where("space_id = ?", parsedSpaceID)

	if groupID != "" {
		query = query.Where("group_id = ?", groupID)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if subCategory != "" {
		query = query.Where("sub_category = ?", subCategory)
	}
	if tag != "" {
		// Pesquisa dentro do Array JSONB se a tag existe!
		query = query.Where("tags ILIKE ?", "%\""+tag+"\"%")
	}
	if searchQuery != "" {
		query = query.Where("title ILIKE ? OR front ILIKE ?", "%"+searchQuery+"%", "%"+searchQuery+"%")
	}

	var flashcards []models.Flashcard
	query.Order("created_at desc").Find(&flashcards)

	c.JSON(http.StatusOK, gin.H{"flashcards": flashcards})
}

// ==========================================================
// ✏️ 3. EDITAR FLASHCARD
// ==========================================================
func UpdateFlashcard(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	cardIDStr := c.Param("card_id")

	parsedSpaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido."})
		return
	}

	parsedCardID, err := uuid.Parse(cardIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Flashcard inválido."})
		return
	}

	playerID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado."})
		return
	}

	var input CreateFlashcardInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	var space models.Space
	database.DB.Where("id = ?", parsedSpaceID).First(&space)

	var card models.Flashcard
	if err := database.DB.Where("id = ? AND space_id = ?", parsedCardID, parsedSpaceID).First(&card).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Flashcard não encontrado."})
		return
	}

	if card.CreatedByID != playerID && space.OwnerID != playerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Sem permissão para editar."})
		return
	}

	tagsStr := string(input.Tags)
	if len(input.Tags) == 0 || tagsStr == "null" {
		tagsStr = card.Tags // Mantém a antiga se não mandar nova
	}

	if err := database.DB.Model(&card).Updates(map[string]interface{}{
		"title":        input.Title,
		"group_id":     input.GroupID,
		"category":     input.Category,
		"sub_category": input.SubCategory,
		"tags":         tagsStr, // 👈 Atualiza a tag
		"front":        input.Front,
		"back":         input.Back,
		"hint":         input.Hint,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar flashcard."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Flashcard atualizado!"})
}

// ==========================================================
// 🗑️ 4. APAGAR FLASHCARD
// ==========================================================
func DeleteFlashcard(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	cardIDStr := c.Param("card_id")

	parsedSpaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido."})
		return
	}

	parsedCardID, err := uuid.Parse(cardIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Flashcard inválido."})
		return
	}

	playerID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado."})
		return
	}

	var space models.Space
	database.DB.Where("id = ?", parsedSpaceID).First(&space)

	var card models.Flashcard
	if err := database.DB.Where("id = ? AND space_id = ?", parsedCardID, parsedSpaceID).First(&card).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Flashcard não encontrado."})
		return
	}

	if card.CreatedByID != playerID && space.OwnerID != playerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Sem permissão."})
		return
	}

	database.DB.Delete(&card)

	c.JSON(http.StatusOK, gin.H{"message": "Flashcard apagado da turma!"})
}
