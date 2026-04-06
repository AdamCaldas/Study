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
// 📥 ESTRUTURAS DE ENTRADA
// ==========================================================

type DisciplineInput struct {
	NotebookID  *uuid.UUID `json:"notebook_id"`
	Name        string     `json:"name"`
	Importance  int        `json:"importance"`
	Performance int        `json:"performance"`
	DayOfWeek   *int       `json:"day_of_week"`
}

type GenerateStrategyInput struct {
	Mode               string            `json:"mode" binding:"required"`
	Source             string            `json:"source"`
	TargetGoal         string            `json:"target_goal"`
	HoursPerDay        float64           `json:"hours_per_day"`
	StudyDays          []int             `json:"study_days"`
	AvailabilityID     *uuid.UUID        `json:"availability_id"`
	FreeTimePreference int               `json:"free_time_preference"`
	MinSessionMin      int               `json:"min_session_minutes"`
	MaxSessionMin      int               `json:"max_session_minutes"`
	Disciplines        []DisciplineInput `json:"disciplines" binding:"required"`
}

type CreatePlanBlockInput struct {
	DayOfWeek  *int       `json:"day_of_week"`
	StartTime  *string    `json:"start_time"`
	EndTime    *string    `json:"end_time"`
	Activity   string     `json:"activity" binding:"required"`
	NotebookID *uuid.UUID `json:"notebook_id"`
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

type SuperEditPlanInput struct {
	TargetGoal    *string  `json:"target_goal"`
	HoursPerDay   *float64 `json:"hours_per_day"`
	StudyDays     []int    `json:"study_days"`
	MinSessionMin *int     `json:"min_session_minutes"`
	MaxSessionMin *int     `json:"max_session_minutes"`
	Blocks        []struct {
		ID               *uuid.UUID `json:"id"`
		Activity         string     `json:"activity"`
		NotebookID       *uuid.UUID `json:"notebook_id"`
		DayOfWeek        *int       `json:"day_of_week"`
		StartTime        *string    `json:"start_time"`
		EndTime          *string    `json:"end_time"`
		SuggestedMinutes int        `json:"suggested_minutes"`
		Importance       int        `json:"importance"`
		Performance      int        `json:"performance"`
	} `json:"blocks"`
}

// ==========================================================
// 🧮 FUNÇÕES AUXILIARES DE TEMPO (HH:MM -> Minutos)
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
	return fmt.Sprintf("%02d:%02d:00", h, mins)
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
// 🚀 1. GERAR CRONOGRAMA (FIXED) - SEMANA TODA CORRIGIDA
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
		c.JSON(400, gin.H{"error": "Dados inválidos."})
		return
	}

	if input.AvailabilityID == nil {
		c.JSON(400, gin.H{"error": "availability_id é obrigatório."})
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
		c.JSON(500, gin.H{"error": "Erro ao ler horários do perfil."})
		return
	}

	var strategy models.StudyStrategy
	if err := tx.Where("space_id = ? AND mode = 'fixed' AND created_by_id = ?", spaceID, parsedUserID).First(&strategy).Error; err != nil {
		strategy = models.StudyStrategy{
			SpaceID:     spaceID,
			Mode:        "fixed",
			CreatedByID: parsedUserID,
		}
		if err := tx.Create(&strategy).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"error": "Erro ao criar estratégia: " + err.Error()})
			return
		}
	}

	tx.Where("strategy_id = ?", strategy.ID).Delete(&models.StudyBlock{})

	notebooksCriados := make(map[string]uuid.UUID)

	for i, disc := range input.Disciplines {
		if disc.NotebookID == nil || *disc.NotebookID == uuid.Nil {
			if idSalvo, jaCriou := notebooksCriados[disc.Name]; jaCriou {
				input.Disciplines[i].NotebookID = &idSalvo
				continue
			}

			var existingNb models.Notebook
			if err := tx.Where("space_id = ? AND name = ?", spaceID, disc.Name).First(&existingNb).Error; err == nil {
				nbID := existingNb.ID
				notebooksCriados[disc.Name] = nbID
				input.Disciplines[i].NotebookID = &nbID
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
				c.JSON(500, gin.H{"error": "Erro ao criar caderno: " + err.Error()})
				return
			}

			newPage := models.Page{
				NotebookID:  newNb.ID,
				Title:       "Anotações - " + disc.Name,
				Content:     "{\"html\": \"<p>Comece a digitar seus resumos aqui...</p>\"}",
				Order:       0,
				CreatedByID: parsedUserID,
				UpdatedByID: parsedUserID,
			}
			if err := tx.Create(&newPage).Error; err != nil {
				tx.Rollback()
				c.JSON(500, gin.H{"error": "Erro ao criar página: " + err.Error()})
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
	strategy.StudyDays = input.StudyDays
	strategy.FreeTimePreference = input.FreeTimePreference
	strategy.MinSessionMin = input.MinSessionMin
	strategy.MaxSessionMin = input.MaxSessionMin

	if err := tx.Save(&strategy).Error; err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"error": "Erro ao salvar estratégia."})
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

	studyDaysMap := make(map[int]bool)
	for _, d := range input.StudyDays {
		studyDaysMap[d] = true
	}

	for _, dayConfig := range dailyAvailability {
		if len(studyDaysMap) > 0 && !studyDaysMap[dayConfig.DayOfWeek] {
			continue
		}

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
				startCopy := fmt.Sprintf("%02d:%02d", (curr/60)%24, curr%60)
				endCopy := fmt.Sprintf("%02d:%02d", ((curr+sessionLength)/60)%24, (curr+sessionLength)%60)

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
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"message":  "Cronograma da semana configurado com sucesso!",
		"strategy": strategy,
		"blocks":   finalBlocks,
	})
}

