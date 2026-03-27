package study

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==========================================================
// 📥 ESTRUTURAS DE ENTRADA (Compartilhadas com cycles.go)
// ==========================================================

type DisciplineInput struct {
	NotebookID  *uuid.UUID `json:"notebook_id"`
	Name        string     `json:"name"`
	Importance  int        `json:"importance"`
	Performance int        `json:"performance"`
}

type DailyScheduleInput struct {
	DayOfWeek  int    `json:"day_of_week"`
	WakeUpTime string `json:"wake_up_time"`
	SleepTime  string `json:"sleep_time"`
	WorkStart  string `json:"work_start"`
	WorkEnd    string `json:"work_end"`
	LunchStart string `json:"lunch_start"`
	LunchEnd   string `json:"lunch_end"`
}

type GenerateStrategyInput struct {
	Mode               string            `json:"mode" binding:"required"`
	Source             string            `json:"source"`
	TargetGoal         string            `json:"target_goal"`
	HoursPerDay        float64           `json:"hours_per_day"`
	AvailabilityID     *uuid.UUID        `json:"availability_id"` // 🚨 Sem binding para não quebrar o ciclo
	FreeTimePreference int               `json:"free_time_preference"`
	MinSessionMin      int               `json:"min_session_minutes"`
	MaxSessionMin      int               `json:"max_session_minutes"`
	Disciplines        []DisciplineInput `json:"disciplines" binding:"required"`
}

type CreateManualBlockInput struct {
	DayOfWeek  *int       `json:"day_of_week"`
	StartTime  *string    `json:"start_time"`
	EndTime    *string    `json:"end_time"`
	Activity   string     `json:"activity" binding:"required"`
	NotebookID *uuid.UUID `json:"notebook_id"`
}

// ==========================================================
// 🧮 FUNÇÕES AUXILIARES
// ==========================================================

func timeToMinutes(t string) int {
	if t == "" {
		return 0
	}
	var h, m int
	fmt.Sscanf(t, "%d:%d", &h, &m)
	return h*60 + m
}

func minutesToTime(m int) string {
	h := (m / 60) % 24
	mins := m % 60
	return fmt.Sprintf("%02d:%02d", h, mins)
}

func calculateDistribution(disciplines []DisciplineInput, dailyMin float64, minSess int, maxSess int) []models.StudyBlock {
	var totalWeight float64 = 0
	var calculatedBlocks []models.StudyBlock
	sequence := 1

	weights := make([]float64, len(disciplines))
	for i, disc := range disciplines {
		w := float64(disc.Importance + (6 - disc.Performance))
		weights[i] = w
		totalWeight += w
	}

	for i, disc := range disciplines {
		proportion := weights[i] / totalWeight
		suggestedMin := int(math.Round(proportion * dailyMin))

		if minSess > 0 && suggestedMin < minSess {
			suggestedMin = minSess
		}

		blocksNeeded := 1
		if maxSess > 0 && suggestedMin > maxSess {
			blocksNeeded = int(math.Ceil(float64(suggestedMin) / float64(maxSess)))
			suggestedMin = suggestedMin / blocksNeeded
		}

		for b := 0; b < blocksNeeded; b++ {
			calculatedBlocks = append(calculatedBlocks, models.StudyBlock{
				NotebookID:       disc.NotebookID,
				Activity:         disc.Name,
				Importance:       disc.Importance,
				Performance:      disc.Performance,
				SuggestedMinutes: suggestedMin,
				Sequence:         sequence,
			})
			sequence++
		}
	}
	return calculatedBlocks
}

