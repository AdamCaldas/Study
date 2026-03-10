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
