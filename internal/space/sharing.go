package space

import (
	"net/http"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==========================================================
// 1️⃣ Amigo solicita acesso pelo código
// ==========================================================
func RequestSpaceAccess(c *gin.Context) {
	userIDInterface, _ := c.Get("userID") // Pega o ID do amigo que quer entrar

	var parsedUserID uuid.UUID
	// 🛡️ Verifica de forma segura o tipo do dado para evitar "Panic" do Go
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		parsedUserID = v // Já vem pronto do seu AuthMiddleware!
	case string:
		parsedUserID, _ = uuid.Parse(v) // Plano B caso venha como texto
	}

	var input struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Código não informado"})
		return
	}

	// 1. Busca o Space pelo código
	var space models.Space
	if err := database.DB.Where("share_code = ?", input.Code).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Código de Space inválido"})
		return
	}

	// 2. Verifica se o dono não está tentando pedir acesso pro próprio space
	if space.OwnerID == parsedUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Você já é o dono deste Space!"})
		return
	}

	// 3. Verifica se o usuário já é um colaborador aprovado
	var existingPermission models.SpacePermission
	if err := database.DB.Where("space_id = ? AND user_id = ?", space.ID, parsedUserID).First(&existingPermission).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Você já faz parte deste Space!"})
		return
	}

	// 4. Verifica se já existe uma solicitação pendente na Sala de Espera
	var existingRequest models.SpaceJoinRequest
	if err := database.DB.Where("space_id = ? AND user_id = ? AND status = 'pending'", space.ID, parsedUserID).First(&existingRequest).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Você já enviou uma solicitação. Aguarde o dono aprovar!"})
		return
	}

	// 5. Cria a solicitação pendente
	newRequest := models.SpaceJoinRequest{
		SpaceID: space.ID,
		UserID:  parsedUserID,
		Status:  "pending",
	}
	if err := database.DB.Create(&newRequest).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao enviar solicitação"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Solicitação de acesso enviada com sucesso para o dono do Space!"})
}

// ==========================================================
// 2️⃣ Dono lista as solicitações pendentes (Sala de Espera)
// ==========================================================
func ListSpaceRequests(c *gin.Context) {
	spaceID := c.Param("space_id")

	// Traz as solicitações e faz JOIN com users para o dono ver o nome de quem pediu
	var requests []struct {
		RequestID string `json:"request_id"`
		UserID    string `json:"user_id"`
		FullName  string `json:"full_name"`
		Status    string `json:"status"`
	}

	err := database.DB.Table("space_join_requests").
		Select("space_join_requests.id as request_id, users.id as user_id, users.full_name, space_join_requests.status").
		Joins("left join users on users.id = space_join_requests.user_id").
		Where("space_join_requests.space_id = ? AND space_join_requests.status = 'pending'", spaceID).
		Scan(&requests).Error

	if err != nil || len(requests) == 0 {
		// Retorna array vazio em vez de null para o Front-end não quebrar
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	c.JSON(http.StatusOK, requests)
}

// ==========================================================
// 3️⃣ Dono aceita ou rejeita a solicitação
// ==========================================================
func RespondSpaceRequest(c *gin.Context) {
	spaceID := c.Param("space_id")
	requestID := c.Param("request_id")

	var input struct {
		Action      string `json:"action" binding:"required"` // "accept" ou "reject"
		AccessLevel string `json:"access_level"`              // "VIEWER" ou "EDITOR" (Seu padrão!)
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ação inválida ou formato incorreto"})
		return
	}

	// Busca a solicitação no banco
	var joinRequest models.SpaceJoinRequest
	if err := database.DB.Where("id = ? AND space_id = ?", requestID, spaceID).First(&joinRequest).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Solicitação não encontrada"})
		return
	}

	// Inicia transação segura
	tx := database.DB.Begin()

	// 🟢 SE O DONO ACEITAR
	if input.Action == "accept" {
		// Atualiza o status do pedido para aprovado
		if err := tx.Model(&joinRequest).Update("status", "approved").Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao aprovar solicitação"})
			return
		}

		// Valida o AccessLevel (Se não mandar nada ou mandar errado, vira VIEWER por segurança)
		if input.AccessLevel != "EDITOR" {
			input.AccessLevel = "VIEWER"
		}

		// Cria a permissão oficial na tabela SpacePermission
		permission := models.SpacePermission{
			SpaceID:     joinRequest.SpaceID,
			UserID:      joinRequest.UserID,
			AccessLevel: input.AccessLevel,
		}

		if err := tx.Create(&permission).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao adicionar usuário ao Space"})
			return
		}

		tx.Commit()
		c.JSON(http.StatusOK, gin.H{"message": "Solicitação aceita! Usuário agora é " + input.AccessLevel})
		return
	}

	// 🔴 SE O DONO REJEITAR
	if input.Action == "reject" {
		if err := tx.Model(&joinRequest).Update("status", "rejected").Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao rejeitar solicitação"})
			return
		}
		tx.Commit()
		c.JSON(http.StatusOK, gin.H{"message": "Solicitação rejeitada com sucesso."})
		return
	}

	// Se mandar uma action bizarra que não é accept nem reject
	c.JSON(http.StatusBadRequest, gin.H{"error": "A 'action' deve ser obrigatoriamente 'accept' ou 'reject'"})
}

// ==========================================================
// 4️⃣ Dono gera/pega o código de compartilhamento
// ==========================================================
func ShareSpace(c *gin.Context) {
	spaceID := c.Param("space_id")

	// Busca o Space no banco de dados
	var space models.Space
	if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado"})
		return
	}

	// Devolve o código de compartilhamento para o Front-end mostrar na tela
	c.JSON(http.StatusOK, gin.H{
		"message":    "Código de compartilhamento recuperado!",
		"share_code": space.ShareCode,
	})
}
