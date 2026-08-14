package auth

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils" // 👈 Import adicionado

	"github.com/MicahParks/keyfunc/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AuthMiddleware é o porteiro que protege as rotas privadas.
// Valida o access token emitido pelo Keycloak usando as chaves públicas do
// realm (JWKS), que são recarregadas em background pelo keyfunc.
func AuthMiddleware(jwks *keyfunc.JWKS) gin.HandlerFunc {
	cfg := loadKeycloakConfig()

	return func(c *gin.Context) {
		tokenString, ok := bearerToken(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Acesso negado: Token não fornecido ou inválido"})
			c.Abort()
			return
		}

		claims := &KeycloakClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, jwks.Keyfunc, cfg.parserOptions()...)
		if err != nil || !token.Valid {
			// Migração: aceita o token HS256 antigo só se ALLOW_LEGACY_JWT estiver ligado.
			if cfg.allowLegacy {
				if userID, legacyErr := parseLegacyToken(tokenString); legacyErr == nil {
					c.Set("userID", userID)
					c.Next()
					return
				}
			}
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":  "Acesso negado: Token inválido ou expirado",
				"detail": authErrorDetail(err),
			})
			c.Abort()
			return
		}

		// A identidade vem inteira do Keycloak: o `sub` do token É o userID.
		// Nada é consultado nem criado na tabela `users` aqui.
		userID, err := uuid.Parse(strings.TrimSpace(claims.Subject))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":  "Acesso negado: token sem `sub` válido",
				"detail": "o `sub` do token precisa ser um UUID",
			})
			c.Abort()
			return
		}

		// Com KEYCLOAK_ONLY, o banco não entra na autenticação: o token basta.
		// Sem ele, garantimos a linha espelho em `users` (id == sub) para as
		// colunas que são do app: xp, plano, streak, bio… Nome, e-mail e role
		// continuam vindo do token nos dois casos.
		if !cfg.onlyToken {
			if err := syncLocalUser(claims, userID); err != nil {
				log.Printf("Keycloak: erro ao sincronizar o utilizador %s: %v", userID, err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":  "Erro ao carregar o utilizador do token",
					"detail": err.Error(),
				})
				c.Abort()
				return
			}
		}

		// O AuthMiddleware é a ÚNICA função do sistema inteiro que "Seta" o ID!
		c.Set("userID", userID)
		c.Set("userEmail", claimEmail(claims))
		c.Set("userName", claims.Name)
		c.Set("userRoles", claims.RealmAccess.Roles)

		c.Next()
	}
}

// bearerToken extrai o token do header Authorization (case-insensitive no "Bearer").
func bearerToken(c *gin.Context) (string, bool) {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(authHeader) < 8 || !strings.EqualFold(authHeader[:7], "Bearer ") {
		return "", false
	}

	tokenString := strings.TrimSpace(authHeader[7:])
	if tokenString == "" {
		return "", false
	}
	return tokenString, true
}

// authErrorDetail devolve uma pista curta do motivo da rejeição, para facilitar
// o debug no Postman sem expor o conteúdo do token.
func authErrorDetail(err error) string {
	switch {
	case err == nil:
		return "token inválido"
	case errors.Is(err, jwt.ErrTokenExpired):
		return "token expirado"
	case errors.Is(err, jwt.ErrTokenInvalidIssuer):
		return "issuer diferente do configurado (confira KEYCLOAK_URL/KEYCLOAK_ISSUER)"
	case errors.Is(err, jwt.ErrTokenInvalidAudience):
		return "audience diferente do configurado (KEYCLOAK_AUDIENCE)"
	case errors.Is(err, jwt.ErrTokenSignatureInvalid), errors.Is(err, jwt.ErrTokenUnverifiable):
		return "assinatura não confere com as chaves do realm"
	default:
		return "token inválido"
	}
}

// parseLegacyToken valida o token HS256 gerado pelo antigo /v1/login.
// Usado apenas quando ALLOW_LEGACY_JWT=true.
func parseLegacyToken(tokenString string) (uuid.UUID, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return uuid.Nil, fmt.Errorf("JWT_SECRET não configurado")
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return uuid.Nil, fmt.Errorf("token legado inválido")
	}

	userIDStr, _ := claims["sub"].(string)
	return uuid.Parse(userIDStr)
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

		// O plano é dado do app (pagamento), não do Keycloak: não existe no
		// token, só no banco. Usamos Find (e não First) porque "sem linha" é um
		// caso normal quando KEYCLOAK_ONLY está ligado — aí não há plano para
		// checar e a catraca deixa passar.
		var found []models.User
		if err := database.DB.Model(&models.User{}).
			Select("subscription_type").
			Where("id = ?", userID).
			Limit(1).
			Find(&found).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao verificar assinatura"})
			c.Abort()
			return
		}

		if len(found) == 0 {
			c.Next()
			return
		}
		user := found[0]

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

// AdminOnly - Middleware que bloqueia qualquer um que não seja DEV ou ADMIN.
// Quem manda é a role do realm no token (admin / dev / realm-admin) — o
// account_type do banco não é mais consultado.
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := utils.GetUserID(c); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autorizado"})
			c.Abort()
			return
		}

		if !HasAdminRole(GetTokenRoles(c)) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":  "ACESSO NEGADO: Área restrita para a Administração.",
				"detail": "o token precisa trazer a role `admin` do realm",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
