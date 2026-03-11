package admin

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ListAllUsers - Traz todos os usuários da base (com opção de busca)
func ListAllUsers(c *gin.Context) {
	// Pega o termo de busca da URL (ex: /users?search=adam)
	search := c.Query("search")

	var users []models.User
	query := database.DB.Model(&models.User{})

	// Se o Admin digitou algo na busca, filtra por Nome, Email ou CPF
	if search != "" {
		query = query.Where("full_name ILIKE ? OR email ILIKE ? OR cpf ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	// Faz a busca ocultando a senha por segurança, ordenando pelos mais recentes
	if err := query.Omit("Password").Order("created_at desc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar usuários", "detalhe": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total": len(users),
		"users": users,
	})
}

// Estrutura para o Admin enviar as edições
type UpdateUserInput struct {
	FullName         string `json:"full_name"`
	Email            string `json:"email"`
	CPF              string `json:"cpf"`
	SubscriptionType string `json:"subscription_type"` // Ex: PREMIUM, FREE
	AccountType      string `json:"account_type"`      // Ex: USER, ADMIN, DEV
}

// UpdateAnyUser - Admin edita os dados de qualquer conta
func UpdateAnyUser(c *gin.Context) {
	targetUserID := c.Param("id")
	var input UpdateUserInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	// Monta apenas o que o Admin quer alterar
	updates := map[string]interface{}{}
	if input.FullName != "" {
		updates["full_name"] = input.FullName
	}
	if input.Email != "" {
		updates["email"] = input.Email
	}
	if input.CPF != "" {
		updates["cpf"] = input.CPF
	}
	if input.SubscriptionType != "" {
		updates["subscription_type"] = input.SubscriptionType
	}
	if input.AccountType != "" {
		updates["account_type"] = input.AccountType
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", targetUserID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar usuário", "detalhe": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Dados do usuário atualizados com sucesso pelo Modo Deus!"})
}

// Estrutura para a nova senha
type ForcePasswordInput struct {
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ForceChangePassword - Admin troca a senha de alguém que esqueceu ou foi hackeado
func ForceChangePassword(c *gin.Context) {
	targetUserID := c.Param("id")
	var input ForcePasswordInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Senha inválida (mínimo 6 caracteres)"})
		return
	}

	// O Admin digita a senha em texto limpo, nós criptografamos antes de salvar no banco
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criptografar a nova senha"})
		return
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", targetUserID).Update("password", string(hashedPassword)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar a nova senha no banco"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Senha alterada com sucesso! O usuário já pode logar com a nova senha."})
}
