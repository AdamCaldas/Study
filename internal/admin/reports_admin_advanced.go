package admin

import (
	"net/http"
	"time"

	"studfy-backend/pkg/cache"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
)

// ==========================================================
// 📉 1. RELATÓRIO DE CHURN E RETENÇÃO
// Descobre quantos usuários estão abandonando a plataforma
// ==========================================================
func GetPlatformRetentionReport(c *gin.Context) {
	cacheKey := "admin_retention_report"
	if cachedData, found := cache.AppCache.Get(cacheKey); found {
		c.JSON(http.StatusOK, cachedData)
		return
	}

	var totalUsers int64
	var activeUsers int64

	// Total de usuários registrados ativos (não deletados)
	database.DB.Table("users").Where("deleted_at IS NULL").Count(&totalUsers)

	// Usuários que logaram nos últimos 30 dias
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	database.DB.Table("users").Where("deleted_at IS NULL AND last_login_at >= ?", thirtyDaysAgo).Count(&activeUsers)

	churnedUsers := totalUsers - activeUsers

	var retentionRate float64
	if totalUsers > 0 {
		retentionRate = (float64(activeUsers) / float64(totalUsers)) * 100
	}

	response := gin.H{
		"total_users":               totalUsers,
		"active_users_30d":          activeUsers,
		"churned_users":             churnedUsers,
		"retention_rate_percentage": retentionRate,
		"status":                    "Saudável",
	}

	if retentionRate < 40 {
		response["status"] = "Crítico: A maioria dos usuários está abandonando o app após o cadastro."
	}

	// Faz cache de 1 hora (relatórios globais pesam no banco)
	cache.AppCache.Set(cacheKey, response, 1*time.Hour)
	c.JSON(http.StatusOK, response)
}

// ==========================================================
// 💰 2. DISTRIBUIÇÃO DE PLANOS (Receita e Crescimento)
// Mostra quantos usuários estão no Free, Pro ou Teacher
// ==========================================================
func GetPlanDistributionReport(c *gin.Context) {
	cacheKey := "admin_plan_distribution"
	if cachedData, found := cache.AppCache.Get(cacheKey); found {
		c.JSON(http.StatusOK, cachedData)
		return
	}

	type PlanStats struct {
		PlanType string `json:"plan_type"`
		Total    int    `json:"total_users"`
	}
	var stats []PlanStats

	// Agrupa e conta os usuários pelo tipo de plano
	database.DB.Table("users").
		Select("plan_type, COUNT(*) as total").
		Where("deleted_at IS NULL").
		Group("plan_type").
		Scan(&stats)

	response := gin.H{"plans": stats}
	cache.AppCache.Set(cacheKey, response, 1*time.Hour)

	c.JSON(http.StatusOK, response)
}

// ==========================================================
// 🏭 3. SAÚDE DA PLATAFORMA (Volume de Criação)
// Mede o engajamento global da plataforma na última semana
// ==========================================================
func GetPlatformHealthStats(c *gin.Context) {
	cacheKey := "admin_health_stats"
	if cachedData, found := cache.AppCache.Get(cacheKey); found {
		c.JSON(http.StatusOK, cachedData)
		return
	}

	sevenDaysAgo := time.Now().AddDate(0, 0, -7)

	var newFlashcards int64
	var newStudySessions int64
	var newSpaces int64

	// Contagem de tudo o que foi criado nos últimos 7 dias na plataforma inteira
	database.DB.Table("flashcards").Where("created_at >= ?", sevenDaysAgo).Count(&newFlashcards)
	database.DB.Table("study_sessions").Where("created_at >= ?", sevenDaysAgo).Count(&newStudySessions)
	database.DB.Table("spaces").Where("created_at >= ?", sevenDaysAgo).Count(&newSpaces)

	response := gin.H{
		"period": "Últimos 7 dias",
		"metrics": gin.H{
			"new_flashcards":     newFlashcards,
			"new_study_sessions": newStudySessions,
			"new_spaces":         newSpaces,
		},
	}

	cache.AppCache.Set(cacheKey, response, 30*time.Minute)
	c.JSON(http.StatusOK, response)
}
