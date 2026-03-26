package study

import (
	"math"
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==========================================================
// 🚀 1. GERAR CICLO (ADAPTIVE - Roleta) - DEFINITIVO
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

	tx := database.DB.Begin()

	var strategy models.StudyStrategy
	// 👇 Busca APENAS o ciclo adaptativo dessa turma
	if err := tx.Where("space_id = ? AND mode = 'adaptive'", spaceID).First(&strategy).Error; err != nil {
		strategy = models.StudyStrategy{
			SpaceID:       spaceID,
			Mode:          "adaptive",
			TargetGoal:    input.TargetGoal,
			HoursPerDay:   input.HoursPerDay,
			MinSessionMin: input.MinSessionMin,
			CurrentStep:   0,
		}
		if err := tx.Create(&strategy).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"error": "Erro do Banco ao criar estratégia: " + err.Error()})
			return
		}
	}

	// Limpa apenas os blocos desse ciclo específico
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
				ColorHex:    "#3B82F6",
				CreatedByID: parsedUserID,
				UpdatedByID: parsedUserID,
			}
			if err := tx.Create(&newNb).Error; err != nil {
				tx.Rollback()
				c.JSON(500, gin.H{"error": "Erro ao criar caderno de " + disc.Name + ": " + err.Error()})
				return
			}

			// 👇 MÁGICA DO BYPASS DE TAGS (Raw Map)
			newPageID := uuid.New()
			err := tx.Table("pages").Create(map[string]interface{}{
				"id":            newPageID,
				"notebook_id":   newNb.ID,
				"title":         "Anotações - " + disc.Name,
				"content":       "{\"html\": \"<p>Comece a digitar seus resumos aqui...</p>\"}",
				"order":         0,
				"created_by_id": parsedUserID,
				"updated_by_id": parsedUserID,
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

	// Atualiza os dados se a estratégia já existia
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
// 📋 2. LISTAR O CICLO (Apenas ADAPTIVE)
// ==========================================================
func ListCycles(c *gin.Context) {
	spaceID := c.Param("space_id")

	var strategy models.StudyStrategy
	if err := database.DB.Preload("Blocks", func(db *gorm.DB) *gorm.DB {
		return db.Order("sequence ASC")
	}).Where("space_id = ? AND mode = 'adaptive'", spaceID).First(&strategy).Error; err != nil {
		c.JSON(200, gin.H{
			"message":     "Nenhum ciclo configurado ainda",
			"study_cycle": nil,
		})
		return
	}

	c.JSON(200, gin.H{
		"study_cycle": strategy,
	})
}

// ==========================================================
// 🔄 3. AVANÇAR A ROLETA (Timer / Advance)
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

	// 1. Salva as horas estudadas pro gráfico
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

	// 2. Gira a roleta (Sempre do Ciclo/Adaptive)
	var strategy models.StudyStrategy
	if err := tx.Preload("Blocks").Where("space_id = ? AND mode = 'adaptive'", spaceID).First(&strategy).Error; err != nil {
		tx.Rollback()
		c.JSON(404, gin.H{"error": "Ciclo não encontrado"})
		return
	}

	totalItems := len(strategy.Blocks)
	if totalItems > 0 {
		strategy.CurrentStep = (strategy.CurrentStep + 1) % totalItems
		tx.Save(&strategy)
	}

	tx.Commit()

	c.JSON(200, gin.H{
		"message":        "Tempo registrado e próximo card liberado!",
		"actual_minutes": input.ActualDuration,
		"next_step":      strategy.CurrentStep,
	})
}

// ==========================================================
// ✏️ 4. CRUD MANUAL DO CICLO (Opcional - Adicionar Card Extra)
// ==========================================================
func CreateCycleBlock(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	var input CreateManualBlockInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Dados inválidos."})
		return
	}

	var strategy models.StudyStrategy
	database.DB.Where("space_id = ? AND mode = 'adaptive'", spaceIDStr).First(&strategy)
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
