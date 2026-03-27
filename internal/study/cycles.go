package study

import (
	"math"
	"net/http"
	"time" // 👈 Importado para o Analytics e datas

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
			MinSessionMin: input.MinSessionMin,
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

			// 👇 RADAR ANTI-DUPLICAÇÃO (A MÁGICA DA FASE 3 AQUI!)
			var existingNb models.Notebook
			if err := tx.Where("space_id = ? AND name = ?", spaceID, disc.Name).First(&existingNb).Error; err == nil {
				// O caderno já existe nesse Space! Reutiliza para não duplicar.
				nbID := existingNb.ID
				notebooksCriados[disc.Name] = nbID
				input.Disciplines[i].NotebookID = &nbID
				continue // Pula para a próxima matéria sem criar nada novo!
			}

			// Se o caderno NÃO existir na turma, aí sim ele cria um novinho:
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
	strategy.MinSessionMin = input.MinSessionMin
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
// 📋 2. LISTAR O CICLO (Com Mágica de Clonagem e Analytics Embutido)
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

	// 🧠 FUNÇÃO NINJA: Calcula os minutos de HOJE desse aluno nessa turma
	getTodayMinutes := func() int {
		var total int
		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		database.DB.Model(&models.StudySession{}).
			Where("user_id = ? AND space_id = ? AND created_at >= ?", parsedUserID, spaceID, startOfDay).
			Select("COALESCE(SUM(actual_minutes), 0)").Scan(&total)
		return total
	}

	var strategy models.StudyStrategy
	// 1️⃣ Tenta buscar o ciclo DO USUÁRIO LOGADO primeiro
	if err := database.DB.Preload("Blocks", func(db *gorm.DB) *gorm.DB {
		return db.Order("sequence ASC")
	}).Where("space_id = ? AND mode = 'adaptive' AND created_by_id = ?", spaceID, parsedUserID).First(&strategy).Error; err != nil {

		// 2️⃣ SE ELE NÃO TEM UM CICLO... VAMOS CLONAR O DO DONO DO SPACE!
		var space models.Space
		if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err == nil {
			var ownerStrategy models.StudyStrategy

			// Pega a estratégia original do Dono do Space
			if err := database.DB.Preload("Blocks").Where("space_id = ? AND mode = 'adaptive' AND created_by_id = ?", spaceID, space.OwnerID).First(&ownerStrategy).Error; err == nil && space.OwnerID != parsedUserID {

				// 🧬 FAZ A CLONAGEM!
				newStrategy := ownerStrategy
				newStrategy.ID = uuid.New()            // Gera um ID novo
				newStrategy.CreatedByID = parsedUserID // Passa pro nome do convidado
				newStrategy.CurrentStep = 0            // 👈 ZERA A ROLETA PRA ELE!
				database.DB.Create(&newStrategy)

				// Clona os cards/matérias vinculados ao ciclo
				var newBlocks []models.StudyBlock
				for _, b := range ownerStrategy.Blocks {
					newBlock := b
					newBlock.ID = uuid.New()
					newBlock.StrategyID = newStrategy.ID // Vincula ao novo ciclo
					newBlocks = append(newBlocks, newBlock)
				}
				if len(newBlocks) > 0 {
					database.DB.Create(&newBlocks)
				}

				newStrategy.Blocks = newBlocks

				// 👇 RESPOSTA CLONADA COM O RELÓGIO (Analytics)
				c.JSON(200, gin.H{
					"study_cycle": newStrategy,
					"analytics": gin.H{
						"today_minutes": getTodayMinutes(),
					},
				})
				return
			}
		}

		c.JSON(200, gin.H{
			"message":     "Nenhum ciclo configurado ainda",
			"study_cycle": nil,
			"analytics":   gin.H{"today_minutes": 0},
		})
		return
	}

	// 👇 RESPOSTA NORMAL COM O RELÓGIO (Analytics)
	c.JSON(200, gin.H{
		"study_cycle": strategy,
		"analytics": gin.H{
			"today_minutes": getTodayMinutes(),
		},
	})
}