// ==========================================================
// 📋 2. LISTAR CRONOGRAMA (Com Blocos Visuais e Clonagem Inteligente)
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
	if err := database.DB.Preload("Blocks", func(db *gorm.DB) *gorm.DB {
		return db.Order("day_of_week ASC, start_time ASC")
	}).Where("space_id = ? AND mode = 'fixed' AND created_by_id = ?", spaceID, parsedUserID).First(&strategy).Error; err != nil {

		// ==========================================================
		// 🧬 LÓGICA DE CLONAR DO DONO (INTELIGENTE)
		// ==========================================================
		var space models.Space
		if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err == nil {
			var ownerStrategy models.StudyStrategy
			if err := database.DB.Preload("Blocks", func(db *gorm.DB) *gorm.DB {
				return db.Order("day_of_week ASC, start_time ASC")
			}).Where("space_id = ? AND mode = 'fixed' AND created_by_id = ?", spaceID, space.OwnerID).First(&ownerStrategy).Error; err == nil && space.OwnerID != parsedUserID {

				// 1. Cria a "Fôrma" nova pro aluno
				newStrategy := ownerStrategy
				newStrategy.ID = uuid.New()
				newStrategy.CreatedByID = parsedUserID
				database.DB.Create(&newStrategy)

				// 2. Olha se esse aluno tem uma Rotina de Vida salva
				var userRoutine models.AvailabilityProfile
				database.DB.Where("user_id = ? AND is_default = ?", parsedUserID, true).First(&userRoutine)
				if userRoutine.ID == uuid.Nil {
					database.DB.Where("user_id = ?", parsedUserID).First(&userRoutine)
				}

				hasRoutine := userRoutine.ID != uuid.Nil && userRoutine.Schedule != ""

				// 3. Clona os blocos do professor um por um
				var newBlocks []models.StudyBlock
				for _, b := range ownerStrategy.Blocks {
					newBlock := b
					newBlock.ID = uuid.New()
					newBlock.StrategyID = newStrategy.ID

					if hasRoutine {
						newBlock.StartTime = nil
						newBlock.EndTime = nil
					}

					newBlocks = append(newBlocks, newBlock)
				}

				if len(newBlocks) > 0 {
					database.DB.Create(&newBlocks)
				}
				newStrategy.Blocks = newBlocks
				strategy = newStrategy

				if hasRoutine {
					c.JSON(http.StatusOK, gin.H{
						"message":               "Cronograma clonado! Os horários foram zerados para respeitar sua rotina. Clique em 'Gerar Automático'.",
						"needs_auto_generation": true,
						"study_strategy":        strategy,
						"routine_blocks":        []map[string]interface{}{},
					})
					return
				}

			} else {
				c.JSON(http.StatusOK, gin.H{"message": "Nenhum cronograma configurado ainda", "study_strategy": nil})
				return
			}
		} else {
			c.JSON(http.StatusOK, gin.H{"message": "Nenhum cronograma configurado ainda", "study_strategy": nil})
			return
		}
	}

	// ==========================================================
	// 📊 LÓGICA DE ANALYTICS (HOJE)
	// ==========================================================
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	intToday := int(now.Weekday())

	var todayLog models.ScheduleLog
	database.DB.Preload("Blocks").Where("schedule_id = ? AND date = ?", strategy.ID, today).First(&todayLog)

	todayBlocksMap := make(map[uuid.UUID]models.ScheduleLogBlock)
	for _, b := range todayLog.Blocks {
		todayBlocksMap[b.BlockID] = b
	}

	var todayAnalyticsBlocks []map[string]interface{}
	shiftMinutes := 0
	lastEndTime := 0

	for _, b := range strategy.Blocks {
		if b.DayOfWeek == nil || *b.DayOfWeek != intToday || b.StartTime == nil || b.EndTime == nil {
			continue
		}

		plannedStartMin := timeToMinutes(*b.StartTime)
		plannedEndMin := timeToMinutes(*b.EndTime)

		if plannedStartMin >= lastEndTime {
			shiftMinutes = 0
		}

		var logData models.ScheduleLogBlock
		executed := false
		if val, ok := todayBlocksMap[b.ID]; ok {
			logData = val
			executed = true
		}

		if executed {
			realEndMin := timeToMinutes(logData.RealEndTime)
			lastEndTime = realEndMin
			if realEndMin > plannedEndMin {
				shiftMinutes = realEndMin - plannedEndMin
			}
			todayAnalyticsBlocks = append(todayAnalyticsBlocks, map[string]interface{}{
				"block_id":           b.ID,
				"activity":           b.Activity,
				"planned_start_time": *b.StartTime + ":00",
				"planned_end_time":   *b.EndTime + ":00",
				"real_start_time":    logData.RealStartTime + ":00",
				"real_end_time":      logData.RealEndTime + ":00",
				"total_minutes":      logData.TotalMinutes,
				"has_recalculation":  logData.HasRecalculation,
			})
		} else {
			adjustedStartMin := plannedStartMin + shiftMinutes
			adjustedEndMin := plannedEndMin + shiftMinutes
			lastEndTime = adjustedEndMin
			todayAnalyticsBlocks = append(todayAnalyticsBlocks, map[string]interface{}{
				"block_id":            b.ID,
				"activity":            b.Activity,
				"planned_start_time":  *b.StartTime + ":00",
				"planned_end_time":    *b.EndTime + ":00",
				"adjusted_start_time": minutesToTime(adjustedStartMin),
				"adjusted_end_time":   minutesToTime(adjustedEndMin),
				"total_minutes":       0,
				"has_recalculation":   shiftMinutes > 0,
			})
		}
	}
	if todayAnalyticsBlocks == nil {
		todayAnalyticsBlocks = []map[string]interface{}{}
	}

	// ==========================================================
	// 🧙‍♂️ A NOVA MÁGICA: INJETAR OS BLOCOS DA ROTINA (FANTASMAS)
	// ==========================================================
	var routineBlocks []map[string]interface{}
	var defaultProfile models.AvailabilityProfile

	database.DB.Where("user_id = ? AND is_default = ?", parsedUserID, true).First(&defaultProfile)
	if defaultProfile.ID == uuid.Nil {
		database.DB.Where("user_id = ?", parsedUserID).First(&defaultProfile)
	}

	if defaultProfile.ID != uuid.Nil && defaultProfile.Schedule != "" {
		var dailyAvailability []DailyScheduleInput
		if err := json.Unmarshal([]byte(defaultProfile.Schedule), &dailyAvailability); err == nil {

			for _, day := range dailyAvailability {
				if day.SleepTime != "" && day.WakeUpTime != "" {
					routineBlocks = append(routineBlocks, map[string]interface{}{
						"activity":    "Dormir",
						"day_of_week": day.DayOfWeek,
						"start_time":  day.SleepTime,
						"end_time":    day.WakeUpTime,
						"type":        "sleep",
						"color_hex":   "#4B5563",
					})
				}
				if day.WorkStart != "" && day.WorkEnd != "" {
					routineBlocks = append(routineBlocks, map[string]interface{}{
						"activity":    "Compromisso Fixo",
						"day_of_week": day.DayOfWeek,
						"start_time":  day.WorkStart,
						"end_time":    day.WorkEnd,
						"type":        "work",
						"color_hex":   "#EF4444",
					})
				}
				if day.LunchStart != "" && day.LunchEnd != "" {
					routineBlocks = append(routineBlocks, map[string]interface{}{
						"activity":    "Almoço/Pausa",
						"day_of_week": day.DayOfWeek,
						"start_time":  day.LunchStart,
						"end_time":    day.LunchEnd,
						"type":        "lunch",
						"color_hex":   "#F59E0B",
					})
				}
			}
		}
	}
	if routineBlocks == nil {
		routineBlocks = []map[string]interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{
		"analytics": gin.H{
			"today_minutes": todayLog.TotalMinutes,
			"today_blocks":  todayAnalyticsBlocks,
		},
		"study_strategy": strategy,
		"routine_blocks": routineBlocks,
	})
}