// ==========================================================
// 🚀 1. GERAR CRONOGRAMA (FIXED) - DEFINITIVO
// ==========================================================
func GenerateAutoPlan(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	spaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "ID do Space inválido"})
		return
	}

	userIDInterface, _ := c.Get("userID")
	var parsedUserID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		parsedUserID = v
	case string:
		parsedUserID, _ = uuid.Parse(v)
	default:
		c.JSON(401, gin.H{"error": "Usuário não autenticado corretamente."})
		return
	}

	var input GenerateStrategyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Dados inválidos enviados pelo front-end."})
		return
	}

	if input.AvailabilityID == nil {
		c.JSON(400, gin.H{"error": "Para gerar um Cronograma Fixo, você precisa enviar o availability_id."})
		return
	}

	tx := database.DB.Begin()

	var availabilityProfile models.AvailabilityProfile
	if err := tx.Where("id = ? AND user_id = ?", input.AvailabilityID, parsedUserID).First(&availabilityProfile).Error; err != nil {
		tx.Rollback()
		c.JSON(404, gin.H{"error": "Perfil de disponibilidade não encontrado."})
		return
	}

	var dailyAvailability []DailyScheduleInput
	if err := json.Unmarshal([]byte(availabilityProfile.Schedule), &dailyAvailability); err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"error": "Erro ao ler os horários do perfil."})
		return
	}

	var strategy models.StudyStrategy
	// 👇 Busca APENAS o cronograma fixo DO USUÁRIO LOGADO
	if err := tx.Where("space_id = ? AND mode = 'fixed' AND created_by_id = ?", spaceID, parsedUserID).First(&strategy).Error; err != nil {
		strategy = models.StudyStrategy{
			SpaceID:     spaceID,
			Mode:        "fixed",
			CreatedByID: parsedUserID, // 👈 Salva que o dono é ele
		}
		if err := tx.Create(&strategy).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"error": "Erro ao criar estratégia principal no banco: " + err.Error()})
			return
		}
	}

	// Limpa apenas os blocos desse cronograma específico
	tx.Where("strategy_id = ?", strategy.ID).Delete(&models.StudyBlock{})

	notebooksCriados := make(map[string]uuid.UUID)

	for i, disc := range input.Disciplines {
		if disc.NotebookID == nil || *disc.NotebookID == uuid.Nil {
			if idSalvo, jaCriou := notebooksCriados[disc.Name]; jaCriou {
				input.Disciplines[i].NotebookID = &idSalvo
				continue
			}

			newNb := models.Notebook{
				SpaceID:     spaceID,
				Name:        disc.Name,
				ColorHex:    "#8B5CF6",
				CreatedByID: parsedUserID,
				UpdatedByID: parsedUserID,
			}
			if err := tx.Create(&newNb).Error; err != nil {
				tx.Rollback()
				c.JSON(500, gin.H{"error": "Erro ao criar caderno de " + disc.Name + ": " + err.Error()})
				return
			}

			// 👇 MÁGICA DO BYPASS DE TAGS (Raw Map) com Datas!
			newPageID := uuid.New()
			err := tx.Table("pages").Create(map[string]interface{}{
				"id":            newPageID,
				"notebook_id":   newNb.ID,
				"title":         "Anotações - " + disc.Name,
				"content":       json.RawMessage("{\"html\": \"<p>Comece a digitar seus resumos aqui...</p>\"}"), // 👈 A SOLUÇÃO
				"order":         0,
				"created_by_id": parsedUserID,
				"updated_by_id": parsedUserID,
				"created_at":    time.Now(),
				"updated_at":    time.Now(),
			}).Error

			if err != nil {
				tx.Rollback()
				c.JSON(500, gin.H{"error": "Erro ao criar página de " + disc.Name + ": " + err.Error()})
				return
			}

			nbID := newNb.ID
			notebooksCriados[disc.Name] = nbID
			input.Disciplines[i].NotebookID = &nbID
		}
	}

	strategy.Source = input.Source
	strategy.TargetGoal = input.TargetGoal
	strategy.HoursPerDay = input.HoursPerDay
	strategy.FreeTimePreference = input.FreeTimePreference
	strategy.MinSessionMin = input.MinSessionMin
	strategy.MaxSessionMin = input.MaxSessionMin

	if err := tx.Save(&strategy).Error; err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"error": "Erro ao salvar parâmetros da estratégia."})
		return
	}

	var finalBlocks []models.StudyBlock
	dailyMinutes := input.HoursPerDay * 60

	baseBlocks := calculateDistribution(input.Disciplines, dailyMinutes, input.MinSessionMin, input.MaxSessionMin)
	subjectIndex := 0
	sessionLength := input.MaxSessionMin
	if sessionLength == 0 {
		sessionLength = 50
	}
	breakLength := 10

	for _, dayConfig := range dailyAvailability {
		timeline := make([]bool, 1440*2)

		wake := timeToMinutes(dayConfig.WakeUpTime)
		sleep := timeToMinutes(dayConfig.SleepTime)
		if sleep < wake {
			sleep += 1440
		}
		workStart := timeToMinutes(dayConfig.WorkStart)
		workEnd := timeToMinutes(dayConfig.WorkEnd)
		lunchStart := timeToMinutes(dayConfig.LunchStart)
		lunchEnd := timeToMinutes(dayConfig.LunchEnd)

		for i := 0; i < wake; i++ {
			timeline[i] = true
		}
		for i := sleep; i < len(timeline); i++ {
			timeline[i] = true
		}
		if workStart > 0 && workEnd > workStart {
			for i := workStart; i < workEnd; i++ {
				timeline[i] = true
			}
		}
		if lunchStart > 0 && lunchEnd > lunchStart {
			for i := lunchStart; i < lunchEnd; i++ {
				timeline[i] = true
			}
		}

		var freeBlocks [][]int
		startFree := -1
		for i := wake; i < sleep; i++ {
			if !timeline[i] && startFree == -1 {
				startFree = i
			} else if timeline[i] && startFree != -1 {
				freeBlocks = append(freeBlocks, []int{startFree, i})
				startFree = -1
			}
		}
		if startFree != -1 {
			freeBlocks = append(freeBlocks, []int{startFree, sleep})
		}

		lazerRestante := input.FreeTimePreference
		if lazerRestante > 0 && len(freeBlocks) > 0 {
			lastBlockIndex := len(freeBlocks) - 1
			blockDuration := freeBlocks[lastBlockIndex][1] - freeBlocks[lastBlockIndex][0]

			if blockDuration > lazerRestante {
				freeBlocks[lastBlockIndex][1] -= lazerRestante
			} else {
				freeBlocks = freeBlocks[:lastBlockIndex]
			}
		}

		for _, fb := range freeBlocks {
			curr := fb[0]
			limit := fb[1]

			for curr+sessionLength <= limit {
				if len(baseBlocks) == 0 {
					break
				}
				baseBlock := baseBlocks[subjectIndex%len(baseBlocks)]

				dayCopy := dayConfig.DayOfWeek
				startCopy := minutesToTime(curr)
				endCopy := minutesToTime(curr + sessionLength)

				newBlock := models.StudyBlock{
					StrategyID:       strategy.ID,
					NotebookID:       baseBlock.NotebookID,
					Activity:         baseBlock.Activity,
					DayOfWeek:        &dayCopy,
					StartTime:        &startCopy,
					EndTime:          &endCopy,
					SuggestedMinutes: sessionLength,
				}
				finalBlocks = append(finalBlocks, newBlock)

				subjectIndex++
				curr += sessionLength + breakLength
			}
		}
	}

	if len(finalBlocks) > 0 {
		if err := tx.Create(&finalBlocks).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"error": "Erro ao salvar os blocos no banco: " + err.Error()})
			return
		}
	} else {
		tx.Rollback()
		c.JSON(400, gin.H{"error": "Não foi possível gerar blocos. Verifique se você tem tempo livre suficiente na sua rotina diária."})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro fatal ao confirmar transação: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Cronograma configurado com sucesso!",
		"strategy": strategy,
		"blocks":   finalBlocks,
	})
}

