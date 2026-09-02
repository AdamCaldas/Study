package study

import (
	"log"
	"net/http"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==========================================================
// 🧠 CURVA DE ESQUECIMENTO (REPETIÇÃO ESPAÇADA)
// ==========================================================
// Antes só existia "agendar para amanhã": nada marcava a revisão como feita e
// não havia intervalo crescente — ou seja, a funcionalidade mais importante de
// um app de estudos existia só pela metade.
//
// A escada de intervalos abaixo é o modelo clássico de repetição espaçada:
// cada vez que o aluno revisa e diz que lembrou, a próxima revisão vai para
// mais longe. Se ele esqueceu, volta para o começo.
var reviewIntervals = []int{1, 3, 7, 15, 30, 60}

// nextReviewDate devolve a data da próxima revisão para um dado degrau.
func nextReviewDate(stage int) (time.Time, int) {
	if stage < 0 {
		stage = 0
	}
	if stage >= len(reviewIntervals) {
		stage = len(reviewIntervals) - 1 // trava no maior intervalo (60 dias)
	}
	return time.Now().AddDate(0, 0, reviewIntervals[stage]), stage
}

type CreateReviewInput struct {
	NoteID string `json:"note_id" binding:"required"`
}

// CreateReview agenda a primeira revisão de uma página.
func CreateReview(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	var input CreateReviewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O ID da anotação (note_id) é obrigatório"})
		return
	}

	parsedNoteID, err := uuid.Parse(input.NoteID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID da anotação inválido"})
		return
	}

	// A página precisa existir de verdade (evita agendar revisão de nada).
	var page models.Page
	if err := database.DB.Select("id").Where("id = ?", parsedNoteID).First(&page).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Página não encontrada"})
		return
	}

	// Já existe revisão pendente desta página para este aluno? Não duplica.
	var pending models.Review
	if err := database.DB.Where("note_id = ? AND user_id = ? AND status = 'pendente'", parsedNoteID, userID).
		First(&pending).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "Esta página já está na sua fila de revisão.",
			"review":  pending,
		})
		return
	}

	reviewDate, stage := nextReviewDate(0)
	newReview := models.Review{
		NoteID:     parsedNoteID,
		UserID:     userID,
		ReviewDate: reviewDate,
		Status:     "pendente",
		Stage:      stage,
	}

	if err := database.DB.Create(&newReview).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao agendar revisão."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Revisão agendada! Você será lembrado em 1 dia.",
		"review":  newReview,
	})
}

// ==========================================================
// 📋 MINHA FILA DE REVISÕES (o que está vencido ou vence hoje)
// ==========================================================
func ListMyReviews(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	p := utils.GetPage(c)

	// ?scope=due (padrão) traz só o que já está na hora; ?scope=all traz tudo.
	scope := c.DefaultQuery("scope", "due")

	type reviewItem struct {
		ID         uuid.UUID `json:"id"`
		NoteID     uuid.UUID `json:"note_id"`
		PageTitle  string    `json:"page_title"`
		NotebookID uuid.UUID `json:"notebook_id"`
		ReviewDate time.Time `json:"review_date"`
		Stage      int       `json:"stage"`
		IsDue      bool      `json:"is_due"`
	}

	base := database.DB.Table("reviews r").
		Joins("JOIN pages p ON p.id = r.note_id").
		Where("r.user_id = ? AND r.status = 'pendente'", userID)

	if scope == "due" {
		base = base.Where("r.review_date <= ?", time.Now())
	}

	var total int64
	base.Count(&total)

	var items []reviewItem
	base.Select("r.id, r.note_id, p.title as page_title, p.notebook_id, r.review_date, r.stage").
		Order("r.review_date ASC").
		Offset(p.Offset()).
		Limit(p.Limit).
		Scan(&items)

	now := time.Now()
	for i := range items {
		items[i].IsDue = !items[i].ReviewDate.After(now)
	}
	if items == nil {
		items = []reviewItem{}
	}

	c.JSON(http.StatusOK, gin.H{"reviews": items, "pagination": p.Meta(total)})
}

// ==========================================================
// ✅ CONCLUIR UMA REVISÃO
// ==========================================================
// O aluno diz se lembrou do conteúdo:
//   - remembered = true  → sobe um degrau (próxima revisão mais distante)
//   - remembered = false → volta ao degrau 0 (revisa de novo amanhã)
func CompleteReview(c *gin.Context) {
	userID, err := utils.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autenticado"})
		return
	}

	reviewID, err := uuid.Parse(c.Param("review_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID da revisão inválido"})
		return
	}

	var input struct {
		Remembered *bool `json:"remembered"`
	}
	// Corpo opcional: sem ele, assume que o aluno lembrou.
	_ = c.ShouldBindJSON(&input)
	remembered := input.Remembered == nil || *input.Remembered

	// 🛡️ A revisão tem que ser do próprio aluno.
	var review models.Review
	if err := database.DB.Where("id = ? AND user_id = ?", reviewID, userID).First(&review).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Revisão não encontrada."})
		return
	}
	if review.Status == "concluida" {
		c.JSON(http.StatusOK, gin.H{"message": "Esta revisão já estava concluída."})
		return
	}

	now := time.Now()
	tx := database.DB.Begin()

	if err := tx.Model(&review).Updates(map[string]interface{}{
		"status":       "concluida",
		"completed_at": now,
	}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao concluir a revisão."})
		return
	}

	// Agenda a próxima: sobe um degrau se lembrou, recomeça se esqueceu.
	nextStage := 0
	if remembered {
		nextStage = review.Stage + 1
	}
	nextDate, appliedStage := nextReviewDate(nextStage)

	next := models.Review{
		NoteID:     review.NoteID,
		UserID:     userID,
		ReviewDate: nextDate,
		Status:     "pendente",
		Stage:      appliedStage,
	}
	if err := tx.Create(&next).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao agendar a próxima revisão."})
		return
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("commit falhou: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Não foi possível concluir a revisão. Tente de novo."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Revisão concluída!",
		"next_review": next,
		"days_ahead":  reviewIntervals[appliedStage],
	})
}