// ==========================================================
// 🛠️ 3. SUPER ROTA DE EDIÇÃO TOTAL DO CRONOGRAMA
// ==========================================================
func UpdateFullPlan(c *gin.Context) {
	spaceIDStr := c.Param("space_id")

	userIDInterface, _ := c.Get("userID")
	var parsedUserID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		parsedUserID = v
	case string:
		parsedUserID, _ = uuid.Parse(v)
	}

	var input SuperEditPlanInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "JSON inválido: " + err.Error()})
		return
	}

	tx := database.DB.Begin()

	var strategy models.StudyStrategy
	if err := tx.Where("space_id = ? AND mode = 'fixed' AND created_by_id = ?", spaceIDStr, parsedUserID).First(&strategy).Error; err != nil {
		tx.Rollback()
		c.JSON(404, gin.H{"error": "Cronograma não encontrado."})
		return
	}

	updates := map[string]interface{}{}
	if input.TargetGoal != nil {
		updates["target_goal"] = *input.TargetGoal
	}
	if input.HoursPerDay != nil {
		updates["hours_per_day"] = *input.HoursPerDay
	}
	if input.StudyDays != nil {
		studyDaysBytes, _ := json.Marshal(input.StudyDays)
		updates["study_days"] = gorm.Expr("?::jsonb", string(studyDaysBytes))
	}
	if input.MinSessionMin != nil {
		updates["min_session_min"] = *input.MinSessionMin
	}
	if input.MaxSessionMin != nil {
		updates["max_session_min"] = *input.MaxSessionMin
	}

	if len(updates) > 0 {
		if err := tx.Model(&strategy).Updates(updates).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"error": "Erro ao atualizar estratégia: " + err.Error()})
			return
		}
	}

	if len(input.Blocks) > 0 {
		tx.Where("strategy_id = ?", strategy.ID).Delete(&models.StudyBlock{})

		var newBlocks []models.StudyBlock
		for _, b := range input.Blocks {
			newBlock := models.StudyBlock{
				StrategyID:       strategy.ID,
				Activity:         b.Activity,
				NotebookID:       b.NotebookID,
				DayOfWeek:        b.DayOfWeek,
				StartTime:        b.StartTime,
				EndTime:          b.EndTime,
				SuggestedMinutes: b.SuggestedMinutes,
				Importance:       b.Importance,
				Performance:      b.Performance,
			}

			if b.ID != nil && *b.ID != uuid.Nil {
				newBlock.ID = *b.ID
			} else {
				newBlock.ID = uuid.New()
			}

			newBlocks = append(newBlocks, newBlock)
		}

		if err := tx.Create(&newBlocks).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"error": "Erro ao salvar a nova grade de horários."})
			return
		}
	}

	tx.Commit()

	c.JSON(200, gin.H{"message": "Cronograma inteiro atualizado com sucesso!"})
}