// ==========================================================
// 🔄 3. AVANÇAR A ROLETA (Timer / Advance) - BLINDADO POR DONO
// ==========================================================
func AdvanceCycleStep(c *gin.Context) {
	spaceID := c.Param("space_id")
	userIDInterface, _ := c.Get("userID")

	var input struct {
		ActualDuration int    `json:"actual_duration"`
		ActivityName   string `json:"activity_name"`
		PlannedMinutes int    `json:"planned_minutes"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "O Front-end precisa enviar o tempo real estudado."})
		return
	}

	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	tx := database.DB.Begin()

	session := models.StudySession{
		UserID:         userID,
		SpaceID:        uuid.MustParse(spaceID),
		ActivityName:   input.ActivityName,
		PlannedMinutes: input.PlannedMinutes,
		ActualMinutes:  input.ActualDuration,
	}

	if err := tx.Create(&session).Error; err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"error": "Erro ao salvar o histórico de tempo."})
		return
	}

	var strategy models.StudyStrategy
	// 👇 BLINDAGEM: Só avança o ciclo do cara logado!
	if err := tx.Preload("Blocks").Where("space_id = ? AND mode = 'adaptive' AND created_by_id = ?", spaceID, userID).First(&strategy).Error; err != nil {
		tx.Rollback()
		c.JSON(404, gin.H{"error": "Ciclo não encontrado para este usuário."})
		return
	}

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
		"message":          "Tempo registrado e próximo card liberado!",
		"actual_minutes":   input.ActualDuration,
		"next_step":        strategy.CurrentStep,
		"next_notebook_id": nextNotebookID,
	})
}

// ==========================================================
// ✏️ 4. CRUD MANUAL DO CICLO (Blindado)
// ==========================================================
func CreateCycleBlock(c *gin.Context) {
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
	// 👇 BLINDAGEM: Garante que tá criando no ciclo DELE.
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
	var input CreateManualBlockInput
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
	MinSessionMin *int     `json:"min_session_minutes"`
	CurrentStep   *int     `json:"current_step"` // Caso o aluno queira pular para o meio do ciclo
	Blocks        []struct {
		ID               *uuid.UUID `json:"id"` // Se for um card já existente
		Activity         string     `json:"activity"`
		NotebookID       *uuid.UUID `json:"notebook_id"`
		SuggestedMinutes int        `json:"suggested_minutes"`
		Sequence         int        `json:"sequence"`
		DayOfWeek        *int       `json:"day_of_week"`
	} `json:"blocks"` // Array exato de como a tela ficou após ele editar
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

	// 1. Acha o ciclo do cara
	var strategy models.StudyStrategy
	if err := tx.Where("space_id = ? AND mode = 'adaptive' AND created_by_id = ?", spaceIDStr, parsedUserID).First(&strategy).Error; err != nil {
		tx.Rollback()
		c.JSON(404, gin.H{"error": "Ciclo não encontrado."})
		return
	}

	// 2. Atualiza os dados principais (SÓ O QUE ELE MANDAR)
	updates := map[string]interface{}{}
	if input.TargetGoal != nil {
		updates["target_goal"] = *input.TargetGoal
	}
	if input.HoursPerDay != nil {
		updates["hours_per_day"] = *input.HoursPerDay
	}
	if input.MinSessionMin != nil {
		updates["min_session_minutes"] = *input.MinSessionMin
	}
	if input.CurrentStep != nil {
		updates["current_step"] = *input.CurrentStep
	}

	if len(updates) > 0 {
		if err := tx.Model(&strategy).Updates(updates).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"error": "Erro ao atualizar estratégia."})
			return
		}
	}

	// 3. Atualiza os Blocos (Apaga tudo e reescreve do jeito que o Front mandou)
	if len(input.Blocks) > 0 {
		// Limpa os blocos velhos
		tx.Where("strategy_id = ?", strategy.ID).Delete(&models.StudyBlock{})

		// Insere a nova ordem customizada pelo aluno
		var newBlocks []models.StudyBlock
		for _, b := range input.Blocks {
			newBlock := models.StudyBlock{
				StrategyID:       strategy.ID,
				Activity:         b.Activity,
				NotebookID:       b.NotebookID,
				SuggestedMinutes: b.SuggestedMinutes,
				Sequence:         b.Sequence,
				DayOfWeek:        b.DayOfWeek,
			}
			// Se o block já tinha ID (foi só editado e não criado do zero), mantém o ID
			if b.ID != nil && *b.ID != uuid.Nil {
				newBlock.ID = *b.ID
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
