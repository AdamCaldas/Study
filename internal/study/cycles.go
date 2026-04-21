package study

import (
	"encoding/json"
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
// 🚀 1. GERAR CICLO (ADAPTIVE - Roleta) - DEFINITIVO E LIMPO
// ==========================================================
func GenerateAutoCycle(c *gin.Context) {
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
		c.JSON(400, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	if input.MinSessionMin <= 0 {
		input.MinSessionMin = 30
	}
	if input.MaxSessionMin <= 0 {
		input.MaxSessionMin = 50
	}

	tx := database.DB.Begin()

	var strategy models.StudyStrategy
	if err := tx.Where("space_id = ? AND mode = 'adaptive' AND created_by_id = ?", spaceID, parsedUserID).First(&strategy).Error; err != nil {
		strategy = models.StudyStrategy{
			SpaceID:       spaceID,
			Mode:          "adaptive",
			TargetGoal:    input.TargetGoal,
			HoursPerDay:   input.HoursPerDay,
			StudyDays:     input.StudyDays,
			MinSessionMin: input.MinSessionMin,
			MaxSessionMin: input.MaxSessionMin,
			CurrentStep:   0,
			CreatedByID:   parsedUserID,
		}
		if err := tx.Create(&strategy).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"error": "Erro do Banco ao criar estratégia: " + err.Error()})
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
				ColorHex:    "#3B82F6",
				CreatedByID: parsedUserID,
				UpdatedByID: parsedUserID,
			}
			if err := tx.Create(&newNb).Error; err != nil {
				tx.Rollback()
				c.JSON(500, gin.H{"error": "Erro ao criar caderno de " + disc.Name + ": " + err.Error()})
				return
			}

			// 👇 A MÁGICA ENTRA AQUI: Criar o Guia (Pasta) antes da Página!
			newGuide := models.Guide{
				NotebookID:  newNb.ID,
				Name:        "Geral", // Nome padrão da primeira pasta
				Order:       0,
				CreatedByID: parsedUserID,
				UpdatedByID: parsedUserID,
			}
			if err := tx.Create(&newGuide).Error; err != nil {
				tx.Rollback()
				c.JSON(500, gin.H{"error": "Erro ao criar guia de " + disc.Name + ": " + err.Error()})
				return
			}

			// 👇 Agora a Página é criada com o GuideID certinho!
			newPage := models.Page{
				NotebookID:  newNb.ID,
				GuideID:     newGuide.ID, // 👈 O ID DA PASTA SALVA A PÁTRIA!
				Title:       "Anotações - " + disc.Name,
				Content:     "{\"html\": \"<p>Comece a digitar seus resumos aqui...</p>\"}",
				Order:       0,
				CreatedByID: parsedUserID,
				UpdatedByID: parsedUserID,
			}
			if err := tx.Create(&newPage).Error; err != nil {
				tx.Rollback()
				c.JSON(500, gin.H{"error": "Erro ao criar página de " + disc.Name + ": " + err.Error()})
				return
			}

			nbID := newNb.ID
			notebooksCriados[disc.Name] = nbID
			input.Disciplines[i].NotebookID = &nbID
		}
	}

	strategy.TargetGoal = input.TargetGoal
	strategy.HoursPerDay = input.HoursPerDay
	strategy.StudyDays = input.StudyDays
	strategy.MinSessionMin = input.MinSessionMin
	strategy.MaxSessionMin = input.MaxSessionMin
	strategy.CurrentStep = 0
	if err := tx.Save(&strategy).Error; err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"error": "Erro ao atualizar parâmetros: " + err.Error()})
		return
	}

	var finalBlocks []models.StudyBlock
	dailyMinutes := input.HoursPerDay * 60

	var totalWeight float64 = 0
	for _, disc := range input.Disciplines {
		totalWeight += float64(disc.Importance + (6 - disc.Performance))
	}

	sequence := 1
	for _, disc := range input.Disciplines {
		weight := float64(disc.Importance + (6 - disc.Performance))
		proportion := weight / totalWeight
		suggestedMin := int(math.Round(proportion * dailyMinutes))

		if input.MinSessionMin > 0 && suggestedMin < input.MinSessionMin {
			suggestedMin = input.MinSessionMin
		}

		finalBlocks = append(finalBlocks, models.StudyBlock{
			StrategyID:       strategy.ID,
			NotebookID:       disc.NotebookID,
			Activity:         disc.Name,
			Importance:       disc.Importance,
			Performance:      disc.Performance,
			SuggestedMinutes: suggestedMin,
			Sequence:         sequence,
			DayOfWeek:        disc.DayOfWeek,
		})
		sequence++
	}

	if len(finalBlocks) > 0 {
		if err := tx.Create(&finalBlocks).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"error": "Erro ao salvar os blocos no banco: " + err.Error()})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro fatal ao confirmar transação: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Ciclo configurado com sucesso!",
		"study_cycle": strategy,
		"blocks":      finalBlocks,
	})
}

