package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// USER
type User struct {
	ID               uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	FullName         string         `gorm:"not null" json:"full_name"`
	Email            string         `gorm:"unique;not null" json:"email"`
	CPF              string         `gorm:"unique;not null" json:"cpf"`
	Password         string         `gorm:"not null" json:"-"`
	ProfilePic       string         `json:"profile_picture_url"`
	SubscriptionType string         `gorm:"default:'FREE_TRIAL'" json:"subscription_type"`
	AccountType      string         `gorm:"default:'USER'" json:"account_type"`
	TrialEndsAt      time.Time      `json:"trial_ends_at"`
	CreatedAt        time.Time      `json:"created_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	XP               int            `json:"xp" gorm:"default:0"`
	LastLoginAt      time.Time      `json:"last_login_at"`
	CurrentStreak    int            `json:"current_streak" gorm:"default:0"`
	HighestStreak    int            `json:"highest_streak" gorm:"default:0"`
	DevicePlatform   string         `json:"device_platform"`
}

// SPACE
type Space struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	OwnerID     uuid.UUID `gorm:"type:uuid;index" json:"owner_id"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Description string    `json:"description"`
	ColorHex    string    `gorm:"size:7;default:'#FFFFFF'" json:"color_hex"`
	Category    string    `json:"category"`                       // study | course
	Status      string    `gorm:"default:'active'" json:"status"` // active | inactive
	Slug        string    `gorm:"uniqueIndex" json:"slug"`
	ShareCode   string    `gorm:"uniqueIndex" json:"share_code"`
	Visibility  string    `gorm:"default:'private'" json:"visibility"` // public | private

	// Permissões e Configurações
	AllowCollaborators bool `gorm:"default:true" json:"allow_collaborators"`
	AllowComments      bool `gorm:"default:true" json:"allow_comments"`
	IsArchived         bool `gorm:"default:false" json:"is_archived"`
	IsShared           bool `gorm:"default:false" json:"is_shared"`
	MaxCollaborators   int  `gorm:"default:10" json:"max_collaborators"`
	ViewCount          int  `json:"view_count" gorm:"default:0"`

	// Relacionamentos em Cascata
	Notebooks []Notebook `gorm:"foreignKey:SpaceID;constraint:OnDelete:CASCADE" json:"notebooks"`

	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastActivity time.Time `json:"last_activity"`
}

