package auth

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AuthMiddleware é o porteiro que protege as rotas privadas
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Pega o cabeçalho de Autorização
		authHeader := c.GetHeader("Authorization")

		// 2. Verifica se o cabeçalho existe e tem o formato correto ("Bearer <token>")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Acesso negado: Token não fornecido ou inválido"})
			c.Abort() // Bloqueia a requisição aqui mesmo
			return
		}

		// 3. Extrai apenas a string do token (tira a palavra "Bearer ")
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// 4. Decodifica e valida o token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Garante que o método de criptografia é o que esperamos
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("método de assinatura inesperado")
			}
			// Retorna o segredo para o validador bater as chaves
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		// Se o token for inválido, expirado ou forjado
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Acesso negado: Token inválido ou expirado"})
			c.Abort()
			return
		}

		// 5. Extrai o ID do usuário (sub) de dentro do token
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Erro ao ler dados do token"})
			c.Abort()
			return
		}

		// Converte a string do ID de volta para uuid.UUID
		userIDStr, _ := claims["sub"].(string)
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "ID de usuário inválido no token"})
			c.Abort()
			return
		}

		// 6. O Pulo do Gato: Salva o ID do usuário no contexto da requisição
		// Assim, a função que vai criar o Space lá na frente sabe exatamente quem está logado!
		c.Set("userID", userID)

		// 7. Libera a passagem para a próxima função (a rota real)
		c.Next()
	}
}

// CheckSpaceLimit é a catraca que verifica o plano do usuário antes de criar um Space
func CheckSpaceLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")

		// 1. Busca qual é o plano atual do usuário no banco
		var user models.User
		if err := database.DB.Select("subscription_type").First(&user, "id = ?", userID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao verificar assinatura"})
			c.Abort()
			return
		}

		// 2. Aplica a regra de negócio: Se for plano grátis, só pode ter 2 Spaces no máximo
		if user.SubscriptionType == "FREE_TRIAL" || user.SubscriptionType == "FREE" {
			var count int64
			database.DB.Model(&models.Space{}).Where("owner_id = ?", userID).Count(&count)

			if count >= 2 {
				// Status 402 Payment Required é o código perfeito para isso!
				c.JSON(http.StatusPaymentRequired, gin.H{
					"error": "Limite do plano Grátis atingido. Faça upgrade para o PRO para criar mais Spaces.",
				})
				c.Abort() // Bloqueia a requisição de chegar no controlador de criar Space
				return
			}
		}

		// Se for PRO ou ainda tiver limite no grátis, libera a passagem!
		c.Next()
	}
}

// CheckSpaceAccess verifica se o usuário é dono do Space ou se é um convidado (amigo)
func CheckSpaceAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")
		spaceID := c.Param("space_id") // Pega o ID do space direto da URL

		// 1. Primeira tentativa: Ele é o dono do Space?
		var space models.Space
		err := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error

		if err == nil {
			// É o dono! Libera a catraca imediatamente.
			c.Next()
			return
		}

		// 2. Segunda tentativa: Ele não é o dono, mas foi convidado? (Está na tabela de permissões?)
		var permission models.SpacePermission
		err = database.DB.Where("space_id = ? AND user_id = ?", spaceID, userID).First(&permission).Error

		if err != nil {
			// Não é dono e não foi convidado. Barrado!
			c.JSON(http.StatusForbidden, gin.H{"error": "Acesso negado. Você não é o dono e não foi convidado para este Space."})
			c.Abort()
			return
		}

		// 3. Se chegou aqui, ele é um amigo convidado!
		// Salvamos o nível de permissão dele (VIEWER ou EDITOR) no contexto para usarmos no futuro
		c.Set("spaceRole", permission.AccessLevel)

		// Libera a catraca!
		c.Next()
	}
}

// AdminOnly - Middleware que bloqueia qualquer um que não seja DEV ou ADMIN
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDContext, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autorizado"})
			c.Abort()
			return
		}

		// Busca o usuário no banco para ver qual é o "AccountType" dele
		var user models.User
		if err := database.DB.Select("account_type").First(&user, "id = ?", userIDContext).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuário não encontrado"})
			c.Abort()
			return
		}

		// A MÁGICA DA SEGURANÇA: Se não for ADMIN nem DEV, chuta pra fora!
		if user.AccountType != "ADMIN" && user.AccountType != "DEV" {
			c.JSON(http.StatusForbidden, gin.H{"error": "ACESSO NEGADO: Área restrita para Desenvolvedores."})
			c.Abort()
			return
		}

		// Se for DEV, libera a passagem!
		c.Next()
	}
}
