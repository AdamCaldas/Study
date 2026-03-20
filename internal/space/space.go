package space

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// O que esperamos receber do Frontend para criar um Space (Atualizado MVP)
type CreateSpaceInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	ColorHex    string `json:"color_hex"`
	Category    string `json:"category"`
	Visibility  string `json:"visibility"`
}

// Cria um novo Space
func CreateSpace(c *gin.Context) {
	// 1. Pega o ID do usuário logado que o AuthMiddleware salvou no contexto
	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuário não autenticado"})
		return
	}

	// Converte ID para UUID com segurança
	var userID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, _ = uuid.Parse(v)
	}

	var input CreateSpaceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados inválidos: " + err.Error()})
		return
	}

	if input.ColorHex == "" {
		input.ColorHex = "#FFFFFF"
	}
	if input.Visibility == "" {
		input.Visibility = "private"
	}

	// 🌟 FASE 2: Busca quem está criando o Space para saber o Cargo dele
	var currentUser models.User
	if err := database.DB.Where("id = ?", userID).First(&currentUser).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuário não encontrado"})
		return
	}

	randomHex := uuid.New().String()[:6]
	slugBase := strings.ToLower(strings.ReplaceAll(input.Name, " ", "-"))
	slug := fmt.Sprintf("%s-%s", slugBase, randomHex)
	shareCode := fmt.Sprintf("SPACE-%s", strings.ToUpper(randomHex))

	newSpace := models.Space{
		OwnerID:     userID,
		Name:        input.Name,
		Description: input.Description,
		ColorHex:    input.ColorHex,
		Category:    input.Category,
		Visibility:  input.Visibility,
		Status:      "active",
		Slug:        slug,
		ShareCode:   shareCode,
	}

	// 🌟 FASE 2: A MÁGICA DA SALA DE AULA 🌟
	// Se quem cria for PROFESSOR, o Space nasce blindado como Classroom
	if currentUser.AccountType == "TEACHER" {
		newSpace.IsClassroom = true
	}

	// 5. Salva no banco de dados
	if err := database.DB.Create(&newSpace).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao criar Space"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Space criado com sucesso!",
		"space":   newSpace,
	})
}

// ListSpaces - Retorna os Spaces que o usuário é dono OU convidado
func ListSpaces(c *gin.Context) {
	userIDContext, _ := c.Get("userID")
	userIDStr := fmt.Sprintf("%v", userIDContext)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "ID de usuário inválido"})
		return
	}

	spaces := []models.Space{}
	subQuery := database.DB.Model(&models.SpacePermission{}).Select("space_id").Where("user_id = ?", userID)

	if err := database.DB.Where("owner_id = ?", userID).Or("id IN (?)", subQuery).Find(&spaces).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao buscar Spaces", "detalhe": err.Error()})
		return
	}

	c.JSON(200, gin.H{"spaces": spaces})
}

type UpdateSpaceInput struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	ColorHex           string `json:"color_hex"`
	Category           string `json:"category"`
	Visibility         string `json:"visibility"`
	Status             string `json:"status"`
	AllowCollaborators *bool  `json:"allow_collaborators"`
	AllowComments      *bool  `json:"allow_comments"`
	MaxCollaborators   int    `json:"max_collaborators"`
}

// UpdateSpace - Atualiza as configurações e permissões
func UpdateSpace(c *gin.Context) {
	spaceID := c.Param("space_id")
	var input UpdateSpaceInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Dados inválidos", "detalhe": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Description != "" {
		updates["description"] = input.Description
	}
	if input.ColorHex != "" {
		updates["color_hex"] = input.ColorHex
	}
	if input.Category != "" {
		updates["category"] = input.Category
	}
	if input.Visibility != "" {
		updates["visibility"] = input.Visibility
	}
	if input.Status != "" {
		updates["status"] = input.Status
	}
	if input.MaxCollaborators > 0 {
		updates["max_collaborators"] = input.MaxCollaborators
	}
	if input.AllowCollaborators != nil {
		updates["allow_collaborators"] = *input.AllowCollaborators
	}
	if input.AllowComments != nil {
		updates["allow_comments"] = *input.AllowComments
	}

	if err := database.DB.Model(&models.Space{}).Where("id = ?", spaceID).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao atualizar Space", "detalhe": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Configurações salvas!"})
}

