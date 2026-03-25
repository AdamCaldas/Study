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
// 🚀 1. GERAR CICLO (ADAPTIVE - Roleta)
// ==========================================================
func GenerateAutoCycle(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	spaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "ID do Space inválido"})
		return
	}

	userID, _ := c.Get("userID")

	var input GenerateStrategyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Dados inválidos enviados pelo front-end."})
		return
	}

	tx := database.DB.Begin()

	// Força o modo ADAPTIVE
	var strategy models.StudyStrategy
	if err := tx.Where("space_id = ? AND mode = 'adaptive'", spaceID).First(&strategy).Error; err != nil {
		strategy = models.StudyStrategy{SpaceID: spaceID, Mode: "adaptive"}
		tx.Create(&strategy)
	}

	// Limpa o ciclo velho desse Space
	tx.Where("strategy_id = ?", strategy.ID).Delete(&models.StudyBlock{})

	// ==========================================================
	// 🛠️ MÁGICA ANTI-DUPLICAÇÃO E CRIAÇÃO DE PÁGINA
	// ==========================================================
	notebooksCriados := make(map[string]uuid.UUID)

	for i, disc := range input.Disciplines {
		if disc.NotebookID == nil || *disc.NotebookID == uuid.Nil {

			// 1. CHECAGEM ANTI-DUPLICATA
			if idSalvo, jaCriou := notebooksCriados[disc.Name]; jaCriou {
				input.Disciplines[i].NotebookID = &idSalvo
				continue
			}

			// 2. Cria APENAS 1 Caderno
			newNb := models.Notebook{
				SpaceID:     spaceID,
				Name:        disc.Name,
				ColorHex:    "#3B82F6", // Azul padrão
				CreatedByID: userID.(uuid.UUID),
				UpdatedByID: userID.(uuid.UUID),
			}
			tx.Create(&newNb)

			// 3. Cria a primeira Página automaticamente
			newPage := models.Page{
				NotebookID:  newNb.ID,
				Title:       "Anotações - " + disc.Name,
				Content:     "{\"html\": \"<p>Comece a digitar seus resumos aqui...</p>\"}",
				Order:       0,
				CreatedByID: userID.(uuid.UUID),
				UpdatedByID: userID.(uuid.UUID),
			}
			tx.Create(&newPage)

			// 4. Salva no mapa e vincula o ID
			notebooksCriados[disc.Name] = newNb.ID
			input.Disciplines[i].NotebookID = &newNb.ID
		}
	}

	strategy.TargetGoal = input.TargetGoal
	strategy.HoursPerDay = input.HoursPerDay
	strategy.MinSessionMin = input.MinSessionMin
	strategy.CurrentStep = 0 // Zera a roleta sempre que recriar
	tx.Save(&strategy)

	var finalBlocks []models.StudyBlock
	dailyMinutes := input.HoursPerDay * 60

	// Matemática exclusiva do Ciclo (1 card por matéria)
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
		tx.Create(&finalBlocks)
	}
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"message":     "Ciclo configurado com sucesso!",
		"study_cycle": strategy, // Chave dedicada
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
