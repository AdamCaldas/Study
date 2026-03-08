package study

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Estruturas para receber os dados do Frontend
type CreateCycleItemInput struct {
	NotebookID  uuid.UUID `json:"notebook_id" binding:"required"`
	Sequence    int       `json:"sequence" binding:"required"`
	DurationMin int       `json:"duration_min" binding:"required"`
}

type CreateStudyCycleInput struct {
	Items []CreateCycleItemInput `json:"items" binding:"required"`
}

func CreateStudyCycle(c *gin.Context) {
	userID, _ := c.Get("userID")
	spaceID := c.Param("space_id")

	// 1. Valida se o utilizador tem acesso ao Space
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Acesso negado a este Space"})
		return
	}

	// 2. Valida o JSON enviado
	var input CreateStudyCycleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	parsedSpaceID, _ := uuid.Parse(spaceID)

	// 3. Cria o Ciclo Principal (Inicia no passo 0)
	newCycle := models.StudyCycle{
		SpaceID:     parsedSpaceID,
		CurrentStep: 0,
	}

	// O GORM permite iniciar uma transação. Se algo falhar, ele reverte tudo.
	tx := database.DB.Begin()

	if err := tx.Create(&newCycle).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar o ciclo de estudos"})
		return
	}

	// 4. Cria os itens (disciplinas) dentro do ciclo
	for _, itemInput := range input.Items {
		newItem := models.StudyCycleItem{
			CycleID:     newCycle.ID,
			NotebookID:  itemInput.NotebookID,
			Sequence:    itemInput.Sequence,
			DurationMin: itemInput.DurationMin,
		}
		if err := tx.Create(&newItem).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao adicionar itens ao ciclo"})
			return
		}
	}

	// Confirma a transação
	tx.Commit()

	c.JSON(http.StatusCreated, gin.H{
		"message": "Ciclo de estudos criado com sucesso!",
		"cycle":   newCycle,
	})
}

// Lista os ciclos de um Space, trazendo também os itens associados
func ListStudyCycles(c *gin.Context) {
	userID, _ := c.Get("userID")
	spaceID := c.Param("space_id")

	// Segurança
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Acesso negado"})
		return
	}

	var cycles []models.StudyCycle
	// Preload("Items") diz ao GORM para fazer um JOIN e trazer os itens automaticamente!
	if err := database.DB.Preload("Items").Where("space_id = ?", spaceID).Find(&cycles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar ciclos"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cycles": cycles})
}

// AdvanceCycleStep move o ponteiro do ciclo para a próxima matéria usando matemática modular
func AdvanceCycleStep(c *gin.Context) {
	userID, _ := c.Get("userID")
	spaceID := c.Param("space_id")
	cycleID := c.Param("cycle_id") // Precisamos saber qual ciclo avançar

	// 1. Segurança: valida se o usuário tem acesso ao Space
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Acesso negado"})
		return
	}

	// 2. Busca o ciclo no banco e JÁ CARREGA as matérias dele (Preload)
	var cycle models.StudyCycle
	if err := database.DB.Preload("Items").Where("id = ? AND space_id = ?", cycleID, spaceID).First(&cycle).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ciclo não encontrado"})
		return
	}

	totalItems := len(cycle.Items)
	if totalItems == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Este ciclo não tem nenhuma matéria para estudar."})
		return
	}

	// 3. A Mágica do Operador Módulo (%)
	// Se totalItems = 3 (0, 1 e 2). E o CurrentStep atual é 2.
	// (2 + 1) = 3. E 3 % 3 = 0. Ele volta pro começo perfeitamente!
	cycle.CurrentStep = (cycle.CurrentStep + 1) % totalItems

	// 4. Salva o novo passo no banco de dados
	if err := database.DB.Save(&cycle).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar o ciclo"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Ciclo avançado com sucesso!",
		"current_step": cycle.CurrentStep,
	})
}

func DeleteStudyCycle(c *gin.Context) {
	cycleID := c.Param("cycle_id")
	// Como colocamos "Cascade Delete" no banco, ao apagar o ciclo, as matérias dele somem juntas automaticamente!
	if err := database.DB.Where("id = ?", cycleID).Delete(&models.StudyCycle{}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao apagar ciclo", "detalhe": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Ciclo apagado com sucesso!"})
}
