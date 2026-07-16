package space

import (
	"net/http"
	"time"

	"studfy-backend/pkg/cache"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ==========================================================
// 🚨 1. ALERTA VERMELHO (Alunos em Risco de Evasão)
// Identifica alunos do Space que estão há muito tempo sem estudar
// ==========================================================
func GetAtRiskStudents(c *gin.Context) {
	spaceID := c.Param("space_id")

	// Verifica a autenticação (Idealmente, aqui teria um middleware verificando se ele é Dono/Admin do Space)
	_, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	cacheKey := "risk_report_" + spaceID
	if cachedData, found := cache.AppCache.Get(cacheKey); found {
		c.JSON(http.StatusOK, cachedData)
		return
	}

	// Lógica Sênior: Buscar alunos cadastrados no Space que não registram sessão de estudo há mais de 15 dias
	fifteenDaysAgo := time.Now().AddDate(0, 0, -15)

	type RiskStudent struct {
		UserID       string    `json:"user_id"`
		Name         string    `json:"name"`
		LastStudy    time.Time `json:"last_study_date"`
		DaysInactive int       `json:"days_inactive"`
	}
	var atRisk []RiskStudent

	// Usamos JOIN para cruzar os membros do Space com a última data de estudo deles
	// NOTA: Estrutura baseada nas tabelas padrões, ajuste os nomes dos campos se necessário
	database.DB.Table("space_members").
		Select("users.id as user_id, users.name as name, MAX(study_sessions.created_at) as last_study").
		Joins("JOIN users ON users.id = space_members.user_id").
		Joins("LEFT JOIN study_sessions ON study_sessions.user_id = users.id").
		Where("space_members.space_id = ?", spaceID).
		Group("users.id, users.name").
		Having("last_study < ? OR last_study IS NULL", fifteenDaysAgo).
		Scan(&atRisk)

	// Calcula os dias exatos de inatividade para o Front-end
	for i, student := range atRisk {
		if student.LastStudy.IsZero() {
			atRisk[i].DaysInactive = 999 // Nunca estudou
		} else {
			atRisk[i].DaysInactive = int(time.Since(student.LastStudy).Hours() / 24)
		}
	}

	response := gin.H{"at_risk_students": atRisk, "total_alerts": len(atRisk)}
	cache.AppCache.Set(cacheKey, response, 30*time.Minute) // Cache de 30 min (dados não mudam tão rápido)

	c.JSON(http.StatusOK, response)
}

// ==========================================================
// 💀 2. ÍNDICE DE MORTALIDADE POR MATERIAL (Dificuldade)
// Mostra quais cadernos/flashcards a turma está errando mais
// ==========================================================
func GetMaterialMortalityRate(c *gin.Context) {
	spaceID := c.Param("space_id")

	cacheKey := "mortality_" + spaceID
	if cachedData, found := cache.AppCache.Get(cacheKey); found {
		c.JSON(http.StatusOK, cachedData)
		return
	}

	// Aqui simularíamos a busca pelas taxas de erro de Flashcards ou Quizzes vinculados a este Space
	// Como não temos a estrutura exata do seu QuizResult na memória, vou criar o esqueleto analítico:
	type MaterialStats struct {
		MaterialName string `json:"material_name"`
		Type         string `json:"type"` // "Flashcard" ou "Simulado"
		ErrorRate    int    `json:"error_rate_percentage"`
	}

	// Mock de dados (Substitua por sua GORM Query cruzando FlashcardReviews com Space_ID)
	stats := []MaterialStats{
		{"Geometria Analítica - Lista 1", "Simulado", 78}, // 78% da turma errou
		{"Conceitos de Citologia", "Flashcard", 65},
		{"Revolução Francesa", "Flashcard", 12}, // Esse está fácil, poucos erros
	}

	var criticalMaterials []MaterialStats
	for _, stat := range stats {
		if stat.ErrorRate > 60 { // Regra de Negócio: Mais de 60% de erro = Material Crítico
			criticalMaterials = append(criticalMaterials, stat)
		}
	}

	response := gin.H{
		"critical_materials": criticalMaterials,
		"message":            "Revise o conteúdo dos materiais críticos, a turma está com muita dificuldade.",
	}
	cache.AppCache.Set(cacheKey, response, 1*time.Hour)

	c.JSON(http.StatusOK, response)
}

// ==========================================================
// 📊 3. ENGAJAMENTO DE CONTEÚDO (Heatmap de Leitura)
// Conta quais materiais estão a ser mais consumidos
// ==========================================================
func GetMaterialEngagement(c *gin.Context) {
	spaceID := c.Param("space_id")

	cacheKey := "engagement_" + spaceID
	if cachedData, found := cache.AppCache.Get(cacheKey); found {
		c.JSON(http.StatusOK, cachedData)
		return
	}

	type EngagementData struct {
		Title        string `json:"title"`
		Views        int    `json:"total_views"`
		Interactions int    `json:"total_interactions"` // Comentários, Dúvidas
	}
	var engagement []EngagementData

	// Query: Pega os Cadernos (Notebooks) deste Space e soma as visualizações
	database.DB.Table("notebooks").
		Select("title, views_count as views, comments_count as interactions").
		Where("space_id = ?", spaceID).
		Order("views DESC").
		Limit(10).
		Scan(&engagement)

	response := gin.H{"top_materials": engagement}
	cache.AppCache.Set(cacheKey, response, 15*time.Minute)

	c.JSON(http.StatusOK, response)
}
