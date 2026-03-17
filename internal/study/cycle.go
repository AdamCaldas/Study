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
// ESTRUTURAS DE ENTRADA (O JSON que o Front-end vai mandar)
// ==========================================================
type CycleDisciplineInput struct {
	NotebookID       *string `json:"notebook_id"`
	Name             string  `json:"name"`
	Importance       int     `json:"importance"`
	Performance      int     `json:"performance"`
	Order            int     `json:"order"`
	SuggestedMinutes int     `json:"suggested_minutes"` // Usado no modo manual
}

type CycleConfigInput struct {
	HoursPerDay       float64  `json:"hours_per_day"`
	AvailableDays     []string `json:"available_days"`
	MinSessionMinutes int      `json:"min_session_minutes"`
	MaxSessionMinutes int      `json:"max_session_minutes"`
}

type CreateCycleRequest struct {
	Name           string                 `json:"name" binding:"required"`
	Description    string                 `json:"description"`
	TargetGoal     string                 `json:"target_goal"`
	TargetDate     *time.Time             `json:"target_date"`
	CycleType      string                 `json:"cycle_type"`
	Visibility     string                 `json:"visibility"`
	Disciplines    []CycleDisciplineInput `json:"disciplines" binding:"required"`
	ScheduleConfig CycleConfigInput       `json:"schedule_config" binding:"required"`
}

// ==========================================================
// ⚙️ ALGORITMO CORE: Distribuição de Tempo
// ==========================================================
func calculateCycleDistribution(disciplines []CycleDisciplineInput, totalDailyHours float64, minSess int, maxSess int) []models.StudyCycleItem {
	var totalWeight float64 = 0
	var calculatedItems []models.StudyCycleItem

	// 1. Calcula os pesos: W = I + (6 - P)
	weights := make([]float64, len(disciplines))
	for i, disc := range disciplines {
		w := float64(disc.Importance + (6 - disc.Performance))
		weights[i] = w
		totalWeight += w
	}

	totalDailyMinutes := totalDailyHours * 60

	// 2. Distribui os minutos baseados na proporção
	for i, disc := range disciplines {
		proportion := weights[i] / totalWeight
		suggestedMin := int(math.Round(proportion * totalDailyMinutes))

		// 3. Trava de Segurança (Min e Max)
		if suggestedMin < minSess {
			suggestedMin = minSess
		}

		// Se o tempo sugerido for maior que a sessão máxima, dividimos em blocos!
		blocksNeeded := 1
		if suggestedMin > maxSess {
			blocksNeeded = int(math.Ceil(float64(suggestedMin) / float64(maxSess)))
			suggestedMin = suggestedMin / blocksNeeded // Divide o tempo pelo número de blocos
		}

		// Adiciona as matérias (se precisar dividir, cria blocos duplicados na roleta)
		for b := 0; b < blocksNeeded; b++ {
			var nbID *uuid.UUID
			if disc.NotebookID != nil && *disc.NotebookID != "" {
				parsed, _ := uuid.Parse(*disc.NotebookID)
				nbID = &parsed
			}

			calculatedItems = append(calculatedItems, models.StudyCycleItem{
				NotebookID:       nbID,
				Name:             disc.Name, // Usado para criar o caderno depois, se nbID for nulo
				Importance:       disc.Importance,
				Performance:      disc.Performance,
				Sequence:         disc.Order + b, // Ajusta a ordem se houver quebra de bloco
				SuggestedMinutes: suggestedMin,
			})
		}
	}

	return calculatedItems
}

