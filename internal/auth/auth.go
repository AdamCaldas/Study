package auth

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils" // 👈 Nosso carteiro de e-mail

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ==========================================================
// 1. REGISTRAR USUÁRIO (Criar conta bloqueada e mandar OTP)
// ==========================================================
type RegisterInput struct {
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	CPF      string `json:"cpf" binding:"required,len=11"`
	Password string `json:"password" binding:"required,min=6"`
}

func Register(c *gin.Context) {
	var input RegisterInput

	// 1. Valida se o JSON recebido está correto
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: Verifique se preencheu todos os campos corretamente."})
		return
	}

	// 2. 🛡️ REGRA DE OURO: 1 CPF = 1 EMAIL. Não deixa duplicar!
	var existingUser models.User
	if err := database.DB.Where("email = ? OR cpf = ?", input.Email, input.CPF).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Já existe uma conta registrada com este E-mail ou CPF."})
		return
	}

	// 3. Criptografa a senha (Bcrypt)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criptografar senha"})
		return
	}

	// 4. Monta o novo usuário NASCENDO BLOQUEADO
	newUser := models.User{
		FullName:        input.FullName,
		Email:           input.Email,
		CPF:             input.CPF,
		Password:        string(hashedPassword),
		IsEmailVerified: false, // 🔒 Nasce bloqueado!
	}
	database.DB.Create(&newUser)

	// 5. Gera o código OTP de 6 dígitos aleatórios
	rand.Seed(time.Now().UnixNano())
	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	// Salva o código no banco com validade de 10 minutos
	otp := models.VerificationCode{
		Email:     newUser.Email,
		Code:      code,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	database.DB.Create(&otp)

	// 6. 📧 Dispara o E-mail em background (Goroutine)
	go utils.SendVerificationEmail(newUser.Email, newUser.FullName, code)

	// 7. Sucesso!
	c.JSON(http.StatusCreated, gin.H{
		"message": "Conta criada! Um código de 6 dígitos foi enviado para o seu e-mail.",
		"email":   newUser.Email,
	})
}

// ==========================================================
// 2. VALIDAR CÓDIGO (Quebrar o cadeado da conta)
// ==========================================================
func VerifyEmailCode(c *gin.Context) {
	var input struct {
		Email string `json:"email" binding:"required,email"`
		Code  string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "E-mail e código são obrigatórios."})
		return
	}

	var otp models.VerificationCode
	if err := database.DB.Where("email = ? AND code = ?", input.Email, input.Code).First(&otp).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Código inválido ou incorreto."})
		return
	}

	// Verifica se passou de 10 minutos
	if time.Now().After(otp.ExpiresAt) {
		database.DB.Delete(&otp) // Limpa o código vencido
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Este código expirou. Solicite um novo."})
		return
	}

	// 🔓 QUEBRA O CADEADO: Atualiza o usuário para VERIFICADO
	database.DB.Model(&models.User{}).Where("email = ?", input.Email).Update("is_email_verified", true)

	// Apaga o código usado por segurança (OTP só serve 1 vez)
	database.DB.Delete(&otp)

	c.JSON(http.StatusOK, gin.H{"message": "E-mail verificado com sucesso! Você já pode fazer login."})
}

// ==========================================================
// 3. LOGIN (Com trava de segurança do E-mail)
// ==========================================================
type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func Login(c *gin.Context) {
	var input LoginInput

	// 1. Valida o JSON recebido
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	var user models.User

	// 2. Busca o usuário no banco pelo e-mail
	if err := database.DB.Where("email = ?", input.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "E-mail ou senha incorretos"})
		return
	}

	// 3. Compara a senha enviada com a senha criptografada do banco
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "E-mail ou senha incorretos"})
		return
	}

	// 🚨 4. NOVA TRAVA DE SEGURANÇA: O E-mail foi validado?
	if !user.IsEmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": "Sua conta ainda não foi verificada. Cheque seu e-mail para pegar o código!"})
		return
	}

	// 5. Se a senha bater e o email estiver validado, gera o Token JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID,
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	})

	// Assina o token com a nossa chave secreta do arquivo .env
	secret := os.Getenv("JWT_SECRET")
	tokenString, err := token.SignedString([]byte(secret))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao gerar o token de acesso"})
		return
	}

	// 6. Devolve o token e os dados básicos para o frontend
	c.JSON(http.StatusOK, gin.H{
		"message": "Login realizado com sucesso!",
		"token":   tokenString,
		"user": gin.H{
			"id":                user.ID,
			"full_name":         user.FullName,
			"subscription_type": user.SubscriptionType,
			"account_type":      user.AccountType,
		},
	})
}

// ==========================================================
// 4. ESQUECI A SENHA (Gera o Link)
// ==========================================================
func ForgotPassword(c *gin.Context) {
	var input struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "E-mail obrigatório."})
		return
	}

	var user models.User
	if err := database.DB.Where("email = ?", input.Email).First(&user).Error; err != nil {
		// Por segurança, sempre dizemos que enviamos para não dar dica a hackers se o e-mail existe
		c.JSON(http.StatusOK, gin.H{"message": "Se o e-mail existir no sistema, você receberá um link de redefinição."})
		return
	}

	// Apaga tokens velhos se o cara clicar 2 vezes no botão
	database.DB.Where("email = ?", input.Email).Delete(&models.PasswordReset{})

	// Gera o Token do link
	token := uuid.New().String()
	database.DB.Create(&models.PasswordReset{
		Email:     user.Email,
		Token:     token,
		ExpiresAt: time.Now().Add(1 * time.Hour), // Vale por 1 hora
	})

	// 🔗 O link que o seu Front-end vai escutar
	frontEndURL := os.Getenv("FRONTEND_URL") // Ex: http://localhost:3000
	if frontEndURL == "" {
		frontEndURL = "http://localhost:3000"
	}
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", frontEndURL, token)

	// 📧 Dispara o E-mail em segundo plano
	go utils.SendPasswordResetEmail(user.Email, user.FullName, resetLink)

	c.JSON(http.StatusOK, gin.H{"message": "Se o e-mail existir no sistema, você receberá um link de redefinição."})
}

// ==========================================================
// 5. REDEFINIR SENHA (Quando ele digita a nova senha no Front)
// ==========================================================
func ResetPassword(c *gin.Context) {
	var input struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token e nova senha são obrigatórios."})
		return
	}

	var reset models.PasswordReset
	if err := database.DB.Where("token = ?", input.Token).First(&reset).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Link de redefinição inválido ou já utilizado."})
		return
	}

	if time.Now().After(reset.ExpiresAt) {
		database.DB.Delete(&reset)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Este link expirou. Solicite um novo."})
		return
	}

	// Criptografa a nova senha e atualiza no banco
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	database.DB.Model(&models.User{}).Where("email = ?", reset.Email).Update("password", string(hashedPassword))

	// Queima o link (deleta do banco para não ser usado de novo)
	database.DB.Delete(&reset)

	c.JSON(http.StatusOK, gin.H{"message": "Senha alterada com sucesso! Você já pode entrar."})
}
