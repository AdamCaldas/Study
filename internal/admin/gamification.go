package admin

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
)

// ==========================================================
// 1️⃣ Alterar XP do Usuário Manualmente (Dar ou Tirar XP)
// ==========================================================
func UpdateUserXP(c *gin.Context) {
	userID := c.Param("id")

	var input struct {
		Operation string `json:"operation" binding:"required"` // "add", "subtract" ou "set"
		Amount    int    `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos. Envie 'operation' e 'amount'."})
		return
	}

	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Usuário não encontrado."})
		return
	}

	// Calcula o novo XP baseado na operação
	switch input.Operation {
	case "add":
		user.XP += input.Amount
	case "subtract":
		user.XP -= input.Amount
		if user.XP < 0 {
			user.XP = 0 // Nunca deixa ficar negativo
		}
	case "set":
		user.XP = input.Amount
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Operação inválida. Use 'add', 'subtract' ou 'set'."})
		return
	}

	// Salva no banco
	if err := database.DB.Model(&user).Update("xp", user.XP).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar XP do usuário."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "XP do usuário atualizado com sucesso!",
		"new_xp":  user.XP,
	})
}

// ==========================================================
// 2️⃣ Listar todas as Regras de Gamificação (Tabela de Preços)
// ==========================================================
func ListGamificationRules(c *gin.Context) {
	var rules []models.GamificationRule
	if err := database.DB.Order("reward_xp desc").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar regras."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

// ==========================================================
// 3️⃣ Criar Nova Regra de Gamificação (Ex: Promoção de Natal)
// ==========================================================
func CreateGamificationRule(c *gin.Context) {
	var rule models.GamificationRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	if err := database.DB.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar regra. Verifique se o 'action_name' já não existe."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Nova regra de XP criada!", "rule": rule})
}

// ==========================================================
// 4️⃣ Atualizar Regra Existente (Nerfar ou Buffar o XP)
// ==========================================================
func UpdateGamificationRule(c *gin.Context) {
	ruleID := c.Param("rule_id")

	var input struct {
		RewardXP    *int   `json:"reward_xp"`
		DailyLimit  *int   `json:"daily_limit"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	updates := map[string]interface{}{}
	if input.RewardXP != nil {
		updates["reward_xp"] = *input.RewardXP
	}
	if input.DailyLimit != nil {
		updates["daily_limit"] = *input.DailyLimit
	}
	if input.Description != "" {
		updates["description"] = input.Description
	}

	if err := database.DB.Model(&models.GamificationRule{}).Where("id = ?", ruleID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar a regra."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Regra de XP atualizada com sucesso! O balanceamento foi alterado."})
}
