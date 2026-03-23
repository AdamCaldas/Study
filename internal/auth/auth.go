package auth

import (
	"fmt"
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
)

// ==========================================================
// 1. REGISTRAR USUÁRIO (Cadastro Direto - Sem Validação)
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

	// 🔓 MUDANÇA AQUI: IsEmailVerified agora é TRUE por padrão
	newUser := models.User{
		FullName:        input.FullName,
		Email:           input.Email,
		CPF:             input.CPF,
		Password:        string(hashedPassword),
		IsEmailVerified: true,
	}
	database.DB.Create(&newUser)

	// OTP e Envio de E-mail removidos temporariamente para agilizar o teste

	c.JSON(http.StatusCreated, gin.H{
		"message": "Conta criada com sucesso! Você já pode fazer login.",
		"email":   newUser.Email,
	})
}

// ==========================================================
// 2. VALIDAR CÓDIGO (Desativado/Mantido para compatibilidade)
// ==========================================================
func VerifyEmailCode(c *gin.Context) {
	// Como você desativou no back, este endpoint apenas retorna sucesso se chamado
	c.JSON(http.StatusOK, gin.H{"message": "E-mail já está verificado no sistema."})
}

// ==========================================================
// 3. LOGIN (Sem trava de E-mail)
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

	// 🔓 MUDANÇA AQUI: Trava de verificação comentada
	/*
		if !user.IsEmailVerified {
			c.JSON(http.StatusForbidden, gin.H{"error": "Sua conta ainda não foi verificada."})
			return
		}
	*/

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
