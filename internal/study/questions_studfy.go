package study

import (
	"encoding/json"
	"net/http"
	"strconv"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
)

// Estrutura do JSON de Opções (Usada tanto aqui quanto no banco do Space)
type QuestionOptionInput struct {
	Letter    string `json:"letter"`
	Text      string `json:"text"`
	IsCorrect bool   `json:"isCorrect"`
}

// O que o Painel Admin manda para criar/editar a questão oficial
type StudfyQuestionInput struct {
	Title         string                `json:"title"`
	Discipline    string                `json:"discipline"`
	Year          int                   `json:"year"`
	QuestionText  string                `json:"question_text" binding:"required"`
	Points        int                   `json:"points"`
	CorrectAnswer string                `json:"correct_answer"`
	Options       []QuestionOptionInput `json:"options"`
	QuestionType  string                `json:"question_type"`
}

// ==========================================================
// 🎓 1. LISTAR QUESTÕES OFICIAIS (Visão App / Alunos)
// ==========================================================
func ListStudfyQuestions(c *gin.Context) {
	searchQuery := c.Query("search")
	discipline := c.Query("discipline")
	yearStr := c.Query("year")
	limitStr := c.DefaultQuery("limit", "50") // Traz 50 por vez pra não travar

	limit, _ := strconv.Atoi(limitStr)

	query := database.DB.Model(&models.StudfyQuestion{})

	if searchQuery != "" {
		query = query.Where("title ILIKE ? OR question_text ILIKE ?", "%"+searchQuery+"%", "%"+searchQuery+"%")
	}
	if discipline != "" {
		query = query.Where("discipline = ?", discipline)
	}
	if yearStr != "" {
		year, _ := strconv.Atoi(yearStr)
		query = query.Where("year = ?", year)
	}

	var questions []models.StudfyQuestion
	query.Order("created_at DESC").Limit(limit).Find(&questions)

	c.JSON(http.StatusOK, gin.H{"questions": questions})
}

// ==========================================================
// ⚡ 2. MODO DEUS: CRIAR QUESTÃO OFICIAL
// ==========================================================
func AdminCreateStudfyQuestion(c *gin.Context) {
	var input StudfyQuestionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	// Converte o Array do Front-end pra String JSON do Banco
	optionsBytes, _ := json.Marshal(input.Options)

	newQ := models.StudfyQuestion{
		Title:         input.Title,
		Discipline:    input.Discipline,
		Year:          input.Year,
		Source:        "STUDFY",
		QuestionText:  input.QuestionText,
		Points:        input.Points,
		CorrectAnswer: input.CorrectAnswer,
		Options:       string(optionsBytes),
		QuestionType:  input.QuestionType,
	}

	if err := database.DB.Create(&newQ).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar questão global."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Questão oficial adicionada com sucesso!", "question": newQ})
}

// ==========================================================
// ⚡ 3. MODO DEUS: EDITAR QUESTÃO OFICIAL
// ==========================================================
func AdminUpdateStudfyQuestion(c *gin.Context) {
	questionID := c.Param("id")
	var input StudfyQuestionInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	optionsBytes, _ := json.Marshal(input.Options)

	if err := database.DB.Model(&models.StudfyQuestion{}).Where("id = ?", questionID).Updates(map[string]interface{}{
		"title":          input.Title,
		"discipline":     input.Discipline,
		"year":           input.Year,
		"question_text":  input.QuestionText,
		"points":         input.Points,
		"correct_answer": input.CorrectAnswer,
		"options":        string(optionsBytes),
		"question_type":  input.QuestionType,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar questão oficial."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Questão oficial atualizada!"})
}

// ==========================================================
// ⚡ 4. MODO DEUS: APAGAR QUESTÃO OFICIAL
// ==========================================================
func AdminDeleteStudfyQuestion(c *gin.Context) {
	questionID := c.Param("id")
	if err := database.DB.Where("id = ?", questionID).Delete(&models.StudfyQuestion{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao apagar questão."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Questão oficial apagada para sempre!"})
}