// ==========================================================
// 📋 2. LISTAR CRONOGRAMA (Com Mágica de Clonagem)
// ==========================================================
func ListPlans(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	spaceID, _ := uuid.Parse(spaceIDStr)

	userIDInterface, _ := c.Get("userID")
	var parsedUserID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		parsedUserID = v
	case string:
		parsedUserID, _ = uuid.Parse(v)
	}

	var strategy models.StudyStrategy
	// 1️⃣ Busca o Plano do Usuário Logado
	if err := database.DB.Preload("Blocks", func(db *gorm.DB) *gorm.DB {
		return db.Order("day_of_week ASC, start_time ASC")
	}).Where("space_id = ? AND mode = 'fixed' AND created_by_id = ?", spaceID, parsedUserID).First(&strategy).Error; err != nil {

		// 2️⃣ SE ELE NÃO TEM... CLONA DO DONO DO SPACE!
		var space models.Space
		if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err == nil {
			var ownerStrategy models.StudyStrategy

			// 👇 AQUI USAMOS O OwnerID DA SUA MODEL
			if err := database.DB.Preload("Blocks").Where("space_id = ? AND mode = 'fixed' AND created_by_id = ?", spaceID, space.OwnerID).First(&ownerStrategy).Error; err == nil && space.OwnerID != parsedUserID {

				// 🧬 FAZ A CLONAGEM
				newStrategy := ownerStrategy
				newStrategy.ID = uuid.New()
				newStrategy.CreatedByID = parsedUserID
				database.DB.Create(&newStrategy)

				var newBlocks []models.StudyBlock
				for _, b := range ownerStrategy.Blocks {
					newBlock := b
					newBlock.ID = uuid.New()
					newBlock.StrategyID = newStrategy.ID
					newBlocks = append(newBlocks, newBlock)
				}
				if len(newBlocks) > 0 {
					database.DB.Create(&newBlocks)
				}

				newStrategy.Blocks = newBlocks
				c.JSON(200, gin.H{"study_strategy": newStrategy})
				return
			}
		}

		c.JSON(200, gin.H{
			"message":        "Nenhum cronograma configurado ainda",
			"study_strategy": nil,
		})
		return
	}

	c.JSON(200, gin.H{
		"study_strategy": strategy,
	})
}

