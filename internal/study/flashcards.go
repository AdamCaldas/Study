package study

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Estrutura para receber os dados do Front-end
type CreateFlashcardInput struct {
	Title       string `json:"title"`
	Category    string `json:"category"`
	SubCategory string `json:"sub_category"`
	Front       string `json:"front"`
	Back        string `json:"back"`
	Hint        string `json:"hint"`

	// Integração com Questões Existentes
	QuestionID     string `json:"question_id"`
	QuestionSource string `json:"question_source"` // "STUDFY" (Global) ou "SPACE" (Turma)

	// 👇 A NOVIDADE: Criar Questão a partir do Flashcard!
	SaveAsSpaceQuestion bool `json:"save_as_space_question"`
}

// ==========================================================
// ➕ 1. CRIAR UM FLASHCARD (Normal, Puxando, ou Gerando Questão)
// ==========================================================
func CreateFlashcard(c *gin.Context) {
	spaceIDStr := c.Param("space_id")

	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	var input CreateFlashcardInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos do Flashcard."})
		return
	}

	// 🧠 CENÁRIO 1: O aluno puxou de uma Questão Existente
	if input.QuestionID != "" {
		if input.QuestionSource == "STUDFY" {
			var q models.StudfyQuestion
			if err := database.DB.Where("id = ?", input.QuestionID).First(&q).Error; err == nil {
				input.Title = q.Title
				input.Category = q.Discipline
				input.Front = q.QuestionText
				input.Back = "Gabarito: " + q.CorrectAnswer
			}
		} else if input.QuestionSource == "SPACE" {
			var q models.SpaceQuestion
			if err := database.DB.Where("id = ?", input.QuestionID).First(&q).Error; err == nil {
				input.Title = q.Title
				input.Category = q.GroupID
				input.Front = q.QuestionText
				input.Back = "Gabarito: " + q.CorrectAnswer
			}
		}
	} else if input.SaveAsSpaceQuestion {
		// 🧠 CENÁRIO 2: O aluno digitou o Flashcard na mão e quer que vire Questão da Turma!
		newSpaceQ := models.SpaceQuestion{
			SpaceID:       uuid.MustParse(spaceIDStr),
			CreatedByID:   userID,
			Title:         input.Title,
			Discipline:    input.Category,
			QuestionText:  input.Front,           // A frente do card vira o enunciado
			CorrectAnswer: input.Back,            // O verso vira a resposta correta
			QuestionType:  "flashcard_generated", // Marca no banco que veio de um Flashcard
			Source:        "CUSTOM",
			Points:        1,
			Options:       "[]", // Array vazio porque flashcard não tem alternativas A, B, C, D
		}
		// Salva no banco de questões do Space ANTES de criar o Flashcard
		database.DB.Create(&newSpaceQ)
	}

	// Monta o modelo final do Flashcard para salvar
	newCard := models.Flashcard{
		SpaceID:     uuid.MustParse(spaceIDStr),
		CreatedByID: userID,
		Title:       input.Title,
		Category:    input.Category,
		SubCategory: input.SubCategory,
		Front:       input.Front,
		Back:        input.Back,
		Hint:        input.Hint,
	}

	if err := database.DB.Create(&newCard).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar Flashcard."})
		return
	}

	// Puxa os dados do criador pra devolver completinho pro Front
	database.DB.Preload("Creator").First(&newCard, newCard.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message":   "Flashcard gerado com sucesso!",
		"flashcard": newCard,
	})
}

// ==========================================================
// 📋 2. LISTAR FLASHCARDS (Com Filtros e Pesquisa de Texto)
// ==========================================================
func ListFlashcards(c *gin.Context) {
	spaceID := c.Param("space_id")

	// Pega os filtros da URL
	category := c.Query("category")
	subCategory := c.Query("sub_category")
	searchQuery := c.Query("search") // 👈 O NOVO FILTRO DE PESQUISA POR TEXTO!

	// O Preload("Creator") traz o Nome do aluno que fez o card
	query := database.DB.Preload("Creator").Where("space_id = ?", spaceID)

	// Aplica os filtros dinamicamente se o Front-end mandar
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if subCategory != "" {
		query = query.Where("sub_category = ?", subCategory)
	}
	if searchQuery != "" {
		// Pesquisa a palavra digitada tanto no Título quanto na Pergunta (Front) da carta
		query = query.Where("title ILIKE ? OR front ILIKE ?", "%"+searchQuery+"%", "%"+searchQuery+"%")
	}

	var flashcards []models.Flashcard
	query.Order("created_at desc").Find(&flashcards)

	c.JSON(http.StatusOK, gin.H{"flashcards": flashcards})
}

// ==========================================================
// ✏️ 3. EDITAR FLASHCARD (Regra de Permissão)
// ==========================================================
func UpdateFlashcard(c *gin.Context) {
	spaceID := c.Param("space_id")
	cardID := c.Param("card_id")
	userIDInterface, _ := c.Get("userID")

	var playerID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		playerID = v
	case string:
		playerID, _ = uuid.Parse(v)
	}

	var input models.Flashcard
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos."})
		return
	}

	var space models.Space
	database.DB.Where("id = ?", spaceID).First(&space)

	var card models.Flashcard
	if err := database.DB.Where("id = ? AND space_id = ?", cardID, spaceID).First(&card).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Flashcard não encontrado."})
		return
	}

	// 🚨 TRAVA DE SEGURANÇA: Só o Dono do Card OU o Dono do Space podem editar!
	if card.CreatedByID != playerID && space.OwnerID != playerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Sem permissão. Apenas o criador do card ou o dono da turma podem editar."})
		return
	}

	database.DB.Model(&card).Updates(map[string]interface{}{
		"title":        input.Title,
		"category":     input.Category,
		"sub_category": input.SubCategory,
		"front":        input.Front,
		"back":         input.Back,
		"hint":         input.Hint,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Flashcard atualizado!"})
}

// ==========================================================
// 🗑️ 4. APAGAR FLASHCARD (Regra de Permissão)
// ==========================================================
func DeleteFlashcard(c *gin.Context) {
	spaceID := c.Param("space_id")
	cardID := c.Param("card_id")
	userIDInterface, _ := c.Get("userID")

	var playerID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		playerID = v
	case string:
		playerID, _ = uuid.Parse(v)
	}

	var space models.Space
	database.DB.Where("id = ?", spaceID).First(&space)

	var card models.Flashcard
	if err := database.DB.Where("id = ? AND space_id = ?", cardID, spaceID).First(&card).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Flashcard não encontrado."})
		return
	}

	// 🚨 TRAVA DE SEGURANÇA: Só o Dono do Card OU o Dono do Space podem excluir!
	if card.CreatedByID != playerID && space.OwnerID != playerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Sem permissão. Apenas o criador do card ou o dono da turma podem excluir."})
		return
	}

	database.DB.Delete(&card)

	c.JSON(http.StatusOK, gin.H{"message": "Flashcard apagado da turma!"})
}