func DeleteSpace(c *gin.Context) {
	spaceID := c.Param("space_id")

	if err := database.DB.Delete(&models.Space{}, "id = ?", spaceID).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao apagar o Space"})
		return
	}

	c.JSON(200, gin.H{"message": "Space deletado com sucesso!"})
}

// GetSpaceByCode - Devolve o "Cartão de Visitas" antes do cara entrar
func GetSpaceByCode(c *gin.Context) {
	code := c.Param("code")

	var preview struct {
		SpaceID        string `json:"space_id"`
		Name           string `json:"name"`
		ColorHex       string `json:"color_hex"`
		OwnerName      string `json:"owner_name"`
		TotalNotebooks int    `json:"total_notebooks"`
		UpdatedAt      string `json:"updated_at"`
	}

	err := database.DB.Table("spaces").
		Select("spaces.id as space_id, spaces.name, spaces.color_hex, users.full_name as owner_name, spaces.updated_at, (SELECT COUNT(id) FROM notebooks WHERE space_id = spaces.id) as total_notebooks").
		Joins("left join users on users.id = spaces.owner_id").
		Where("spaces.share_code = ?", code).
		Scan(&preview).Error

	if err != nil || preview.SpaceID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado. Verifique o código."})
		return
	}

	c.JSON(http.StatusOK, preview)
}

type JoinSpaceInput struct {
	Code string `json:"code" binding:"required"`
}

// JoinSpaceByCode - Adiciona o usuário ao Space via código de convite
func JoinSpaceByCode(c *gin.Context) {
	userIDContext, _ := c.Get("userID")
	userIDStr := fmt.Sprintf("%v", userIDContext)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Usuário inválido"})
		return
	}

	var input JoinSpaceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Código não fornecido", "detalhe": err.Error()})
		return
	}

	var space models.Space
	if err := database.DB.Where("share_code = ?", input.Code).First(&space).Error; err != nil {
		c.JSON(404, gin.H{"error": "Space não encontrado ou código inválido"})
		return
	}

	if space.OwnerID == userID {
		c.JSON(400, gin.H{"error": "Você já é o dono deste Space!"})
		return
	}

	var existingPermission models.SpacePermission
	if err := database.DB.Where("user_id = ? AND space_id = ?", userID, space.ID).First(&existingPermission).Error; err == nil {
		c.JSON(400, gin.H{"error": "Você já faz parte deste Space!"})
		return
	}

	// Como padrão, todos entram como VIEWER. Se for um Classroom, eles NUNCA vão passar disso.
	newPermission := models.SpacePermission{
		UserID:      userID,
		SpaceID:     space.ID,
		AccessLevel: "VIEWER",
	}

	if err := database.DB.Create(&newPermission).Error; err != nil {
		c.JSON(500, gin.H{"error": "Erro ao entrar no Space", "detalhe": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message": "Bem-vindo ao Space!",
		"space":   space,
	})
}

// ==========================================================
// 🕵️ DOSSIÊ DO ALUNO (Fase 2: Notas Privadas do Professor)
// ==========================================================
func GetOrUpdateStudentDossier(c *gin.Context) {
	teacherIDInterface, _ := c.Get("userID")
	var teacherID uuid.UUID
	switch v := teacherIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	spaceID, _ := uuid.Parse(c.Param("space_id"))
	studentID, _ := uuid.Parse(c.Param("student_id"))

	// 1. Verifica se quem está acessando é realmente o dono do Space (Professor)
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, teacherID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas o professor da turma tem acesso ao Dossiê."})
		return
	}

	// 2. LER (GET)
	if c.Request.Method == "GET" {
		var dossier models.StudentDossier
		database.DB.Where("space_id = ? AND student_id = ?", spaceID, studentID).First(&dossier)
		c.JSON(http.StatusOK, gin.H{"dossier": dossier})
		return
	}

	// 3. ATUALIZAR/CRIAR (PUT)
	if c.Request.Method == "PUT" {
		var input struct {
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Conteúdo inválido."})
			return
		}

		var dossier models.StudentDossier
		result := database.DB.Where("space_id = ? AND student_id = ?", spaceID, studentID).First(&dossier)

		if result.Error != nil {
			// Cria um novo Dossiê
			newDossier := models.StudentDossier{
				SpaceID:   spaceID,
				StudentID: studentID,
				TeacherID: teacherID,
				Content:   input.Content,
			}
			database.DB.Create(&newDossier)
			c.JSON(http.StatusOK, gin.H{"message": "Dossiê criado com sucesso!", "dossier": newDossier})
		} else {
			// Atualiza o existente
			database.DB.Model(&dossier).Update("content", input.Content)
			c.JSON(http.StatusOK, gin.H{"message": "Dossiê atualizado com sucesso!"})
		}
	}
}

