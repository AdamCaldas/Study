package study

import (
	"encoding/json"
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils" // 👈 Import global

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SpaceQuestionInput struct {
	Title         string                `json:"title"`
	Discipline    string                `json:"discipline"`
	Year          int                   `json:"year"`
	QuestionText  string                `json:"question_text" binding:"required"`
	Points        int                   `json:"points"`
	CorrectAnswer string                `json:"correct_answer"`
	GroupID       string                `json:"group_id"`
	Options       []QuestionOptionInput `json:"options"`
	QuestionType  string                `json:"question_type"`
}

// ==========================================================
// ➕ 1. CRIAR QUESTÃO DO ZERO NO SPACE
// ==========================================================
func CreateSpaceQuestion(c *gin.Context) {
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

	var input SpaceQuestionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	optionsBytes, _ := json.Marshal(input.Options)

	newQuestion := models.SpaceQuestion{
		SpaceID:       parsedSpaceID,
		CreatedByID:   userID,
		Title:         input.Title,
		Discipline:    input.Discipline,
		Year:          input.Year,
		QuestionText:  input.QuestionText,
		Points:        input.Points,
		CorrectAnswer: input.CorrectAnswer,
		GroupID:       input.GroupID,
		Options:       string(optionsBytes),
		QuestionType:  input.QuestionType,
		Source:        "CUSTOM",
	}

	if err := database.DB.Create(&newQuestion).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar a questão na turma."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Questão adicionada à turma!", "question": newQuestion})
}

// ==========================================================
// 📋 2. LISTAR QUESTÕES DA TURMA (Com filtro de Editais)
// ==========================================================
func ListSpaceQuestions(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	parsedSpaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido."})
		return
	}

	groupID := c.Query("group_id")

	query := database.DB.Where("space_id = ?", parsedSpaceID)

	if groupID != "" {
		query = query.Where("group_id = ?", groupID)
	}

	var questions []models.SpaceQuestion
	query.Order("created_at DESC").Find(&questions)

	c.JSON(http.StatusOK, gin.H{"questions": questions})
}

// ==========================================================
// 🪄 3. A MÁGICA: CLONAR DO BANCO STUDFY PARA A TURMA
// ==========================================================
func CloneStudfyQuestion(c *gin.Context) {
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

	var input struct {
		StudfyQuestionID uuid.UUID `json:"studfy_question_id" binding:"required"`
		GroupID          string    `json:"group_id"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "É necessário informar o ID da questão oficial."})
		return
	}

	var original models.StudfyQuestion
	if err := database.DB.Where("id = ?", input.StudfyQuestionID).First(&original).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Questão oficial não encontrada."})
		return
	}

	clonedQuestion := models.SpaceQuestion{
		SpaceID:       parsedSpaceID,
		CreatedByID:   userID,
		Title:         original.Title,
		Discipline:    original.Discipline,
		Year:          original.Year,
		QuestionText:  original.QuestionText,
		Points:        original.Points,
		CorrectAnswer: original.CorrectAnswer,
		Options:       original.Options,
		QuestionType:  original.QuestionType,
		GroupID:       input.GroupID,
		Source:        "CLONED",
	}

	if err := database.DB.Create(&clonedQuestion).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao clonar a questão."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Questão clonada com sucesso para a sua turma!", "question": clonedQuestion})
}

// ==========================================================
// ✏️ 4. EDITAR QUESTÃO DA TURMA
// ==========================================================
func UpdateSpaceQuestion(c *gin.Context) {
	questionIDStr := c.Param("question_id")
	spaceIDStr := c.Param("space_id")

	parsedQuestionID, err := uuid.Parse(questionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID da Questão inválido."})
		return
	}

	var input SpaceQuestionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	optionsBytes, _ := json.Marshal(input.Options)

	if err := database.DB.Model(&models.SpaceQuestion{}).Where("id = ? AND space_id = ?", parsedQuestionID, spaceIDStr).Updates(map[string]interface{}{
		"title":          input.Title,
		"discipline":     input.Discipline,
		"year":           input.Year,
		"question_text":  input.QuestionText,
		"points":         input.Points,
		"correct_answer": input.CorrectAnswer,
		"group_id":       input.GroupID,
		"options":        string(optionsBytes),
		"question_type":  input.QuestionType,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar questão."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Questão da turma atualizada!"})
}

// ==========================================================
// 🗑️ 5. APAGAR QUESTÃO DA TURMA
// ==========================================================
func DeleteSpaceQuestion(c *gin.Context) {
	questionIDStr := c.Param("question_id")
	spaceIDStr := c.Param("space_id")

	parsedQuestionID, err := uuid.Parse(questionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID da Questão inválido."})
		return
	}

	if err := database.DB.Where("id = ? AND space_id = ?", parsedQuestionID, spaceIDStr).Delete(&models.SpaceQuestion{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao apagar questão."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Questão apagada do banco da turma!"})
}
