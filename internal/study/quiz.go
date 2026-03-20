package study

import (
	"encoding/json"
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

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
	// 1. Pega o ID de quem está criando (Tem que ser Professor)
	teacherIDInterface, _ := c.Get("userID")
	var teacherID uuid.UUID
	switch v := teacherIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	// 2. Verifica se ele realmente é um TEACHER
	var teacher models.User
	if err := database.DB.Where("id = ? AND account_type = 'TEACHER'", teacherID).First(&teacher).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas professores podem ter um Banco de Questões."})
		return
	}

	// 3. Recebe os dados da questão
	var input models.QuestionBankItem
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados da questão inválidos."})
		return
	}

	input.TeacherID = teacherID

	// 4. Salva no banco pessoal dele
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
	// Busca todas as questões criadas por este professor
	if err := database.DB.Where("teacher_id = ?", teacherID).Order("created_at desc").Find(&questions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar banco de questões."})
		return
	}

	// Se for null, devolve array vazio pro Front não quebrar
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

	// 1. Puxa a Prova e as Perguntas Originais do Banco
	var quiz models.Quiz
	if err := database.DB.Preload("Questions").Where("id = ?", quizID).First(&quiz).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Simulado não encontrado."})
		return
	}

	// 2. Recebe as respostas do Aluno
	var input struct {
		Answers map[string]string `json:"answers"` // Map: "ID_DA_PERGUNTA": "A) 2"
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato de respostas inválido."})
		return
	}

	// 3. O Robô Corretor (Sistema Híbrido)
	var totalScore float64 = 0
	hasOpenEnded := false // Flag para saber se o professor precisa corrigir manualmente depois

	for _, question := range quiz.Questions {
		studentAnswer, answered := input.Answers[question.ID.String()]

		if question.QuestionType == "multiple_choice" {
			// Correção Automática
			if answered && studentAnswer == question.CorrectAnswer {
				totalScore += float64(question.Points)
			}
		} else if question.QuestionType == "open_ended" {
			// Questão Dissertativa: O robô não corrige. Aciona a flag pro Professor!
			hasOpenEnded = true
		}
	}

	// 4. Define o Status do Resultado
	status := "completed"
	if hasOpenEnded {
		status = "pending_review" // Aguardando o professor
	}

	// 5. Salva o Boletim do Aluno
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
// ⚔️ FASE 4: PROFESSOR LANÇA A NOTA MANUAL (Híbrido)
// ==========================================================
func GradeQuizManual(c *gin.Context) {
	resultID := c.Param("result_id")

	// Só pega a nota extra que o professor quer adicionar
	var input struct {
		ExtraPoints float64 `json:"extra_points" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pontuação inválida."})
		return
	}

	// Busca o boletim pendente
	var result models.QuizResult
	if err := database.DB.Where("id = ?", resultID).First(&result).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Resultado não encontrado."})
		return
	}

	// Atualiza com a nota do professor e fecha a prova
	database.DB.Model(&result).Updates(map[string]interface{}{
		"score":  gorm.Expr("score + ?", input.ExtraPoints),
		"status": "completed",
	})

	c.JSON(http.StatusOK, gin.H{"message": "Nota manual lançada com sucesso!"})
}

// ==========================================================
// 🚨 FASE 4: SISTEMA ANTI-COLA (Pega no flagra)
// ==========================================================
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

	// O Front-end manda um POST silencioso pra cá quando a aba do navegador perde o foco.
	// Nós salvamos direto no ActivityLog para o Professor ver depois no Painel!
	cheatLog := models.ActivityLog{
		SpaceID: quiz.SpaceID,
		UserID:  userID,
		Action:  "⚠️ ALERTA ANTI-COLA: O aluno saiu da tela ou trocou de aba durante o Simulado!",
	}
	database.DB.Create(&cheatLog)

	c.JSON(http.StatusOK, gin.H{"message": "Infração registrada silenciosamente."})
}

// ==========================================================
// 🎓 FASE 4: EMITIR CERTIFICADO DE CONCLUSÃO (Formatura)
// ==========================================================
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

	// 1. Verifica se o usuário já tem o certificado desse Space (para não gerar 2x)
	var existingCert models.Certificate
	if err := database.DB.Where("space_id = ? AND user_id = ?", spaceID, userID).First(&existingCert).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{
			"message":     "Você já possui este certificado!",
			"certificate": existingCert,
		})
		return
	}

	// 2. Busca todas as notas (Boletim) do aluno neste Space
	var results []models.QuizResult
	database.DB.Where("space_id = ? AND user_id = ?", spaceID, userID).Find(&results)

	if len(results) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Você precisa fazer as provas antes de pedir o certificado."})
		return
	}

	// 3. Calcula a Média Geral do Aluno
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

	// 4. REGRA DE NEGÓCIO: Média Mínima para aprovação (Ex: 6.0 ou 60%)
	// Supondo que a prova vale 10, a média para passar é 6.
	if average < 6.0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "Sua média foi baixa. Estude mais um pouco e refaça os simulados para conseguir o certificado!",
			"sua_media": average,
		})
		return
	}

	// 5. O Aluno passou! Gera o Diploma oficial no banco.
	newCert := models.Certificate{
		SpaceID:      uuid.MustParse(spaceID),
		UserID:       userID,
		AverageScore: average,
	}
	database.DB.Create(&newCert)

	// O Front-end pega esse JSON de sucesso e mostra a tela de confetes com o botão "Baixar PDF"
	c.JSON(http.StatusCreated, gin.H{
		"message":     "Parabéns! Você concluiu o curso com sucesso.",
		"certificate": newCert,
	})
}
