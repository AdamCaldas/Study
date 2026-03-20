package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// USER - O Perfil Completo "De Milhões"
type User struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	FullName string    `gorm:"not null" json:"full_name"`

	// 👇 NOVOS CAMPOS DE IDENTIDADE E PERSONALIZAÇÃO
	Nickname         string     `gorm:"size:50" json:"nickname"`                 // Nome fictício
	Bio              string     `gorm:"type:text" json:"bio"`                    // Descrição/Sobre mim
	BirthDate        *time.Time `json:"birth_date"`                              // Data de nascimento
	Gender           string     `gorm:"size:20" json:"gender"`                   // Gênero
	BannerPic        string     `json:"banner_picture_url"`                      // Fundo Retangular (Suporta GIF)
	IsProfilePrivate bool       `gorm:"default:false" json:"is_profile_private"` // Ocultar perfil?

	// CAMPOS ORIGINAIS MANTIDOS
	Email            string         `gorm:"unique;not null" json:"email"`
	CPF              string         `gorm:"unique;not null" json:"cpf"`
	CNPJ             string         `gorm:"unique;default:null" json:"cnpj"`
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

	Title         string `gorm:"size:50" json:"title"`     // ex: "DESENVOLVEDOR"
	Location      string `gorm:"size:100" json:"location"` // ex: "Brasil"
	FollowerCount int64  `gorm:"-" json:"follower_count"`  // 👈 ADICIONE ISSO
}

// SPACE
type Space struct {
	ID              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	OwnerID         uuid.UUID `gorm:"type:uuid;index" json:"owner_id"`
	Name            string    `gorm:"size:100;not null" json:"name"`
	Description     string    `json:"description"`
	ColorHex        string    `gorm:"size:7;default:'#FFFFFF'" json:"color_hex"`
	Category        string    `json:"category"`                       // study | course
	Status          string    `gorm:"default:'active'" json:"status"` // active | inactive
	Slug            string    `gorm:"uniqueIndex" json:"slug"`
	ShareCode       string    `gorm:"uniqueIndex" json:"share_code"`
	Visibility      string    `gorm:"default:'private'" json:"visibility"` // public | private
	IsClassroom     bool      `gorm:"default:false" json:"is_classroom"`
	IsRankingActive bool      `gorm:"default:true" json:"is_ranking_active"`

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

	// 👇 FASE 3: AGENDAMENTO DE CONTEÚDO
	UnlockAt *time.Time `json:"unlock_at"`          // Quando vai ser liberado (Ponteiro permite nulo)
	IsLocked bool       `gorm:"-" json:"is_locked"` // Campo Virtual: O Backend avisa se ainda está trancado

	// 👇 NOVOS RELACIONAMENTOS
	Guides []Guide `gorm:"foreignKey:NotebookID;constraint:OnDelete:CASCADE" json:"guides"`
	Pages  []Page  `gorm:"foreignKey:NotebookID;constraint:OnDelete:CASCADE" json:"pages"` // Páginas soltas (sem guia)

	OwnerName   string `gorm:"-" json:"owner_name"`
	UpdaterName string `gorm:"-" json:"updater_name"` // 👈 ADICIONE ISSO

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

	OwnerName   string `gorm:"-" json:"owner_name"`   // Campo Virtual
	UpdaterName string `gorm:"-" json:"updater_name"` // 👈 ADICIONE ISSO

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

	Tags []PageTag `gorm:"type:jsonb;default:'[]'" json:"tags"`

	OwnerName   string `gorm:"-" json:"owner_name"`
	UpdaterName string `gorm:"-" json:"updater_name"` // 👈 ADICIONE ISSO

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
	// 👇 FASE 3: AGENDAMENTO DE CONTEÚDO
	UnlockAt *time.Time `json:"unlock_at"`
	IsLocked bool       `gorm:"-" json:"is_locked"`
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
	QuizID         uuid.UUID `gorm:"type:uuid;index;not null" json:"quiz_id"` // 👈 ADICIONE ISSO AQUI! (CRÍTICO)
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

// PageTag - Estrutura das tags coloridas das páginas
type PageTag struct {
	Name     string `json:"name"`
	ColorHex string `json:"color_hex"`
}

// ==========================================================
// 🤝 REDE SOCIAL: SISTEMA DE SEGUIDORES
// ==========================================================
type Follower struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	FollowerID  uuid.UUID `gorm:"type:uuid;index;not null" json:"follower_id"`  // Quem apertou o botão de seguir (Aluno)
	FollowingID uuid.UUID `gorm:"type:uuid;index;not null" json:"following_id"` // Quem está sendo seguido (Professor)
	CreatedAt   time.Time `json:"created_at"`
}

// ==========================================================
// 🕵️ DOSSIÊ DO ALUNO (Notas Privadas do Professor)
// ==========================================================
type StudentDossier struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index;not null" json:"space_id"`
	StudentID uuid.UUID `gorm:"type:uuid;index;not null" json:"student_id"`
	TeacherID uuid.UUID `gorm:"type:uuid;index;not null" json:"teacher_id"`
	Content   string    `gorm:"type:text" json:"content"` // O texto que o professor vai digitar
	UpdatedAt time.Time `json:"updated_at"`
}

