package study

import (
	"net/http"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/cache"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ==========================================================
// 🟩 1. MAPA DE CONSISTÊNCIA (Estilo GitHub Heatmap)
// Mostra os quadradinhos verdes dos últimos 365 dias
// ==========================================================
func GetStudyHeatmap(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	cacheKey := "heatmap_" + userID.String()
	if cachedData, found := cache.AppCache.Get(cacheKey); found {
		c.JSON(http.StatusOK, cachedData)
		return
	}

	// Busca os dados do último ano
	oneYearAgo := time.Now().AddDate(-1, 0, 0)

	type HeatmapData struct {
		Date  string `json:"date"`
		Total int    `json:"total_minutes"`
	}
	var heatmap []HeatmapData

	// Query mágica: Agrupa as sessões por dia e soma os minutos!
	database.DB.Model(&models.StudySession{}).
		Select("DATE(created_at) as date, SUM(actual_minutes) as total").
		Where("user_id = ? AND created_at >= ?", userID, oneYearAgo).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&heatmap)

	response := gin.H{"heatmap": heatmap}
	cache.AppCache.Set(cacheKey, response, 15*time.Minute) // Faz cache por 15 min (é um relatório anual, não muda a cada segundo)

	c.JSON(http.StatusOK, response)
}

// ==========================================================
// 🎯 2. MATRIZ DE FORÇAS E FRAQUEZAS
// Descobre onde o aluno é talentoso e onde ele precisa de ajuda
// ==========================================================
func GetStrengthsAndWeaknesses(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	cacheKey := "strengths_" + userID.String()
	if cachedData, found := cache.AppCache.Get(cacheKey); found {
		c.JSON(http.StatusOK, cachedData)
		return
	}

	// Aqui agregamos os minutos estudados por "Matéria" (ActivityName)
	type SubjectStats struct {
		Subject string `json:"subject"`
		Minutes int    `json:"minutes"`
	}
	var stats []SubjectStats

	database.DB.Model(&models.StudySession{}).
		Select("activity_name as subject, SUM(actual_minutes) as minutes").
		Where("user_id = ? AND activity_name != ''").
		Group("activity_name").
		Scan(&stats)

	// A Inteligência de Negócio: Classificar baseado no tempo gasto
	// Num cenário real completo, cruzaríamos com a nota do Quiz (QuizResult)
	var talents []string  // Estuda pouco e vai bem (Para o futuro cruzar com notas)
	var hardWork []string // Estuda muito e garante
	var alerts []string   // Matérias ignoradas ou com dívida

	for _, s := range stats {
		if s.Minutes > 600 { // Mais de 10 horas
			hardWork = append(hardWork, s.Subject)
		} else if s.Minutes > 120 { // Entre 2h e 10h
			talents = append(talents, s.Subject)
		} else {
			alerts = append(alerts, s.Subject)
		}
	}

	response := gin.H{
		"matrix": gin.H{
			"talents":   talents,
			"hard_work": hardWork,
			"alerts":    alerts,
		},
	}

	cache.AppCache.Set(cacheKey, response, 30*time.Minute)
	c.JSON(http.StatusOK, response)
}

// ==========================================================
// ⚖️ 3. EFICIÊNCIA DE TEMPO (Teoria vs Prática)
// ==========================================================
func GetTimeEfficiency(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	// Conta quantos Flashcards o aluno criou (Prática Ativa)
	var totalFlashcards int64
	database.DB.Model(&models.Flashcard{}).Where("created_by_id = ?", userID).Count(&totalFlashcards)

	// Conta quantas Páginas/Anotações ele criou (Teoria Passiva)
	var totalNotes int64
	database.DB.Model(&models.Page{}).Where("created_by_id = ?", userID).Count(&totalNotes)

	total := float64(totalFlashcards + totalNotes)
	if total == 0 {
		total = 1 // Evita divisão por zero
	}

	practicePercent := (float64(totalFlashcards) / total) * 100
	theoryPercent := (float64(totalNotes) / total) * 100

	status := "Equilibrado"
	if practicePercent < 30 {
		status = "Alerta: Você está lendo muito e praticando pouco! Crie mais flashcards."
	} else if theoryPercent < 20 {
		status = "Alerta: Muito foco em prática, cuidado para não pular a base teórica."
	}

	c.JSON(http.StatusOK, gin.H{
		"theory_percentage":   int(theoryPercent),
		"practice_percentage": int(practicePercent),
		"status_message":      status,
	})
}