// ==========================================================
// 🧪 ROTA 1: SIMULAR CICLO (O Preview para o usuário)
// ==========================================================
func SimulateStudyCycle(c *gin.Context) {
	var req CreateCycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	// Executa a Matemática
	items := calculateCycleDistribution(req.Disciplines, req.ScheduleConfig.HoursPerDay, req.ScheduleConfig.MinSessionMinutes, req.ScheduleConfig.MaxSessionMinutes)

	// Calcula os Totais de Estudo
	daysCount := len(req.ScheduleConfig.AvailableDays)
	weeklyHours := req.ScheduleConfig.HoursPerDay * float64(daysCount)
	monthlyHours := weeklyHours * 4.3 // Média de semanas num mês

	// Formatação amigável para o Front
	totalDailyStr := formatHours(req.ScheduleConfig.HoursPerDay)

	c.JSON(http.StatusOK, gin.H{
		"totals": gin.H{
			"daily":   totalDailyStr,
			"weekly":  formatHours(weeklyHours),
			"monthly": formatHours(monthlyHours),
		},
		"distribution": items,
	})
}

// Função auxiliar consertada! Formata float para string bonita (ex: "4h 30min")
func formatHours(h float64) string {
	hours := int(math.Floor(h))
	minutes := int(math.Round((h - float64(hours)) * 60))
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dmin", hours, minutes)
}

// ==========================================================
// 💾 ROTA 2: CRIAR O CICLO REAL (Salva no Banco + Auto-Cria Cadernos)
// ==========================================================
func CreateStudyCycle(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	spaceID, _ := uuid.Parse(spaceIDStr)

	// Pega o ID do criador de forma segura
	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	var req CreateCycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	// 1. Validação de Metas
	if req.TargetGoal != "" && req.TargetDate == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Se uma meta foi definida, a data alvo (target_date) é obrigatória."})
		return
	}
	if req.TargetDate != nil && req.TargetDate.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A data alvo não pode estar no passado."})
		return
	}

	// 2. Calcula os tempos (Automático ou Manual)
	var itemsToSave []models.StudyCycleItem

	if req.CycleType == "automatic" || req.CycleType == "" {
		itemsToSave = calculateCycleDistribution(req.Disciplines, req.ScheduleConfig.HoursPerDay, req.ScheduleConfig.MinSessionMinutes, req.ScheduleConfig.MaxSessionMinutes)
	} else {
		// Modo Manual: Aceita cegamente os tempos que o front mandou!
		for _, disc := range req.Disciplines {
			var nbID *uuid.UUID
			if disc.NotebookID != nil && *disc.NotebookID != "" {
				parsed, _ := uuid.Parse(*disc.NotebookID)
				nbID = &parsed
			}
			itemsToSave = append(itemsToSave, models.StudyCycleItem{
				NotebookID:       nbID,
				Name:             disc.Name,
				Importance:       disc.Importance,
				Performance:      disc.Performance,
				Sequence:         disc.Order,
				SuggestedMinutes: disc.SuggestedMinutes,
			})
		}
	}

	// Inicia Transação Segura
	tx := database.DB.Begin()

	// 3. Lookup & Create de Cadernos
	for i, item := range itemsToSave {
		if item.NotebookID == nil {
			// Cria um novo automaticamente!
			newNb := models.Notebook{
				SpaceID:     spaceID,
				Name:        item.Name,
				ColorHex:    "#8B5CF6", // Roxo Padrão
				CreatedByID: userID,
				UpdatedByID: userID,
			}
			if err := tx.Create(&newNb).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar caderno automático: " + item.Name})
				return
			}
			// Associa o ID novo à roleta
			itemsToSave[i].NotebookID = &newNb.ID
		}
	}

	// Transforma array de dias em JSON String
	availableDaysJSON, _ := json.Marshal(req.ScheduleConfig.AvailableDays)

	// 4. Salva o Cabeçalho do Ciclo
	newCycle := models.StudyCycle{
		SpaceID:       spaceID,
		Name:          req.Name,
		Description:   req.Description,
		TargetGoal:    req.TargetGoal,
		TargetDate:    req.TargetDate,
		CycleType:     req.CycleType,
		Visibility:    req.Visibility,
		HoursPerDay:   req.ScheduleConfig.HoursPerDay,
		AvailableDays: string(availableDaysJSON),
		MinSessionMin: req.ScheduleConfig.MinSessionMinutes,
		MaxSessionMin: req.ScheduleConfig.MaxSessionMinutes,
		CreatedByID:   userID,
		Items:         itemsToSave,
	}

	if err := tx.Create(&newCycle).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar o Ciclo de Estudos."})
		return
	}

	tx.Commit()

	c.JSON(http.StatusCreated, gin.H{
		"message": "Ciclo criado com sucesso!",
		"cycle":   newCycle,
	})
}