// ==========================================================
// 📋 2. LISTAR O CICLO (Preserva o JSON original do banco)
// ==========================================================
func ListCycles(c *gin.Context) {
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
		return db.Order("sequence ASC")
	}).Where("space_id = ? AND mode = 'adaptive' AND created_by_id = ?", spaceID, parsedUserID).First(&strategy).Error; err != nil {

		var space models.Space
		if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err == nil {
			var ownerStrategy models.StudyStrategy

			if err := database.DB.Preload("Blocks").Where("space_id = ? AND mode = 'adaptive' AND created_by_id = ?", spaceID, space.OwnerID).First(&ownerStrategy).Error; err == nil && space.OwnerID != parsedUserID {

				newStrategy := ownerStrategy
				newStrategy.ID = uuid.New()
				newStrategy.CreatedByID = parsedUserID
				newStrategy.CurrentStep = 0
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
				strategy = newStrategy
			} else {
				c.JSON(200, gin.H{"message": "Nenhum ciclo configurado ainda", "study_cycle": nil})
				return
			}
		} else {
			c.JSON(200, gin.H{"message": "Nenhum ciclo configurado ainda", "study_cycle": nil})
			return
		}
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if strategy.UpdatedAt.Before(today) {
		strategy.CurrentStep = 0
		database.DB.Save(&strategy)
	}

	var todayLog models.CycleLog
	database.DB.Preload("Blocks").Where("cycle_id = ? AND date = ?", strategy.ID, today).First(&todayLog)

	todayBlocksMap := make(map[uuid.UUID]models.CycleLogBlock)
	var todayAnalyticsBlocks []map[string]interface{}

	for _, b := range todayLog.Blocks {
		todayBlocksMap[b.BlockID] = b // Mantém o BlockID para o banco original
		todayAnalyticsBlocks = append(todayAnalyticsBlocks, map[string]interface{}{
			"block_id":         b.BlockID,
			"activity":         b.Activity,
			"total_minutes":    b.TotalMinutes,
			"planned_minutes":  b.PlannedMinutes,
			"missing_minutes":  b.MissingMinutes,
			"last_activity_at": b.LastActivityAt,
		})
	}
	if todayAnalyticsBlocks == nil {
		todayAnalyticsBlocks = []map[string]interface{}{}
	}

	var responseBlocks []map[string]interface{}
	for _, block := range strategy.Blocks {
		logData := todayBlocksMap[block.ID]

		responseBlocks = append(responseBlocks, map[string]interface{}{
			"id":                        block.ID,
			"strategy_id":               block.StrategyID,
			"notebook_id":               block.NotebookID,
			"activity":                  block.Activity,
			"color_hex":                 block.ColorHex,
			"sequence":                  block.Sequence,
			"importance":                block.Importance,
			"performance":               block.Performance,
			"suggested_minutes":         block.SuggestedMinutes,
			"day_of_week":               block.DayOfWeek,
			"accumulated_minutes_today": logData.TotalMinutes,
			"missing_minutes_today":     logData.MissingMinutes,
			"last_activity_at":          logData.LastActivityAt,
		})
	}

	var pastLogs []models.CycleLog
	database.DB.Preload("Blocks").Where("cycle_id = ? AND date < ?", strategy.ID, today).Order("date DESC").Find(&pastLogs)

	var formattedLogs []map[string]interface{}
	for _, pl := range pastLogs {
		var logBlocks []map[string]interface{}
		for _, plb := range pl.Blocks {
			logBlocks = append(logBlocks, map[string]interface{}{
				"block_id":         plb.BlockID,
				"activity":         plb.Activity,
				"total_minutes":    plb.TotalMinutes,
				"planned_minutes":  plb.PlannedMinutes,
				"missing_minutes":  plb.MissingMinutes,
				"last_activity_at": plb.LastActivityAt,
			})
		}
		formattedLogs = append(formattedLogs, map[string]interface{}{
			"date":          pl.Date.Format("2006-01-02"),
			"total_minutes": pl.TotalMinutes,
			"blocks":        logBlocks,
		})
	}
	if formattedLogs == nil {
		formattedLogs = []map[string]interface{}{}
	}

	c.JSON(200, gin.H{
		"analytics": gin.H{
			"today_minutes": todayLog.TotalMinutes,
			"today_blocks":  todayAnalyticsBlocks,
		},
		"study_cycle": gin.H{
			"id":                   strategy.ID,
			"space_id":             strategy.SpaceID,
			"mode":                 strategy.Mode,
			"source":               strategy.Source,
			"target_goal":          strategy.TargetGoal,
			"hours_per_day":        strategy.HoursPerDay,
			"study_days":           strategy.StudyDays,
			"min_session_minutes":  strategy.MinSessionMin,
			"max_session_minutes":  strategy.MaxSessionMin,
			"free_time_preference": strategy.FreeTimePreference,
			"current_step":         strategy.CurrentStep,
			"blocks":               responseBlocks,
			"created_by_id":        strategy.CreatedByID,
			"created_at":           strategy.CreatedAt,
			"updated_at":           strategy.UpdatedAt,
		},
		"study_logs": formattedLogs,
	})
}

