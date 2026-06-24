package study

import (
	"encoding/json"
	"net/http"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils" // 👈 Import global adicionado

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
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

// ==========================================================
// 📝 CRIAR SIMULADO E PERGUNTAS
// ==========================================================
func CreateQuiz(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	parsedSpaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido."})
		return
	}

	var input CreateQuizInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados do Quiz inválidos."})
		return
	}

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar o Simulado."}) // 👈 Erro bruto ocultado!
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Simulado criado com sucesso!", "quiz": newQuiz})
}

// ==========================================================
// 📋 LISTAR SIMULADOS DA TURMA
// ==========================================================
func ListSpaceQuizzes(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	parsedSpaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido."})
		return
	}

	var quizzes []models.Quiz
	// O Preload traz as perguntas embutidas no JSON
	database.DB.Preload("Questions").Where("space_id = ?", parsedSpaceID).Find(&quizzes)

	now := time.Now()
	for i := range quizzes {
		// Regra de Trava de Tempo (Simulados agendados)
		if quizzes[i].UnlockAt != nil && quizzes[i].UnlockAt.After(now) {
			quizzes[i].IsLocked = true
			quizzes[i].Questions = []models.QuizQuestion{} // Esconde as perguntas até a data!
		}
	}

	c.JSON(http.StatusOK, gin.H{"quizzes": quizzes})
}

// ==========================================================
// ⚔️ SUBMETER PROVA E CORREÇÃO AUTOMÁTICA
// ==========================================================
func SubmitQuiz(c *gin.Context) {
	quizIDStr := c.Param("quiz_id")
	parsedQuizID, err := uuid.Parse(quizIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Quiz inválido."})
		return
	}

	// 👇 Limpeza do ID aplicada!
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado."})
		return
	}

	var quiz models.Quiz
	if err := database.DB.Preload("Questions").Where("id = ?", parsedQuizID).First(&quiz).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Simulado não encontrado."})
		return
	}

	var input struct {
		Answers map[string]string `json:"answers"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato de respostas inválido."})
		return
	}

	var totalScore float64 = 0
	hasOpenEnded := false

	// Motor de Correção
	for _, question := range quiz.Questions {
		studentAnswer, answered := input.Answers[question.ID.String()]

		if question.QuestionType == "multiple_choice" {
			if answered && studentAnswer == question.CorrectAnswer {
				totalScore += float64(question.Points)
			}
		} else if question.QuestionType == "open_ended" || question.QuestionType == "flashcard_generated" {
			hasOpenEnded = true
		}
	}

	status := "completed"
	if hasOpenEnded {
		status = "pending_review" // Se tem pergunta dissertativa, o professor precisa dar a nota
	}

	result := models.QuizResult{
		QuizID:         quiz.ID,
		UserID:         userID,
		SpaceID:        quiz.SpaceID,
		Score:          totalScore,
		TotalQuestions: len(quiz.Questions),
		Status:         status,
	}
	database.DB.Create(&result)

	c.JSON(http.StatusOK, gin.H{
		"message": "Prova finalizada com sucesso!",
		"result":  result,
	})
}

// ==========================================================
// ✍️ CORREÇÃO MANUAL DO PROFESSOR (Para questões abertas)
// ==========================================================
func GradeQuizManual(c *gin.Context) {
	resultIDStr := c.Param("result_id")
	parsedResultID, err := uuid.Parse(resultIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Resultado inválido."})
		return
	}

	var input struct {
		ExtraPoints float64 `json:"extra_points" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pontuação inválida."})
		return
	}

	var result models.QuizResult
	if err := database.DB.Where("id = ?", parsedResultID).First(&result).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Resultado não encontrado."})
		return
	}

	database.DB.Model(&result).Updates(map[string]interface{}{
		"score":  gorm.Expr("score + ?", input.ExtraPoints),
		"status": "completed",
	})

	c.JSON(http.StatusOK, gin.H{"message": "Nota manual lançada com sucesso!"})
}

// ==========================================================
// 🚨 ALERTA ANTI-COLA
// ==========================================================
func ReportCheatAttempt(c *gin.Context) {
	quizIDStr := c.Param("quiz_id")
	parsedQuizID, err := uuid.Parse(quizIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Quiz inválido."})
		return
	}

	// 👇 Limpeza do ID aplicada!
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado."})
		return
	}

	var quiz models.Quiz
	database.DB.Select("space_id").Where("id = ?", parsedQuizID).First(&quiz)

	cheatLog := models.ActivityLog{
		SpaceID: quiz.SpaceID,
		UserID:  userID,
		Action:  "⚠️ ALERTA ANTI-COLA: O aluno saiu da tela ou trocou de aba durante o Simulado!",
	}
	database.DB.Create(&cheatLog)

	c.JSON(http.StatusOK, gin.H{"message": "Infração registrada silenciosamente."})
}

// ==========================================================
// 🎓 EMISSÃO DE CERTIFICADOS
// ==========================================================
func ClaimCertificate(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	parsedSpaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido."})
		return
	}

	// 👇 Limpeza do ID aplicada!
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado."})
		return
	}

	var existingCert models.Certificate
	if err := database.DB.Where("space_id = ? AND user_id = ?", parsedSpaceID, userID).First(&existingCert).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{
			"message":     "Você já possui este certificado!",
			"certificate": existingCert,
		})
		return
	}

	var results []models.QuizResult
	database.DB.Where("space_id = ? AND user_id = ?", parsedSpaceID, userID).Find(&results)

	if len(results) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Você precisa fazer as provas antes de pedir o certificado."})
		return
	}

	var totalScore float64 = 0
	var pendingExams bool = false

	for _, res := range results {
		if res.Status == "pending_review" {
			pendingExams = true
		}
		totalScore += res.Score
	}

	if pendingExams {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O Professor ainda está corrigindo algumas de suas provas. Aguarde!"})
		return
	}

	average := totalScore / float64(len(results))

	if average < 6.0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "Sua média foi baixa. Estude mais um pouco e refaça os simulados para conseguir o certificado!",
			"sua_media": average,
		})
		return
	}

	newCert := models.Certificate{
		SpaceID:      parsedSpaceID,
		UserID:       userID,
		AverageScore: average,
	}
	database.DB.Create(&newCert)

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Parabéns! Você concluiu o curso com sucesso.",
		"certificate": newCert,
	})
}