// SPACE PERMISSIONS (Amigos - O Novo Sistema de Permissões Granulares)
type SpacePermission struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"space_id"`
	UserID      uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	AccessLevel string    `gorm:"type:varchar(20);not null" json:"access_level"` // "CUSTOM", "VIEWER", "EDITOR"
	JoinedAt    time.Time `gorm:"autoCreateTime" json:"joined_at"`

	// 🟢 INFORMAÇÕES DO SPACE (Baseado na sua foto)
	CanEditSpaceInfo  bool `gorm:"default:false" json:"can_edit_space_info"`  // Alterar nome e descrição
	CanEditSpaceColor bool `gorm:"default:false" json:"can_edit_space_color"` // Mudar cor do space

	// 🟠 GERENCIAMENTO DE CONTEÚDO (Baseado na sua foto)
	CanCreateContent bool `gorm:"default:false" json:"can_create_content"` // Criar cadernos, notas
	CanEditContent   bool `gorm:"default:false" json:"can_edit_content"`   // Modificar conteúdo existente
	CanDeleteContent bool `gorm:"default:false" json:"can_delete_content"` // Remover cadernos, notas
	CanManageTags    bool `gorm:"default:false" json:"can_manage_tags"`    // Criar e editar tags

	// 🔵 PERMISSÕES AVANÇADAS (Baseado na sua foto)
	CanManageMembers  bool `gorm:"default:false" json:"can_manage_members"`  // Adicionar, remover pessoas
	CanSendInvites    bool `gorm:"default:false" json:"can_send_invites"`    // Enviar convites
	CanSearchContent  bool `gorm:"default:true"  json:"can_search_content"`  // Buscar conteúdos (Geralmente true pra todos)
	CanChangeSettings bool `gorm:"default:false" json:"can_change_settings"` // Alterar configurações gerais

	// 🟣 EXTRAS STUD-FY (O bônus para deixar seu app incomparável)
	CanManagePlans   bool `gorm:"default:false" json:"can_manage_plans"`   // Pode alterar a Agenda/Planos de Estudo
	CanManageCycles  bool `gorm:"default:false" json:"can_manage_cycles"`  // Pode criar/editar Ciclos (A Roleta)
	CanManageQuizzes bool `gorm:"default:false" json:"can_manage_quizzes"` // Pode criar e editar Simulados/Provas
}

// 🌟 NOVA TABELA: PERMISSÃO GRANULAR (Trava por Caderno)
type NotebookPermission struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	NotebookID  uuid.UUID `gorm:"type:uuid;index" json:"notebook_id"`
	UserID      uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	AccessLevel string    `gorm:"type:varchar(20);not null" json:"access_level"` // VIEWER, EDITOR
}

// ==========================================================
// 📚 ESTRUTURA DE DOCUMENTOS: NOTEBOOK -> GUIDE -> PAGE
// ==========================================================

// NOTEBOOK (A Capa do Caderno)
type Notebook struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID  uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	Name     string    `gorm:"size:100;not null" json:"name"`
	ColorHex string    `gorm:"size:7" json:"color_hex"`

	// 👇 NOVOS RELACIONAMENTOS
	Guides []Guide `gorm:"foreignKey:NotebookID;constraint:OnDelete:CASCADE" json:"guides"`
	Pages  []Page  `gorm:"foreignKey:NotebookID;constraint:OnDelete:CASCADE" json:"pages"` // Páginas soltas (sem guia)

	OwnerName string `gorm:"-" json:"owner_name"`

	// 👇 AUDITORIA (A ASSINATURA DIGITAL)
	CreatedByID uuid.UUID `gorm:"type:uuid" json:"created_by_id"`
	UpdatedByID uuid.UUID `gorm:"type:uuid" json:"updated_by_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// 📁 GUIDE (A Nova "Pasta" estilo Google Docs)
type Guide struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	NotebookID uuid.UUID `gorm:"type:uuid;index;not null" json:"notebook_id"`

	// 👇 A MÁGICA DO SUB-GUIDE: Uma guia pode pertencer a outra guia!
	ParentGuideID *uuid.UUID `gorm:"type:uuid;index" json:"parent_guide_id"`

	Name        string `gorm:"size:255;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	Icon        string `gorm:"size:50" json:"icon"`
	ColorHex    string `gorm:"size:7" json:"color_hex"`
	Order       int    `json:"order"`

	OwnerName string `gorm:"-" json:"owner_name"` // Campo Virtual

	// Relacionamentos em Cascata
	Pages     []Page  `gorm:"foreignKey:GuideID;constraint:OnDelete:CASCADE" json:"pages"`
	SubGuides []Guide `gorm:"foreignKey:ParentGuideID;constraint:OnDelete:CASCADE" json:"sub_guides"`

	CreatedByID uuid.UUID `gorm:"type:uuid" json:"created_by_id"`
	UpdatedByID uuid.UUID `gorm:"type:uuid" json:"updated_by_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// 📄 PAGE (O Arquivo de Texto JSONB)
