package study // ou a pasta que você preferir colocar

import (
	"net/http"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==========================================================
// 📊 1. RELATÓRIO PESSOAL (A Vida do Usuário)
// ==========================================================
func GetPersonalDashboard(c *gin.Context) {
	userIDInterface, _ := c.Get("userID")
	var parsedUserID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		parsedUserID = v
	case string:
		parsedUserID, _ = uuid.Parse(v)
	}

	// 1. Dados Básicos do Usuário
	var user models.User
	database.DB.Select("xp, current_streak, highest_streak, last_login_at, created_at").Where("id = ?", parsedUserID).First(&user)

	// 2. Total de Horas Estudadas (Agregação Leve)
	var studyStats struct {
		TotalMinutes int `json:"total_minutes"`
		TotalExtra   int `json:"total_extra_minutes"`
	}
	database.DB.Model(&models.StudySession{}).
		Select("COALESCE(SUM(actual_minutes), 0) as total_minutes, COALESCE(SUM(GREATEST(actual_minutes - planned_minutes, 0)), 0) as total_extra").
		Where("user_id = ?", parsedUserID).
		Scan(&studyStats)

	// 3. Contagem de Spaces (Criados vs Participando)
	var spacesOwned int64
	database.DB.Model(&models.Space{}).Where("owner_id = ?", parsedUserID).Count(&spacesOwned)

	var spacesJoined int64
	database.DB.Model(&models.SpacePermission{}).Where("user_id = ? AND access_level != 'owner'", parsedUserID).Count(&spacesJoined)

	// Resposta Mastigada para o Front-end
	c.JSON(http.StatusOK, gin.H{
		"account": gin.H{
			"xp":             user.XP,
			"current_streak": user.CurrentStreak,
			"highest_streak": user.HighestStreak,
			"days_active":    int(time.Since(user.CreatedAt).Hours() / 24),
			"last_login":     user.LastLoginAt,
		},
		"study_metrics": gin.H{
			"total_hours":         studyStats.TotalMinutes / 60,
			"total_minutes":       studyStats.TotalMinutes,
			"total_extra_minutes": studyStats.TotalExtra,
		},
		"networking": gin.H{
			"spaces_owned":  spacesOwned,
			"spaces_joined": spacesJoined,
		},
	})
}

// ==========================================================
// 📊 2. RELATÓRIO DA TURMA/SPACE (O Olho de Deus)
// ==========================================================
func GetSpaceDashboard(c *gin.Context) {
	spaceIDStr := c.Param("space_id")

	// 1. Visão Geral do Space
	var space models.Space
	database.DB.Select("view_count, created_at").Where("id = ?", spaceIDStr).First(&space)

	// 2. Contagem de Colaboradores e Conteúdos
	var totalCollaborators, totalNotebooks, totalQuizzes int64
	database.DB.Model(&models.SpacePermission{}).Where("space_id = ?", spaceIDStr).Count(&totalCollaborators)
	database.DB.Model(&models.Notebook{}).Where("space_id = ?", spaceIDStr).Count(&totalNotebooks)
	database.DB.Model(&models.Quiz{}).Where("space_id = ?", spaceIDStr).Count(&totalQuizzes)

	// 3. Estudo Coletivo (Quantas horas a turma inteira já estudou aqui?)
	var totalSpaceStudyMinutes int
	database.DB.Model(&models.StudySession{}).
		Where("space_id = ?", spaceIDStr).
		Select("COALESCE(SUM(actual_minutes), 0)").Scan(&totalSpaceStudyMinutes)

	// ==========================================================
	// 👇 4. Quem estudou mais? (O ÚNICO LUGAR QUE MEXEMOS!)
	// ==========================================================
	type TopStudent struct {
		UserID     uuid.UUID `json:"user_id"`
		FullName   string    `json:"full_name"`           // 👈 Adicionado
		ProfilePic string    `json:"profile_picture_url"` // 👈 Adicionado
		Minutes    int       `json:"minutes"`
	}
	var topStudents []TopStudent

	// O JOIN Ninja para puxar a foto e o nome do aluno junto com os minutos
	database.DB.Table("study_sessions").
		Select("study_sessions.user_id, users.full_name, users.profile_pic, SUM(study_sessions.actual_minutes) as minutes").
		Joins("JOIN users ON users.id = study_sessions.user_id").
		Where("study_sessions.space_id = ?", spaceIDStr).
		Group("study_sessions.user_id, users.full_name, users.profile_pic").
		Order("minutes DESC").
		Limit(3).
		Scan(&topStudents)
	// ==========================================================

	// 5. Atividade Recente (Últimos 7 dias) para medir quem entrou/saiu
	var recentActivity int64
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	database.DB.Model(&models.ActivityLog{}).
		Where("space_id = ? AND created_at >= ?", spaceIDStr, sevenDaysAgo).
		Count(&recentActivity)

	c.JSON(http.StatusOK, gin.H{
		"overview": gin.H{
			"total_views":         space.ViewCount,
			"total_collaborators": totalCollaborators,
			"days_created":        int(time.Since(space.CreatedAt).Hours() / 24),
		},
		"content": gin.H{
			"notebooks": totalNotebooks,
			"quizzes":   totalQuizzes,
		},
		"engagement": gin.H{
			"total_study_hours":     totalSpaceStudyMinutes / 60,
			"total_study_minutes":   totalSpaceStudyMinutes,
			"recent_actions_7_days": recentActivity,
		},
		"top_students": topStudents, // Agora devolve com as fotos!
	})
}
