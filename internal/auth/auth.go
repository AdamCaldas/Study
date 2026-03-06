package auth

import (
	"net/http"
	"os"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Estrutura que define o que esperamos receber do Frontend no cadastro
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	// 2. Criptografa a senha (Bcrypt)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criptografar senha"})
		return
	}

	// 3. Monta o novo usuário
	newUser := models.User{
		FullName: input.FullName,
		Email:    input.Email,
		CPF:      input.CPF,
		Password: string(hashedPassword),
	}

	// 4. Tenta salvar no banco de dados
	if err := database.DB.Create(&newUser).Error; err != nil {
		// Se der erro (ex: Email ou CPF já existem), o GORM avisa
		c.JSON(http.StatusConflict, gin.H{"error": "Email ou CPF já cadastrados"})
		return
	}

	// 5. Sucesso!
	c.JSON(http.StatusCreated, gin.H{
		"message": "Usuário criado com sucesso!",
		"user": gin.H{
			"id":        newUser.ID,
			"full_name": newUser.FullName,
			"email":     newUser.Email,
		},
	})
}

// O que esperamos receber do Frontend no login
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

	// 4. Se a senha bater, gera o Token JWT
	// O token carrega o ID do usuário e expira em 24 horas
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

	// 5. Devolve o token e os dados básicos para o frontend
	c.JSON(http.StatusOK, gin.H{
		"message": "Login realizado com sucesso!",
		"token":   tokenString,
		"user": gin.H{
			"id":                user.ID,
			"full_name":         user.FullName,
			"subscription_type": user.SubscriptionType,
		},
	})
}