// ==========================================================
// 🔄 3. AVANÇAR A ROLETA E SALVAR LOGS DIÁRIOS
// ==========================================================
func AdvanceCycleStep(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	userIDInterface, _ := c.Get("userID")

	var input struct {
		ActualDuration int       `json:"actual_duration"`
		ActivityName   string    `json:"activity_name"`
		PlannedMinutes int       `json:"planned_minutes"`
		BlockID        uuid.UUID `json:"block_id"`
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
	if err := tx.Preload("Blocks").Where("space_id = ? AND mode = 'adaptive' AND created_by_id = ?", spaceID, userID).First(&strategy).Error; err != nil {
		tx.Rollback()
		c.JSON(404, gin.H{"error": "Ciclo não encontrado."})
		return
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if strategy.UpdatedAt.Before(today) {
		strategy.CurrentStep = 0
	}

	var cycleLog models.CycleLog
	if err := tx.Where("user_id = ? AND space_id = ? AND cycle_id = ? AND date = ?", userID, spaceID, strategy.ID, today).First(&cycleLog).Error; err != nil {
		cycleLog = models.CycleLog{UserID: userID, SpaceID: spaceID, CycleID: strategy.ID, Date: today, TotalMinutes: 0}
		tx.Create(&cycleLog)
	}
	cycleLog.TotalMinutes += input.ActualDuration
	tx.Save(&cycleLog)

	missing := input.PlannedMinutes - input.ActualDuration
	if missing < 0 {
		missing = 0
	}

	if input.BlockID != uuid.Nil {
		var logBlock models.CycleLogBlock
		if err := tx.Where("cycle_log_id = ? AND block_id = ?", cycleLog.ID, input.BlockID).First(&logBlock).Error; err != nil {
			logBlock = models.CycleLogBlock{CycleLogID: cycleLog.ID, BlockID: input.BlockID, Activity: input.ActivityName}
			tx.Create(&logBlock)
		}
		logBlock.TotalMinutes += input.ActualDuration
		logBlock.PlannedMinutes += input.PlannedMinutes
		logBlock.MissingMinutes += missing
		logBlock.LastActivityAt = now
		tx.Save(&logBlock)
	}

	session := models.StudySession{
		UserID:         userID,
		SpaceID:        spaceID,
		ActivityName:   input.ActivityName,
		PlannedMinutes: input.PlannedMinutes,
		ActualMinutes:  input.ActualDuration,
	}
	tx.Create(&session)

	var nextNotebookID *uuid.UUID
	totalItems := len(strategy.Blocks)
	if totalItems > 0 {
		strategy.CurrentStep = (strategy.CurrentStep + 1) % totalItems
		tx.Save(&strategy)

		for _, block := range strategy.Blocks {
			if block.Sequence == (strategy.CurrentStep + 1) {
				nextNotebookID = block.NotebookID
				break
			}
		}
	}

	tx.Commit()

	c.JSON(200, gin.H{
		"message":          "Tempo e dívidas acumulados com sucesso!",
		"actual_minutes":   input.ActualDuration,
		"missing_minutes":  missing,
		"next_step":        strategy.CurrentStep,
		"next_notebook_id": nextNotebookID,
	})
}

// ==========================================================
// ✏️ 4. CRUD MANUAL DO CICLO (Blindado)
// ==========================================================
type CreateCycleBlockInput struct {
	Activity   string     `json:"activity" binding:"required"`
	NotebookID *uuid.UUID `json:"notebook_id"`
}

func CreateCycleBlock(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	var input CreateCycleBlockInput
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
	database.DB.Where("space_id = ? AND mode = 'adaptive' AND created_by_id = ?", spaceIDStr, parsedUserID).First(&strategy)
	if strategy.ID == uuid.Nil {
		c.JSON(400, gin.H{"error": "Você precisa gerar um ciclo base primeiro."})
		return
	}

	newBlock := models.StudyBlock{
		StrategyID: strategy.ID,
		Activity:   input.Activity,
		NotebookID: input.NotebookID,
	}

	database.DB.Create(&newBlock)
	c.JSON(201, gin.H{"message": "Card adicionado ao ciclo!", "block": newBlock})
}

func UpdateCycleBlock(c *gin.Context) {
	blockID := c.Param("block_id")
	var input CreateCycleBlockInput
	c.ShouldBindJSON(&input)

	database.DB.Model(&models.StudyBlock{}).Where("id = ?", blockID).Updates(map[string]interface{}{
		"activity":    input.Activity,
		"notebook_id": input.NotebookID,
	})
	c.JSON(200, gin.H{"message": "Card atualizado"})
}

func DeleteCycleBlock(c *gin.Context) {
	blockID := c.Param("block_id")
	database.DB.Where("id = ?", blockID).Delete(&models.StudyBlock{})
	c.JSON(200, gin.H{"message": "Card removido"})
}

// ==========================================================
// 🛠️ 5. SUPER ROTA DE EDIÇÃO TOTAL (Sync de Ciclo)
// ==========================================================
type SuperEditCycleInput struct {
	TargetGoal    *string  `json:"target_goal"`
	HoursPerDay   *float64 `json:"hours_per_day"`
	StudyDays     []int    `json:"study_days"`
	MinSessionMin *int     `json:"min_session_minutes"`
	MaxSessionMin *int     `json:"max_session_minutes"`
	CurrentStep   *int     `json:"current_step"`
	Blocks        []struct {
		ID               *uuid.UUID `json:"id"`
		Activity         string     `json:"activity"`
		NotebookID       *uuid.UUID `json:"notebook_id"`
		SuggestedMinutes int        `json:"suggested_minutes"`
		Sequence         int        `json:"sequence"`
		DayOfWeek        *int       `json:"day_of_week"`
		// 👇 OS DOIS CAMPOS QUE FALTAVAM ESTÃO AQUI AGORA! 👇
		Importance  int `json:"importance"`
		Performance int `json:"performance"`
	} `json:"blocks"`
}

func UpdateFullCycle(c *gin.Context) {
	spaceIDStr := c.Param("space_id")

	userIDInterface, _ := c.Get("userID")
	var parsedUserID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		parsedUserID = v
	case string:
		parsedUserID, _ = uuid.Parse(v)
	}

	var input SuperEditCycleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "JSON inválido: " + err.Error()})
		return
	}

	tx := database.DB.Begin()

	var strategy models.StudyStrategy
	if err := tx.Where("space_id = ? AND mode = 'adaptive' AND created_by_id = ?", spaceIDStr, parsedUserID).First(&strategy).Error; err != nil {
		tx.Rollback()
		c.JSON(404, gin.H{"error": "Ciclo não encontrado."})
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
	if input.CurrentStep != nil {
		updates["current_step"] = *input.CurrentStep
	}

	if len(updates) > 0 {
		if err := tx.Model(&strategy).Updates(updates).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"error": "Erro ao atualizar estratégia: " + err.Error()})
			return
		}
	}

	// Salva os IDs velhos na memória antes de apagar!
	var oldBlocks []models.StudyBlock
	tx.Where("strategy_id = ?", strategy.ID).Find(&oldBlocks)

	oldIdMap := make(map[string]uuid.UUID)
	for _, ob := range oldBlocks {
		if ob.NotebookID != nil && *ob.NotebookID != uuid.Nil {
			oldIdMap[ob.NotebookID.String()] = ob.ID
		} else {
			oldIdMap[ob.Activity] = ob.ID
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
				SuggestedMinutes: b.SuggestedMinutes,
				Sequence:         b.Sequence,
				DayOfWeek:        b.DayOfWeek,
				// 👇 SALVANDO OS DADOS NO BANCO AGORA! 👇
				Importance:  b.Importance,
				Performance: b.Performance,
			}

			// Tenta usar o ID que o Front mandou ou resgata o original da memória
			if b.ID != nil && *b.ID != uuid.Nil {
				newBlock.ID = *b.ID
			} else {
				var key string
				if b.NotebookID != nil && *b.NotebookID != uuid.Nil {
					key = b.NotebookID.String()
				} else {
					key = b.Activity
				}

				if savedID, exists := oldIdMap[key]; exists {
					newBlock.ID = savedID
				}
			}

			newBlocks = append(newBlocks, newBlock)
		}

		if err := tx.Create(&newBlocks).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"error": "Erro ao salvar a nova ordem dos cards."})
			return
		}
	}

	tx.Commit()

	c.JSON(200, gin.H{"message": "Ciclo inteiro atualizado com sucesso!"})
}
