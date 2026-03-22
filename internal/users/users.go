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

// ==========================================================
// 🎓 TRANSFORMAR USUÁRIO EM PROFESSOR (Onboarding B2B)
// ==========================================================
func BecomeTeacher(c *gin.Context) {
	// 1. Pega o ID do usuário logado
	userIDInterface, _ := c.Get("userID")

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

	// 2. Cria a estrutura para receber o CNPJ do Front-end
	var input struct {
		CNPJ string `json:"cnpj" binding:"required"`
	}

	// 3. Valida se o Front mandou o JSON certinho
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O CNPJ é obrigatório para criar uma conta de Professor."})
		return
	}

	// 🚧 Dica de Sênior: No futuro, você pode colocar um código aqui que bate
	// na "BrasilAPI" para conferir se esse CNPJ existe mesmo na Receita Federal.
	// Por enquanto, vamos só salvar direto.

	// 4. Prepara a atualização no banco de dados
	updates := map[string]interface{}{
		"account_type": "TEACHER", // 👈 A mágica acontece aqui!
		"cnpj":         input.CNPJ,
	}

	// 5. Salva no banco de dados
	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar a conta para Professor."})
		return
	}

	// 6. Devolve a resposta de Sucesso pro Mayan
	c.JSON(http.StatusOK, gin.H{
		"message":      "Parabéns! Agora você é um Professor no StudFy.",
		"account_type": "TEACHER",
		"cnpj":         input.CNPJ,
	})
}

// ==========================================================
// 🎓 VER PERFIL PÚBLICO DO PROFESSOR (A Vitrine)
// ==========================================================
func GetTeacherProfile(c *gin.Context) {
	// 1. Quem está acessando? (Para o botão de seguir)
	viewerIDInterface, _ := c.Get("userID")
	var viewerID uuid.UUID
	switch v := viewerIDInterface.(type) {
	case uuid.UUID:
		viewerID = v
	case string:
		viewerID, _ = uuid.Parse(v)
	}

	// 2. ID do professor na URL
	teacherIDStr := c.Param("id")
	teacherID, err := uuid.Parse(teacherIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do professor inválido."})
		return
	}

	// 3. Busca os dados públicos do Professor (SEGURANÇA: Nada de dados sensíveis aqui!)
	var teacher models.User
	if err := database.DB.Select("id, full_name, nickname, bio, profile_pic, banner_pic, title, location").
		Where("id = ? AND account_type = 'TEACHER'", teacherID).
		First(&teacher).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Professor não encontrado."})
		return
	}

	// 4. Conta quantos seguidores ele tem
	var followerCount int64
	database.DB.Model(&models.Follower{}).Where("following_id = ?", teacherID).Count(&followerCount)

	// 5. Verifica se o aluno logado já segue esse professor
	var isFollowing bool
	var follow models.Follower
	if err := database.DB.Where("follower_id = ? AND following_id = ?", viewerID, teacherID).First(&follow).Error; err == nil {
		isFollowing = true
	}

	// 6. Busca os Spaces (Salas de Aula) que ele deixou como PÚBLICOS
	var publicSpaces []models.Space
	database.DB.Select("id, name, description, color_hex, category").
		Where("owner_id = ? AND visibility = 'public'", teacherID).
		Find(&publicSpaces)

	if publicSpaces == nil {
		publicSpaces = []models.Space{}
	}

	// 7. Monta o JSON Mastigadinho para o Mayan
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
			"is_following":        isFollowing, // 👈 Pro Front renderizar o botão certo
		},
		"public_spaces": publicSpaces,
	})
}

// ==========================================================
// 🌟 SEGUIR UM PROFESSOR
// ==========================================================
func FollowTeacher(c *gin.Context) {
	// 1. Quem está clicando no botão de seguir?
	followerIDInterface, _ := c.Get("userID")
	followerID, _ := uuid.Parse(followerIDInterface.(string))

	// 2. Quem ele quer seguir? (Vem na URL: /teachers/:id/follow)
	teacherIDStr := c.Param("id")
	teacherID, err := uuid.Parse(teacherIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do professor inválido."})
		return
	}

	// 3. Regra de Ouro: Não pode seguir a si mesmo
	if followerID == teacherID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Você não pode seguir a si mesmo."})
		return
	}

	// 4. Regra: Só pode seguir se o alvo for um Professor
	var teacher models.User
	if err := database.DB.Where("id = ? AND account_type = 'TEACHER'", teacherID).First(&teacher).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Professor não encontrado ou usuário não é um professor."})
		return
	}

	// 5. Verifica se já segue (para não duplicar no banco)
	var existingFollow models.Follower
	if err := database.DB.Where("follower_id = ? AND following_id = ?", followerID, teacherID).First(&existingFollow).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "Você já segue este professor."})
		return
	}

	// 6. Cria a conexão no banco!
	newFollow := models.Follower{
		FollowerID:  followerID,
		FollowingID: teacherID,
	}
	database.DB.Create(&newFollow)

	c.JSON(http.StatusOK, gin.H{"message": "Você agora está seguindo " + teacher.FullName})
}

// ==========================================================
// 💔 DEIXAR DE SEGUIR UM PROFESSOR
// ==========================================================
func UnfollowTeacher(c *gin.Context) {
	followerIDInterface, _ := c.Get("userID")
	followerID, _ := uuid.Parse(followerIDInterface.(string))
	teacherID, _ := uuid.Parse(c.Param("id"))

	// Deleta a conexão do banco
	if err := database.DB.Where("follower_id = ? AND following_id = ?", followerID, teacherID).Delete(&models.Follower{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao deixar de seguir."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Você deixou de seguir este professor."})
}

// ==========================================================
// ⚙️ ATUALIZAR CONFIGURAÇÕES DO APP (Settings)
// ==========================================================
func UpdateMySettings(c *gin.Context) {
	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	// Estrutura do JSON que o Front-end vai mandar
	var input struct {
		Theme             string `json:"theme" binding:"required"`              // Ex: "dark" ou "light"
		PushNotifications *bool  `json:"push_notifications" binding:"required"` // Ponteiro para aceitar false corretamente
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos. Envie theme e push_notifications."})
		return
	}

	// Atualiza apenas as duas colunas no banco de dados
	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"theme":              input.Theme,
		"push_notifications": *input.PushNotifications,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar configurações."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Configurações salvas com sucesso!",
		"settings": gin.H{
			"theme":              input.Theme,
			"push_notifications": *input.PushNotifications,
		},
	})
}
