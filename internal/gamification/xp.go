package gamification

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils" // 👈 Import inserido
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RewardInput struct {
	Action string `json:"action" binding:"required"`
	Amount int    `json:"amount"`
}

// ==========================================================
// ⚡ CONCEDER XP PARA O USUÁRIO (Baseado nas Regras do Admin)
// ==========================================================
func RewardXP(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var input struct {
		Action string `json:"action" binding:"required"`
		Amount int    `json:"amount"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ação inválida"})
		return
	}

	xpToAward := input.Amount

	if xpToAward <= 0 {
		var rule models.GamificationRule
		if err := database.DB.Where("action_name = ?", input.Action).First(&rule).Error; err == nil {
			xpToAward = rule.RewardXP
		} else {
			xpToAward = 5
		}
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("xp", gorm.Expr("xp + ?", xpToAward)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar XP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "XP ganho com sucesso!",
		"action":    input.Action,
		"xp_earned": xpToAward,
	})
}

// ==========================================================
// ⚡ CRIAR MISSÃO RELÂMPAGO (Visão do Professor)
// ==========================================================
func CreateFlashMission(c *gin.Context) {
	teacherID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	spaceID, _ := uuid.Parse(c.Param("space_id"))

	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, teacherID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas o professor do Space pode criar missões."})
		return
	}

	var input struct {
		Title       string    `json:"title" binding:"required"`
		Description string    `json:"description"`
		RewardXP    int       `json:"reward_xp"`
		ExpiresAt   time.Time `json:"expires_at" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos. Verifique o formato da data."})
		return
	}

	newMission := models.FlashMission{
		SpaceID:     spaceID,
		TeacherID:   teacherID,
		Title:       input.Title,
		Description: input.Description,
		RewardXP:    input.RewardXP,
		ExpiresAt:   input.ExpiresAt,
	}

	if err := database.DB.Create(&newMission).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar missão."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Missão Relâmpago criada!", "mission": newMission})
}

// ==========================================================
// ⚡ LISTAR MISSÕES ATIVAS DO SPACE (Visão do Aluno) - OTIMIZADO 🚀
// ==========================================================
func GetActiveMissions(c *gin.Context) {
	spaceID := c.Param("space_id")
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	// 1. Busca todas as missões ativas (1ª Query)
	var missions []models.FlashMission
	now := time.Now()
	database.DB.Where("space_id = ? AND expires_at > ?", spaceID, now).Find(&missions)

	type MissionResponse struct {
		models.FlashMission
		IsCompleted bool `json:"is_completed"`
	}

	var response []MissionResponse

	if len(missions) == 0 {
		c.JSON(http.StatusOK, gin.H{"missions": []MissionResponse{}})
		return
	}

	// 2. Extrai apenas os IDs das missões para fazer a busca em lote
	var missionIDs []uuid.UUID
	for _, m := range missions {
		missionIDs = append(missionIDs, m.ID)
	}

	// 3. Busca TODAS as conclusões deste usuário de UMA SÓ VEZ (2ª Query - A Mágica ✨)
	var completions []models.MissionCompletion
	database.DB.Where("mission_id IN ? AND user_id = ?", missionIDs, userID).Find(&completions)

	// 4. Cria um "Dicionário" (Mapa em Memória O(1)) para acesso instantâneo
	completionMap := make(map[uuid.UUID]bool)
	for _, c := range completions {
		completionMap[c.MissionID] = true
	}

	// 5. Monta a resposta sem encostar no banco de dados!
	for _, m := range missions {
		isComp := completionMap[m.ID]
		response = append(response, MissionResponse{FlashMission: m, IsCompleted: isComp})
	}

	c.JSON(http.StatusOK, gin.H{"missions": response})
}

// ==========================================================
// ⚡ CONCLUIR MISSÃO E GANHAR XP (Visão do Aluno)
// ==========================================================
func CompleteFlashMission(c *gin.Context) {
	missionID := c.Param("mission_id")
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var mission models.FlashMission
	if err := database.DB.Where("id = ?", missionID).First(&mission).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Missão não encontrada."})
		return
	}

	if time.Now().After(mission.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O tempo desta missão já expirou!"})
		return
	}

	var existing models.MissionCompletion
	if err := database.DB.Where("mission_id = ? AND user_id = ?", mission.ID, userID).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Já resgatou o prémio desta missão!"})
		return
	}

	tx := database.DB.Begin()

	completion := models.MissionCompletion{
		MissionID: mission.ID,
		UserID:    userID,
	}
	if err := tx.Create(&completion).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao registar conclusão."})
		return
	}

	if err := tx.Model(&models.User{}).Where("id = ?", userID).Update("xp", gorm.Expr("xp + ?", mission.RewardXP)).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao conceder XP."})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"message":   "Missão cumprida! XP resgatado.",
		"xp_earned": mission.RewardXP,
	})
}