// ==========================================================
// 🔄 AVANÇAR O PASSO DA ROLETA
// ==========================================================
func AdvanceCycleStep(c *gin.Context) {
	cycleID := c.Param("cycle_id")
	var cycle models.StudyCycle

	// Busca o ciclo e já traz as matérias para sabermos o tamanho da roleta
	if err := database.DB.Preload("Items").Where("id = ?", cycleID).First(&cycle).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ciclo não encontrado"})
		return
	}

	totalItems := len(cycle.Items)
	if totalItems > 0 {
		// Avança um passo. O módulo (%) faz a roleta voltar pro 0 quando chega no fim!
		cycle.CurrentStep = (cycle.CurrentStep + 1) % totalItems
		database.DB.Save(&cycle)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Roleta girou para a próxima matéria!",
		"current_step": cycle.CurrentStep,
	})
}

// ==========================================================
// ⭐ ATIVAR CICLO (Favoritar a Roleta Atual)
// ==========================================================
func ActivateCycle(c *gin.Context) {
	cycleID := c.Param("cycle_id")
	spaceID := c.Param("space_id")

	tx := database.DB.Begin()

	// 1. Desativa todos os ciclos deste Space (Só pode ter um ativo por vez)
	tx.Model(&models.StudyCycle{}).Where("space_id = ?", spaceID).Update("is_active", false)

	// 2. Ativa apenas o ciclo que o usuário escolheu
	if err := tx.Model(&models.StudyCycle{}).Where("id = ?", cycleID).Update("is_active", true).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao ativar o ciclo"})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "Ciclo ativado! Agora ele é a roleta principal do Space."})
}

// ==========================================================
// 🗑️ DELETAR CICLO
// ==========================================================
func DeleteStudyCycle(c *gin.Context) {
	cycleID := c.Param("cycle_id")

	// O GORM vai apagar em cascata os items desse ciclo também
	if err := database.DB.Where("id = ?", cycleID).Delete(&models.StudyCycle{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao deletar o ciclo"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ciclo deletado com sucesso!"})
}

// ==========================================================
// 📋 LISTAR TODOS OS CICLOS DO SPACE (V2 OTIMIZADA)
// ==========================================================
func ListStudyCycles(c *gin.Context) {
	spaceID := c.Param("space_id")
	var cycles []models.StudyCycle

	// Busca os ciclos e os itens
	if err := database.DB.Preload("Items").Where("space_id = ?", spaceID).Order("created_at desc").Find(&cycles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar ciclos."})
		return
	}

	// Criar um DTO para formatar a saída perfeita pro Front-end
	var response []map[string]interface{}

	for _, cycle := range cycles {
		// 1. Converter AvailableDays (String) de volta para Array Real
		var daysArray []string
		if cycle.AvailableDays != "" {
			json.Unmarshal([]byte(cycle.AvailableDays), &daysArray)
		}

		// 2. Preencher os nomes das matérias
		var formattedItems []map[string]interface{}
		for _, item := range cycle.Items {
			var notebookName string
			if item.NotebookID != nil {
				database.DB.Table("notebooks").Select("name").Where("id = ?", item.NotebookID).Scan(&notebookName)
			}

			formattedItems = append(formattedItems, map[string]interface{}{
				"id":                item.ID,
				"notebook_id":       item.NotebookID,
				"name":              notebookName, // 👈 Devolve o nome da matéria pro Front!
				"sequence":          item.Sequence,
				"importance":        item.Importance,
				"performance":       item.Performance,
				"suggested_minutes": item.SuggestedMinutes,
			})
		}

		// 3. Monta o objeto final do ciclo
		response = append(response, map[string]interface{}{
			"id":                  cycle.ID,
			"name":                cycle.Name,
			"description":         cycle.Description,
			"target_goal":         cycle.TargetGoal,
			"target_date":         cycle.TargetDate,
			"cycle_type":          cycle.CycleType,
			"visibility":          cycle.Visibility,
			"hours_per_day":       cycle.HoursPerDay,
			"available_days":      daysArray, // 👈 Array real para os botões do Mayan
			"min_session_minutes": cycle.MinSessionMin,
			"max_session_minutes": cycle.MaxSessionMin,
			"is_active":           cycle.IsActive,
			"items":               formattedItems,
		})
	}

	// Evita retornar null se não houver ciclos
	if response == nil {
		response = []map[string]interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{"cycles": response})
}

