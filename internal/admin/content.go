package admin

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
)

// ListAllSpaces - Visão de Raio-X: Vê TODOS os Spaces do banco de dados
func ListAllSpaces(c *gin.Context) {
	var spaces []models.Space

	// Busca todos os spaces ordenados pelos criados mais recentemente
	if err := database.DB.Order("created_at desc").Find(&spaces).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar Spaces", "detalhe": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":  len(spaces),
		"spaces": spaces,
	})
}

// Estrutura para receber o ID do novo dono
type TransferSpaceInput struct {
	NewOwnerID string `json:"new_owner_id" binding:"required"`
}

// TransferSpaceOwnership - Passa a coroa: Transfere o Space de um usuário para outro
func TransferSpaceOwnership(c *gin.Context) {
	spaceID := c.Param("id")
	var input TransferSpaceInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do novo dono é obrigatório"})
		return
	}

	// Atualiza o OwnerID do Space ignorando qualquer checagem de permissão normal
	if err := database.DB.Model(&models.Space{}).Where("id = ?", spaceID).Update("owner_id", input.NewOwnerID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao transferir a posse do Space", "detalhe": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "A posse do Space foi transferida com sucesso!"})
}

// RemoveUserFromSpace - Expulsão Sumária: Remove um convidado do Space à força
func RemoveUserFromSpace(c *gin.Context) {
	spaceID := c.Param("id")
	targetUserID := c.Param("user_id")

	// Deleta a permissão da tabela SpacePermission
	if err := database.DB.Where("space_id = ? AND user_id = ?", spaceID, targetUserID).Delete(&models.SpacePermission{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao expulsar o usuário", "detalhe": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Usuário removido do Space com sucesso."})
}

// DeleteAnySpace - Exclusão Absoluta: Apaga qualquer Space do sistema
func DeleteAnySpace(c *gin.Context) {
	spaceID := c.Param("id")

	// Como sua tabela tem OnDelete:CASCADE, apagar o Space apaga os Cadernos dele junto!
	if err := database.DB.Where("id = ?", spaceID).Delete(&models.Space{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao deletar o Space", "detalhe": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Space aniquilado com sucesso pelo Modo Deus!"})
}