// ==========================================================
// 💬 FASE 5: ALUNO ENVIA UMA DÚVIDA
// ==========================================================
func CreatePageDoubt(c *gin.Context) {
	spaceID := c.Param("space_id")
	pageID := c.Param("page_id")

	userIDInterface, _ := c.Get("userID")
	var studentID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		studentID = v
	case string:
		studentID, _ = uuid.Parse(v)
	}

	var input struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O conteúdo da dúvida é obrigatório."})
		return
	}

	newDoubt := models.PageDoubt{
		SpaceID:   uuid.MustParse(spaceID),
		PageID:    uuid.MustParse(pageID),
		StudentID: studentID,
		Content:   input.Content,
	}

	if err := database.DB.Create(&newDoubt).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao enviar dúvida."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Dúvida enviada ao professor!",
		"doubt":   newDoubt,
	})
}

// ==========================================================
// 💬 FASE 5: PROFESSOR LISTA TODAS AS DÚVIDAS DA TURMA
// ==========================================================
func ListSpaceDoubts(c *gin.Context) {
	spaceID := c.Param("space_id")

	var doubts []models.PageDoubt
	// Ordena para mostrar as não resolvidas primeiro, e as mais antigas no topo
	database.DB.Where("space_id = ?", spaceID).Order("resolved asc, created_at asc").Find(&doubts)

	if doubts == nil {
		doubts = []models.PageDoubt{}
	}

	c.JSON(http.StatusOK, gin.H{"doubts": doubts})
}