type Page struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	NotebookID uuid.UUID `gorm:"type:uuid;index;not null" json:"notebook_id"` // Sempre vinculada a um caderno

	// 👇 NOVO: A página agora sabe em qual Guia/Pasta ela está!
	GuideID *uuid.UUID `gorm:"type:uuid;index" json:"guide_id"`

	Title   string `gorm:"size:255;not null" json:"title"`
	Content string `gorm:"type:jsonb" json:"content"`
	Order   int    `json:"order"` // Para numerar as páginas na lista

	OwnerName string `gorm:"-" json:"owner_name"`

	// 👇 AUDITORIA (A ASSINATURA DIGITAL)
	CreatedByID uuid.UUID `gorm:"type:uuid" json:"created_by_id"`
	UpdatedByID uuid.UUID `gorm:"type:uuid" json:"updated_by_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ==========================================================
// RESTO DOS MODELOS GERAIS
// ==========================================================

// QUICK NOTE
type QuickNote struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	Title     string    `gorm:"size:255" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	Color     string    `gorm:"size:7" json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

// STUDY CYCLE (A Roleta Inteligente)
type StudyCycle struct {
	ID            uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID       uuid.UUID  `gorm:"type:uuid;index" json:"space_id"`
	Name          string     `gorm:"size:255;not null;default:'Meu Ciclo'" json:"name"`
	Description   string     `json:"description"`
	TargetGoal    string     `gorm:"size:255" json:"target_goal"`
	TargetDate    *time.Time `json:"target_date"`                                   // Ponteiro para permitir nulo
	CycleType     string     `gorm:"size:50;default:'automatic'" json:"cycle_type"` // automatic | manual
	Visibility    string     `gorm:"size:50;default:'private'" json:"visibility"`   // public | private
	HoursPerDay   float64    `json:"hours_per_day"`
	AvailableDays string     `gorm:"type:jsonb" json:"available_days"` // Array de dias salvo como JSONB
	MinSessionMin int        `gorm:"default:30" json:"min_session_minutes"`
	MaxSessionMin int        `gorm:"default:90" json:"max_session_minutes"`
	CurrentStep   int        `gorm:"default:0" json:"current_step"`
	IsActive      bool       `gorm:"default:false" json:"is_active"`

	Items []StudyCycleItem `gorm:"foreignKey:CycleID;constraint:OnDelete:CASCADE" json:"items"`

	CreatedByID uuid.UUID `gorm:"type:uuid" json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type StudyCycleItem struct {
	ID               uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CycleID          uuid.UUID  `gorm:"type:uuid;index" json:"cycle_id"`
	NotebookID       *uuid.UUID `gorm:"type:uuid" json:"notebook_id"` // Ponteiro, pois pode vir vazio (null) do Front
	Name             string     `gorm:"-" json:"name"`                // Campo Virtual (não salva no DB) para o Front mandar o nome
	Sequence         int        `json:"sequence"`
	Importance       int        `gorm:"type:int2;default:3" json:"importance"`
	Performance      int        `gorm:"type:int2;default:3" json:"performance"`
	SuggestedMinutes int        `json:"suggested_minutes"` // Tempo calculado pelo Algoritmo
}

// STUDY PLAN
type StudyPlan struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID    uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	DayOfWeek  int       `json:"day_of_week"`
	StartTime  string    `json:"start_time"`
	EndTime    string    `json:"end_time"`
	NotebookID uuid.UUID `gorm:"type:uuid" json:"notebook_id"`
	Activity   string    `json:"activity"`
	ColorHex   string    `gorm:"size:7;default:'#3B82F6'" json:"color_hex"`
}

// POMODORO SESSION
type PomodoroSession struct {
	ID         uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID     uuid.UUID  `gorm:"type:uuid;index" json:"user_id"`
	Duration   int        `json:"duration_minutes"`
	CreatedAt  time.Time  `json:"created_at"`
	SpaceID    *uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	NotebookID *uuid.UUID `gorm:"type:uuid;index" json:"notebook_id"`
}

// MOOD CHECK-IN (Humor)
type MoodCheckIn struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Mood      string    `gorm:"size:50;not null" json:"mood"`
	CreatedAt time.Time `json:"created_at"`
}

// SPACE TAGS (Para categorizar os Spaces)
type SpaceTag struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index;constraint:OnDelete:CASCADE" json:"space_id"`
	Tag       string    `gorm:"size:50;not null" json:"tag"`
	CreatedAt time.Time `json:"created_at"`
}

// REVIEWS (Revisão de anotações/cards)
type Review struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	NoteID     uuid.UUID `gorm:"type:uuid;index;constraint:OnDelete:CASCADE" json:"note_id"` // FK para QuickNote
	ReviewDate time.Time `json:"review_date"`
	Status     string    `gorm:"size:20;not null" json:"status"` // ex: "revisado", "pendente"
	CreatedAt  time.Time `json:"created_at"`
}

// QUIZ / SIMULADO (O cabeçalho da prova)
type Quiz struct {
	ID          uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID     uuid.UUID      `gorm:"type:uuid;index" json:"space_id"`
	Title       string         `gorm:"size:255;not null" json:"title"`
	Description string         `json:"description"`
	Questions   []QuizQuestion `gorm:"foreignKey:QuizID;constraint:OnDelete:CASCADE" json:"questions"`
	CreatedAt   time.Time      `json:"created_at"`
}

