package notebook

import (
	"net/http"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils" // 👈 Import adicionado

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// A função auxiliar local getUserID foi REMOVIDA! Agora usamos utils.GetUserID

// ==========================================================
// 🛡️ O LEÃO DE CHÁCARA GRANULAR
// Verifica se o usuário pode mexer neste caderno específico
// ==========================================================
func canEditNotebook(spaceID uuid.UUID, notebookID uuid.UUID, userID uuid.UUID) bool {
	// 1. É o dono do Space?
	var space models.Space
	if err := database.DB.Select("id").Where("id = ? AND owner_id = ?", spaceID, userID).First(&space).Error; err == nil {
		return true // Pode tudo
	}

	// 2. Tem permissão direta no Caderno?
	var nbPerm models.NotebookPermission
	if err := database.DB.Select("access_level").Where("notebook_id = ? AND user_id = ?", notebookID, userID).First(&nbPerm).Error; err == nil {
		return nbPerm.AccessLevel == utils.AccessEditor // 👈 Usando a Constante!
	}

	// 3. Tem permissão geral de editor no Space?
	var spPerm models.SpacePermission
	if err := database.DB.Select("can_create_content").Where("space_id = ? AND user_id = ?", spaceID, userID).First(&spPerm).Error; err == nil {
		return spPerm.CanCreateContent // Se ele pode criar conteúdo no space, ele pode editar cadernos
	}

	return false
}

// ==========================================================
// 1️⃣ CREATE NOTEBOOK
// ==========================================================
type CreateNotebookInput struct {
	Name       string `json:"name" binding:"required"`
	ColorHex   string `json:"color_hex"`
	Visibility string `json:"visibility"`
}

func CreateNotebook(c *gin.Context) {
	// 👇 Usando a função blindada global
	parsedUserID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Utilizador não autenticado"})
		return
	}

	spaceIDStr := c.Param("space_id")
	parsedSpaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido"})
		return
	}

	var space models.Space
	isOwner := database.DB.Where("id = ? AND owner_id = ?", parsedSpaceID, parsedUserID).First(&space).Error == nil

	// Verifica a permissão granular nova ("can_create_content")
	var permission models.SpacePermission
	canCreate := database.DB.Where("space_id = ? AND user_id = ? AND can_create_content = true", parsedSpaceID, parsedUserID).First(&permission).Error == nil

	if !isOwner && !canCreate {
		c.JSON(http.StatusForbidden, gin.H{"error": "Você não tem permissão para criar cadernos neste Space."})
		return
	}

	var input CreateNotebookInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."}) // 👈 Erro bruto ocultado (Fase 2.1)
		return
	}

	if input.ColorHex == "" {
		input.ColorHex = "#E0E0E0"
	}
	// Se não enviar visibilidade, o padrão é público
	if input.Visibility == "" {
		input.Visibility = "public"
	}

	newNotebook := models.Notebook{
		SpaceID:     parsedSpaceID,
		Name:        input.Name,
		ColorHex:    input.ColorHex,
		Visibility:  input.Visibility,
		CreatedByID: parsedUserID, // ASSINATURA
		UpdatedByID: parsedUserID, // ASSINATURA
	}

	if err := database.DB.Create(&newNotebook).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar Caderno."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Caderno criado!", "notebook": newNotebook})
}

// ==========================================================
// 2️⃣ UPDATE NOTEBOOK
// ==========================================================
type UpdateNotebookInput struct {
	Name       string `json:"name"`
	ColorHex   string `json:"color_hex"`
	Visibility string `json:"visibility"`

	// 👇 NOVOS CAMPOS PARA SALVAR O ESTADO DO ZUSTAND (FRONT-END)
	IsFullWidth       *bool `json:"is_full_width"`
	IsPageCardVisible *bool `json:"is_page_card_visible"`
	IsPaginated       *bool `json:"is_paginated"`
}