// ==========================================================
// ✏️ 3. CRUD MANUAL DOS BLOCOS DO CRONOGRAMA (BLINDADO)
// ==========================================================
func CreateStudyPlan(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	var input CreateManualBlockInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Dados inválidos."})
		return
	}

	userIDInterface, _ := c.Get("userID")
	var parsedUserID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		parsedUserID = v
	case string:
		parsedUserID, _ = uuid.Parse(v)
	}

	var strategy models.StudyStrategy
	// 👇 BLINDAGEM: Garante que tá criando no plano DELE.
	database.DB.Where("space_id = ? AND mode = 'fixed' AND created_by_id = ?", spaceIDStr, parsedUserID).First(&strategy)
	if strategy.ID == uuid.Nil {
		c.JSON(400, gin.H{"error": "Você precisa gerar um cronograma base primeiro."})
		return
	}

	newBlock := models.StudyBlock{
		StrategyID: strategy.ID,
		DayOfWeek:  input.DayOfWeek,
		StartTime:  input.StartTime,
		EndTime:    input.EndTime,
		Activity:   input.Activity,
		NotebookID: input.NotebookID,
	}

	database.DB.Create(&newBlock)
	c.JSON(201, gin.H{"message": "Bloco adicionado!", "block": newBlock})
}

func CreateMultipleStudyPlans(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	var input struct {
		Plans []CreateManualBlockInput `json:"plans"`
	}
	c.ShouldBindJSON(&input)

	userIDInterface, _ := c.Get("userID")
	var parsedUserID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		parsedUserID = v
	case string:
		parsedUserID, _ = uuid.Parse(v)
	}

	var strategy models.StudyStrategy
	// 👇 BLINDAGEM NO EM MASSA TAMBÉM
	database.DB.Where("space_id = ? AND mode = 'fixed' AND created_by_id = ?", spaceIDStr, parsedUserID).First(&strategy)

	var blocks []models.StudyBlock
	for _, p := range input.Plans {
		blocks = append(blocks, models.StudyBlock{
			StrategyID: strategy.ID,
			DayOfWeek:  p.DayOfWeek,
			StartTime:  p.StartTime,
			EndTime:    p.EndTime,
			Activity:   p.Activity,
			NotebookID: p.NotebookID,
		})
	}
	database.DB.Create(&blocks)
	c.JSON(201, gin.H{"message": "Blocos salvos com sucesso!"})
}

