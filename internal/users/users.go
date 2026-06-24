package users

import (
	"encoding/json"
	"net/http"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils" // 👈 Import global

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ==========================================================
// 1️⃣ Puxa o Perfil Completo
// ==========================================================
func GetMyProfile(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Utilizador não autenticado"})
		return
	}

	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuário não encontrado"})
		return
	}

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

	var qtdNotebooks, qtdNotes, qtdStrategies int64
	database.DB.Model(&models.Notebook{}).Where("created_by_id = ?", userID).Count(&qtdNotebooks)
	database.DB.Model(&models.StudyStrategy{}).Where("created_by_id = ?", userID).Count(&qtdStrategies)

	database.DB.Table("quick_notes").
		Joins("JOIN spaces ON spaces.id = quick_notes.space_id").
		Where("spaces.owner_id = ?", userID).
		Count(&qtdNotes)

	var userStrategies []models.StudyStrategy
	database.DB.Preload("Blocks").
		Joins("JOIN spaces ON spaces.id = study_strategies.space_id").
		Joins("LEFT JOIN space_permissions ON space_permissions.space_id = spaces.id").
		Where("spaces.owner_id = ? OR space_permissions.user_id = ?", userID, userID).
		Group("study_strategies.id").
		Find(&userStrategies)

	if userStrategies == nil {
		userStrategies = []models.StudyStrategy{}
	}

	var ownedSpaces []models.Space
	database.DB.Select("id, name, color_hex, category").Where("owner_id = ?", userID).Find(&ownedSpaces)
	if ownedSpaces == nil {
		ownedSpaces = []models.Space{}
	}

	var guestSpaces []struct {
		SpaceID           string `json:"space_id"`
		Name              string `json:"name"`
		ColorHex          string `json:"color_hex"`
		AccessLevel       string `json:"access_level"`
		OwnerName         string `json:"owner_name"`
		ProfilePictureURL string `json:"profile_picture_url"`
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

	var availabilityProfiles []models.AvailabilityProfile
	database.DB.Where("user_id = ?", userID).Find(&availabilityProfiles)
	if availabilityProfiles == nil {
		availabilityProfiles = []models.AvailabilityProfile{}
	}

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
		"usage_stats": gin.H{
			"qtd_notebooks":  qtdNotebooks,
			"qtd_notes":      qtdNotes,
			"qtd_strategies": qtdStrategies,
		},
		"study_strategies":      userStrategies,
		"owned_spaces":          ownedSpaces,
		"guest_spaces":          guestSpaces,
		"availability_profiles": availabilityProfiles,
	})
}

// ==========================================================
// 2️⃣ Atualiza os dados do Perfil
// ==========================================================
func UpdateMyProfile(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var input struct {
		FullName         string     `json:"full_name"`
		Nickname         string     `json:"nickname"`
		Bio              string     `json:"bio"`
		BirthDate        *time.Time `json:"birth_date"`
		Gender           string     `json:"gender"`
		ProfilePic       string     `json:"profile_picture_url"`
		BannerPic        string     `json:"banner_picture_url"`
		IsProfilePrivate *bool      `json:"is_profile_private"`
		Title            string     `json:"title"`
		Location         string     `json:"location"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar perfil."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Perfil atualizado com sucesso!"})
}

// ==========================================================
// 🔐 3️⃣ Atualizar Senha
// ==========================================================
func UpdatePassword(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var input struct {
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Senha inválida."})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao processar nova senha."})
		return
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("password", string(hashedPassword)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar nova senha."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Senha atualizada com sucesso!"})
}

// ==========================================================
// 🚨 4️⃣ Deleta conta
// ==========================================================
func DeleteMyAccount(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	if err := database.DB.Where("id = ?", userID).Delete(&models.User{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao excluir conta."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Conta excluída."})
}

// ==========================================================
// 🎓 TRANSFORMAR USUÁRIO EM PROFESSOR
// ==========================================================
func BecomeTeacher(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var input struct {
		CNPJ string `json:"cnpj" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O CNPJ é obrigatório."})
		return
	}

	updates := map[string]interface{}{
		"account_type": utils.RoleTeacher,
		"cnpj":         input.CNPJ,
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar conta."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Agora você é um Professor no StudFy."})
}