// ==========================================================
// 💬 FASE 5: PROFESSOR (OU MONITOR) RESPONDE A DÚVIDA
// ==========================================================
func AnswerPageDoubt(c *gin.Context) {
	doubtID := c.Param("doubt_id")

	teacherIDInterface, _ := c.Get("userID")
	var teacherID uuid.UUID
	switch v := teacherIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	var input struct {
		Answer string `json:"answer" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A resposta não pode ser vazia."})
		return
	}

	var doubt models.PageDoubt
	if err := database.DB.Where("id = ?", doubtID).First(&doubt).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dúvida não encontrada."})
		return
	}

	// Atualiza com a resposta e marca como resolvida
	database.DB.Model(&doubt).Updates(map[string]interface{}{
		"answer":      input.Answer,
		"answered_by": teacherID,
		"resolved":    true,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Dúvida respondida com sucesso!",
	})
}

// ==========================================================
// 📢 FASE 5: O MEGAFONE (Avisos em Massa do Professor)
// ==========================================================
func SendMegaphoneMessage(c *gin.Context) {
	spaceID := c.Param("space_id")

	userIDInterface, _ := c.Get("userID")
	var teacherID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	// 1. Verifica se a Sala de Aula existe e puxa o dono
	var space models.Space
	if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado."})
		return
	}

	// 2. Trava de Segurança: Apenas o Dono ou Monitor podem usar o Megafone
	isAuthorized := (space.OwnerID == teacherID)
	if !isAuthorized {
		var perm models.SpacePermission
		database.DB.Where("space_id = ? AND user_id = ?", spaceID, teacherID).First(&perm)
		if perm.AccessLevel == "EDITOR" || perm.AccessLevel == "MONITOR" {
			isAuthorized = true
		}
	}

	if !isAuthorized {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas o professor ou os monitores podem disparar avisos na turma."})
		return
	}

	// 3. O Professor digita o título e o texto do aviso
	var input struct {
		Title   string `json:"title" binding:"required"`
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Título e Mensagem são obrigatórios."})
		return
	}

	// 4. Mágica do Backend: Monta o "Alvo" (TargetID) formatado como array JSON
	targetIDsJSON := fmt.Sprintf(`["%s"]`, spaceID)

	// 5. Salva na Tabela Global de Notificações
	notification := models.Notification{
		Title:       input.Title,
		Message:     input.Message,
		Type:        "NEWS",   // Pode ser NEWS, INFO ou BELL (Fica a critério do Front mapear o ícone)
		Audience:    "SPACES", // 👈 O Segredo! O Backend só vai entregar pra quem é dessa turma
		TargetIDs:   targetIDsJSON,
		IsActive:    true,
		CreatedByID: teacherID,
	}

	if err := database.DB.Create(&notification).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao disparar o megafone."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Aviso disparado com sucesso para toda a turma!",
		"notification": notification,
	})
}

// ==========================================================
// 📷 FASE 5: GERAR QR CODE DE PRESENÇA (Visão do Professor)
// ==========================================================
func GenerateAttendanceQR(c *gin.Context) {
	spaceID := c.Param("space_id")
	userIDInterface, _ := c.Get("userID")

	var teacherID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	// 1. Verifica se a Sala de Aula existe
	var space models.Space
	if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado."})
		return
	}

	// 2. Trava de Segurança: Apenas o Dono ou Monitor
	isAuthorized := (space.OwnerID == teacherID)
	if !isAuthorized {
		var perm models.SpacePermission
		database.DB.Where("space_id = ? AND user_id = ?", spaceID, teacherID).First(&perm)
		if perm.AccessLevel == "EDITOR" || perm.AccessLevel == "MONITOR" {
			isAuthorized = true
		}
	}

	if !isAuthorized {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas o professor ou monitores podem gerar a lista de presença."})
		return
	}

	// 3. Cria um token único impossível de fraudar
	qrToken := uuid.New().String()

	// 4. Define que o QR Code só vale por 15 minutos
	expires := time.Now().Add(15 * time.Minute)

	session := models.AttendanceSession{
		SpaceID:   uuid.MustParse(spaceID),
		TeacherID: teacherID,
		QRCode:    qrToken,
		ExpiresAt: expires,
		IsActive:  true,
	}
	database.DB.Create(&session)

	// O Front-end pega esse "qr_token" e usa uma biblioteca (ex: qrcode.react) para desenhar o quadrado preto e branco na tela!
	c.JSON(http.StatusCreated, gin.H{
		"message":    "QR Code gerado! Mostre no telão.",
		"qr_token":   qrToken,
		"expires_at": expires,
	})
}

// ==========================================================
// 📷 FASE 5: REGISTRAR PRESENÇA (Visão do Aluno)
// ==========================================================
func RegisterAttendance(c *gin.Context) {
	userIDInterface, _ := c.Get("userID")
	var studentID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		studentID = v
	case string:
		studentID, _ = uuid.Parse(v)
	}

	// O app mobile do aluno lê a câmera e manda a string do QR Code pra cá
	var input struct {
		QRToken string `json:"qr_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token do QR Code é obrigatório."})
		return
	}

	// 1. Busca se esse QR Code existe e está ativo no banco
	var session models.AttendanceSession
	if err := database.DB.Where("qr_code = ? AND is_active = true", input.QRToken).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "QR Code inválido ou a chamada já foi encerrada."})
		return
	}

	// 2. Verifica se o professor já fechou a chamada pelo tempo
	if time.Now().After(session.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O tempo para responder a chamada esgotou!"})
		return
	}

	// 3. Verifica se o aluno já "bateu o ponto" (pra não registrar duas vezes)
	var existing models.AttendanceRecord
	if err := database.DB.Where("session_id = ? AND student_id = ?", session.ID, studentID).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "Sua presença já estava confirmada!"})
		return
	}

	// 4. Salva a presença do aluno
	record := models.AttendanceRecord{
		SessionID: session.ID,
		StudentID: studentID,
	}
	database.DB.Create(&record)

	// Bônus: Dá um XPzinho pro aluno por ter ido na aula!
	database.DB.Model(&models.User{}).Where("id = ?", studentID).Update("xp", gorm.Expr("xp + ?", 10))

	c.JSON(http.StatusOK, gin.H{"message": "Presença confirmada com sucesso! +10 XP"})
}

