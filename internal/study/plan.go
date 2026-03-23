package study

import (
	"fmt"
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CreateStudyPlanInput struct {
	DayOfWeek  int       `json:"day_of_week" binding:"required"` // 0 = Domingo, 1 = Segunda...
	StartTime  string    `json:"start_time" binding:"required"`  // Ex: "08:00"
	EndTime    string    `json:"end_time" binding:"required"`    // Ex: "10:00"
	NotebookID uuid.UUID `json:"notebook_id"`
	Activity   string    `json:"activity"` // Caso não seja um caderno, pode ser só um texto ex: "Revisão Geral"
}

func CreateStudyPlan(c *gin.Context) {
	userID, _ := c.Get("userID")
	spaceID := c.Param("space_id")

	// 1. Valida se o usuário é dono do Space
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Acesso negado a este Space"})
		return
	}

	// 2. Valida a entrada de dados do Frontend
	var input CreateStudyPlanInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: verifique os horários e o dia da semana"})
		return
	}

	// 3. Monta o bloco do cronograma
	parsedSpaceID, _ := uuid.Parse(spaceID)
	newPlan := models.StudyPlan{
		SpaceID:    parsedSpaceID,
		DayOfWeek:  input.DayOfWeek,
		StartTime:  input.StartTime,
		EndTime:    input.EndTime,
		NotebookID: input.NotebookID,
		Activity:   input.Activity,
	}

	// 4. Salva no banco de dados
	if err := database.DB.Create(&newPlan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar horário no cronograma"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Horário adicionado ao cronograma com sucesso!",
		"plan":    newPlan,
	})
}

// ListPlans - Lista os planos de estudo (agenda semanal) do Space
func ListPlans(c *gin.Context) {
	spaceID := c.Param("space_id")

	// 1. Pega o ID do usuário logado
	userIDContext, _ := c.Get("userID")
	userIDStr := fmt.Sprintf("%v", userIDContext)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "ID de usuário inválido"})
		return
	}

	// 2. CHECAGEM DE SEGURANÇA (O Leão de Chácara)
	var space models.Space
	var permission models.SpacePermission

	isOwner := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error == nil
	isGuest := database.DB.Where("space_id = ? AND user_id = ?", spaceID, userID).First(&permission).Error == nil

	// Se não for dono e não for convidado, BLOQUEIA!
	if !isOwner && !isGuest {
		c.JSON(403, gin.H{"error": "Acesso Negado: Você não tem permissão para ver a Agenda deste Space."})
		return
	}

	// 3. BUSCA OS PLANOS (Agenda)
	var plans []models.StudyPlan
	if err := database.DB.Where("space_id = ?", spaceID).Find(&plans).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao carregar a agenda", "detalhe": err.Error()})
		return
	}

	// Devolve os planos para o Front-end
	c.JSON(200, gin.H{"plans": plans})
}

type UpdateStudyPlanInput struct {
	DayOfWeek  int       `json:"day_of_week"`
	StartTime  string    `json:"start_time"`
	EndTime    string    `json:"end_time"`
	Activity   string    `json:"activity"`
	NotebookID uuid.UUID `json:"notebook_id"`
}

func UpdateStudyPlan(c *gin.Context) {
	planID := c.Param("plan_id")
	var input UpdateStudyPlanInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Dados inválidos"})
		return
	}

	if err := database.DB.Model(&models.StudyPlan{}).Where("id = ?", planID).Updates(models.StudyPlan{
		DayOfWeek:  input.DayOfWeek,
		StartTime:  input.StartTime,
		EndTime:    input.EndTime,
		Activity:   input.Activity,
		NotebookID: input.NotebookID,
	}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao atualizar cronograma", "detalhe": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Cronograma atualizado!"})
}

func DeleteStudyPlan(c *gin.Context) {
	planID := c.Param("plan_id")
	if err := database.DB.Where("id = ?", planID).Delete(&models.StudyPlan{}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao apagar atividade", "detalhe": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Atividade removida do cronograma!"})
}

// ==========================================================
// 🤖 NOVO FLUXO: GERADOR AUTOMÁTICO DE PLANO DE ESTUDOS
// ==========================================================

// Estrutura que o Front-end vai nos enviar
type AutoPlanInput struct {
	WakeUpTime         string `json:"wake_up_time" binding:"required"`   // Ex: "07:00"
	SleepTime          string `json:"sleep_time" binding:"required"`     // Ex: "23:00"
	WorkStart          string `json:"work_start"`                        // Ex: "08:00"
	WorkEnd            string `json:"work_end"`                          // Ex: "18:00"
	LunchStart         string `json:"lunch_start"`                       // Ex: "12:00"
	LunchEnd           string `json:"lunch_end"`                         // Ex: "13:00"
	FreeTimePreference int    `json:"free_time_preference"`              // Minutos de lazer exigidos por dia (Ex: 60)
	DaysAvailable      []int  `json:"days_available" binding:"required"` // Ex: [1, 2, 3, 4, 5] (Segunda a Sexta)
}

// Funções auxiliares para converter horas em minutos e vice-versa
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