func UpdateStudyPlan(c *gin.Context) {
	blockID := c.Param("plan_id")
	var input CreateManualBlockInput
	c.ShouldBindJSON(&input)

	database.DB.Model(&models.StudyBlock{}).Where("id = ?", blockID).Updates(map[string]interface{}{
		"day_of_week": input.DayOfWeek,
		"start_time":  input.StartTime,
		"end_time":    input.EndTime,
		"activity":    input.Activity,
		"notebook_id": input.NotebookID,
	})
	c.JSON(200, gin.H{"message": "Bloco atualizado"})
}

func DeleteStudyPlan(c *gin.Context) {
	blockID := c.Param("plan_id")
	database.DB.Where("id = ?", blockID).Delete(&models.StudyBlock{})
	c.JSON(200, gin.H{"message": "Bloco removido"})
}

// ==========================================================
// 📊 GET ANALYTICS (Geral para Ambos os Modos)
// ==========================================================
func GetMyStudyAnalytics(c *gin.Context) {
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

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	var sessions []models.StudySession

	if err := database.DB.Where("user_id = ? AND created_at >= ?", userID, thirtyDaysAgo).Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar histórico de estudos."})
		return
	}

	var todayMinutes, weekMinutes, extraMinutes int
	subjectTotals := make(map[string]int)

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfWeek := startOfDay.AddDate(0, 0, -int(now.Weekday()))

	for _, s := range sessions {
		subjectTotals[s.ActivityName] += s.ActualMinutes

		if s.ActualMinutes > s.PlannedMinutes {
			extraMinutes += (s.ActualMinutes - s.PlannedMinutes)
		}

		if s.CreatedAt.After(startOfDay) {
			todayMinutes += s.ActualMinutes
		}
		if s.CreatedAt.After(startOfWeek) {
			weekMinutes += s.ActualMinutes
		}
	}

	type SubjectStat struct {
		Name    string `json:"name"`
		Minutes int    `json:"minutes"`
	}
	var distribution []SubjectStat
	for name, mins := range subjectTotals {
		distribution = append(distribution, SubjectStat{Name: name, Minutes: mins})
	}

	c.JSON(http.StatusOK, gin.H{
		"overview": gin.H{
			"today_minutes": todayMinutes,
			"week_minutes":  weekMinutes,
			"extra_minutes": extraMinutes,
		},
		"subject_distribution": distribution,
	})
}

// ==========================================================
// ⏰ ATUALIZAR ROTINA DO ALUNO (Disponibilidade de Horário)
// ==========================================================
func UpdateAvailabilityProfile(c *gin.Context) {
	profileID := c.Param("availability_id")

	// Pega o ID de quem está logado pra garantir segurança
	userIDInterface, _ := c.Get("userID")
	var parsedUserID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		parsedUserID = v
	case string:
		parsedUserID, _ = uuid.Parse(v)
	}

	// Espera receber aquele mesmo array de dias da semana (Schedule)
	var input struct {
		Schedule []DailyScheduleInput `json:"schedule" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato de rotina inválido."})
		return
	}

	// Converte o array de volta pra string JSON pra salvar no banco
	scheduleJSON, _ := json.Marshal(input.Schedule)

	// Atualiza o perfil no banco de dados
	if err := database.DB.Model(&models.AvailabilityProfile{}).
		Where("id = ? AND user_id = ?", profileID, parsedUserID).
		Update("schedule", string(scheduleJSON)).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar a rotina."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sua rotina foi atualizada com sucesso! Você já pode gerar um novo cronograma."})
}