// ==========================================================
// 📊 FASE 7: TERMÔMETRO DA TURMA (Analytics do Professor)
// ==========================================================
func GetClassThermometer(c *gin.Context) {
	spaceID := c.Param("space_id")

	userIDInterface, _ := c.Get("userID")
	var teacherID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	// 1. Trava de Segurança: Apenas Dono ou Monitor podem ver o relatório
	var space models.Space
	if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado."})
		return
	}

	isAuthorized := (space.OwnerID == teacherID)
	if !isAuthorized {
		var perm models.SpacePermission
		database.DB.Where("space_id = ? AND user_id = ?", spaceID, teacherID).First(&perm)
		if perm.AccessLevel == "EDITOR" || perm.AccessLevel == "MONITOR" {
			isAuthorized = true
		}
	}

	if !isAuthorized {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas o professor ou monitores podem acessar o Termômetro da Turma."})
		return
	}

	// 2. Métrica A: Média Geral da Sala nos Simulados
	var results []models.QuizResult
	database.DB.Where("space_id = ? AND status = 'completed'", spaceID).Find(&results)

	var totalScore float64 = 0
	var averageScore float64 = 0
	if len(results) > 0 {
		for _, r := range results {
			totalScore += r.Score
		}
		averageScore = totalScore / float64(len(results))
	}

	// 3. Métrica B: Alunos em Risco de Evasão (Média abaixo de 6.0)
	// Usamos SQL Raw (GORM) para agrupar as notas de cada aluno e achar quem está mal
	type StudentRisk struct {
		UserID   string  `json:"user_id"`
		FullName string  `json:"full_name"`
		Email    string  `json:"email"`
		Average  float64 `json:"average"`
	}

	var atRisk []StudentRisk
	database.DB.Raw(`
		SELECT u.id as user_id, u.full_name, u.email, AVG(qr.score) as average
		FROM quiz_results qr
		JOIN users u ON u.id = qr.user_id
		WHERE qr.space_id = ? AND qr.status = 'completed'
		GROUP BY u.id, u.full_name, u.email
		HAVING AVG(qr.score) < 6.0
		ORDER BY average ASC
	`, spaceID).Scan(&atRisk)

	if atRisk == nil {
		atRisk = []StudentRisk{}
	}

	// 4. Métrica C: Engajamento Base (Quantos Pomodoros a turma já fez)
	var totalPomodoros int64
	database.DB.Model(&models.PomodoroSession{}).Where("space_id = ?", spaceID).Count(&totalPomodoros)

	// 5. Devolve o relatório mastigado para o Front-end desenhar os gráficos!
	c.JSON(http.StatusOK, gin.H{
		"overview": gin.H{
			"total_exams_taken":    len(results),
			"class_average":        averageScore,
			"total_focus_sessions": totalPomodoros,
		},
		"students_at_risk": atRisk, // O painel vermelho pro professor ligar o alerta
	})
}