// ==========================================================
// 🎓 VER PERFIL PÚBLICO DO PROFESSOR
// ==========================================================
func GetTeacherProfile(c *gin.Context) {
	viewerID, _ := utils.GetUserID(c)

	teacherIDStr := c.Param("id")
	teacherID, err := uuid.Parse(teacherIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do professor inválido."})
		return
	}

	var teacher models.User
	if err := database.DB.Select("id, full_name, nickname, bio, profile_pic, banner_pic, title, location").
		Where("id = ? AND account_type = ?", teacherID, utils.RoleTeacher).
		First(&teacher).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Professor não encontrado."})
		return
	}

	var followerCount int64
	database.DB.Model(&models.Follower{}).Where("following_id = ?", teacherID).Count(&followerCount)

	var isFollowing bool
	if err := database.DB.Where("follower_id = ? AND following_id = ?", viewerID, teacherID).First(&models.Follower{}).Error; err == nil {
		isFollowing = true
	}

	var publicSpaces []models.Space
	database.DB.Select("id, name, description, color_hex, category").
		Where("owner_id = ? AND visibility = 'public'", teacherID).
		Find(&publicSpaces)

	if publicSpaces == nil {
		publicSpaces = []models.Space{}
	}

	c.JSON(http.StatusOK, gin.H{
		"teacher": gin.H{
			"id":                  teacher.ID,
			"full_name":           teacher.FullName,
			"nickname":            teacher.Nickname,
			"bio":                 teacher.Bio,
			"profile_picture_url": teacher.ProfilePic,
			"banner_picture_url":  teacher.BannerPic,
			"title":               teacher.Title,
			"location":            teacher.Location,
			"follower_count":      followerCount,
			"is_following":        isFollowing,
		},
		"public_spaces": publicSpaces,
	})
}

// ==========================================================
// 🌟 SEGUIR UM PROFESSOR
// ==========================================================
func FollowTeacher(c *gin.Context) {
	followerID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	teacherIDStr := c.Param("id")
	teacherID, err := uuid.Parse(teacherIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do professor inválido."})
		return
	}

	if followerID == teacherID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Não pode seguir-se a si mesmo."})
		return
	}

	var teacher models.User
	if err := database.DB.Where("id = ? AND account_type = ?", teacherID, utils.RoleTeacher).First(&teacher).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Professor não encontrado."})
		return
	}

	var existingFollow models.Follower
	if err := database.DB.Where("follower_id = ? AND following_id = ?", followerID, teacherID).First(&existingFollow).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "Já segue este professor."})
		return
	}

	newFollow := models.Follower{
		FollowerID:  followerID,
		FollowingID: teacherID,
	}
	database.DB.Create(&newFollow)

	c.JSON(http.StatusOK, gin.H{"message": "Agora está a seguir " + teacher.FullName})
}

// ==========================================================
// 💔 DEIXAR DE SEGUIR
// ==========================================================
func UnfollowTeacher(c *gin.Context) {
	followerID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	teacherIDStr := c.Param("id")
	teacherID, err := uuid.Parse(teacherIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido."})
		return
	}

	if err := database.DB.Where("follower_id = ? AND following_id = ?", followerID, teacherID).Delete(&models.Follower{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao deixar de seguir."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deixou de seguir este professor."})
}

// ==========================================================
// ⚙️ ATUALIZAR CONFIGURAÇÕES DO APP
// ==========================================================
func UpdateMySettings(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var input struct {
		Theme             string `json:"theme" binding:"required"`
		PushNotifications *bool  `json:"push_notifications" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"theme":              input.Theme,
		"push_notifications": *input.PushNotifications,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar configurações."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configurações salvas!"})
}

// ==========================================================
// ⏰ SALVAR ROTINA
// ==========================================================
func SaveAvailabilityProfile(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var input struct {
		Name      string `json:"name" binding:"required"`
		Schedule  any    `json:"schedule" binding:"required"`
		IsDefault bool   `json:"is_default"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	scheduleJSON, _ := json.Marshal(input.Schedule)

	profile := models.AvailabilityProfile{
		UserID:    userID,
		Name:      input.Name,
		Schedule:  string(scheduleJSON),
		IsDefault: input.IsDefault,
	}

	if input.IsDefault {
		database.DB.Model(&models.AvailabilityProfile{}).
			Where("user_id = ?", userID).
			Update("is_default", false)
	}

	database.DB.Create(&profile)

	c.JSON(http.StatusCreated, gin.H{"message": "Rotina salva!", "profile": profile})
}

// ==========================================================
// ⏰ EDITAR ROTINA
// ==========================================================
func UpdateAvailabilityProfile(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	profileID := c.Param("availability_id")

	var input struct {
		Name      string `json:"name"`
		Schedule  any    `json:"schedule"`
		IsDefault *bool  `json:"is_default"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	var profile models.AvailabilityProfile
	if err := database.DB.Where("id = ? AND user_id = ?", profileID, userID).First(&profile).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rotina não encontrada."})
		return
	}

	updates := map[string]interface{}{}

	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Schedule != nil {
		scheduleJSON, _ := json.Marshal(input.Schedule)
		updates["schedule"] = string(scheduleJSON)
	}
	if input.IsDefault != nil {
		updates["is_default"] = *input.IsDefault
		if *input.IsDefault {
			database.DB.Model(&models.AvailabilityProfile{}).
				Where("user_id = ? AND id != ?", userID, profileID).
				Update("is_default", false)
		}
	}

	if err := database.DB.Model(&profile).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar rotina."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Rotina atualizada!"})
}
