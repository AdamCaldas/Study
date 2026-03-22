package admin

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==========================================================
// 🐛 1. USUÁRIO REPORTA UM BUG (Visão do Aluno/Professor)
// ==========================================================
func ReportBug(c *gin.Context) {
	userIDInterface, _ := c.Get("userID")
	var reporterID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		reporterID = v
	case string:
		reporterID, _ = uuid.Parse(v)
	}

	var input struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Título e Descrição são obrigatórios."})
		return
	}

	bug := models.BugReport{
		ReporterID:  reporterID,
		Title:       input.Title,
		Description: input.Description,
		Status:      "UNREAD", // Nasce automaticamente como Não Lido
	}

	if err := database.DB.Create(&bug).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao enviar reporte. Tente novamente."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Bug reportado com sucesso! Nossa equipe técnica já foi notificada.",
		"bug":     bug,
	})
}

// ==========================================================
// 📋 2. LISTAR BUGS (Visão do Admin / Kanban)
// ==========================================================
func ListBugs(c *gin.Context) {
	var bugs []models.BugReport

	// Puxa todos os bugs e já traz os dados de quem reportou (Preload)
	// Ordena do mais novo pro mais velho
	if err := database.DB.Preload("Reporter").Order("created_at desc").Find(&bugs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar a lista de bugs."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bugs": bugs})
}

// ==========================================================
// 🔄 3. MOVER CARD NO KANBAN (Atualizar Status)
// ==========================================================
func UpdateBugStatus(c *gin.Context) {
	bugID := c.Param("id")

	var input struct {
		Status string `json:"status" binding:"required"` // Deve ser: UNREAD, ANALYSIS ou RESOLVED
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O novo status é obrigatório."})
		return
	}

	// Atualiza apenas a coluna de status
	if err := database.DB.Model(&models.BugReport{}).Where("id = ?", bugID).Update("status", input.Status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar o status do bug."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Status do ticket atualizado para: " + input.Status})
}