// ==========================================================
// 🤖 FASE 7: CRIAR REGRA DE AUTOMAÇÃO (Visão do Professor)
// ==========================================================
func CreateAutomationRule(c *gin.Context) {
	spaceID := c.Param("space_id")

	teacherIDInterface, _ := c.Get("userID")
	var teacherID uuid.UUID
	switch v := teacherIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	// 1. Trava de Segurança
	var space models.Space
	if err := database.DB.Where("id = ? AND owner_id = ?", spaceID, teacherID).First(&space).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas o professor pode criar automações."})
		return
	}

	// 2. Recebe a "Fórmula"
	var input struct {
		ConditionType   string    `json:"condition_type" binding:"required"`
		ConditionValue  float64   `json:"condition_value" binding:"required"`
		ActionType      string    `json:"action_type" binding:"required"`
		TargetContentID uuid.UUID `json:"target_content_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dados da automação inválidos."})
		return
	}

	// 3. Salva o robô no banco
	rule := models.AutomationRule{
		SpaceID:         space.ID,
		TeacherID:       teacherID,
		ConditionType:   input.ConditionType,
		ConditionValue:  input.ConditionValue,
		ActionType:      input.ActionType,
		TargetContentID: input.TargetContentID,
	}

	database.DB.Create(&rule)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Automação ativada! O StudFy vai monitorar as notas da turma.",
		"rule":    rule,
	})
}

// ==========================================================
// ⚙️ O MOTOR INVISÍVEL (Roda sozinho no Background)
// ==========================================================
// Você pode chamar essa função dentro do seu "SubmitQuiz" quando a nota é calculada!
func RunAutomationEngine(spaceID uuid.UUID, studentID uuid.UUID, score float64) {
	var rules []models.AutomationRule

	// Busca todos os robôs ativos nesta sala de aula
	database.DB.Where("space_id = ? AND is_active = true", spaceID).Find(&rules)

	for _, rule := range rules {
		// Verifica a Condição: "Se a nota for menor que X"
		if rule.ConditionType == "score_below" && score < rule.ConditionValue {

			// Executa a Ação: "Destranca o Caderno Extra"
			if rule.ActionType == "unlock_notebook" {

				// 1. Dá permissão de visualização do Caderno específico só para este aluno!
				permission := models.NotebookPermission{
					NotebookID:  rule.TargetContentID,
					UserID:      studentID,
					AccessLevel: "VIEWER",
				}
				// Usa o FirstOrCreate para não duplicar se ele for mal em 2 provas
				database.DB.Where(models.NotebookPermission{NotebookID: rule.TargetContentID, UserID: studentID}).FirstOrCreate(&permission)

				// 2. Manda uma notificação push avisando o aluno do reforço!
				targetJSON := fmt.Sprintf(`["%s"]`, studentID)
				notification := models.Notification{
					Title:       "📚 Material de Recuperação Liberado!",
					Message:     "O professor liberou um caderno extra de reforço para te ajudar com os estudos. Dê uma olhada!",
					Type:        "BELL",
					Audience:    "USERS",
					TargetIDs:   targetJSON,
					CreatedByID: rule.TeacherID,
				}
				database.DB.Create(&notification)
			}
		}
	}
}

// ==========================================================
// 📊 FASE 7: EXPORTAR DIÁRIO DE CLASSE (.CSV)
// ==========================================================
func ExportClassDiaryCSV(c *gin.Context) {
	spaceID := c.Param("space_id")

	userIDInterface, _ := c.Get("userID")
	var teacherID uuid.UUID
	switch v := userIDInterface.(type) {
	case uuid.UUID:
		teacherID = v
	case string:
		teacherID, _ = uuid.Parse(v)
	}

	// 1. Trava de Segurança (Apenas professor ou monitor)
	var space models.Space
	if err := database.DB.Where("id = ?", spaceID).First(&space).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Space não encontrado."})
		return
	}

	isAuthorized := (space.OwnerID == teacherID)
	if !isAuthorized {
		var perm models.SpacePermission
		database.DB.Where("space_id = ? AND user_id = ?", spaceID, teacherID).First(&perm)
		if perm.AccessLevel == "EDITOR" || perm.AccessLevel == "MONITOR" {
			isAuthorized = true
		}
	}

	if !isAuthorized {
		c.JSON(http.StatusForbidden, gin.H{"error": "Apenas o professor pode baixar o diário de classe."})
		return
	}

	// 2. Busca todos os Alunos desta Sala (Nível VIEWER)
	var students []models.User
	database.DB.Table("users").
		Joins("JOIN space_permissions ON space_permissions.user_id = users.id").
		Where("space_permissions.space_id = ? AND space_permissions.access_level = 'VIEWER'", spaceID).
		Find(&students)

	// 3. Prepara o Arquivo CSV na Memória RAM
	b := &bytes.Buffer{}
	writer := csv.NewWriter(b)

	// 4. Escreve o Cabeçalho da Planilha (Linha 1)
	writer.Write([]string{"Nome do Aluno", "Email", "XP Acumulado", "Total de Presencas", "Media Geral (Provas)"})

	// 5. Preenche os dados aluno por aluno
	for _, student := range students {
		// Calcula as presenças
		var presences int64
		database.DB.Table("attendance_records").
			Joins("JOIN attendance_sessions ON attendance_sessions.id = attendance_records.session_id").
			Where("attendance_sessions.space_id = ? AND attendance_records.student_id = ?", spaceID, student.ID).
			Count(&presences)

		// Calcula a Média Geral
		var results []models.QuizResult
		database.DB.Where("space_id = ? AND user_id = ? AND status = 'completed'", spaceID, student.ID).Find(&results)
		var totalScore float64 = 0
		var average float64 = 0
		if len(results) > 0 {
			for _, r := range results {
				totalScore += r.Score
			}
			average = totalScore / float64(len(results))
		}

		// Escreve a linha do aluno na planilha
		writer.Write([]string{
			student.FullName,
			student.Email,
			fmt.Sprintf("%d", student.XP),
			fmt.Sprintf("%d", presences),
			fmt.Sprintf("%.2f", average),
		})
	}

	// Garante que tudo foi escrito no Buffer
	writer.Flush()

	// 6. MÁGICA: Configura os Headers para forçar o Navegador a fazer o DOWNLOAD do arquivo
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename=diario_de_classe_studfy.csv")
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Transfer-Encoding", "binary")

	// 7. Envia o arquivo para o Front-end
	c.Data(http.StatusOK, "text/csv", b.Bytes())
}
