package auth

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils" // 👈 Import adicionado

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AuthMiddleware é o porteiro que protege as rotas privadas
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Acesso negado: Token não fornecido ou inválido"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("método de assinatura inesperado")
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Acesso negado: Token inválido ou expirado"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Erro ao ler dados do token"})
			c.Abort()
			return
		}

		userIDStr, _ := claims["sub"].(string)
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "ID de utilizador inválido no token"})
			c.Abort()
			return
		}

		// O AuthMiddleware é a ÚNICA função do sistema inteiro que "Seta" o ID!
		c.Set("userID", userID)

		c.Next()
	}
}

// CheckSpaceLimit é a catraca que verifica o plano do usuário antes de criar um Space
func CheckSpaceLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 👇 Limpeza com a nossa função global!
		userID, err := utils.GetUserID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
			c.Abort()
			return
		}

		var user models.User
		if err := database.DB.Select("subscription_type").First(&user, "id = ?", userID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao verificar assinatura"})
			c.Abort()
			return
		}

		// 👇 Substituição das Strings Mágicas pelas Constantes
		if user.SubscriptionType == utils.PlanFreeTrial || user.SubscriptionType == utils.PlanFree {
			var count int64
			database.DB.Model(&models.Space{}).Where("owner_id = ?", userID).Count(&count)

			if count >= 2 {
				c.JSON(http.StatusPaymentRequired, gin.H{
					"error": "Limite do plano Grátis atingido. Faça upgrade para o PRO para criar mais Spaces.",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// CheckSpaceAccess verifica se o usuário é dono do Space ou se é um convidado (amigo)
func CheckSpaceAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := utils.GetUserID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
			c.Abort()
			return
		}

		spaceIDStr := c.Param("space_id")
		parsedSpaceID, err := uuid.Parse(spaceIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido na URL"})
			c.Abort()
			return
		}

		var space models.Space
		err = database.DB.Where("id = ? AND owner_id = ?", parsedSpaceID, userID).First(&space).Error

		if err == nil {
			c.Next()
			return
		}

		var permission models.SpacePermission
		err = database.DB.Where("space_id = ? AND user_id = ?", parsedSpaceID, userID).First(&permission).Error

		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Acesso negado. Você não é o dono e não foi convidado para este Space."})
			c.Abort()
			return
		}

		c.Set("spaceRole", permission.AccessLevel)
		c.Next()
	}
}

// AdminOnly - Middleware que bloqueia qualquer um que não seja DEV ou ADMIN
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := utils.GetUserID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autorizado"})
			c.Abort()
			return
		}

		var user models.User
		if err := database.DB.Select("account_type").First(&user, "id = ?", userID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Utilizador não encontrado"})
			c.Abort()
			return
		}

		// 👇 Substituição das Strings Mágicas pelas Constantes
		if user.AccountType != utils.RoleAdmin && user.AccountType != utils.RoleDev {
			c.JSON(http.StatusForbidden, gin.H{"error": "ACESSO NEGADO: Área restrita para a Administração."})
			c.Abort()
			return
		}

		c.Next()
	}
}
