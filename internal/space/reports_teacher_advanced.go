package space

import (
	"net/http"
	"time"

	"studfy-backend/pkg/cache"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
)

// ==========================================================
// 🚨 1. ALERTA VERMELHO (Alunos em Risco de Evasão)
// Alunos da turma que não registram sessão de estudo há mais de 15 dias.
// ==========================================================
// O acesso já é filtrado pelo middleware RequireSpaceStaff na rota.
func GetAtRiskStudents(c *gin.Context) {
	spaceID := c.Param("space_id")

	cacheKey := "risk_report_" + spaceID
	if cachedData, found := cache.AppCache.Get(cacheKey); found {
		c.JSON(http.StatusOK, cachedData)
		return
	}

	fifteenDaysAgo := time.Now().AddDate(0, 0, -15)

	type RiskStudent struct {
		UserID       string     `json:"user_id"`
		Name         string     `json:"name"`
		Email        string     `json:"email"`
		LastStudy    *time.Time `json:"last_study_date"`
		DaysInactive int        `json:"days_inactive"`
		NeverStudied bool       `json:"never_studied"`
	}
	var atRisk []RiskStudent

	// Cruza os MEMBROS da turma (space_permissions) com a última sessão de estudo
	// que cada um registrou NESTA turma.
	// Antes esta consulta usava a tabela `space_members` e a coluna `users.name`,
	// que não existem — o relatório quebrava sempre.
	err := database.DB.Raw(`
		SELECT
			u.id        AS user_id,
			u.full_name AS name,
			u.email     AS email,
			MAX(ss.created_at) AS last_study
		FROM space_permissions sp
		JOIN users u ON u.id = sp.user_id
		LEFT JOIN study_sessions ss
			   ON ss.user_id = sp.user_id AND ss.space_id = sp.space_id
		WHERE sp.space_id = ? AND u.deleted_at IS NULL
		GROUP BY u.id, u.full_name, u.email
		HAVING MAX(ss.created_at) < ? OR MAX(ss.created_at) IS NULL
		ORDER BY last_study ASC NULLS FIRST
		LIMIT 200
	`, spaceID, fifteenDaysAgo).Scan(&atRisk).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao gerar o relatório de alunos em risco."})
		return
	}

	// Calcula os dias exatos de inatividade para o Front-end
	for i := range atRisk {
		if atRisk[i].LastStudy == nil {
			atRisk[i].NeverStudied = true
			atRisk[i].DaysInactive = -1 // o front mostra "nunca estudou"
		} else {
			atRisk[i].DaysInactive = int(time.Since(*atRisk[i].LastStudy).Hours() / 24)
		}
	}

	if atRisk == nil {
		atRisk = []RiskStudent{}
	}

	response := gin.H{"at_risk_students": atRisk, "total_alerts": len(atRisk)}
	cache.AppCache.Set(cacheKey, response, 30*time.Minute)

	c.JSON(http.StatusOK, response)
}