// ==========================================================
// ⏱️ 4. EXECUTAR E SALVAR BLOCO DO CRONOGRAMA
// ==========================================================
func ExecutePlanBlock(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	userIDInterface, _ := c.Get("userID")

	var input struct {
		BlockID          uuid.UUID `json:"block_id"`
		ActivityName     string    `json:"activity"`
		PlannedStartTime string    `json:"planned_start_time"`
		PlannedEndTime   string    `json:"planned_end_time"`
		RealStartTime    string    `json:"real_start_time"`
		RealEndTime      string    `json:"real_end_time"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "JSON inválido."})
		return
	}

	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	spaceID := uuid.MustParse(spaceIDStr)
	tx := database.DB.Begin()

	var strategy models.StudyStrategy
	if err := tx.Where("space_id = ? AND mode = 'fixed' AND created_by_id = ?", spaceID, userID).First(&strategy).Error; err != nil {
		tx.Rollback()
		c.JSON(404, gin.H{"error": "Cronograma não encontrado."})
		return
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var scheduleLog models.ScheduleLog
	if err := tx.Where("user_id = ? AND space_id = ? AND schedule_id = ? AND date = ?", userID, spaceID, strategy.ID, today).First(&scheduleLog).Error; err != nil {
		scheduleLog = models.ScheduleLog{UserID: userID, SpaceID: spaceID, ScheduleID: strategy.ID, Date: today, TotalMinutes: 0}
		tx.Create(&scheduleLog)
	}

	realStart := timeToMinutes(input.RealStartTime)
	realEnd := timeToMinutes(input.RealEndTime)
	duration := realEnd - realStart
	if duration < 0 {
		duration = 0
	}

	scheduleLog.TotalMinutes += duration
	tx.Save(&scheduleLog)

	hasRecalc := false
	plannedEnd := timeToMinutes(input.PlannedEndTime)
	if realEnd > plannedEnd {
		hasRecalc = true
	}

	if input.BlockID != uuid.Nil {
		var logBlock models.ScheduleLogBlock
		if err := tx.Where("schedule_log_id = ? AND block_id = ?", scheduleLog.ID, input.BlockID).First(&logBlock).Error; err != nil {
			logBlock = models.ScheduleLogBlock{
				ScheduleLogID:    scheduleLog.ID,
				BlockID:          input.BlockID,
				Activity:         input.ActivityName,
				PlannedStartTime: input.PlannedStartTime,
				PlannedEndTime:   input.PlannedEndTime,
			}
			tx.Create(&logBlock)
		}
		logBlock.RealStartTime = input.RealStartTime
		logBlock.RealEndTime = input.RealEndTime
		logBlock.TotalMinutes += duration
		logBlock.HasRecalculation = hasRecalc
		tx.Save(&logBlock)
	}

	plannedStart := timeToMinutes(input.PlannedStartTime)
	session := models.StudySession{
		UserID:         userID,
		SpaceID:        spaceID,
		ActivityName:   input.ActivityName,
		PlannedMinutes: plannedEnd - plannedStart,
		ActualMinutes:  duration,
	}
	tx.Create(&session)

	tx.Commit()

	c.JSON(200, gin.H{
		"message":        "Tempo de cronograma registrado com sucesso!",
		"actual_minutes": duration,
	})
}

// ==========================================================
// ✏️ 5. CRUD MANUAL DOS BLOCOS DO CRONOGRAMA
// ==========================================================
func CreateStudyPlan(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	var input CreatePlanBlockInput
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
		Plans []CreatePlanBlockInput `json:"plans"`
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
	var input CreatePlanBlockInput
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
// 🤖 6. AUTOFIT - ENCAIXE INTELIGENTE DOS BLOCOS CLONADOS
// ==========================================================
// 👇 RENOMEADO DE "GenerateAutoPlan" PARA "AutoFitPlanBlocks" PARA NÃO DAR ERRO 👇
func AutoFitPlanBlocks(c *gin.Context) {
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
	if err := database.DB.Preload("Blocks").Where("space_id = ? AND created_by_id = ?", spaceID, parsedUserID).First(&strategy).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cronograma não encontrado para este usuário."})
		return
	}

	var userRoutine models.AvailabilityProfile
	database.DB.Where("user_id = ? AND is_default = ?", parsedUserID, true).First(&userRoutine)
	if userRoutine.ID == uuid.Nil {
		database.DB.Where("user_id = ?", parsedUserID).First(&userRoutine)
	}

	routineMap := make(map[int]DailyScheduleInput)
	if userRoutine.ID != uuid.Nil && userRoutine.Schedule != "" {
		var dailyAvailability []DailyScheduleInput
		if err := json.Unmarshal([]byte(userRoutine.Schedule), &dailyAvailability); err == nil {
			for _, day := range dailyAvailability {
				routineMap[day.DayOfWeek] = day
			}
		}
	}

	blocksByDay := make(map[int][]models.StudyBlock)
	for _, b := range strategy.Blocks {
		if b.DayOfWeek != nil {
			blocksByDay[*b.DayOfWeek] = append(blocksByDay[*b.DayOfWeek], b)
		}
	}

	for day, blocks := range blocksByDay {
		routine, hasRoutine := routineMap[day]

		var currentCursorMin int

		if hasRoutine && routine.WakeUpTime != "" {
			currentCursorMin = timeToMinutes(routine.WakeUpTime) + 30
		} else {
			currentCursorMin = 8 * 60
		}

		for _, block := range blocks {
			if block.StartTime != nil && block.EndTime != nil {
				endMin := timeToMinutes(*block.EndTime)
				if endMin > currentCursorMin {
					currentCursorMin = endMin
				}
				continue
			}

			duration := 60

			if hasRoutine {
				if routine.WorkStart != "" && routine.WorkEnd != "" {
					workStartMin := timeToMinutes(routine.WorkStart)
					workEndMin := timeToMinutes(routine.WorkEnd)
					if currentCursorMin+duration > workStartMin && currentCursorMin < workEndMin {
						currentCursorMin = workEndMin + 30
					}
				}

				if routine.LunchStart != "" && routine.LunchEnd != "" {
					lunchStartMin := timeToMinutes(routine.LunchStart)
					lunchEndMin := timeToMinutes(routine.LunchEnd)
					if currentCursorMin+duration > lunchStartMin && currentCursorMin < lunchEndMin {
						currentCursorMin = lunchEndMin + 30
					}
				}
			}

			startStr := minutesToTime(currentCursorMin)
			endStr := minutesToTime(currentCursorMin + duration)

			database.DB.Model(&models.StudyBlock{}).Where("id = ?", block.ID).Updates(map[string]interface{}{
				"start_time": startStr,
				"end_time":   endStr,
			})

			currentCursorMin += duration + 15
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cronograma otimizado com sucesso com base na sua rotina!",
	})
}
