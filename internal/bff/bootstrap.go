package bff

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
// 🚀 BOOTSTRAP: Carrega o App inteiro em 1 requisição
// ==========================================================
func GetAppBootstrap(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	// 1️⃣ Busca o Perfil
	var user models.User
	database.DB.Select("id, full_name, nickname, profile_pic, xp, current_streak, created_at").Where("id = ?", userID).First(&user)

	// 2️⃣ Busca os Spaces (Turmas)
	var spaces []models.Space
	database.DB.Table("spaces").
		Select("spaces.*").
		Joins("LEFT JOIN space_permissions sp ON sp.space_id = spaces.id").
		Where("spaces.owner_id = ? OR sp.user_id = ?", userID, userID).
		Find(&spaces)

	// 3️⃣ Busca Notificações Não Lidas (Apenas calouros vs veteranos)
	isNewUser := time.Since(user.CreatedAt).Hours() < (7 * 24)
	var notifications []models.Notification
	database.DB.Raw(`
		SELECT n.* FROM notifications n
		LEFT JOIN notification_reads nr ON n.id = nr.notification_id AND nr.user_id = ?
		WHERE n.is_active = true AND n.start_at <= NOW() AND (n.end_at IS NULL OR n.end_at > NOW()) AND nr.id IS NULL
		AND (n.audience = 'GLOBAL' OR (n.audience = 'USERS' AND n.target_ids::jsonb @> ?) OR (n.audience = 'NEW_USERS' AND ? = true) OR (n.audience = 'VETERANS' AND ? = false))
		ORDER BY n.created_at DESC
	`, userID, `"`+userID.String()+`"`, isNewUser, isNewUser).Scan(&notifications)

	// 4️⃣ Busca o Dashboard do Cache (Se não tiver, retorna vazio e o front busca depois)
	cacheKey := "dashboard_" + userID.String()
	dashboardData, found := cache.AppCache.Get(cacheKey)
	if !found {
		dashboardData = nil // Opcional: Se não estiver no cache, o front busca separado depois para não atrasar o Bootstrap
	}

	// 🎁 5️⃣ Empacota tudo e manda para o Front-end!
	c.JSON(http.StatusOK, gin.H{
		"user":          user,
		"spaces":        spaces,
		"notifications": notifications,
		"dashboard":     dashboardData, // Pode vir preenchido ou nulo
	})
}