// ==========================================================
// 💀 2. ÍNDICE DE DIFICULDADE POR SIMULADO
// Mostra em quais provas a turma mais erra.
// ==========================================================
// ⚠️ Esta função DEVOLVIA UMA LISTA FIXA ESCRITA NO CÓDIGO ("Geometria
// Analítica 78%"...). O professor tomava decisão pedagógica com número
// inventado. Agora ela calcula de verdade, a partir das notas reais.
//
// Cálculo: para cada simulado da turma, compara a média de pontos obtidos com o
// total de pontos possíveis. Taxa de erro = 100 − aproveitamento médio.
func GetMaterialMortalityRate(c *gin.Context) {
	spaceID := c.Param("space_id")

	cacheKey := "mortality_" + spaceID
	if cachedData, found := cache.AppCache.Get(cacheKey); found {
		c.JSON(http.StatusOK, cachedData)
		return
	}

	type MaterialStats struct {
		QuizID       string  `json:"quiz_id"`
		MaterialName string  `json:"material_name"`
		Type         string  `json:"type"`
		Attempts     int     `json:"attempts"`
		AverageScore float64 `json:"average_score"`
		MaxScore     float64 `json:"max_score"`
		ErrorRate    float64 `json:"error_rate_percentage"`
	}
	var stats []MaterialStats

	err := database.DB.Raw(`
		SELECT
			q.id    AS quiz_id,
			q.title AS material_name,
			'Simulado' AS type,
			COUNT(qr.id) AS attempts,
			COALESCE(AVG(qr.score), 0) AS average_score,
			COALESCE(pts.total_points, 0) AS max_score,
			CASE
				WHEN COALESCE(pts.total_points, 0) > 0
				THEN GREATEST(0, 100 - (COALESCE(AVG(qr.score), 0) / pts.total_points * 100))
				ELSE 0
			END AS error_rate
		FROM quizzes q
		LEFT JOIN quiz_results qr
			   ON qr.quiz_id = q.id AND qr.status = 'completed'
		LEFT JOIN (
			SELECT quiz_id, SUM(points) AS total_points
			FROM quiz_questions
			GROUP BY quiz_id
		) pts ON pts.quiz_id = q.id
		WHERE q.space_id = ?
		GROUP BY q.id, q.title, pts.total_points
		HAVING COUNT(qr.id) > 0
		ORDER BY error_rate DESC
		LIMIT 20
	`, spaceID).Scan(&stats).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao calcular o índice de dificuldade."})
		return
	}

	// Regra de negócio: acima de 60% de erro, o material é crítico.
	criticalMaterials := []MaterialStats{}
	for _, stat := range stats {
		if stat.ErrorRate > 60 {
			criticalMaterials = append(criticalMaterials, stat)
		}
	}

	if stats == nil {
		stats = []MaterialStats{}
	}

	message := "A turma está com bom aproveitamento nos simulados."
	if len(criticalMaterials) > 0 {
		message = "Reveja o conteúdo dos materiais críticos: a turma está com muita dificuldade."
	} else if len(stats) == 0 {
		message = "Ainda não há simulados respondidos nesta turma."
	}

	response := gin.H{
		"all_materials":      stats,
		"critical_materials": criticalMaterials,
		"message":            message,
	}
	cache.AppCache.Set(cacheKey, response, 15*time.Minute)

	c.JSON(http.StatusOK, response)
}

// ==========================================================
// 📊 3. ENGAJAMENTO DE CONTEÚDO
// Quais cadernos a turma mais consome (foco registrado) e onde mais trava.
// ==========================================================
// Antes esta consulta pedia notebooks.title / views_count / comments_count —
// nenhuma dessas colunas existe. Agora usa sinais reais: sessões de foco
// (pomodoro) apontando para o caderno e dúvidas abertas nas páginas dele.
func GetMaterialEngagement(c *gin.Context) {
	spaceID := c.Param("space_id")

	cacheKey := "engagement_" + spaceID
	if cachedData, found := cache.AppCache.Get(cacheKey); found {
		c.JSON(http.StatusOK, cachedData)
		return
	}

	type EngagementData struct {
		NotebookID    string `json:"notebook_id"`
		Title         string `json:"title"`
		Pages         int    `json:"total_pages"`
		FocusSessions int    `json:"focus_sessions"`
		FocusMinutes  int    `json:"focus_minutes"`
		Doubts        int    `json:"total_doubts"`
	}
	var engagement []EngagementData

	err := database.DB.Raw(`
		SELECT
			n.id   AS notebook_id,
			n.name AS title,
			COALESCE(pg.total, 0)      AS pages,
			COALESCE(ps.sessions, 0)   AS focus_sessions,
			COALESCE(ps.minutes, 0)    AS focus_minutes,
			COALESCE(db.total, 0)      AS doubts
		FROM notebooks n
		LEFT JOIN (
			SELECT notebook_id, COUNT(*) AS total FROM pages GROUP BY notebook_id
		) pg ON pg.notebook_id = n.id
		LEFT JOIN (
			SELECT notebook_id, COUNT(*) AS sessions, COALESCE(SUM(duration), 0) AS minutes
			FROM pomodoro_sessions
			WHERE notebook_id IS NOT NULL
			GROUP BY notebook_id
		) ps ON ps.notebook_id = n.id
		LEFT JOIN (
			SELECT p.notebook_id, COUNT(d.id) AS total
			FROM page_doubts d
			JOIN pages p ON p.id = d.page_id
			GROUP BY p.notebook_id
		) db ON db.notebook_id = n.id
		WHERE n.space_id = ?
		ORDER BY focus_minutes DESC, doubts DESC
		LIMIT 20
	`, spaceID).Scan(&engagement).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao gerar o relatório de engajamento."})
		return
	}

	if engagement == nil {
		engagement = []EngagementData{}
	}

	response := gin.H{"top_materials": engagement}
	cache.AppCache.Set(cacheKey, response, 15*time.Minute)

	c.JSON(http.StatusOK, response)
}
