package study

import (
	"net/http"
	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==========================================================
// 📥 ESTRUTURA PARA RECEBER O BARALHO DO FRONT-END
// ==========================================================
type CreateDeckInput struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Category    string `json:"category"`
	SubCategory string `json:"sub_category"`
	Cards       []struct {
		ID    *uuid.UUID `json:"id"` // Opcional (usado na edição)
		Front string     `json:"front" binding:"required"`
		Back  string     `json:"back" binding:"required"`
	} `json:"cards"`
}

// ==========================================================
// ➕ 1. CRIAR OU EDITAR BARALHO DE FLASHCARDS COMPLETO
// ==========================================================
// Aqui o Dono do Space manda o baralho inteiro de uma vez (Super Update/Create)
func SaveFlashcardDeck(c *gin.Context) {
	spaceIDStr := c.Param("space_id")
	spaceID := uuid.MustParse(spaceIDStr)

	userIDInterface, _ := c.Get("userID")
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	var input CreateDeckInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos do Flashcard."})
		return
	}

	deckIDStr := c.Query("deck_id") // Se vier na URL, é Edição. Se não, é Criação.

	tx := database.DB.Begin()

	var deck models.FlashcardDeck
	if deckIDStr != "" {
		// É uma EDIÇÃO
		if err := tx.Where("id = ? AND space_id = ?", deckIDStr, spaceID).First(&deck).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusNotFound, gin.H{"error": "Baralho não encontrado."})
			return
		}
		deck.Title = input.Title
		deck.Description = input.Description
		deck.Category = input.Category
		deck.SubCategory = input.SubCategory
		tx.Save(&deck)

		// Apaga as cartas velhas para recriar (Cuidado: perde estatísticas individuais das cartas se houver no futuro)
		// Para ficar simples e robusto agora: Apaga e recria.
		tx.Where("deck_id = ?", deck.ID).Delete(&models.Flashcard{})
	} else {
		// É uma CRIAÇÃO NOVA
		deck = models.FlashcardDeck{
			SpaceID:     spaceID,
			CreatedByID: userID,
			Title:       input.Title,
			Description: input.Description,
			Category:    input.Category,
			SubCategory: input.SubCategory,
		}
		if err := tx.Create(&deck).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar baralho."})
			return
		}
	}

	// Cria as Cartas
	var newCards []models.Flashcard
	for _, cardInput := range input.Cards {
		newCards = append(newCards, models.Flashcard{
			DeckID: deck.ID,
			Front:  cardInput.Front,
			Back:   cardInput.Back,
		})
	}

	if len(newCards) > 0 {
		tx.Create(&newCards)
	}

	tx.Commit()

	// Retorna o baralho recém salvo com as cartas
	database.DB.Preload("Cards").First(&deck, deck.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Baralho salvo com sucesso!",
		"deck":    deck,
	})
}

// ==========================================================
// 📋 2. LISTAR BARALHOS (Com Filtros)
// ==========================================================
func ListFlashcardDecks(c *gin.Context) {
	spaceID := c.Param("space_id")
	category := c.Query("category")        // Filtro: ?category=Matemática
	subCategory := c.Query("sub_category") // Filtro: ?sub_category=Álgebra

	query := database.DB.Preload("Cards").Where("space_id = ?", spaceID)

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if subCategory != "" {
		query = query.Where("sub_category = ?", subCategory)
	}

	var decks []models.FlashcardDeck
	query.Order("created_at desc").Find(&decks)

	c.JSON(http.StatusOK, gin.H{"decks": decks})
}

// ==========================================================
// 🗑️ 3. APAGAR UM BARALHO
// ==========================================================
func DeleteFlashcardDeck(c *gin.Context) {
	spaceID := c.Param("space_id")
	deckID := c.Param("deck_id")

	// O GORM vai apagar as cartas em cascata automaticamente devido à configuração OnDelete:CASCADE
	if err := database.DB.Where("id = ? AND space_id = ?", deckID, spaceID).Delete(&models.FlashcardDeck{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao deletar baralho."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Baralho removido da turma!"})
}
