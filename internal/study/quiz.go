package study

import (
	"encoding/json"
	"fmt"
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"time"

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

// ==========================================================
// 🏦 SALVAR QUESTÃO NO BANCO GLOBAL (Fase 3)
// ==========================================================
func SaveToQuestionBank(c *gin.Context) {
	teacherIDInterface, _ := c.Get("userID")
	var teacherID uuid.UUID
	switch v := teacherIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	var teacher models.User
	if err := database.DB.Where("id = ? AND account_type = 'TEACHER'", teacherID).First(&teacher).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas professores podem ter um Banco de Questões."})
		return
	}

	var input models.QuestionBankItem
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados da questão inválidos."})
		return
	}

	input.TeacherID = teacherID

	if err := database.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar no Banco de Questões."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Questão salva no seu Banco Global!",
		"question": input,
	})
}

// ==========================================================
// 🏦 LISTAR BANCO DE QUESTÕES DO PROFESSOR (Fase 3)
// ==========================================================
func GetMyQuestionBank(c *gin.Context) {
	teacherIDInterface, _ := c.Get("userID")
	var teacherID uuid.UUID
	switch v := teacherIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	var questions []models.QuestionBankItem
	if err := database.DB.Where("teacher_id = ?", teacherID).Order("created_at desc").Find(&questions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar banco de questões."})
		return
	}

	if questions == nil {
		questions = []models.QuestionBankItem{}
	}

	c.JSON(http.StatusOK, gin.H{"questions": questions})
}

// ==========================================================
// ⚔️ FASE 4: SUBMETER PROVA E CORREÇÃO AUTOMÁTICA
// ==========================================================
func SubmitQuiz(c *gin.Context) {
	quizID := c.Param("quiz_id")
	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	var quiz models.Quiz
	if err := database.DB.Preload("Questions").Where("id = ?", quizID).First(&quiz).Error; err != nil {
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

	for _, question := range quiz.Questions {
		studentAnswer, answered := input.Answers[question.ID.String()]

		if question.QuestionType == "multiple_choice" {
			if answered && studentAnswer == question.CorrectAnswer {
				totalScore += float64(question.Points)
			}
		} else if question.QuestionType == "open_ended" {
			hasOpenEnded = true
		}
	}

	status := "completed"
	if hasOpenEnded {
		status = "pending_review"
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

func GradeQuizManual(c *gin.Context) {
	resultID := c.Param("result_id")

	var input struct {
		ExtraPoints float64 `json:"extra_points" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pontuação inválida."})
		return
	}

	var result models.QuizResult
	if err := database.DB.Where("id = ?", resultID).First(&result).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Resultado não encontrado."})
		return
	}

	database.DB.Model(&result).Updates(map[string]interface{}{
		"score":  gorm.Expr("score + ?", input.ExtraPoints),
		"status": "completed",
	})

	c.JSON(http.StatusOK, gin.H{"message": "Nota manual lançada com sucesso!"})
}

func ReportCheatAttempt(c *gin.Context) {
	quizID := c.Param("quiz_id")
	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	var quiz models.Quiz
	database.DB.Select("space_id").Where("id = ?", quizID).First(&quiz)

	cheatLog := models.ActivityLog{
		SpaceID: quiz.SpaceID,
		UserID:  userID,
		Action:  "⚠️ ALERTA ANTI-COLA: O aluno saiu da tela ou trocou de aba durante o Simulado!",
	}
	database.DB.Create(&cheatLog)

	c.JSON(http.StatusOK, gin.H{"message": "Infração registrada silenciosamente."})
}

func ClaimCertificate(c *gin.Context) {
	spaceID := c.Param("space_id")
	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	var existingCert models.Certificate
	if err := database.DB.Where("space_id = ? AND user_id = ?", spaceID, userID).First(&existingCert).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{
			"message":     "Você já possui este certificado!",
			"certificate": existingCert,
		})
		return
	}

	var results []models.QuizResult
	database.DB.Where("space_id = ? AND user_id = ?", spaceID, userID).Find(&results)

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
		SpaceID:      uuid.MustParse(spaceID),
		UserID:       userID,
		AverageScore: average,
	}
	database.DB.Create(&newCert)

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Parabéns! Você concluiu o curso com sucesso.",
		"certificate": newCert,
	})
}

func ListSpaceQuizzes(c *gin.Context) {
	spaceID := c.Param("space_id")

	var quizzes []models.Quiz
	database.DB.Preload("Questions").Where("space_id = ?", spaceID).Find(&quizzes)

	now := time.Now()
	for i := range quizzes {
		if quizzes[i].UnlockAt != nil && quizzes[i].UnlockAt.After(now) {
			quizzes[i].IsLocked = true
			quizzes[i].Questions = []models.QuizQuestion{}
		}
	}

	c.JSON(http.StatusOK, gin.H{"quizzes": quizzes})
}

// ==========================================================
// 📥 FASE 5: IMPORTAR E PUXAR QUESTÕES DO ENEM (Via JSON do Yunger7)
// ==========================================================

// Estruturas de espelhamento do repositório ENEM API
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
// 📥 FASE 5: IMPORTAR E PUXAR QUESTÕES DO ENEM (Via JSON do Yunger7)
// ==========================================================

// 🚀 1. Sincronizador de Questões (Insere JSON no banco - ALUNOS E PROFS)
func ImportEnemQuestions(c *gin.Context) {
	// Pega o ID de quem está logado (Aluno, Professor ou Admin)
	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	// O front-end envia um objeto com "questions": [ { ...questões do enem... } ]
	var input struct {
		Questions []EnemQuestion `json:"questions" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato JSON inválido. Verifique a estrutura.", "details": err.Error()})
		return
	}

	var newQuestions []models.QuestionBankItem

	for _, q := range input.Questions {
		// Acha a letra da resposta correta no JSON
		correctLetter := ""
		for _, alt := range q.Alternatives {
			if alt.IsCorrect {
				correctLetter = alt.Letter
				break
			}
		}

		optionsBytes, _ := json.Marshal(q.Alternatives)

		// Junta o título da questão e o contexto na mesma string formatada com HTML
		questionText := fmt.Sprintf("<b>%s (%d) - %s</b><br><br>%s", q.Title, q.Year, q.Discipline, q.Context)

		newQuestions = append(newQuestions, models.QuestionBankItem{
			TeacherID:     userID, // 👈 Salvamos o ID do Aluno ou Professor aqui como o "Dono" do Upload!
			QuestionText:  questionText,
			QuestionType:  "multiple_choice",
			Options:       string(optionsBytes),
			CorrectAnswer: correctLetter,
			Points:        1,
		})
	}

	// Batch Insert: Salva dezenas/centenas de questões de uma vez
	if len(newQuestions) > 0 {
		if err := database.DB.Create(&newQuestions).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar as questões no banco."})
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": fmt.Sprintf("%d questões do ENEM importadas com sucesso!", len(newQuestions)),
	})
}

// 🚀 2. Rota para o Front-end consumir as questões nos Desafios/Estudos
func GetPublicQuestions(c *gin.Context) {
	var questions []models.QuestionBankItem

	// Buscamos 30 questões de forma aleatória para o Front montar a bateria do aluno
	if err := database.DB.Order("RANDOM()").Limit(30).Find(&questions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar questões."})
		return
	}

	if questions == nil {
		questions = []models.QuestionBankItem{}
	}

	c.JSON(http.StatusOK, gin.H{"questions": questions})
}
