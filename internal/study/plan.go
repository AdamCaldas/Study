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
	Mode               string            `json:"mode" binding:"required"`
	Source             string            `json:"source"` // "user" ou "institution"
	TargetGoal         string            `json:"target_goal"`
	HoursPerDay        float64           `json:"hours_per_day"`
	AvailabilityID     *uuid.UUID        `json:"availability_id" binding:"required"` // Puxa do Perfil Global
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

	// 👉 1. A MÁGICA AQUI: Buscar a rotina global do usuário!
	var availabilityProfile models.AvailabilityProfile
	if err := tx.Where("id = ? AND user_id = ?", input.AvailabilityID, userID).First(&availabilityProfile).Error; err != nil {
		tx.Rollback()
		c.JSON(404, gin.H{"error": "Perfil de disponibilidade não encontrado."})
		return
	}

	// Transformar a string JSON do banco de volta em um Array de horários pro motor ler
	var dailyAvailability []DailyScheduleInput
	if err := json.Unmarshal([]byte(availabilityProfile.Schedule), &dailyAvailability); err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"error": "Erro ao ler os horários do perfil."})
		return
	}

	// 👉 2. Destruição e Recriação da Estratégia
	var strategy models.StudyStrategy
	if err := tx.Where("space_id = ?", spaceID).First(&strategy).Error; err != nil {
		strategy = models.StudyStrategy{SpaceID: spaceID}
		tx.Create(&strategy)
	}

	tx.Where("strategy_id = ?", strategy.ID).Delete(&models.StudyBlock{})

	// Cria os cadernos vazios
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

	// 👉 3. Atualizar a Estratégia com o novo campo "Source"
	strategy.Mode = input.Mode
	if input.Source != "" {
		strategy.Source = input.Source
	} else {
		strategy.Source = "user"
	}
	strategy.TargetGoal = input.TargetGoal
	strategy.HoursPerDay = input.HoursPerDay
	strategy.FreeTimePreference = input.FreeTimePreference
	strategy.MinSessionMin = input.MinSessionMin
	strategy.MaxSessionMin = input.MaxSessionMin
	tx.Save(&strategy)

	dailyMinutes := input.HoursPerDay * 60

	// 👉 4. Chama a Matemática enviando a rotina do Perfil Global
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

		// Iterando sobre as configurações INDIVIDUAIS do Perfil Global
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
// 🔄 4. AVANÇAR PASSO E REGISTRAR TEMPO REAL (Para Relatórios)
// ==========================================================
func AdvanceStrategyStep(c *gin.Context) {
	spaceID := c.Param("space_id")
	userID, _ := c.Get("userID")

	// Estrutura para receber os dados do cronômetro do Front-end
	var input struct {
		ActualDuration int    `json:"actual_duration"` // Tempo real que o aluno ficou (ex: 180 min)
		ActivityName   string `json:"activity_name"`   // Nome da matéria (ex: Matemática)
		PlannedMinutes int    `json:"planned_minutes"` // O tempo que o sistema tinha sugerido (ex: 60 min)
	}

	// Se o Front não mandar o tempo, a gente dá erro para não perder o relatório
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "O Front-end precisa enviar o tempo real estudado."})
		return
	}

	tx := database.DB.Begin()

	// 1. Criamos o registro do "Ponto" (Sessão de Estudo)
	session := models.StudySession{
		UserID:         userID.(uuid.UUID),
		SpaceID:        uuid.MustParse(spaceID),
		ActivityName:   input.ActivityName,
		PlannedMinutes: input.PlannedMinutes,
		ActualMinutes:  input.ActualDuration, // Salvando as horas reais!
	}

	if err := tx.Create(&session).Error; err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"error": "Erro ao salvar o histórico de tempo."})
		return
	}

	// 2. Avançamos o card do ciclo (Lógica original mantida)
	var strategy models.StudyStrategy
	if err := tx.Preload("Blocks").Where("space_id = ?", spaceID).First(&strategy).Error; err != nil {
		tx.Rollback()
		c.JSON(404, gin.H{"error": "Estratégia não encontrada"})
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
// 📊 GET ANALYTICS: Relatório de Desempenho do Aluno
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

	// 1. Buscamos todas as sessões do aluno (Vamos focar nos últimos 30 dias para não pesar o banco)
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	var sessions []models.StudySession

	if err := database.DB.Where("user_id = ? AND created_at >= ?", userID, thirtyDaysAgo).Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar histórico de estudos."})
		return
	}

	// 2. Variáveis para somar os totais
	var todayMinutes, weekMinutes, extraMinutes int
	subjectTotals := make(map[string]int)

	// Lógica para descobrir o início de "Hoje" e o início da "Semana"
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfWeek := startOfDay.AddDate(0, 0, -int(now.Weekday())) // Volta até o Domingo

	// 3. A Matemática (Iterando sobre as sessões salvas)
	for _, s := range sessions {
		// Gráfico de Pizza: Soma tempo por matéria
		subjectTotals[s.ActivityName] += s.ActualMinutes

		// Calcula apenas o "suor extra" (O que ele estudou a mais do que o planejado)
		if s.ActualMinutes > s.PlannedMinutes {
			extraMinutes += (s.ActualMinutes - s.PlannedMinutes)
		}

		// Filtros de tempo
		if s.CreatedAt.After(startOfDay) {
			todayMinutes += s.ActualMinutes
		}
		if s.CreatedAt.After(startOfWeek) {
			weekMinutes += s.ActualMinutes
		}
	}

	// 4. Formata o mapa de matérias num array de JSON para o Front-end ler fácil
	type SubjectStat struct {
		Name    string `json:"name"`
		Minutes int    `json:"minutes"`
	}
	var distribution []SubjectStat
	for name, mins := range subjectTotals {
		distribution = append(distribution, SubjectStat{Name: name, Minutes: mins})
	}

	// 5. O Payload Perfeito
	c.JSON(http.StatusOK, gin.H{
		"overview": gin.H{
			"today_minutes": todayMinutes,
			"week_minutes":  weekMinutes,
			"extra_minutes": extraMinutes,
		},
		"subject_distribution": distribution,
	})
}
