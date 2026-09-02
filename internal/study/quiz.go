package study

import (
	"encoding/json"
	"math"
	"net/http"
	"time"

	"studfy-backend/internal/activity"
	"studfy-backend/internal/auth"
	"studfy-backend/internal/models"
	"studfy-backend/internal/space"
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

	// 📄 Paginado: antes esta rota devolvia TODOS os simulados da turma com TODAS
	// as questões de cada um embutidas. Uma turma com 40 provas de 50 questões
	// mandava 2 mil registros para cada aluno que abrisse a aba.
	p := utils.GetPage(c)

	base := database.DB.Model(&models.Quiz{}).Where("space_id = ?", parsedSpaceID)

	var total int64
	base.Count(&total)

	// `with_questions=true` para quem realmente precisa do conteúdo (responder a
	// prova). A listagem normal traz só a capa: título, descrição e o total.
	withQuestions := c.Query("with_questions") == "true"

	query := database.DB.Where("space_id = ?", parsedSpaceID)
	if withQuestions {
		query = query.Preload("Questions")
	}

	var quizzes []models.Quiz
	query.Order("created_at desc").
		Offset(p.Offset()).
		Limit(p.Limit).
		Find(&quizzes)

	// Conta as questões de cada simulado numa única consulta (evita N+1).
	counts := map[uuid.UUID]int{}
	if len(quizzes) > 0 {
		ids := make([]uuid.UUID, 0, len(quizzes))
		for _, q := range quizzes {
			ids = append(ids, q.ID)
		}
		var rows []struct {
			QuizID uuid.UUID
			Total  int
		}
		database.DB.Model(&models.QuizQuestion{}).
			Select("quiz_id, COUNT(*) as total").
			Where("quiz_id IN ?", ids).
			Group("quiz_id").
			Scan(&rows)
		for _, r := range rows {
			counts[r.QuizID] = r.Total
		}
	}

	// 🔒 O gabarito só pode sair para quem corrige a prova. Antes, qualquer aluno
	// que listasse os simulados recebia `correct_answer` de todas as questões.
	isStaff := auth.CanSeeQuizAnswers(c)

	now := time.Now()
	type quizResponse struct {
		models.Quiz
		QuestionCount int `json:"question_count"`
	}
	response := make([]quizResponse, 0, len(quizzes))

	for i := range quizzes {
		// Regra de Trava de Tempo (Simulados agendados)
		if quizzes[i].UnlockAt != nil && quizzes[i].UnlockAt.After(now) {
			quizzes[i].IsLocked = true
			quizzes[i].Questions = []models.QuizQuestion{} // Esconde as perguntas até a data!
		}
		if !isStaff {
			for j := range quizzes[i].Questions {
				quizzes[i].Questions[j].CorrectAnswer = ""
			}
		}
		response = append(response, quizResponse{
			Quiz:          quizzes[i],
			QuestionCount: counts[quizzes[i].ID],
		})
	}

	c.JSON(http.StatusOK, gin.H{"quizzes": response, "pagination": p.Meta(total)})
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

	// 🛡️ O simulado tem que ser desta turma (evita responder prova de outro Space).
	if spaceIDStr := c.Param("space_id"); spaceIDStr != "" {
		if parsedSpaceID, err := uuid.Parse(spaceIDStr); err == nil && quiz.SpaceID != parsedSpaceID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Este simulado não pertence a esta turma."})
			return
		}
	}

	// 🔒 Trava de agendamento: não dá pra responder antes de liberar.
	if quiz.UnlockAt != nil && quiz.UnlockAt.After(time.Now()) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Este simulado ainda não foi liberado."})
		return
	}

	// 🚫 Sem reenvio: uma tentativa por aluno (impede farm de nota/certificado).
	var jaFez int64
	database.DB.Model(&models.QuizResult{}).Where("quiz_id = ? AND user_id = ?", quiz.ID, userID).Count(&jaFez)
	if jaFez > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Você já enviou este simulado."})
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

		switch question.QuestionType {
		case "multiple_choice":
			if answered && studentAnswer == question.CorrectAnswer {
				totalScore += float64(question.Points)
			}
		case "open_ended", "flashcard_generated":
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
	if err := database.DB.Create(&result).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao registrar o resultado da prova."})
		return
	}

	// 📜 Registra no histórico da turma (o professor vê quem entregou e quando).
	activity.Log(quiz.SpaceID, userID, "entregou o simulado \""+quiz.Title+"\"")

	// 🤖 Dispara as automações do professor (ex.: liberar caderno de reforço para
	// quem foi mal). O motor existia pronto no código mas NUNCA era chamado:
	// o professor criava a regra e nada acontecia. Só faz sentido quando a nota
	// já é final — provas com questão dissertativa esperam a correção.
	if status == "completed" {
		space.RunAutomationEngine(quiz.SpaceID, userID, totalScore)
	}

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

	// extra_points NÃO é "required" (0 é uma nota válida para questão errada).
	var input struct {
		ExtraPoints float64 `json:"extra_points"`
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

	// 🛡️ O resultado tem que ser desta turma (o guarda de rota já exige cargo,
	// mas isto impede corrigir prova de outra turma via result_id solto).
	if spaceIDStr := c.Param("space_id"); spaceIDStr != "" {
		if parsedSpaceID, err := uuid.Parse(spaceIDStr); err == nil && result.SpaceID != parsedSpaceID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Este resultado não pertence a esta turma."})
			return
		}
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

// notaMinimaCertificado é o corte para emitir o certificado, na escala 0..10.
const notaMinimaCertificado = 6.0

// mediaDoAluno devolve a média das provas concluídas na escala 0..10.
//
// Cada prova é convertida em aproveitamento — pontos obtidos ÷ pontos
// possíveis — para que provas de tamanhos diferentes pesem igual. Sem isso, o
// corte comparava pontos brutos e o resultado dependia de quantas questões a
// prova tinha, não de quanto o aluno acertou.
func mediaDoAluno(spaceID, userID uuid.UUID) (float64, error) {
	var linhas []struct {
		Score       float64
		TotalPoints float64
	}

	err := database.DB.Raw(`
		SELECT qr.score AS score,
		       COALESCE(pts.total_points, 0) AS total_points
		FROM quiz_results qr
		LEFT JOIN (
			SELECT quiz_id, SUM(points) AS total_points
			FROM quiz_questions
			GROUP BY quiz_id
		) pts ON pts.quiz_id = qr.quiz_id
		WHERE qr.space_id = ? AND qr.user_id = ? AND qr.status = 'completed'
	`, spaceID, userID).Scan(&linhas).Error
	if err != nil {
		return 0, err
	}
	if len(linhas) == 0 {
		return 0, nil
	}

	var soma float64
	var contadas int
	for _, l := range linhas {
		if l.TotalPoints <= 0 {
			continue // prova sem questões pontuadas não entra na média
		}
		aproveitamento := l.Score / l.TotalPoints
		if aproveitamento > 1 {
			aproveitamento = 1 // pontos extras do professor não passam de 100%
		}
		if aproveitamento < 0 {
			aproveitamento = 0
		}
		soma += aproveitamento * 10
		contadas++
	}
	if contadas == 0 {
		return 0, nil
	}
	return soma / float64(contadas), nil
}

// arredonda deixa a nota com uma casa decimal, para mostrar na tela.
func arredonda(n float64) float64 {
	return math.Round(n*10) / 10
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

	pendingExams := false
	for _, res := range results {
		if res.Status == "pending_review" {
			pendingExams = true
		}
	}
	if pendingExams {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O Professor ainda está corrigindo algumas de suas provas. Aguarde!"})
		return
	}

	// 📊 A média precisa ser em NOTA (0 a 10), não em pontos brutos.
	//
	// Antes o corte de 6.0 era comparado com a soma de pontos: uma prova de 20
	// questões valia 20 pontos e passava fácil, enquanto uma prova de 5 questões
	// NUNCA passava — nem gabaritando, porque 5 < 6. O aluno era reprovado pelo
	// tamanho da prova, não pelo desempenho.
	//
	// Agora cada prova vira aproveitamento (acertos ÷ total possível) e a média
	// é a média desses aproveitamentos, na escala 0..10.
	media, err := mediaDoAluno(parsedSpaceID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao calcular a sua média."})
		return
	}

	if media < notaMinimaCertificado {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "Sua média foi baixa. Estude mais um pouco e refaça os simulados para conseguir o certificado!",
			"sua_media":   arredonda(media),
			"nota_minima": notaMinimaCertificado,
		})
		return
	}
	average := media

	newCert := models.Certificate{
		SpaceID:      parsedSpaceID,
		UserID:       userID,
		AverageScore: average,
	}
	// O índice UNIQUE (space_id, user_id) impede dois certificados para o mesmo
	// aluno na mesma turma quando ele clica duas vezes seguidas.
	if err := database.DB.Create(&newCert).Error; err != nil {
		var existing models.Certificate
		if database.DB.Where("space_id = ? AND user_id = ?", parsedSpaceID, userID).First(&existing).Error == nil {
			c.JSON(http.StatusOK, gin.H{"message": "Você já possui este certificado!", "certificate": existing})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao emitir o certificado."})
		return
	}

	activity.Log(parsedSpaceID, userID, "conquistou o certificado da turma")

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Parabéns! Você concluiu o curso com sucesso.",
		"certificate": newCert,
	})
}
