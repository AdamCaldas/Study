package gamification

import (
	"encoding/json"
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==========================================================
// ⚔️ 1. CRIAR DESAFIO (Lançar a Luva)
// ==========================================================
func CreateArenaMatch(c *gin.Context) {
	spaceID := c.Param("space_id")
	userIDInterface, _ := c.Get("userID")

	var challengerID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		challengerID = v
	case string:
		challengerID, _ = uuid.Parse(v)
	}

	var input struct {
		OpponentID     *uuid.UUID `json:"opponent_id"`     // Se mandar Null, é "Aberto pra quem quiser"
		TotalQuestions int        `json:"total_questions"` // Máximo 50
		TimeLimitMin   int        `json:"time_limit_min"`  // Máximo 120
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	// Regras de Limite
	if input.TotalQuestions <= 0 || input.TotalQuestions > 50 {
		input.TotalQuestions = 20
	}
	if input.TimeLimitMin <= 0 || input.TimeLimitMin > 120 {
		input.TimeLimitMin = 30
	}

	// Sorteia as questões do Banco Global! (A mágica acontece aqui)
	var randomQuestions []models.QuestionBankItem
	database.DB.Order("RANDOM()").Limit(input.TotalQuestions).Find(&randomQuestions)

	if len(randomQuestions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O Banco de Questões está vazio! Importe as questões do ENEM primeiro."})
		return
	}

	// Extrai só os IDs e converte pra JSON String
	var qIDs []uuid.UUID
	for _, q := range randomQuestions {
		qIDs = append(qIDs, q.ID)
	}
	qIDsBytes, _ := json.Marshal(qIDs)

	// Cria a Mesa do Duelo
	match := models.ArenaMatch{
		SpaceID:        uuid.MustParse(spaceID),
		ChallengerID:   challengerID,
		OpponentID:     input.OpponentID,
		Status:         "pending",
		QuestionIDs:    string(qIDsBytes),
		TotalQuestions: len(qIDs),
		TimeLimitMin:   input.TimeLimitMin,
	}
	database.DB.Create(&match)

	c.JSON(http.StatusCreated, gin.H{"message": "Desafio lançado com sucesso!", "match": match})
}

// ==========================================================
// 🤝 2. ACEITAR DESAFIO (Entrar na Arena)
// ==========================================================
func AcceptArenaMatch(c *gin.Context) {
	matchID := c.Param("match_id")
	userIDInterface, _ := c.Get("userID")
	var playerID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		playerID = v
	case string:
		playerID, _ = uuid.Parse(v)
	}

	var match models.ArenaMatch
	if err := database.DB.Where("id = ?", matchID).First(&match).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Desafio não encontrado."})
		return
	}

	if match.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Este desafio já está em andamento ou foi finalizado."})
		return
	}

	// Se era um desafio fechado pra alguém específico, verifica se é ele mesmo
	if match.OpponentID != nil && *match.OpponentID != playerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Este desafio foi enviado para outro aluno."})
		return
	}

	// Oponente confirmou presença! A luta começou.
	match.OpponentID = &playerID
	match.Status = "active"
	database.DB.Save(&match)

	c.JSON(http.StatusOK, gin.H{"message": "Desafio aceito! Que vença o melhor."})
}

// ==========================================================
// 🎲 3. PUXAR A PROVA DO DESAFIO (Com Anti-Cheat)
// ==========================================================
func GetArenaQuestions(c *gin.Context) {
	matchID := c.Param("match_id")

	var match models.ArenaMatch
	if err := database.DB.Where("id = ?", matchID).First(&match).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Desafio não encontrado."})
		return
	}

	var qIDs []uuid.UUID
	json.Unmarshal([]byte(match.QuestionIDs), &qIDs)

	var questions []models.QuestionBankItem
	database.DB.Where("id IN ?", qIDs).Find(&questions)

	// ANTI-CHEAT: Vamos limpar a resposta correta antes de mandar pro Front-end!
	for i := range questions {
		questions[i].CorrectAnswer = "SECRET"
	}

	c.JSON(http.StatusOK, gin.H{
		"match_info": gin.H{
			"total_questions": match.TotalQuestions,
			"time_limit_min":  match.TimeLimitMin,
		},
		"questions": questions,
	})
}

