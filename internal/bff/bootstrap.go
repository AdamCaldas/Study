package bff

import (
	"net/http"
	"time"

	"studfy-backend/internal/admin"
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
	// Sem lista de colunas: o front consome o objeto inteiro — sem
	// `account_type` o RoleGuard não monta o layout e a tela fica em branco,
	// sem erro nenhum. `password` e `deleted_at` são `json:"-"` no model, então
	// não vazam na resposta.
	// O erro é tratado: um `First` silencioso devolvia o usuário zerado
	// (id 00000000-…) quando não havia linha, e o front não tinha como saber.
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":  "Perfil não encontrado para o usuário do token",
			"detail": "com KEYCLOAK_ONLY ativo nada é espelhado na tabela `users`",
		})
		return
	}

	// 2️⃣ Busca os Spaces (Turmas)
	var spaces []models.Space
	database.DB.Table("spaces").
		Select("spaces.*").
		Joins("LEFT JOIN space_permissions sp ON sp.space_id = spaces.id").
		Where("spaces.owner_id = ? OR sp.user_id = ?", userID, userID).
		Find(&spaces)

	// 3️⃣ Busca Notificações Não Lidas.
	// Usa a MESMA consulta do mural (admin.NotificationFeedSQL) — antes havia uma
	// cópia aqui que não entregava os avisos do Megafone (público SPACES).
	isNewUser := time.Since(user.CreatedAt).Hours() < (7 * 24)
	var notifications []models.Notification
	database.DB.Raw(admin.NotificationFeedSQL,
		userID,
		`"`+userID.String()+`"`,
		isNewUser,
		isNewUser,
		userID,
		userID,
	).Scan(&notifications)

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