// ==========================================================
// 🏦 FASE 3: BANCO DE QUESTÕES GLOBAL DO PROFESSOR
// ==========================================================
type QuestionBankItem struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TeacherID     uuid.UUID `gorm:"type:uuid;index;not null" json:"teacher_id"`
	QuestionText  string    `gorm:"type:text;not null" json:"question_text"`
	QuestionType  string    `gorm:"size:50;not null" json:"question_type"` // "multiple_choice" ou "open_ended"
	Options       string    `gorm:"type:jsonb" json:"options"`             // Array JSON. Ex: '["A) 2", "B) 4"]'
	CorrectAnswer string    `gorm:"type:text" json:"correct_answer"`
	Points        int       `gorm:"default:1" json:"points"`
	CreatedAt     time.Time `json:"created_at"`
}

// ==========================================================
// ⚡ FASE 3: MISSÕES RELÂMPAGO (Quests com Timer)
// ==========================================================
type FlashMission struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID     uuid.UUID `gorm:"type:uuid;index;not null" json:"space_id"`
	TeacherID   uuid.UUID `gorm:"type:uuid;not null" json:"teacher_id"`
	Title       string    `gorm:"size:255;not null" json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	RewardXP    int       `gorm:"default:50" json:"reward_xp"` // Quanto XP o aluno ganha
	ExpiresAt   time.Time `json:"expires_at"`                  // O Timer! Quando a missão acaba
	CreatedAt   time.Time `json:"created_at"`
}

// Tabela para evitar que o aluno complete a missão duas vezes
type MissionCompletion struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MissionID   uuid.UUID `gorm:"type:uuid;index;not null" json:"mission_id"`
	UserID      uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
	CompletedAt time.Time `gorm:"autoCreateTime" json:"completed_at"`
}

// ==========================================================
// 🎓 FASE 4: CERTIFICADO DE CONCLUSÃO
// ==========================================================
type Certificate struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID      uuid.UUID `gorm:"type:uuid;index;not null" json:"space_id"`
	UserID       uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
	AverageScore float64   `json:"average_score"` // A nota final do aluno no curso
	IssuedAt     time.Time `gorm:"autoCreateTime" json:"issued_at"`
}

// ==========================================================
// 💬 FASE 5: PLANTÃO DE DÚVIDAS (Fórum da Sala de Aula)
// ==========================================================
type PageDoubt struct {
	ID         uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID    uuid.UUID  `gorm:"type:uuid;index;not null" json:"space_id"`
	PageID     uuid.UUID  `gorm:"type:uuid;index;not null" json:"page_id"`
	StudentID  uuid.UUID  `gorm:"type:uuid;index;not null" json:"student_id"`
	Content    string     `gorm:"type:text;not null" json:"content"` // A pergunta do aluno
	Resolved   bool       `gorm:"default:false" json:"resolved"`     // Status da dúvida
	Answer     string     `gorm:"type:text" json:"answer"`           // A resposta do professor
	AnsweredBy *uuid.UUID `gorm:"type:uuid" json:"answered_by"`      // Quem respondeu (Prof ou Monitor)
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// ==========================================================
// 📷 FASE 5: CHECK-IN POR QR CODE (Lista de Presença)
// ==========================================================
// A Sessão gerada pelo Professor no telão
type AttendanceSession struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index;not null" json:"space_id"`
	TeacherID uuid.UUID `gorm:"type:uuid;not null" json:"teacher_id"`
	QRCode    string    `gorm:"size:255;uniqueIndex;not null" json:"qr_code"` // O Token único que vai virar o desenho do QR
	ExpiresAt time.Time `json:"expires_at"`                                   // Validade (ex: 15 minutos)
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// O registro de cada aluno que escaneou
type AttendanceRecord struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SessionID uuid.UUID `gorm:"type:uuid;index;not null" json:"session_id"`
	StudentID uuid.UUID `gorm:"type:uuid;index;not null" json:"student_id"`
	CheckInAt time.Time `gorm:"autoCreateTime" json:"check_in_at"`
}

// ==========================================================
// 🏆 FASE 6: EMBLEMAS CUSTOMIZADOS (Badges)
// ==========================================================

// O Emblema criado pelo Professor
type Badge struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID     uuid.UUID `gorm:"type:uuid;index;not null" json:"space_id"`
	TeacherID   uuid.UUID `gorm:"type:uuid;not null" json:"teacher_id"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	IconURL     string    `gorm:"type:text;not null" json:"icon_url"` // Link da imagem ou GIF
	CreatedAt   time.Time `json:"created_at"`
}

// A carteira de emblemas do Aluno
type UserBadge struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"` // Aluno que ganhou
	BadgeID   uuid.UUID `gorm:"type:uuid;index;not null" json:"badge_id"`
	AwardedBy uuid.UUID `gorm:"type:uuid;not null" json:"awarded_by"` // Professor que entregou
	AwardedAt time.Time `gorm:"autoCreateTime" json:"awarded_at"`

	// Preload: Traz os dados do emblema quando formos mostrar o Perfil do aluno
	Badge Badge `gorm:"foreignKey:BadgeID" json:"badge"`
}

// ==========================================================
// 🤖 FASE 7: GATILHOS DE AUTOMAÇÃO (Analytics B2B)
// ==========================================================
type AutomationRule struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index;not null" json:"space_id"`
	TeacherID uuid.UUID `gorm:"type:uuid;not null" json:"teacher_id"`

	// A Condição (O "SE")
	ConditionType  string  `gorm:"size:50;not null" json:"condition_type"` // Ex: "score_below"
	ConditionValue float64 `json:"condition_value"`                        // Ex: 6.0

	// A Ação (O "ENTÃO")
	ActionType      string    `gorm:"size:50;not null" json:"action_type"`         // Ex: "unlock_notebook"
	TargetContentID uuid.UUID `gorm:"type:uuid;not null" json:"target_content_id"` // O ID do Caderno Extra

	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}