// ==========================================================
// ⚖️ 4. ENVIAR RESPOSTAS E O JUIZ DECLARAR VENCEDOR
// ==========================================================
func SubmitArenaMatch(c *gin.Context) {
	matchID := c.Param("match_id")
	userIDInterface, _ := c.Get("userID")
	var playerID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		playerID = v
	case string:
		playerID, _ = uuid.Parse(v)
	}

	var match models.ArenaMatch
	if err := database.DB.Where("id = ?", matchID).First(&match).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Desafio não encontrado."})
		return
	}

	var input struct {
		Answers          map[string]string `json:"answers"`            // Ex: "uuid-questao": "a"
		TimeTakenSeconds int               `json:"time_taken_seconds"` // Tempo gasto no relógio
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	// 1. Puxa as questões originais pra corrigir (com o gabarito de verdade)
	var qIDs []uuid.UUID
	json.Unmarshal([]byte(match.QuestionIDs), &qIDs)
	var questions []models.QuestionBankItem
	database.DB.Where("id IN ?", qIDs).Find(&questions)

	// 2. Corrige a prova
	correctCount := 0
	for _, q := range questions {
		if studentAns, ok := input.Answers[q.ID.String()]; ok {
			if studentAns == q.CorrectAnswer {
				correctCount++
			}
		}
	}

	// 3. Salva os resultados pra quem enviou (Desafiante ou Oponente)
	isChallenger := playerID == match.ChallengerID

	if isChallenger {
		match.ChallengerScore = &correctCount
		match.ChallengerTime = &input.TimeTakenSeconds
	} else if match.OpponentID != nil && playerID == *match.OpponentID {
		match.OpponentScore = &correctCount
		match.OpponentTime = &input.TimeTakenSeconds
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Você não faz parte deste desafio."})
		return
	}

	// 4. O JUIZ ENTRA EM AÇÃO: Os dois terminaram?
	if match.ChallengerScore != nil && match.OpponentScore != nil {
		match.Status = "completed"

		// Regra de Vitória
		if *match.ChallengerScore > *match.OpponentScore {
			match.WinnerID = &match.ChallengerID
		} else if *match.OpponentScore > *match.ChallengerScore {
			match.WinnerID = match.OpponentID
		} else {
			// Empate nos pontos! Quem foi mais rápido?
			if *match.ChallengerTime < *match.OpponentTime {
				match.WinnerID = &match.ChallengerID
			} else if *match.OpponentTime < *match.ChallengerTime {
				match.WinnerID = match.OpponentID
			}
			// Se o tempo for EXATAMENTE igual (raríssimo), fica null (Empate Técnico)
		}

		// (Opcional) Aqui você pode dar XP extra pro WinnerID chamando a sua regra de Gamificação!
	}

	database.DB.Save(&match)

	// Resposta pro Front
	if match.Status == "completed" {
		c.JSON(http.StatusOK, gin.H{
			"message":    "Partida Finalizada!",
			"status":     "completed",
			"your_score": correctCount,
			"winner_id":  match.WinnerID,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"message":    "Suas respostas foram salvas. Aguardando o oponente terminar!",
			"status":     "waiting_opponent",
			"your_score": correctCount,
		})
	}
}

// ==========================================================
// 📜 5. LISTAR HISTÓRICO DE DESAFIOS DO SPACE
// ==========================================================
func ListArenaMatches(c *gin.Context) {
	spaceID := c.Param("space_id")

	var matches []models.ArenaMatch
	database.DB.Where("space_id = ?", spaceID).Order("created_at desc").Find(&matches)

	c.JSON(http.StatusOK, gin.H{"matches": matches})
}
