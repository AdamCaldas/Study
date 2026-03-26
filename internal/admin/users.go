package admin

import (
	"fmt"
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

// ==========================================================
// 💀 O BOTÃO DO THANOS: Aniquila um usuário e TODO o seu império
// ==========================================================
func DeleteAnyUser(c *gin.Context) {
	userID := c.Param("id")

	// Inicia uma transação (Ou apaga tudo, ou não apaga nada)
	tx := database.DB.Begin()

	// 1. Remove ele de todos os Spaces onde ele era apenas convidado
	tx.Unscoped().Where("user_id = ?", userID).Delete(&models.SpacePermission{})
	tx.Unscoped().Where("user_id = ?", userID).Delete(&models.SpaceJoinRequest{})

	// 2. Apaga TODOS os Spaces onde ele é o DONO
	// (Como a gente configurou o OnDelete:CASCADE na model, o banco de dados
	// vai apagar os Cadernos, Páginas, Ciclos e Quizzes dele automaticamente!)
	tx.Unscoped().Where("owner_id = ?", userID).Delete(&models.Space{})

	// 3. Apaga os rastros soltos (Pomodoros, Logs de Humor, Atividades e Pagamentos)
	tx.Unscoped().Where("user_id = ?", userID).Delete(&models.PomodoroSession{})
	tx.Unscoped().Where("user_id = ?", userID).Delete(&models.MoodCheckIn{})
	tx.Unscoped().Where("user_id = ?", userID).Delete(&models.ActivityLog{})
	tx.Unscoped().Where("user_id = ?", userID).Delete(&models.PaymentHistory{})

	// 4. O GOLPE FINAL: Deleta o Usuário do banco de dados real (Unscoped bypassa o soft delete)
	result := tx.Unscoped().Where("id = ?", userID).Delete(&models.User{})

	if result.Error != nil {
		tx.Rollback() // Deu erro? Cancela a destruição pra não deixar o banco quebrado
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha na aniquilação: " + result.Error.Error()})
		return
	}

	if result.RowsAffected == 0 {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuário fantasma! Ele não existe ou já foi de arrasta pra cima."})
		return
	}

	// Confirma a destruição total!
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"message": "Usuário ANIQUILADO com sucesso! Foi de base, virou saudade e não sobrou nem poeira no banco de dados. 💀💥🧹",
	})
}

// Estrutura para receber o array de IDs dos condenados
type MassDeleteInput struct {
	UserIDs []string `json:"user_ids" binding:"required,min=1"`
}

// ==========================================================
// 💥 O ESTALAR DE DEDOS DO THANOS: Aniquilação em Massa
// ==========================================================
func MassDeleteUsers(c *gin.Context) {
	var input MassDeleteInput

	// Recebe o array de IDs do Front-end
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Lista de alvos inválida. Mande um array 'user_ids' com pelo menos um ID."})
		return
	}

	// Inicia a transação (Ou apaga o bonde todo, ou não apaga ninguém)
	tx := database.DB.Begin()

	// 1. Remove a galera de todos os Spaces onde eram apenas convidados
	tx.Unscoped().Where("user_id IN ?", input.UserIDs).Delete(&models.SpacePermission{})
	tx.Unscoped().Where("user_id IN ?", input.UserIDs).Delete(&models.SpaceJoinRequest{})

	// 2. Apaga TODOS os Spaces onde eles eram os DONOS (O Cascata vai levar cadernos, páginas, etc)
	tx.Unscoped().Where("owner_id IN ?", input.UserIDs).Delete(&models.Space{})

	// 3. Apaga os rastros soltos da galera (Pomodoros, Logs de Humor, Atividades e Pagamentos)
	tx.Unscoped().Where("user_id IN ?", input.UserIDs).Delete(&models.PomodoroSession{})
	tx.Unscoped().Where("user_id IN ?", input.UserIDs).Delete(&models.MoodCheckIn{})
	tx.Unscoped().Where("user_id IN ?", input.UserIDs).Delete(&models.ActivityLog{})
	tx.Unscoped().Where("user_id IN ?", input.UserIDs).Delete(&models.PaymentHistory{})

	// 4. O GOLPE FINAL: Deleta os Usuários do banco de dados real
	result := tx.Unscoped().Where("id IN ?", input.UserIDs).Delete(&models.User{})

	if result.Error != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "A Manopla do Infinito falhou: " + result.Error.Error()})
		return
	}

	if result.RowsAffected == 0 {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Nenhum alvo encontrado! Talvez eles já tenham ido de arrasta pra cima."})
		return
	}

	// Confirma a aniquilação!
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("ANIQUILAÇÃO EM MASSA CONCLUÍDA! %d usuários foram de arrasta pra cima. O Thanos estalou os dedos e limpou o servidor! 💀💥🌩️", result.RowsAffected),
	})
}
