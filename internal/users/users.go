package users

import (
	"net/http"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt" // 👈 A mágica da criptografia aqui!
)

// ==========================================================
// 1️⃣ Puxa o Perfil Completo (Raio-X de Desempenho para o Front)
// ==========================================================
func GetMyProfile(c *gin.Context) {
	userIDInterface, _ := c.Get("userID")

	// Converte ID para UUID com segurança
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro de autenticação"})
		return
	}

	// 1. Busca os dados do Usuário (Nome, Bio, Foto, Banner, XP, Streak, etc)
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuário não encontrado"})
		return
	}

	// ---------------------------------------------------------
	// 🧙‍♂️ MÁGICA: Calcular Resumo de Desempenho
	// ---------------------------------------------------------

	// A. Busca as "Habilidades" (Nomes dos Cadernos que o usuário criou)
	var skills []string
	database.DB.Table("notebooks").
		Select("DISTINCT name").
		Where("created_by_id = ?", userID).
		Limit(10). // Pega só as top 10 habilidades
		Scan(&skills)

	if skills == nil {
		skills = []string{}
	}

	// B. Contagem total de Cadernos e Páginas Criadas
	var totalNotebooks int64
	var totalPages int64
	database.DB.Model(&models.Notebook{}).Where("created_by_id = ?", userID).Count(&totalNotebooks)
	database.DB.Model(&models.Page{}).Where("created_by_id = ?", userID).Count(&totalPages)

	// C. Busca as "Conquistas Recentes" (Mock para o Front-end montar os ícones)
	achievements := []gin.H{
		{"id": 1, "name": "Mestre das Revisões", "icon_url": "url-do-trofeu", "is_unlocked": true},
		{"id": 2, "name": "Foco Absoluto", "icon_url": "url-do-trofeu", "is_unlocked": true},
		{"id": 3, "name": "Escritor Ávido", "icon_url": "url-do-trofeu", "is_unlocked": false},
	}
	// ---------------------------------------------------------

	// 2. Busca os Spaces que ele é o DONO
	var ownedSpaces []models.Space
	database.DB.Select("id, name, color_hex, category").Where("owner_id = ?", userID).Find(&ownedSpaces)

	// 3. Busca os Spaces que ele é CONVIDADO
	var guestSpaces []struct {
		SpaceID     string `json:"space_id"`
		Name        string `json:"name"`
		ColorHex    string `json:"color_hex"`
		AccessLevel string `json:"access_level"`
		OwnerName   string `json:"owner_name"`
		UpdatedAt   string `json:"updated_at"`
	}

	database.DB.Table("spaces").
		Select("spaces.id as space_id, spaces.name, spaces.color_hex, space_permissions.access_level, users.full_name as owner_name, spaces.updated_at").
		Joins("join space_permissions on space_permissions.space_id = spaces.id").
		Joins("join users on users.id = spaces.owner_id").
		Where("space_permissions.user_id = ?", userID).
		Scan(&guestSpaces)

	// Garante arrays vazios no JSON
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

	// 4. Monta o JSON de Resposta
	c.JSON(http.StatusOK, gin.H{
		"profile": user,
		"stats": gin.H{
			"xp":                  user.XP,
			"current_streak":      user.CurrentStreak,
			"skills":              skills,
			"recent_achievements": achievements,
			"total_notebooks":     totalNotebooks,
			"total_pages":         totalPages,
		},
		"owned_spaces": ownedSpaces,
		"guest_spaces": guestSpaces,
	})
}

// ==========================================================
// 2️⃣ Atualiza os dados do Perfil (Incluindo os novos do Mayan)
// ==========================================================
func UpdateMyProfile(c *gin.Context) {
	userID, _ := c.Get("userID")

	var input struct {
		FullName         string     `json:"full_name"`
		Nickname         string     `json:"nickname"`
		Bio              string     `json:"bio"`
		BirthDate        *time.Time `json:"birth_date"`
		Gender           string     `json:"gender"`
		ProfilePic       string     `json:"profile_picture_url"`
		BannerPic        string     `json:"banner_picture_url"`
		IsProfilePrivate *bool      `json:"is_profile_private"`
		Title            string     `json:"title"`    // 👈 NOVO: "DESENVOLVEDOR"
		Location         string     `json:"location"` // 👈 NOVO: "Brasil"
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if input.FullName != "" {
		updates["full_name"] = input.FullName
	}
	if input.Nickname != "" {
		updates["nickname"] = input.Nickname
	}
	if input.Bio != "" {
		updates["bio"] = input.Bio
	}
	if input.BirthDate != nil {
		updates["birth_date"] = input.BirthDate
	}
	if input.Gender != "" {
		updates["gender"] = input.Gender
	}
	if input.ProfilePic != "" {
		updates["profile_pic"] = input.ProfilePic
	}
	if input.BannerPic != "" {
		updates["banner_pic"] = input.BannerPic
	}
	if input.Title != "" {
		updates["title"] = input.Title
	}
	if input.Location != "" {
		updates["location"] = input.Location
	}
	if input.IsProfilePrivate != nil {
		updates["is_profile_private"] = *input.IsProfilePrivate
	}

	if len(updates) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "Nada para atualizar."})
		return
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar perfil"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Perfil atualizado com sucesso!"})
}

// ==========================================================
// 🔐 3️⃣ Atualizar Senha (Segurança com Bcrypt)
// ==========================================================
func UpdatePassword(c *gin.Context) {
	userID, _ := c.Get("userID")

	var input struct {
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A nova senha deve ter pelo menos 6 caracteres."})
		return
	}

	// Criptografa a nova senha gerando um hash seguro
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao processar nova senha"})
		return
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("password", string(hashedPassword)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar nova senha no banco"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Senha atualizada com sucesso!"})
}

// ==========================================================
// 🚨 4️⃣ Deleta a própria conta (O Botão Vermelho)
// ==========================================================
func DeleteMyAccount(c *gin.Context) {
	userID, _ := c.Get("userID")

	if err := database.DB.Where("id = ?", userID).Delete(&models.User{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao excluir conta."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Conta excluída com sucesso."})
}