// QUIZ QUESTION (As perguntas da prova)
type QuizQuestion struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	QuizID        uuid.UUID `gorm:"type:uuid;index" json:"quiz_id"`
	QuestionText  string    `gorm:"type:text;not null" json:"question_text"`
	QuestionType  string    `gorm:"size:50;not null" json:"question_type"` // "multiple_choice" ou "open_ended"
	Options       string    `gorm:"type:jsonb" json:"options"`             // Array JSON. Ex: '["A) 2", "B) 4"]'
	CorrectAnswer string    `gorm:"type:text" json:"correct_answer"`       // A resposta certa
	Points        int       `gorm:"default:1" json:"points"`               // Quanto vale a questão
}

// QUIZ / SIMULADOS (Para relatórios de notas e gargalos)
type QuizResult struct {
	ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID         uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	SpaceID        uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	Score          float64   `json:"score"`
	TotalQuestions int       `json:"total_questions"`
	Status         string    `gorm:"size:20;default:'completed'" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// LOG DE ATIVIDADE DIÁRIA (Para Mapas de Calor, Horários de Pico e Dispositivos)
type ActivityLog struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Action    string    `gorm:"size:100;not null" json:"action"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// HISTÓRICO DE PAGAMENTOS (Para relatórios Financeiros B2B - MRR, Inadimplência)
type PaymentHistory struct {
	ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID         uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Amount         float64   `json:"amount"`
	Currency       string    `gorm:"size:3;default:'BRL'" json:"currency"`
	Status         string    `gorm:"size:20;not null" json:"status"` // "PAID", "FAILED", "REFUNDED"
	PaymentDate    time.Time `json:"payment_date"`
	SubscriptionID string    `json:"subscription_id"` // ID do gateway (Stripe/MercadoPago)
}

// Sala de Espera (Solicitações de Acesso)
type SpaceJoinRequest struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Status    string    `gorm:"size:20;default:'pending'" json:"status"` // pending, approved, rejected
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// ==========================================================
// REGRA DE GAMIFICAÇÃO (O Motor de XP Dinâmico)
// ==========================================================
type GamificationRule struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ActionName  string    `gorm:"size:100;unique;not null" json:"action_name"` // ex: "complete_pomodoro", "create_summary"
	RewardXP    int       `gorm:"not null;default:0" json:"reward_xp"`         // Quanto XP ganha
	DailyLimit  int       `gorm:"default:0" json:"daily_limit"`                // Limite de vezes por dia (0 = ilimitado)
	Description string    `json:"description"`                                 // Ex: "Completar um Pomodoro de 25 min"
	UpdatedAt   time.Time `json:"updated_at"`
}

// ==========================================================
// 🔔 SISTEMA DE NOTIFICAÇÕES (Mural, Sino e Pop-up)
// ==========================================================
type Notification struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Title     string    `gorm:"size:255;not null" json:"title"`
	Message   string    `gorm:"type:text;not null" json:"message"`
	Type      string    `gorm:"size:50;not null" json:"type"`     // POPUP | BELL | NEWS
	Audience  string    `gorm:"size:50;not null" json:"audience"` // GLOBAL | SPACES | USERS
	TargetIDs string    `gorm:"type:jsonb" json:"target_ids"`     // Array de IDs salvo como JSON (Vazio se for GLOBAL)
	IsActive  bool      `gorm:"default:true" json:"is_active"`    // Botão de pânico: false esconde a notificação na hora

	// 👇 NOVOS CAMPOS DE PRAZO
	StartAt time.Time  `gorm:"default:now()" json:"start_at"`
	EndAt   *time.Time `json:"end_at"` // Opcional: Data para o aviso sumir sozinho

	CreatedByID uuid.UUID `gorm:"type:uuid" json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Tabela leve só para registrar quem clicou em "Lido"
type NotificationRead struct {
	ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	NotificationID uuid.UUID `gorm:"type:uuid;index;not null" json:"notification_id"`
	UserID         uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
	ReadAt         time.Time `gorm:"autoCreateTime" json:"read_at"`
}