func UpdateNotebook(c *gin.Context) {
	// 👇 Usando a função blindada
	parsedUserID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Utilizador não autenticado"})
		return
	}

	notebookIDStr := c.Param("notebook_id")
	parsedNotebookID, err := uuid.Parse(notebookIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Caderno inválido"})
		return
	}

	var notebook models.Notebook
	if err := database.DB.Where("id = ?", parsedNotebookID).First(&notebook).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Caderno não encontrado"})
		return
	}

	if !canEditNotebook(notebook.SpaceID, parsedNotebookID, parsedUserID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Você não tem permissão para editar este caderno."})
		return
	}

	var input UpdateNotebookInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	// Criamos um mapa para atualizar apenas o que o front-end enviar no JSON
	updates := map[string]interface{}{
		"updated_by_id": parsedUserID, // ASSINATURA
	}

	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.ColorHex != "" {
		updates["color_hex"] = input.ColorHex
	}
	if input.Visibility != "" {
		updates["visibility"] = input.Visibility
	}

	if input.IsFullWidth != nil {
		updates["is_full_width"] = *input.IsFullWidth
	}
	if input.IsPageCardVisible != nil {
		updates["is_page_card_visible"] = *input.IsPageCardVisible
	}
	if input.IsPaginated != nil {
		updates["is_paginated"] = *input.IsPaginated
	}

	if err := database.DB.Model(&notebook).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar o caderno."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Caderno atualizado!"})
}

// ==========================================================
// 🗑️ DELETE NOTEBOOK (Com remoção automática do Ciclo!)
// ==========================================================
func DeleteNotebook(c *gin.Context) {
	notebookID := c.Param("notebook_id")

	// 👇 A MÁGICA AQUI: Se o caderno estava no Ciclo de Estudos, apagamos o bloco dele!
	database.DB.Where("notebook_id = ?", notebookID).Delete(&models.StudyBlock{})

	// O GORM Cascata vai apagar as Guias e as Páginas sozinhas!
	if err := database.DB.Where("id = ?", notebookID).Delete(&models.Notebook{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao apagar caderno."}) // 👈 Erro bruto ocultado
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Caderno apagado e removido do ciclo com sucesso!"})
}

// ListNotebooks mantido provisoriamente para não quebrar a Fase 4
func ListNotebooks(c *gin.Context) {
	spaceID := c.Param("space_id")
	var notebooks []models.Notebook
	database.DB.Where("space_id = ?", spaceID).Find(&notebooks)
	c.JSON(http.StatusOK, gin.H{"notebooks": notebooks})
}

// ==========================================================
// 📚 3. GET /spaces/:space_id/notebooks (SÓ AS CAPAS DOS CADERNOS)
// ==========================================================
func ListSpaceNotebooks(c *gin.Context) {
	spaceID := c.Param("space_id")

	var notebooks []models.Notebook

	// 👇 A MÁGICA AQUI: O banco faz um JOIN com a tabela de usuários para puxar o nome do criador e do editor!
	database.DB.
		Select("notebooks.*, criador.full_name as owner_name, editor.full_name as updater_name").
		Joins("LEFT JOIN users criador ON notebooks.created_by_id = criador.id").
		Joins("LEFT JOIN users editor ON notebooks.updated_by_id = editor.id").
		Where("notebooks.space_id = ?", spaceID).
		Find(&notebooks)

	now := time.Now()
	for i := range notebooks {
		if notebooks[i].UnlockAt != nil && notebooks[i].UnlockAt.After(now) {
			notebooks[i].IsLocked = true
		}
	}

	c.JSON(http.StatusOK, gin.H{"notebooks": notebooks})
}

// ==========================================================
// 📖 4. GET /notebooks/:notebook_id (O RECHEIO DO CADERNO)
// ==========================================================
func GetNotebookFull(c *gin.Context) {
	notebookID := c.Param("notebook_id")

	var notebook models.Notebook
	// Aqui sim a gente usa o Preload para trazer as Pastas (Guides) e as Páginas!
	if err := database.DB.
		Preload("Pages", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" asc") // Já traz ordenado bonitinho!
		}).
		Preload("Guides.Pages", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" asc")
		}).
		Preload("Guides.SubGuides.Pages"). // Se tiver sub-pastas
		Where("id = ?", notebookID).
		First(&notebook).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Caderno não encontrado"})
		return
	}

	// Trava de segurança extra (Cadeado temporal)
	if notebook.UnlockAt != nil && notebook.UnlockAt.After(time.Now()) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Este caderno ainda está trancado pelo professor."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"notebook": notebook})
}
