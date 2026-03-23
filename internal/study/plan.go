package study

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==========================================================
// 📥 ESTRUTURAS DE ENTRADA (O novo Payload Unificado)
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
	Mode               string               `json:"mode" binding:"required"`
	TargetGoal         string               `json:"target_goal"`
	HoursPerDay        float64              `json:"hours_per_day"`
	DailyAvailability  []DailyScheduleInput `json:"daily_availability" binding:"required"`
	FreeTimePreference int                  `json:"free_time_preference"`
	MinSessionMin      int                  `json:"min_session_minutes"`
	MaxSessionMin      int                  `json:"max_session_minutes"`
	Disciplines        []DisciplineInput    `json:"disciplines" binding:"required"`
}

type CreateManualBlockInput struct {
	DayOfWeek  *int       `json:"day_of_week"`
	StartTime  *string    `json:"start_time"`
	EndTime    *string    `json:"end_time"`
	Activity   string     `json:"activity" binding:"required"`
	NotebookID *uuid.UUID `json:"notebook_id"`
}

// ==========================================================
// 🧮 FUNÇÕES AUXILIARES (Matemática e Tempo)
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
				Activity:         "Estudar: " + disc.Name,
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
// 🚀 1. O MOTOR UNIFICADO (Gera Ciclo ou Cronograma)
// ==========================================================
func GenerateAutoPlan(c *gin.Context) {
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

	var strategy models.StudyStrategy
	if err := tx.Where("space_id = ?", spaceID).First(&strategy).Error; err != nil {
		strategy = models.StudyStrategy{SpaceID: spaceID}
		tx.Create(&strategy)
	}

	tx.Where("strategy_id = ?", strategy.ID).Delete(&models.StudyBlock{})

	for i, disc := range input.Disciplines {
		if disc.NotebookID == nil || *disc.NotebookID == uuid.Nil {
			newNb := models.Notebook{
				SpaceID:     spaceID,
				Name:        disc.Name,
				ColorHex:    "#8B5CF6",
				CreatedByID: userID.(uuid.UUID),
				UpdatedByID: userID.(uuid.UUID),
			}
			tx.Create(&newNb)
			input.Disciplines[i].NotebookID = &newNb.ID
		}
	}

	availDaysJSON, _ := json.Marshal(input.DailyAvailability)
	strategy.Mode = input.Mode
	strategy.TargetGoal = input.TargetGoal
	strategy.HoursPerDay = input.HoursPerDay
	strategy.DailyAvailability = string(availDaysJSON)
	strategy.FreeTimePreference = input.FreeTimePreference
	strategy.MinSessionMin = input.MinSessionMin
	strategy.MaxSessionMin = input.MaxSessionMin
	tx.Save(&strategy)

	dailyMinutes := input.HoursPerDay * 60
	baseBlocks := calculateDistribution(input.Disciplines, dailyMinutes, input.MinSessionMin, input.MaxSessionMin)
	var finalBlocks []models.StudyBlock

	if input.Mode == "adaptive" {
		for _, block := range baseBlocks {
			block.StrategyID = strategy.ID
			finalBlocks = append(finalBlocks, block)
		}

	} else {
		subjectIndex := 0
		sessionLength := input.MaxSessionMin
		if sessionLength == 0 {
			sessionLength = 50
		}
		breakLength := 10

		// 🧙‍♂️ MÁGICA ATUALIZADA: Iterando sobre as configurações INDIVIDUAIS de cada dia
		for _, dayConfig := range input.DailyAvailability {
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

					dayCopy := dayConfig.DayOfWeek // Pega o dia correto da config
					startCopy := minutesToTime(curr)
					endCopy := minutesToTime(curr + sessionLength)

					newBlock := models.StudyBlock{
						StrategyID: strategy.ID,
						NotebookID: baseBlock.NotebookID,
						Activity:   baseBlock.Activity,
						DayOfWeek:  &dayCopy,
						StartTime:  &startCopy,
						EndTime:    &endCopy,
					}
					finalBlocks = append(finalBlocks, newBlock)

					subjectIndex++
					curr += sessionLength + breakLength
				}
			}
		}
	}

	if len(finalBlocks) > 0 {
		tx.Create(&finalBlocks)
	}
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"message":  "Estratégia configurada com sucesso!",
		"strategy": strategy,
		"blocks":   finalBlocks,
	})
}

// ==========================================================
// 📋 2. LISTAR ESTRATÉGIA E BLOCOS
// ==========================================================
func ListPlans(c *gin.Context) {
	spaceID := c.Param("space_id")

	var strategy models.StudyStrategy
	if err := database.DB.Preload("Blocks").Where("space_id = ?", spaceID).First(&strategy).Error; err != nil {
		c.JSON(200, gin.H{"message": "Nenhuma estratégia configurada ainda", "strategy": nil})
		return
	}

	c.JSON(200, gin.H{"strategy": strategy})
}

// ==========================================================
// ✏️ 3. CRUD MANUAL DOS BLOCOS
// ==========================================================
func CreateStudyPlan(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	var input CreateManualBlockInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Dados inválidos."})
		return
	}

	var strategy models.StudyStrategy
	database.DB.Where("space_id = ?", spaceIDStr).First(&strategy)
	if strategy.ID == uuid.Nil {
		c.JSON(400, gin.H{"error": "Você precisa gerar uma estratégia base primeiro."})
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

	var strategy models.StudyStrategy
	database.DB.Where("space_id = ?", spaceIDStr).First(&strategy)

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
// 🔄 4. AVANÇAR PASSO (Para o modo Adaptive)
// ==========================================================
func AdvanceStrategyStep(c *gin.Context) {
	spaceID := c.Param("space_id")
	var strategy models.StudyStrategy

	if err := database.DB.Preload("Blocks").Where("space_id = ?", spaceID).First(&strategy).Error; err != nil {
		c.JSON(404, gin.H{"error": "Estratégia não encontrada"})
		return
	}

	totalItems := len(strategy.Blocks)
	if totalItems > 0 {
		strategy.CurrentStep = (strategy.CurrentStep + 1) % totalItems
		database.DB.Save(&strategy)
	}

	c.JSON(200, gin.H{"message": "Avançado!", "current_step": strategy.CurrentStep})
}
