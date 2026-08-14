package auth

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/cache"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Tempo mínimo entre duas sincronizações do mesmo usuário com o Keycloak.
const keycloakUserCacheTTL = 5 * time.Minute

// KeycloakClaims são os campos do access token do Keycloak que nos interessam.
// O `sub` (id do usuário no Keycloak) vem de RegisteredClaims.Subject e é o
// ÚNICO lugar de onde a identidade sai: não consultamos a tabela `users`.
type KeycloakClaims struct {
	jwt.RegisteredClaims

	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
	GivenName         string `json:"given_name"`

	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// keycloakConfig é lido uma única vez, quando o middleware é montado.
type keycloakConfig struct {
	issuer      string // se vazio, não valida o `iss` (só a assinatura via JWKS)
	audience    string // se vazio, não valida o `aud`
	allowLegacy bool   // aceita também o token HS256 antigo (/v1/login)
	onlyToken   bool   // KEYCLOAK_ONLY: não espelha nada na tabela `users`
}

// loadKeycloakConfig lê o ambiente uma única vez, mesmo que o middleware seja
// montado em vários grupos de rotas (/v1/app e /v1/admin).
var loadKeycloakConfig = sync.OnceValue(readKeycloakConfig)

func readKeycloakConfig() keycloakConfig {
	cfg := keycloakConfig{
		issuer:      strings.TrimRight(os.Getenv("KEYCLOAK_ISSUER"), "/"),
		audience:    os.Getenv("KEYCLOAK_AUDIENCE"),
		allowLegacy: isEnvTrue("ALLOW_LEGACY_JWT"), // desligado por padrão
		onlyToken:   isEnvTrue("KEYCLOAK_ONLY"),
	}

	// Sem KEYCLOAK_ISSUER explícito, usamos a própria KEYCLOAK_URL como issuer
	// esperado. Se nenhuma das duas estiver definida (dev puro), validamos
	// apenas a assinatura contra o JWKS do realm.
	if cfg.issuer == "" {
		cfg.issuer = strings.TrimRight(os.Getenv("KEYCLOAK_URL"), "/")
	}

	if cfg.issuer == "" {
		log.Println("Keycloak: KEYCLOAK_ISSUER/KEYCLOAK_URL não definidos — validando apenas a assinatura do token (JWKS).")
	} else {
		log.Printf("Keycloak: issuer esperado = %s", cfg.issuer)
	}
	if cfg.allowLegacy {
		log.Println("⚠️  Keycloak: ALLOW_LEGACY_JWT ativo — tokens HS256 antigos (JWT_SECRET) continuam aceitos.")
	}
	if cfg.onlyToken {
		log.Println("Keycloak: KEYCLOAK_ONLY ativo — a identidade sai inteira do token, nada é lido nem criado na tabela `users`.")
	}

	return cfg
}

// KeycloakOnly diz se o app roda sem espelhar o usuário no banco: o `sub`, o
// nome, o e-mail e as roles saem todos do token, e a tabela `users` não é
// consultada para autenticar nem para criar um Space.
func KeycloakOnly() bool { return loadKeycloakConfig().onlyToken }

func isEnvTrue(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}

// parserOptions monta as validações do jwt/v5 a partir da configuração.
func (cfg keycloakConfig) parserOptions() []jwt.ParserOption {
	opts := []jwt.ParserOption{
		// Keycloak assina com RSA; travar os algoritmos evita ataques de
		// confusão de algoritmo (ex.: alg=none / alg=HS256).
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "PS256", "PS384", "PS512"}),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30 * time.Second),
	}
	if cfg.issuer != "" {
		opts = append(opts, jwt.WithIssuer(cfg.issuer))
	}
	if cfg.audience != "" {
		opts = append(opts, jwt.WithAudience(cfg.audience))
	}
	return opts
}

// syncLocalUser garante que existe uma linha em `users` com id == `sub` do
// token. A linha é um ESPELHO do Keycloak: quem manda em id, nome, e-mail e
// role é sempre o token. As colunas que são do app (xp, plano, streak, bio…)
// ficam só no banco, porque não existem no token.
//
// Roda no máximo uma vez a cada keycloakUserCacheTTL por usuário.
func syncLocalUser(claims *KeycloakClaims, userID uuid.UUID) error {
	cacheKey := "kc_sync:" + userID.String()
	if _, found := cache.AppCache.Get(cacheKey); found {
		return nil
	}

	email := claimEmail(claims)
	fullName := claimFullName(claims, email)

	var existing []models.User
	err := database.DB.Model(&models.User{}).
		Select("id", "full_name", "email", "account_type").
		Where("id = ?", userID).
		Limit(1).
		Find(&existing).Error
	if err != nil {
		return err
	}

	if len(existing) == 0 {
		if err := createMirroredUser(claims, userID, email, fullName); err != nil {
			return err
		}
		cache.AppCache.Set(cacheKey, true, keycloakUserCacheTTL)
		return nil
	}

	// Linha já existe: atualiza o que o Keycloak manda, se mudou.
	updates := map[string]interface{}{}
	if fullName != "" && existing[0].FullName != fullName {
		updates["full_name"] = fullName
	}
	if email != "" && !strings.EqualFold(existing[0].Email, email) {
		updates["email"] = email
	}
	// Só promove: nunca rebaixa quem virou TEACHER pelo /me/become-teacher.
	if HasAdminRole(claims.RealmAccess.Roles) && existing[0].AccountType != utils.RoleAdmin {
		updates["account_type"] = utils.RoleAdmin
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
			return err
		}
		log.Printf("Keycloak: perfil sincronizado do token para %s (%v)", userID, keysOf(updates))
	}

	cache.AppCache.Set(cacheKey, true, keycloakUserCacheTTL)
	return nil
}

