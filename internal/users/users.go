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

	// 1. Busca os dados do Usuário (Nome, Email, etc)
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuário não encontrado"})
		return
	}

	// 2. Busca os Spaces que ele é o DONO (OwnerID == userID)
	var ownedSpaces []models.Space
	database.DB.Select("id, name, color_hex, category").Where("owner_id = ?", userID).Find(&ownedSpaces)

	// 3. Busca os Spaces que ele é CONVIDADO (Membro)
	// Fazemos um JOIN matador para trazer o nome do Space e qual o AccessLevel dele lá dentro
	var guestSpaces []struct {
		SpaceID     string `json:"space_id"`
		Name        string `json:"name"`
		ColorHex    string `json:"color_hex"`
		AccessLevel string `json:"access_level"`
	}

	database.DB.Table("spaces").
		Select("spaces.id as space_id, spaces.name, spaces.color_hex, space_permissions.access_level").
		Joins("join space_permissions on space_permissions.space_id = spaces.id").
		Where("space_permissions.user_id = ?", userID).
		Scan(&guestSpaces)

	// Garante que o Front-end receba um array vazio [] em vez de 'null' se não houver nada
	if guestSpaces == nil {
		guestSpaces = []struct {
			SpaceID     string `json:"space_id"`
			Name        string `json:"name"`
			ColorHex    string `json:"color_hex"`
			AccessLevel string `json:"access_level"`
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
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	// Atualiza apenas os campos enviados no banco
	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"full_name": input.FullName,
		"age":       input.Age,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar perfil"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Perfil atualizado com sucesso!"})
}
