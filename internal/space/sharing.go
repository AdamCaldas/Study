package space

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ShareSpaceInput struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role" binding:"required"` // O frontend ainda envia "role"
}

func ShareSpace(c *gin.Context) {
	userID, _ := c.Get("userID")
	spaceIDStr := c.Param("space_id")
	spaceID, _ := uuid.Parse(spaceIDStr)

	var input ShareSpaceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: e-mail e cargo são obrigatórios"})
		return
	}

	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas o dono pode compartilhar este Space"})
		return
	}

	var targetUser models.User
	if err := database.DB.Where("email = ?", input.Email).First(&targetUser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuário convidado não encontrado no sistema"})
		return
	}

	if targetUser.ID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Você já é o dono deste Space"})
		return
	}

	// 👇 CORREÇÃO 1: Usando AccessLevel
	newPermission := models.SpacePermission{
		SpaceID:     spaceID,
		UserID:      targetUser.ID,
		AccessLevel: input.Role,
	}

	if err := database.DB.Create(&newPermission).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Este usuário já possui acesso a este Space ou houve um erro no banco"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Space compartilhado com sucesso!",
		"friend":  targetUser.FullName,
		"role":    input.Role,
	})
}
