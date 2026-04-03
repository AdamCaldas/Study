package study

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// O Link oficial do GitHub salvo direto no Back-end!
const EnemGitHubURL = "https://raw.githubusercontent.com/yunger7/enem-api/main/public/api/questions.json"

// ==========================================================
// 📥 ESTRUTURAS
// ==========================================================
type EnemAlternative struct {
	Letter    string  `json:"letter"`
	Text      string  `json:"text"`
	File      *string `json:"file"`
	IsCorrect bool    `json:"isCorrect"`
}

type EnemQuestion struct {
	Title        string            `json:"title"`
	Discipline   string            `json:"discipline"`
	Year         int               `json:"year"`
	Context      string            `json:"context"`
	Alternatives []EnemAlternative `json:"alternatives"`
}

// ==========================================================
// 🤖 1. ROBÔ DO BACK-END: PUXAR DIRETO DO GITHUB
// ==========================================================
func FetchEnemFromWeb(c *gin.Context) {
	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	// O Back-end vai sozinho na URL do GitHub!
	resp, err := http.Get(EnemGitHubURL)
	if err != nil || resp.StatusCode != 200 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao baixar as questões do GitHub oficial."})
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	var enemQuestions []EnemQuestion
	if err := json.Unmarshal(bodyBytes, &enemQuestions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O JSON do GitHub mudou de formato ou é incompatível."})
		return
	}

	var newQuestions []models.QuestionBankItem

	for _, q := range enemQuestions {
		correctLetter := ""
		for _, alt := range q.Alternatives {
			if alt.IsCorrect {
				correctLetter = alt.Letter
				break
			}
		}

		optionsBytes, _ := json.Marshal(q.Alternatives)
		questionText := fmt.Sprintf("<b>%s</b><br><br>%s", q.Title, q.Context) // Ano e Matéria agora tem colunas próprias!

		newQuestions = append(newQuestions, models.QuestionBankItem{
			TeacherID:     userID,
			Title:         q.Title,
			Discipline:    q.Discipline,
			Year:          q.Year,
			Source:        "ENEM",
			QuestionText:  questionText,
			QuestionType:  "multiple_choice",
			Options:       string(optionsBytes),
			CorrectAnswer: correctLetter,
			Points:        1,
		})
	}

	// Batch Insert!
	if len(newQuestions) > 0 {
		if err := database.DB.Create(&newQuestions).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar no banco de dados."})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("SUCESSO! %d questões do ENEM foram sincronizadas!", len(newQuestions)),
	})
}

// ==========================================================
// ➕ 2. CRIAR QUESTÃO MANUAL (O Aluno cria do zero)
// ==========================================================
func CreateCustomQuestion(c *gin.Context) {
	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	var input models.QuestionBankItem
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos para criar questão."})
		return
	}

	input.TeacherID = userID
	input.Source = "CUSTOM" // Marca que foi o aluno/professor que fez
	input.QuestionType = "multiple_choice"

	if err := database.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar sua questão no banco."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Questão criada com sucesso!", "question": input})
}

// ==========================================================
// 📋 3. LISTAR TODAS AS QUESTÕES (Com Filtros Pesados)
// ==========================================================
func ListAllQuestions(c *gin.Context) {
	searchQuery := c.Query("search")    // Buscar no texto
	discipline := c.Query("discipline") // Filtro: ?discipline=Matemática
	yearStr := c.Query("year")          // Filtro: ?year=2020
	source := c.Query("source")         // Filtro: ?source=ENEM ou ?source=CUSTOM
	limitStr := c.DefaultQuery("limit", "50")

	limit, _ := strconv.Atoi(limitStr)

	query := database.DB.Model(&models.QuestionBankItem{})

	if searchQuery != "" {
		query = query.Where("question_text ILIKE ?", "%"+searchQuery+"%")
	}
	if discipline != "" {
		query = query.Where("discipline = ?", discipline)
	}
	if yearStr != "" {
		year, _ := strconv.Atoi(yearStr)
		query = query.Where("year = ?", year)
	}
	if source != "" {
		query = query.Where("source = ?", source)
	}

	var questions []models.QuestionBankItem
	query.Order("created_at DESC").Limit(limit).Find(&questions)

	c.JSON(http.StatusOK, gin.H{"questions": questions})
}

// ==========================================================
// ✏️ 4. EDITAR UMA QUESTÃO ESPECÍFICA
// ==========================================================
func UpdateQuestion(c *gin.Context) {
	questionID := c.Param("id")

	var input models.QuestionBankItem
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	if err := database.DB.Model(&models.QuestionBankItem{}).Where("id = ?", questionID).Updates(map[string]interface{}{
		"title":          input.Title,
		"discipline":     input.Discipline,
		"year":           input.Year,
		"question_text":  input.QuestionText,
		"options":        input.Options,
		"correct_answer": input.CorrectAnswer,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar questão."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Questão atualizada com sucesso!"})
}

// ==========================================================
// 🗑️ 5. APAGAR UMA QUESTÃO ESPECÍFICA
// ==========================================================
func DeleteQuestion(c *gin.Context) {
	questionID := c.Param("id")

	if err := database.DB.Where("id = ?", questionID).Delete(&models.QuestionBankItem{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao deletar questão."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Questão apagada para sempre!"})
}
