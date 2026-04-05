package auth

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
)

// ==========================================================
// 1. REGISTRAR USUÁRIO (Agora enviando e-mail de verdade!)
// ==========================================================
type RegisterInput struct {
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	CPF      string `json:"cpf" binding:"required,len=11"`
	Password string `json:"password" binding:"required,min=6"`
}

func Register(c *gin.Context) {
	var input RegisterInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: Verifique os campos."})
		return
	}

	var existingUser models.User
	if err := database.DB.Where("email = ? OR cpf = ?", input.Email, input.CPF).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Já existe uma conta com este E-mail ou CPF."})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criptografar senha"})
		return
	}

	// 🔒 Volta a ser FALSE, ele precisa verificar o e-mail agora
	newUser := models.User{
		FullName:        input.FullName,
		Email:           input.Email,
		CPF:             input.CPF,
		Password:        string(hashedPassword),
		IsEmailVerified: false,
	}
	database.DB.Create(&newUser)

	// 🎲 Gera código de 6 dígitos
	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	// Salva no banco de dados com validade de 10 minutos
	verificationCode := models.VerificationCode{
		Email:     newUser.Email,
		Code:      code,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	database.DB.Create(&verificationCode)

	// 📧 Dispara o e-mail (usando seu utils que já tá pronto!)
	go utils.SendVerificationEmail(newUser.Email, newUser.FullName, code)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Conta criada! Verifique seu e-mail para ativar a conta.",
		"email":   newUser.Email,
	})
}

// ==========================================================
// 2. VALIDAR CÓDIGO DE E-MAIL
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

	var verification models.VerificationCode
	if err := database.DB.Where("email = ? AND code = ?", input.Email, input.Code).First(&verification).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Código inválido ou incorreto."})
		return
	}

	if time.Now().After(verification.ExpiresAt) {
		database.DB.Delete(&verification)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Este código expirou. Solicite um novo."})
		return
	}

	// 🔓 Ativa a conta do usuário
	database.DB.Model(&models.User{}).Where("email = ?", input.Email).Update("is_email_verified", true)
	database.DB.Delete(&verification) // Limpa o código usado

	c.JSON(http.StatusOK, gin.H{"message": "E-mail verificado com sucesso! Você já pode fazer login."})
}

// ==========================================================
// 3. LOGIN (Com trava de E-mail religada)
// ==========================================================
type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func Login(c *gin.Context) {
	var input LoginInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	var user models.User

	if err := database.DB.Where("email = ?", input.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "E-mail ou senha incorretos"})
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "E-mail ou senha incorretos"})
		return
	}

	// 🔒 Religa a Trava de Verificação
	if !user.IsEmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": "Sua conta ainda não foi verificada. Cheque seu e-mail."})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID,
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	})

	secret := os.Getenv("JWT_SECRET")
	tokenString, err := token.SignedString([]byte(secret))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao gerar o token de acesso"})
		return
	}

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
// 4. ESQUECI A SENHA
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
		c.JSON(http.StatusOK, gin.H{"message": "Se o e-mail existir, você receberá um link."})
		return
	}

	database.DB.Where("email = ?", input.Email).Delete(&models.PasswordReset{})

	token := uuid.New().String()
	database.DB.Create(&models.PasswordReset{
		Email:     user.Email,
		Token:     token,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})

	frontEndURL := os.Getenv("FRONTEND_URL")
	if frontEndURL == "" {
		frontEndURL = "http://localhost:3000"
	}
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", frontEndURL, token)

	go utils.SendPasswordResetEmail(user.Email, user.FullName, resetLink)

	c.JSON(http.StatusOK, gin.H{"message": "Se o e-mail existir, você receberá um link."})
}

// ==========================================================
// 5. REDEFINIR SENHA
// ==========================================================
func ResetPassword(c *gin.Context) {
	var input struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados obrigatórios ausentes."})
		return
	}

	var reset models.PasswordReset
	if err := database.DB.Where("token = ?", input.Token).First(&reset).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Link inválido."})
		return
	}

	if time.Now().After(reset.ExpiresAt) {
		database.DB.Delete(&reset)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Link expirado."})
		return
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	database.DB.Model(&models.User{}).Where("email = ?", reset.Email).Update("password", string(hashedPassword))

	database.DB.Delete(&reset)

	c.JSON(http.StatusOK, gin.H{"message": "Senha alterada com sucesso!"})
}

// ==========================================================
// 6. 🌐 LOGIN/CADASTRO COM GOOGLE (O Passo 2)
// ==========================================================
type GoogleLoginInput struct {
	Token string `json:"token" binding:"required"`
}

func GoogleAuth(c *gin.Context) {
	var input GoogleLoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token do Google é obrigatório."})
		return
	}

	// 1. Valida o Token direto com os servidores do Google (Segurança Máxima)
	payload, err := idtoken.Validate(context.Background(), input.Token, "")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token do Google inválido ou expirado."})
		return
	}

	// 2. Extrai os dados que o Google devolveu
	email := payload.Claims["email"].(string)
	name := payload.Claims["name"].(string)
	picture := ""
	if pic, ok := payload.Claims["picture"]; ok {
		picture = pic.(string)
	}

	// 3. Procura no banco de dados se esse usuário já existe
	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		// 🚨 USUÁRIO NÃO EXISTE: Vamos cadastrar ele automaticamente!

		// Gera uma senha aleatória absurda só pro banco não reclamar
		randomPassword, _ := bcrypt.GenerateFromPassword([]byte(uuid.New().String()), bcrypt.DefaultCost)

		user = models.User{
			FullName:        name,
			Email:           email,
			ProfilePic:      picture,
			Password:        string(randomPassword),
			IsEmailVerified: true,                                // Já vem verificado pelo Google!
			CPF:             "GOOGLE-" + uuid.New().String()[:4], // Temporário pra não quebrar a regra de CPF único
		}

		if err := database.DB.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar conta via Google."})
			return
		}
	}

	// 4. Usuário logado/criado! Vamos gerar o Token JWT do StudFy
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID,
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	})

	secret := os.Getenv("JWT_SECRET")
	tokenString, _ := token.SignedString([]byte(secret))

	c.JSON(http.StatusOK, gin.H{
		"message": "Login com Google realizado com sucesso!",
		"token":   tokenString,
		"user": gin.H{
			"id":                user.ID,
			"full_name":         user.FullName,
			"profile_picture":   user.ProfilePic,
			"subscription_type": user.SubscriptionType,
			"account_type":      user.AccountType,
		},
	})
}