// A Rota Principal
func GenerateAutoPlan(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	spaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido."})
		return
	}

	var input AutoPlanInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: Verifique os horários enviados."})
		return
	}

	// Busca se a turma tem um Ciclo de Estudos Ativo para puxarmos os nomes das matérias
	var activeCycle models.StudyCycle
	database.DB.Preload("Items").Where("space_id = ? AND is_active = true", spaceID).First(&activeCycle)

	var generatedPlans []models.StudyPlan
	subjectIndex := 0

	// Variáveis de tempo em minutos
	wake := timeToMinutes(input.WakeUpTime)
	sleep := timeToMinutes(input.SleepTime)
	if sleep < wake { // Caso ele durma depois da meia-noite (Ex: 01:00)
		sleep += 1440
	}
	workStart := timeToMinutes(input.WorkStart)
	workEnd := timeToMinutes(input.WorkEnd)
	lunchStart := timeToMinutes(input.LunchStart)
	lunchEnd := timeToMinutes(input.LunchEnd)

	// Gera o plano para cada dia selecionado
	for _, day := range input.DaysAvailable {
		// Cria uma linha do tempo de 24h (1440 minutos)
		// true = Ocupado, false = Livre
		timeline := make([]bool, 1440*2) // *2 para lidar com dias que viram a madrugada

		// 1. Bloqueia o tempo de sono
		for i := 0; i < wake; i++ {
			timeline[i] = true
		}
		for i := sleep; i < len(timeline); i++ {
			timeline[i] = true
		}

		// 2. Bloqueia o trabalho/escola
		if workStart > 0 && workEnd > workStart {
			for i := workStart; i < workEnd; i++ {
				timeline[i] = true
			}
		}

		// 3. Bloqueia o almoço
		if lunchStart > 0 && lunchEnd > lunchStart {
			for i := lunchStart; i < lunchEnd; i++ {
				timeline[i] = true
			}
		}

		// 4. Extrai os blocos livres (Mágica do Algoritmo)
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
		if startFree != -1 { // Se o dia terminar livre
			freeBlocks = append(freeBlocks, []int{startFree, sleep})
		}

		// 5. Deduz o tempo livre (lazer) do último bloco do dia
		lazerRestante := input.FreeTimePreference
		if lazerRestante > 0 && len(freeBlocks) > 0 {
			lastBlockIndex := len(freeBlocks) - 1
			blockDuration := freeBlocks[lastBlockIndex][1] - freeBlocks[lastBlockIndex][0]

			if blockDuration > lazerRestante {
				freeBlocks[lastBlockIndex][1] -= lazerRestante
			} else {
				// Se o lazer pedido for maior que o ultimo bloco inteiro, cancela o bloco
				freeBlocks = freeBlocks[:lastBlockIndex]
			}
		}

		// 6. Fatiar os blocos grandes em Sessões de Estudo (Ex: 50 minutos estudo + 10 min pausa)
		sessionLength := 50
		breakLength := 10

		for _, block := range freeBlocks {
			currentTime := block[0]
			endTime := block[1]

			for currentTime+sessionLength <= endTime {
				// Define qual matéria estudar puxando da roleta do ciclo ativo
				activityName := "Sessão de Estudo"
				var notebookID *uuid.UUID = nil

				if len(activeCycle.Items) > 0 {
					item := activeCycle.Items[subjectIndex%len(activeCycle.Items)]
					if item.Name != "" {
						activityName = "Estudar: " + item.Name
					}
					notebookID = item.NotebookID
					subjectIndex++
				}

				// Salva no banco de dados
				newPlan := models.StudyPlan{
					SpaceID:   spaceID,
					DayOfWeek: day,
					StartTime: minutesToTime(currentTime),
					EndTime:   minutesToTime(currentTime + sessionLength),
					Activity:  activityName,
				}
				if notebookID != nil {
					newPlan.NotebookID = *notebookID
				}

				database.DB.Create(&newPlan)
				generatedPlans = append(generatedPlans, newPlan)

				// Avança o tempo (Sessão + Pausa)
				currentTime += sessionLength + breakLength
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Plano de estudos gerado automaticamente com sucesso!",
		"plans_created": len(generatedPlans),
		"agenda":        generatedPlans,
	})
}

// 1. Nova estrutura para receber uma lista do Frontend
type CreateMultipleStudyPlansInput struct {
	Plans []CreateStudyPlanInput `json:"plans" binding:"required"`
}

// 2. Nova Função para criar vários de uma vez
func CreateMultipleStudyPlans(c *gin.Context) {
	userID, _ := c.Get("userID")
	spaceID := c.Param("space_id")

	// Valida se o usuário é dono do Space
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Acesso negado a este Space"})
		return
	}

	var input CreateMultipleStudyPlansInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: envie uma lista de planos"})
		return
	}

	parsedSpaceID, _ := uuid.Parse(spaceID)
	var createdPlans []models.StudyPlan

	// Inicia uma Transação no banco (ou tudo salva ou nada salva, mais seguro)
	tx := database.DB.Begin()

	for _, item := range input.Plans {
		newPlan := models.StudyPlan{
			SpaceID:    parsedSpaceID,
			DayOfWeek:  item.DayOfWeek,
			StartTime:  item.StartTime,
			EndTime:    item.EndTime,
			NotebookID: item.NotebookID,
			Activity:   item.Activity,
		}

		if err := tx.Create(&newPlan).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar um dos horários"})
			return
		}
		createdPlans = append(createdPlans, newPlan)
	}

	tx.Commit()

	c.JSON(http.StatusCreated, gin.H{
		"message": fmt.Sprintf("%d horários adicionados com sucesso!", len(createdPlans)),
		"plans":   createdPlans,
	})
}