// ==========================================================
// 🏆 FASE 6: CRIAR EMBLEMA (Visão do Professor)
// ==========================================================
func CreateBadge(c *gin.Context) {
	spaceID := c.Param("space_id")
	teacherID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, teacherID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas o professor dono do Space pode criar emblemas."})
		return
	}

	var input struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		IconURL     string `json:"icon_url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nome e Ícone são obrigatórios."})
		return
	}

	newBadge := models.Badge{
		SpaceID:     space.ID,
		TeacherID:   teacherID,
		Name:        input.Name,
		Description: input.Description,
		IconURL:     input.IconURL,
	}

	if err := database.DB.Create(&newBadge).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar emblema."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Emblema forjado com sucesso!",
		"badge":   newBadge,
	})
}

// ==========================================================
// 🏆 FASE 6: DISTRIBUIR EMBLEMA PARA O ALUNO
// ==========================================================
func AwardBadge(c *gin.Context) {
	spaceID := c.Param("space_id")
	badgeID := c.Param("badge_id")
	studentID := c.Param("student_id")

	teacherID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var badge models.Badge
	if err := database.DB.Where("id = ? AND space_id = ? AND teacher_id = ?", badgeID, spaceID, teacherID).First(&badge).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Emblema não encontrado ou sem permissão."})
		return
	}

	var existing models.UserBadge
	if err := database.DB.Where("user_id = ? AND badge_id = ?", studentID, badgeID).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Este aluno já possui este emblema no peito!"})
		return
	}

	userBadge := models.UserBadge{
		UserID:    uuid.MustParse(studentID),
		BadgeID:   badge.ID,
		AwardedBy: teacherID,
	}

	if err := database.DB.Create(&userBadge).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao distribuir o emblema."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Emblema distribuído com sucesso para o aluno!",
	})
}

// ==========================================================
// 🏆 FASE 6: LIGAR/DESLIGAR RANKING (Visão do Professor)
// ==========================================================
func ToggleSpaceRanking(c *gin.Context) {
	spaceID := c.Param("space_id")

	teacherID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, teacherID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas o professor dono do Space pode alterar esta configuração."})
		return
	}

	var input struct {
		IsActive *bool `json:"is_active" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O status (true/false) é obrigatório."})
		return
	}

	database.DB.Model(&space).Update("is_ranking_active", *input.IsActive)

	statusMsg := "desativado"
	if *input.IsActive {
		statusMsg = "ativado"
	}

	c.JSON(http.StatusOK, gin.H{"message": "O Ranking da turma foi " + statusMsg + " com sucesso!"})
}

// ==========================================================
// 🏆 FASE 6: OBTER RANKING DA TURMA (O Leaderboard)
// ==========================================================
func GetSpaceRanking(c *gin.Context) {
	spaceID := c.Param("space_id")

	var space models.Space
	if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado."})
		return
	}

	if !space.IsRankingActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "O professor desativou o ranking de XP para esta turma."})
		return
	}

	var ranking []struct {
		UserID     uuid.UUID `json:"user_id"`
		FullName   string    `json:"full_name"`
		Nickname   string    `json:"nickname"`
		ProfilePic string    `json:"profile_picture_url"`
		XP         int       `json:"xp"`
	}

	database.DB.Table("users").
		Select("DISTINCT users.id as user_id, users.full_name, users.nickname, users.profile_pic, users.xp").
		Joins("LEFT JOIN space_permissions ON space_permissions.user_id = users.id").
		Where("space_permissions.space_id = ? OR users.id = ?", spaceID, space.OwnerID).
		Order("users.xp DESC").
		Limit(50).
		Scan(&ranking)

	if ranking == nil {
		ranking = []struct {
			UserID     uuid.UUID `json:"user_id"`
			FullName   string    `json:"full_name"`
			Nickname   string    `json:"nickname"`
			ProfilePic string    `json:"profile_picture_url"`
			XP         int       `json:"xp"`
		}{}
	}

	c.JSON(http.StatusOK, gin.H{
		"is_ranking_active": space.IsRankingActive,
		"ranking":           ranking,
	})
}
