package study

import (
	"net/http"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ==========================================================
// 📊 RELATÓRIOS 1 E 2: PRODUTIVIDADE E RAIO-X DE DISCIPLINAS
// ==========================================================
func GetProductivityReport(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	var sessions []models.StudySession
	database.DB.Where("user_id = ? AND created_at >= ?", userID, sevenDaysAgo).Find(&sessions)

	// Agrupamento para Gráfico de Barras (Tempo por Dia)
	dailyStats := make(map[string]int)
	// Agrupamento para Gráfico de Pizza (Tempo por Matéria)
	subjectStats := make(map[string]int)

	for _, s := range sessions {
		dateStr := s.CreatedAt.Format("2006-01-02")
		dailyStats[dateStr] += s.ActualMinutes
		subjectStats[s.ActivityName] += s.ActualMinutes
	}

	var dailyChart []map[string]interface{}
	for date, mins := range dailyStats {
		dailyChart = append(dailyChart, map[string]interface{}{"date": date, "minutes": mins})
	}

	var subjectChart []map[string]interface{}
	for name, mins := range subjectStats {
		subjectChart = append(subjectChart, map[string]interface{}{"subject": name, "minutes": mins})
	}

	c.JSON(http.StatusOK, gin.H{
		"daily_trend": dailyChart,   // Para o Gráfico de Linha/Barra
		"subject_pie": subjectChart, // Para o Gráfico de Pizza
	})
}

// ==========================================================
// 📈 RELATÓRIO 3: EVOLUÇÃO DE NOTAS EM SIMULADOS
// ==========================================================
func GetQuizPerformanceReport(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var results []models.QuizResult
	database.DB.Where("user_id = ? AND status = 'completed'", userID).Order("created_at ASC").Limit(10).Find(&results)

	var performanceChart []map[string]interface{}
	for _, r := range results {
		// Calcula a porcentagem de acerto
		var percentage float64 = 0
		if r.TotalQuestions > 0 {
			percentage = (r.Score / float64(r.TotalQuestions)) * 100
		}

		performanceChart = append(performanceChart, map[string]interface{}{
			"date":       r.CreatedAt.Format("2006-01-02"),
			"score":      r.Score,
			"percentage": percentage,
		})
	}

	c.JSON(http.StatusOK, gin.H{"quiz_performance": performanceChart})
}

// ==========================================================
// 🧘 RELATÓRIOS 4 E 5: TERMÔMETRO DE FOCO E HUMOR
// ==========================================================
func GetFocusAndMoodReport(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	// Pomodoros
	var totalPomodoros int64
	database.DB.Model(&models.PomodoroSession{}).Where("user_id = ? AND created_at >= ?", userID, thirtyDaysAgo).Count(&totalPomodoros)

	// Moods (Agrupados por Sentimento)
	type MoodResult struct {
		Mood  string `json:"mood"`
		Count int    `json:"count"`
	}
	var moodStats []MoodResult
	database.DB.Model(&models.MoodCheckIn{}).
		Select("mood, count(*) as count").
		Where("user_id = ? AND created_at >= ?", userID, thirtyDaysAgo).
		Group("mood").
		Scan(&moodStats)

	c.JSON(http.StatusOK, gin.H{
		"pomodoros_last_30_days": totalPomodoros,
		"mood_distribution":      moodStats,
	})
}

// ==========================================================
// 🏆 RELATÓRIOS 6 E 10: GAMIFICAÇÃO E ARENA
// ==========================================================
func GetGamificationReport(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var user models.User
	database.DB.Select("xp, current_streak, highest_streak").Where("id = ?", userID).First(&user)

	// Status na Arena
	var totalMatches, wins int64
	database.DB.Model(&models.ArenaMatch{}).Where("challenger_id = ? OR opponent_id = ?", userID, userID).Count(&totalMatches)
	database.DB.Model(&models.ArenaMatch{}).Where("winner_id = ?", userID).Count(&wins)

	winRate := 0.0
	if totalMatches > 0 {
		winRate = (float64(wins) / float64(totalMatches)) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"xp":             user.XP,
		"current_streak": user.CurrentStreak,
		"highest_streak": user.HighestStreak,
		"arena": gin.H{
			"total_matches": totalMatches,
			"wins":          wins,
			"win_rate":      winRate,
		},
	})
}

// ==========================================================
// ⚠️ RELATÓRIO 7: DÍVIDA DE MATÉRIAS (Atrasos)
// ==========================================================
func GetStudyDebtReport(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	// Busca dívidas no Ciclo Adaptativo
	type DebtResult struct {
		Activity string `json:"activity"`
		Debt     int    `json:"missing_minutes"`
	}
	var cycleDebts []DebtResult

	database.DB.Table("cycle_log_blocks").
		Select("cycle_log_blocks.activity, SUM(cycle_log_blocks.missing_minutes) as debt").
		Joins("JOIN cycle_logs ON cycle_logs.id = cycle_log_blocks.cycle_log_id").
		Where("cycle_logs.user_id = ? AND cycle_log_blocks.missing_minutes > 0", userID).
		Group("cycle_log_blocks.activity").
		Scan(&cycleDebts)

	c.JSON(http.StatusOK, gin.H{"subject_debts": cycleDebts})
}

// ==========================================================
// 🤝 RELATÓRIO 8: ENGAJAMENTO NA TURMA
// ==========================================================
func GetSpaceEngagementReport(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var flashcardsCreated, notesCreated, doubtsAsked int64

	database.DB.Model(&models.Flashcard{}).Where("created_by_id = ?", userID).Count(&flashcardsCreated)
	// QuickNotes usa o SpaceID para identificar, mas vamos assumir um modelo de log de atividade para o engajamento geral
	database.DB.Model(&models.PageDoubt{}).Where("student_id = ?", userID).Count(&doubtsAsked)

	c.JSON(http.StatusOK, gin.H{
		"flashcards_created": flashcardsCreated,
		"doubts_asked":       doubtsAsked,
		"notes_created":      notesCreated,
		"total_interactions": flashcardsCreated + doubtsAsked, // Somatório para nível de engajamento
	})
}

// ==========================================================
// 🧠 RELATÓRIO 9: CURVA DE ESQUECIMENTO (Revisões Pendentes)
// ==========================================================
func GetPendingReviewsReport(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	today := time.Now()
	nextWeek := today.AddDate(0, 0, 7)

	// Trazemos as revisões do aluno baseadas nas anotações que ele criou
	type ReviewResult struct {
		Date  string `json:"date"`
		Count int    `json:"count"`
	}
	var reviews []ReviewResult

	database.DB.Table("reviews").
		Select("DATE(review_date) as date, count(*) as count").
		Joins("JOIN pages ON pages.id = reviews.note_id"). // Ajuste conforme seu modelo de anotação
		Where("pages.created_by_id = ? AND status = 'pendente' AND review_date BETWEEN ? AND ?", userID, today, nextWeek).
		Group("DATE(review_date)").
		Scan(&reviews)

	c.JSON(http.StatusOK, gin.H{"upcoming_reviews": reviews})
}