// ==========================================================
// ✏️ EDITAR CICLO (A Marcha Ré)
// ==========================================================
func UpdateStudyCycle(c *gin.Context) {
	cycleID := c.Param("cycle_id")
	spaceIDStr := c.Param("space_id")
	spaceID, _ := uuid.Parse(spaceIDStr)

	// Pega o ID do usuário para caso precise criar cadernos novos
	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	var req CreateCycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	var cycle models.StudyCycle
	if err := database.DB.Where("id = ? AND space_id = ?", cycleID, spaceID).First(&cycle).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ciclo não encontrado."})
		return
	}

	var itemsToSave []models.StudyCycleItem

	// MÁGICA DO MODO AUTOMÁTICO VS MANUAL
	if req.CycleType == "automatic" {
		itemsToSave = calculateCycleDistribution(req.Disciplines, req.ScheduleConfig.HoursPerDay, req.ScheduleConfig.MinSessionMinutes, req.ScheduleConfig.MaxSessionMinutes)
	} else {
		// MODO MANUAL: Aceita cegamente os minutos que o Front-end enviou!
		for _, disc := range req.Disciplines {
			var nbID *uuid.UUID
			if disc.NotebookID != nil && *disc.NotebookID != "" {
				parsed, _ := uuid.Parse(*disc.NotebookID)
				nbID = &parsed
			}
			itemsToSave = append(itemsToSave, models.StudyCycleItem{
				NotebookID:       nbID,
				Name:             disc.Name,
				Importance:       disc.Importance,
				Performance:      disc.Performance,
				Sequence:         disc.Order,
				SuggestedMinutes: disc.SuggestedMinutes, // O tempo que o Mayan definiu!
			})
		}
	}

	// Inicia Transação Segura
	tx := database.DB.Begin()

	// Cria cadernos automáticos se não existirem
	for i, item := range itemsToSave {
		if item.NotebookID == nil {
			newNb := models.Notebook{
				SpaceID: spaceID, Name: item.Name, ColorHex: "#8B5CF6", CreatedByID: userID, UpdatedByID: userID,
			}
			if err := tx.Create(&newNb).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar caderno: " + item.Name})
				return
			}
			itemsToSave[i].NotebookID = &newNb.ID
		}
	}

	// APAGA os itens velhos e SALVA os novos (O jeito mais limpo de atualizar listas)
	tx.Where("cycle_id = ?", cycle.ID).Delete(&models.StudyCycleItem{})

	availableDaysJSON, _ := json.Marshal(req.ScheduleConfig.AvailableDays)

	// Atualiza os dados do Ciclo principal
	cycle.Name = req.Name
	cycle.Description = req.Description
	cycle.TargetGoal = req.TargetGoal
	cycle.TargetDate = req.TargetDate
	cycle.CycleType = req.CycleType
	cycle.Visibility = req.Visibility
	cycle.HoursPerDay = req.ScheduleConfig.HoursPerDay
	cycle.AvailableDays = string(availableDaysJSON)
	cycle.MinSessionMin = req.ScheduleConfig.MinSessionMinutes
	cycle.MaxSessionMin = req.ScheduleConfig.MaxSessionMinutes
	cycle.Items = itemsToSave // O GORM insere os novos itens magicamente!

	if err := tx.Save(&cycle).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar o ciclo."})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{"message": "Ciclo atualizado com sucesso!", "cycle": cycle})
}
