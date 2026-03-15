package users

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
)

// ==========================================================
// 1️⃣ Puxa o Perfil + Separação de Spaces (Dono vs Convidado)
// ==========================================================
func GetMyProfile(c *gin.Context) {
	userID, _ := c.Get("userID")

	// 1. Busca os dados do Usuário (Nome, Email, Foto, etc)
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuário não encontrado"})
		return
	}

	// 2. Busca os Spaces que ele é o DONO (OwnerID == userID)
	var ownedSpaces []models.Space
	database.DB.Select("id, name, color_hex, category").Where("owner_id = ?", userID).Find(&ownedSpaces)

	// 3. Busca os Spaces que ele é CONVIDADO (Membro) - Agora com Nome do Dono e Data!
	var guestSpaces []struct {
		SpaceID     string `json:"space_id"`
		Name        string `json:"name"`
		ColorHex    string `json:"color_hex"`
		AccessLevel string `json:"access_level"`
		OwnerName   string `json:"owner_name"` // 👈 Traz o nome do dono para o Front-end
		UpdatedAt   string `json:"updated_at"` // 👈 Traz a data de modificação
	}

	database.DB.Table("spaces").
		Select("spaces.id as space_id, spaces.name, spaces.color_hex, space_permissions.access_level, users.full_name as owner_name, spaces.updated_at").
		Joins("join space_permissions on space_permissions.space_id = spaces.id").
		Joins("join users on users.id = spaces.owner_id"). // Faz a ponte para pegar o nome do dono
		Where("space_permissions.user_id = ?", userID).
		Scan(&guestSpaces)

	// Garante que o Front-end receba um array vazio [] em vez de 'null' se não houver nada
	if guestSpaces == nil {
		guestSpaces = []struct {
			SpaceID     string `json:"space_id"`
			Name        string `json:"name"`
			ColorHex    string `json:"color_hex"`
			AccessLevel string `json:"access_level"`
			OwnerName   string `json:"owner_name"`
			UpdatedAt   string `json:"updated_at"`
		}{}
	}
	if ownedSpaces == nil {
		ownedSpaces = []models.Space{}
	}

	// 4. Monta o JSON de Resposta Perfeito
	c.JSON(http.StatusOK, gin.H{
		"profile":      user,
		"owned_spaces": ownedSpaces, // Spaces que ele manda
		"guest_spaces": guestSpaces, // Spaces que ele é só convidado
	})
}

// ==========================================================
// 2️⃣ Atualiza os dados do Perfil
// ==========================================================
func UpdateMyProfile(c *gin.Context) {
	userID, _ := c.Get("userID")

	// Adapte os campos aqui conforme o que você tem na sua models.User
	var input struct {
		FullName string `json:"full_name"`
		Age      int    `json:"age"`
		PhotoURL string `json:"photo_url"` // 👈 Novo campo para a foto!
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	// Atualiza apenas os campos enviados no banco
	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"full_name": input.FullName,
		"age":       input.Age,
		"photo_url": input.PhotoURL, // 👈 Salva a foto no banco
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar perfil"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Perfil atualizado com sucesso!"})
}

// ==========================================================
// 3️⃣ Deleta a própria conta (O Botão Vermelho)
// ==========================================================
func DeleteMyAccount(c *gin.Context) {
	userID, _ := c.Get("userID")

	// Apaga a conta do usuário no banco de dados
	if err := database.DB.Where("id = ?", userID).Delete(&models.User{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao excluir conta. Tente novamente mais tarde."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Conta excluída com sucesso. Sentiremos sua falta!"})
}
