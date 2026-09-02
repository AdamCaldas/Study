package space

import (
	"net/http"

	"studfy-backend/internal/activity"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CreateQuickNoteInput struct {
	Title   string `json:"title"`
	Content string `json:"content" binding:"required"`
	Color   string `json:"color"`
}

func CreateQuickNote(c *gin.Context) {
	// Puxa o ID do usuário de forma segura
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	spaceIDStr := c.Param("space_id")
	parsedSpaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido"})
		return
	}

	// A permissão já foi conferida na rota (CheckSpaceAccess +
	// RequireSpaceCreateContent): quem chega aqui é membro da turma E pode
	// criar conteúdo. Antes o handler exigia ser DONO, então o colaborador
	// autorizado pelo professor levava 403 sem motivo aparente.
	var space models.Space
	if err := database.DB.Select("id").Where("id = ?", parsedSpaceID).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado"})
		return
	}

	// 2. Valida a entrada
	var input CreateQuickNoteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O conteúdo da nota é obrigatório"})
		return
	}

	if input.Color == "" {
		input.Color = "#FFF9C4" // Cor de post-it amarelo clarinho padrão
	}

	// 3. Monta e salva a nota
	newNote := models.QuickNote{
		SpaceID: parsedSpaceID,
		Title:   input.Title,
		Content: input.Content,
		Color:   input.Color,
	}

	if err := database.DB.Create(&newNote).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar nota"})
		return
	}

	activity.Log(parsedSpaceID, userID, "criou uma nota rápida")

	c.JSON(http.StatusCreated, gin.H{"message": "Nota criada!", "note": newNote})
}

func ListSpaceNotes(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	spaceIDStr := c.Param("space_id")
	parsedSpaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID do Space inválido"})
		return
	}

	// 1. É o dono?
	var space models.Space
	isOwner := database.DB.Where("id = ? AND owner_id = ?", parsedSpaceID, userID).First(&space).Error == nil

	// 2. É convidado?
	var perm models.SpacePermission
	isGuest := database.DB.Where("space_id = ? AND user_id = ?", parsedSpaceID, userID).First(&perm).Error == nil

	// LEÃO DE CHÁCARA: SE NÃO É DONO E NÃO É CONVIDADO, BLOQUEIA!
	if !isOwner && !isGuest {
		c.JSON(http.StatusForbidden, gin.H{"error": "Acesso Negado: Não tem permissão para ver as Notas deste Space."})
		return
	}

	// 3. BUSCA AS NOTAS (Se passou da segurança, liberta a leitura)
	var notes []models.QuickNote
	if err := database.DB.Where("space_id = ?", parsedSpaceID).Order("created_at desc").Limit(300).Find(&notes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao carregar as notas"})
		return
	}

	// Devolve as notas para o Front-end
	c.JSON(http.StatusOK, gin.H{"quick_notes": notes})
}

type UpdateQuickNoteInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Color   string `json:"color"`
}

func UpdateQuickNote(c *gin.Context) {
	noteIDStr := c.Param("note_id")
	parsedNoteID, err := uuid.Parse(noteIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID da nota inválido"})
		return
	}

	var input UpdateQuickNoteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos"})
		return
	}

	// 🛡️ A nota tem que ser desta turma: sem isto, um note_id de outra turma
	// seria aceito (a rota só confere que você é membro DESTA).
	if err := database.DB.Model(&models.QuickNote{}).
		Where("id = ? AND space_id = ?", parsedNoteID, c.Param("space_id")).
		Updates(models.QuickNote{
			Title:   input.Title,
			Content: input.Content,
			Color:   input.Color,
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao atualizar a nota"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Nota atualizada!"})
}

func DeleteQuickNote(c *gin.Context) {
	noteIDStr := c.Param("note_id")
	parsedNoteID, err := uuid.Parse(noteIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID da nota inválido"})
		return
	}

	// 🛡️ Só apaga se a nota for desta turma.
	res := database.DB.Where("id = ? AND space_id = ?", parsedNoteID, c.Param("space_id")).Delete(&models.QuickNote{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao apagar a nota"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Nota não encontrada nesta turma"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Nota apagada!"})
}
