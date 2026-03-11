package study

import (
	"encoding/json"
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Estrutura que o Front-end envia para criar a prova inteira de uma vez
type CreateQuizInput struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Questions   []struct {
		QuestionText  string          `json:"question_text" binding:"required"`
		QuestionType  string          `json:"question_type" binding:"required"`
		Options       json.RawMessage `json:"options"` // Recebe um array literal JSON do front
		CorrectAnswer string          `json:"correct_answer"`
		Points        int             `json:"points"`
	} `json:"questions"`
}

// CreateQuiz - Cria o simulado e as perguntas
func CreateQuiz(c *gin.Context) {
	spaceID := c.Param("space_id")
	var input CreateQuizInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados do Quiz inválidos"})
		return
	}

	parsedSpaceID, _ := uuid.Parse(spaceID)

	// Monta o Quiz
	newQuiz := models.Quiz{
		SpaceID:     parsedSpaceID,
		Title:       input.Title,
		Description: input.Description,
	}

	// Monta as perguntas
	for _, qInput := range input.Questions {
		optionsStr := string(qInput.Options)
		if len(qInput.Options) == 0 {
			optionsStr = "[]" // Se for texto livre, salva um array vazio
		}

		newQuiz.Questions = append(newQuiz.Questions, models.QuizQuestion{
			QuestionText:  qInput.QuestionText,
			QuestionType:  qInput.QuestionType,
			Options:       optionsStr,
			CorrectAnswer: qInput.CorrectAnswer,
			Points:        qInput.Points,
		})
	}

	// Salva TUDO no banco de dados de uma vez (O GORM é inteligente e salva as relações)
	if err := database.DB.Create(&newQuiz).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar o Quiz", "detalhe": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Simulado criado com sucesso!", "quiz": newQuiz})
}

// ListQuizzes - Lista os simulados do Space para o aluno responder
func ListQuizzes(c *gin.Context) {
	spaceID := c.Param("space_id")
	var quizzes []models.Quiz

	// O Preload("Questions") já traz as perguntas embutidas no JSON!
	if err := database.DB.Preload("Questions").Where("space_id = ?", spaceID).Find(&quizzes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar simulados"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"quizzes": quizzes})
}