func createMirroredUser(claims *KeycloakClaims, userID uuid.UUID, email, fullName string) error {
	if email == "" {
		return fmt.Errorf("token do Keycloak sem e-mail: adicione o mapper de e-mail no client")
	}

	// Senha aleatória: quem autentica é o Keycloak, esse hash nunca é usado.
	randomPassword, err := bcrypt.GenerateFromPassword([]byte(uuid.NewString()), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	newUser := models.User{
		ID:               userID, // 👈 o id É o `sub` do token
		FullName:         fullName,
		Email:            email,
		CPF:              "KC-" + uuid.NewString()[:8], // placeholder: a coluna é unique/not null
		Password:         string(randomPassword),
		IsEmailVerified:  claims.EmailVerified,
		AccountType:      accountTypeFromRoles(claims.RealmAccess.Roles),
		SubscriptionType: utils.PlanFreeTrial,
	}

	if err := database.DB.Create(&newUser).Error; err != nil {
		// Conta legada: mesmo e-mail com outro id (nasceu antes do Keycloak).
		// O e-mail é unique, então o insert falha. Isso precisa ser alinhado no
		// banco — o id da linha tem que virar o `sub`.
		if id, lookupErr := findUserID("LOWER(email) = ?", email); lookupErr == nil && id != uuid.Nil {
			return fmt.Errorf("conta legada %s tem o e-mail %s mas id diferente do sub %s: alinhe o id da linha com o sub", id, email, userID)
		}
		return err
	}

	log.Printf("Keycloak: linha criada em users com id = sub (%s <%s>)", newUser.ID, newUser.Email)
	return nil
}

// findUserID devolve o id do usuário que casa com a condição, ou uuid.Nil se
// não existir. Usamos Find (e não First) para que "não encontrado" seja um caso
// normal, sem poluir o log do GORM com "record not found".
func findUserID(condition string, args ...interface{}) (uuid.UUID, error) {
	var found []models.User
	err := database.DB.Model(&models.User{}).
		Select("id").
		Where(condition, args...).
		Limit(1).
		Find(&found).Error
	if err != nil {
		return uuid.Nil, err
	}
	if len(found) == 0 {
		return uuid.Nil, nil
	}
	return found[0].ID, nil
}

func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// claimFullName monta o nome a partir do token.
func claimFullName(claims *KeycloakClaims, email string) string {
	for _, candidate := range []string{claims.Name, claims.GivenName, claims.PreferredUsername} {
		if name := strings.TrimSpace(candidate); name != "" {
			return name
		}
	}
	if idx := strings.Index(email, "@"); idx > 0 {
		return email[:idx]
	}
	return "Usuário Keycloak"
}

// accountTypeFromRoles traduz as roles do realm para o account_type do banco.
func accountTypeFromRoles(roles []string) string {
	for _, role := range roles {
		switch strings.ToLower(role) {
		case "admin", "dev", "realm-admin":
			return utils.RoleAdmin
		case "teacher", "professor":
			return utils.RoleTeacher
		}
	}
	return utils.RoleUser
}

// claimEmail pega o e-mail do token, caindo para o preferred_username quando o
// realm usa e-mail como username e não tem o mapper de e-mail configurado.
func claimEmail(claims *KeycloakClaims) string {
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if email == "" && strings.Contains(claims.PreferredUsername, "@") {
		email = strings.ToLower(strings.TrimSpace(claims.PreferredUsername))
	}
	return email
}

// HasTeacherRole diz se o token traz a role de professor do realm.
func HasTeacherRole(roles []string) bool {
	for _, role := range roles {
		switch strings.ToLower(role) {
		case "teacher", "professor":
			return true
		}
	}
	return false
}

// HasAdminRole diz se o token traz uma role administrativa do realm.
func HasAdminRole(roles []string) bool {
	for _, role := range roles {
		switch strings.ToLower(role) {
		case "admin", "dev", "realm-admin":
			return true
		}
	}
	return false
}

// GetTokenRoles devolve as roles do realm que o middleware guardou no contexto.
func GetTokenRoles(c *gin.Context) []string {
	if value, exists := c.Get("userRoles"); exists {
		if roles, ok := value.([]string); ok {
			return roles
		}
	}
	return nil
}
