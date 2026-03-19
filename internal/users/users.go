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

	// 1. Busca os dados do Usuário
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuário não encontrado"})
		return
	}

	// ---------------------------------------------------------
	// 🧙‍♂️ MÁGICA: Calcular Resumo de Desempenho
	// ---------------------------------------------------------
	var skills []string
	database.DB.Table("notebooks").
		Select("DISTINCT name").
		Where("created_by_id = ?", userID).
		Limit(10).
		Scan(&skills)

	if skills == nil {
		skills = []string{}
	}

	var totalNotebooks, totalPages int64
	database.DB.Model(&models.Notebook{}).Where("created_by_id = ?", userID).Count(&totalNotebooks)
	database.DB.Model(&models.Page{}).Where("created_by_id = ?", userID).Count(&totalPages)

	achievements := []gin.H{
		{"id": 1, "name": "Mestre das Revisões", "icon_url": "url-do-trofeu", "is_unlocked": true},
		{"id": 2, "name": "Foco Absoluto", "icon_url": "url-do-trofeu", "is_unlocked": true},
		{"id": 3, "name": "Escritor Ávido", "icon_url": "url-do-trofeu", "is_unlocked": false},
	}

	// ---------------------------------------------------------
	// 🌟 NOVO: As Estatísticas (Usage Stats) pedidas pelo Mayan
	// ---------------------------------------------------------
	var qtdNotebooks, qtdNotes, qtdCycles int64
	database.DB.Model(&models.Notebook{}).Where("created_by_id = ?", userID).Count(&qtdNotebooks)
	database.DB.Model(&models.StudyCycle{}).Where("created_by_id = ?", userID).Count(&qtdCycles)

	// Como Notas pertencem ao Space, contamos as notas dentro dos Spaces que ele é dono
	database.DB.Table("quick_notes").
		Joins("JOIN spaces ON spaces.id = quick_notes.space_id").
		Where("spaces.owner_id = ?", userID).
		Count(&qtdNotes)

	// 🌟 NOVO: Busca de todos os Planos de Estudo do usuário (A Agenda Inteira)
	var studyPlans []models.StudyPlan
	database.DB.Table("study_plans").
		Select("DISTINCT study_plans.*").
		Joins("JOIN spaces ON spaces.id = study_plans.space_id").
		Joins("LEFT JOIN space_permissions ON space_permissions.space_id = spaces.id").
		Where("spaces.owner_id = ? OR space_permissions.user_id = ?", userID, userID).
		Find(&studyPlans)

	if studyPlans == nil {
		studyPlans = []models.StudyPlan{}
	}

	// ---------------------------------------------------------
	// 2. Busca os Spaces que ele é o DONO
	// ---------------------------------------------------------
	var ownedSpaces []models.Space
	database.DB.Select("id, name, color_hex, category").Where("owner_id = ?", userID).Find(&ownedSpaces)
	if ownedSpaces == nil {
		ownedSpaces = []models.Space{}
	}

	// ---------------------------------------------------------
	// 3. Busca os Spaces que ele é CONVIDADO (🌟 AGORA COM A FOTO DO DONO)
	// ---------------------------------------------------------
	var guestSpaces []struct {
		SpaceID           string `json:"space_id"`
		Name              string `json:"name"`
		ColorHex          string `json:"color_hex"`
		AccessLevel       string `json:"access_level"`
		OwnerName         string `json:"owner_name"`
		ProfilePictureURL string `json:"profile_picture_url"` // 👈 NOVA COLUNA INJETADA
		UpdatedAt         string `json:"updated_at"`
	}

	database.DB.Table("spaces").
		Select("spaces.id as space_id, spaces.name, spaces.color_hex, space_permissions.access_level, users.full_name as owner_name, users.profile_pic as profile_picture_url, spaces.updated_at").
		Joins("join space_permissions on space_permissions.space_id = spaces.id").
		Joins("join users on users.id = spaces.owner_id").
		Where("space_permissions.user_id = ?", userID).
		Scan(&guestSpaces)

	if guestSpaces == nil {
		guestSpaces = []struct {
			SpaceID           string `json:"space_id"`
			Name              string `json:"name"`
			ColorHex          string `json:"color_hex"`
			AccessLevel       string `json:"access_level"`
			OwnerName         string `json:"owner_name"`
			ProfilePictureURL string `json:"profile_picture_url"`
			UpdatedAt         string `json:"updated_at"`
		}{}
	}

	// 4. Monta o JSON GIGANTE de Resposta com tudo que o Front-end pediu
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
		"usage_stats": gin.H{ // 👈 AS CONTAGENS AQUI
			"qtd_notebooks": qtdNotebooks,
			"qtd_notes":     qtdNotes,
			"qtd_cycles":    qtdCycles,
		},
		"study_plans":  studyPlans, // 👈 A AGENDA COMPLETA AQUI
		"owned_spaces": ownedSpaces,
		"guest_spaces": guestSpaces, // 👈 OS CONVITES COM FOTO AQUI
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
