package gamification

import (
	"fmt"
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RewardInput struct {
	Action string `json:"action" binding:"required"`
	Amount int    `json:"amount"`
}

func RewardXP(c *gin.Context) {
	userIDContext, _ := c.Get("userID")
	userIDStr := fmt.Sprintf("%v", userIDContext)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "ID de usuário inválido"})
		return
	}

	var input RewardInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Ação inválida"})
		return
	}

	xpToAward := input.Amount
	// Se o front-end não mandar o valor (mandar 0), usamos um valor padrão
	if xpToAward <= 0 {
		switch input.Action {
		case "completed_pomodoro":
			xpToAward = 25
		case "created_note":
			xpToAward = 10
		default:
			xpToAward = 5
		}
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("xp", gorm.Expr("xp + ?", xpToAward)).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao atualizar XP", "detalhe": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "XP ganho!", "xp_earned": xpToAward})
}

// ==========================================================
// ⚡ CRIAR MISSÃO RELÂMPAGO (Visão do Professor)
// ==========================================================
func CreateFlashMission(c *gin.Context) {
	teacherIDInterface, _ := c.Get("userID")
	var teacherID uuid.UUID
	switch v := teacherIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	spaceID, _ := uuid.Parse(c.Param("space_id"))

	// 1. Verifica se ele é o dono do Space
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, teacherID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas o professor do Space pode criar missões."})
		return
	}

	// 2. Recebe os dados da Missão
	var input struct {
		Title       string    `json:"title" binding:"required"`
		Description string    `json:"description"`
		RewardXP    int       `json:"reward_xp"`
		ExpiresAt   time.Time `json:"expires_at" binding:"required"` // Front-end manda a data final
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
// ⚡ LISTAR MISSÕES ATIVAS DO SPACE (Visão do Aluno)
// ==========================================================
func GetActiveMissions(c *gin.Context) {
	spaceID := c.Param("space_id")
	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	var missions []models.FlashMission
	now := time.Now()

	// Busca missões onde o tempo AINDA NÃO EXPIROU
	database.DB.Where("space_id = ? AND expires_at > ?", spaceID, now).Find(&missions)

	// Dica de Sênior: Vamos avisar o Front-end se o aluno JÁ completou essa missão
	// para o botão ficar cinza (disabled) na tela.
	type MissionResponse struct {
		models.FlashMission
		IsCompleted bool `json:"is_completed"`
	}

	var response []MissionResponse
	for _, m := range missions {
		var completion models.MissionCompletion
		isComp := false
		if err := database.DB.Where("mission_id = ? AND user_id = ?", m.ID, userID).First(&completion).Error; err == nil {
			isComp = true
		}
		response = append(response, MissionResponse{FlashMission: m, IsCompleted: isComp})
	}

	if response == nil {
		response = []MissionResponse{}
	}

	c.JSON(http.StatusOK, gin.H{"missions": response})
}

// ==========================================================
// ⚡ CONCLUIR MISSÃO E GANHAR XP (Visão do Aluno)
// ==========================================================
func CompleteFlashMission(c *gin.Context) {
	missionID := c.Param("mission_id")
	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	// 1. Busca a missão
	var mission models.FlashMission
	if err := database.DB.Where("id = ?", missionID).First(&mission).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Missão não encontrada."})
		return
	}

	// 2. Verifica se o tempo já estourou
	if time.Now().After(mission.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O tempo desta missão já expirou!"})
		return
	}

	// 3. Verifica se o aluno já completou antes
	var existing models.MissionCompletion
	if err := database.DB.Where("mission_id = ? AND user_id = ?", mission.ID, userID).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Você já resgatou o prêmio desta missão!"})
		return
	}

	tx := database.DB.Begin()

	// 4. Salva a conclusão
	completion := models.MissionCompletion{
		MissionID: mission.ID,
		UserID:    userID,
	}
	if err := tx.Create(&completion).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao registrar conclusão."})
		return
	}

	// 5. Injeta o XP na conta do aluno
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

	teacherIDInterface, _ := c.Get("userID")
	var teacherID uuid.UUID
	switch v := teacherIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	// 1. Verifica se quem está criando é realmente o dono do Space
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, teacherID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas o professor dono do Space pode criar emblemas."})
		return
	}

	// 2. Recebe os dados do emblema
	var input struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		IconURL     string `json:"icon_url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nome e Ícone são obrigatórios."})
		return
	}

	// 3. Forja o emblema no banco
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

	teacherIDInterface, _ := c.Get("userID")
	var teacherID uuid.UUID
	switch v := teacherIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	// 1. Confere se o emblema existe e foi criado por esse professor neste Space
	var badge models.Badge
	if err := database.DB.Where("id = ? AND space_id = ? AND teacher_id = ?", badgeID, spaceID, teacherID).First(&badge).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Emblema não encontrado ou você não tem permissão para usá-lo."})
		return
	}

	// 2. Confere se o aluno JÁ TEM esse emblema (pra não entregar duplicado)
	var existing models.UserBadge
	if err := database.DB.Where("user_id = ? AND badge_id = ?", studentID, badgeID).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Este aluno já possui este emblema no peito!"})
		return
	}

	// 3. Pendura a medalha no peito do aluno
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

	teacherIDInterface, _ := c.Get("userID")
	var teacherID uuid.UUID
	switch v := teacherIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	// 1. Verifica se quem está mexendo é o dono do Space
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, teacherID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas o professor dono do Space pode alterar esta configuração."})
		return
	}

	// 2. Recebe o novo status (true ou false)
	var input struct {
		IsActive *bool `json:"is_active" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O status (true/false) é obrigatório."})
		return
	}

	// 3. Salva no banco de dados
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

	// 1. Busca o Space para ver se o ranking está ligado e quem é o dono
	var space models.Space
	if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado."})
		return
	}

	// 2. Se o professor desligou, a gente tranca a porta e devolve erro 403
	if !space.IsRankingActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "O professor desativou o ranking de XP para esta turma."})
		return
	}

	// 3. Monta a estrutura de resposta para o Mayan desenhar o pódio
	var ranking []struct {
		UserID     uuid.UUID `json:"user_id"`
		FullName   string    `json:"full_name"`
		Nickname   string    `json:"nickname"`
		ProfilePic string    `json:"profile_picture_url"`
		XP         int       `json:"xp"`
	}

	// 4. Mágica do SQL: Puxa o Dono + Todos os Alunos convidados, ordena pelo XP (Do maior pro menor)
	database.DB.Table("users").
		Select("DISTINCT users.id as user_id, users.full_name, users.nickname, users.profile_pic, users.xp").
		Joins("LEFT JOIN space_permissions ON space_permissions.user_id = users.id").
		Where("space_permissions.space_id = ? OR users.id = ?", spaceID, space.OwnerID).
		Order("users.xp DESC").
		Limit(50). // Limita aos Top 50 para não pesar o banco
		Scan(&ranking)

	// Garante que não devolve null pro Front-end
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
