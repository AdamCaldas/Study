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

	Nickname         string     `gorm:"size:50" json:"nickname"`
	Bio              string     `gorm:"type:text" json:"bio"`
	BirthDate        *time.Time `json:"birth_date"`
	Gender           string     `gorm:"size:20" json:"gender"`
	BannerPic        string     `json:"banner_picture_url"`
	IsProfilePrivate bool       `gorm:"default:false" json:"is_profile_private"`

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

	IsEmailVerified bool `gorm:"default:false" json:"is_email_verified"`

	Title         string `gorm:"size:50" json:"title"`
	Location      string `gorm:"size:100" json:"location"`
	FollowerCount int64  `gorm:"-" json:"follower_count"`

	Theme             string `gorm:"size:20;default:'dark'" json:"theme"`
	PushNotifications bool   `gorm:"default:true" json:"push_notifications"`
}

// SPACE
type Space struct {
	ID              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	OwnerID         uuid.UUID `gorm:"type:uuid;index" json:"owner_id"`
	Name            string    `gorm:"size:100;not null" json:"name"`
	Description     string    `json:"description"`
	ColorHex        string    `gorm:"size:7;default:'#FFFFFF'" json:"color_hex"`
	Category        string    `json:"category"`
	Status          string    `gorm:"default:'active'" json:"status"`
	Slug            string    `gorm:"uniqueIndex" json:"slug"`
	ShareCode       string    `gorm:"uniqueIndex" json:"share_code"`
	Visibility      string    `gorm:"default:'private'" json:"visibility"`
	IsClassroom     bool      `gorm:"default:false" json:"is_classroom"`
	IsRankingActive bool      `gorm:"default:true" json:"is_ranking_active"`

	AllowCollaborators bool `gorm:"default:true" json:"allow_collaborators"`
	AllowComments      bool `gorm:"default:true" json:"allow_comments"`
	IsArchived         bool `gorm:"default:false" json:"is_archived"`
	IsShared           bool `gorm:"default:false" json:"is_shared"`
	MaxCollaborators   int  `gorm:"default:10" json:"max_collaborators"`
	ViewCount          int  `json:"view_count" gorm:"default:0"`

	Notebooks []Notebook `gorm:"foreignKey:SpaceID;constraint:OnDelete:CASCADE" json:"notebooks"`

	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastActivity time.Time `json:"last_activity"`
}

// SPACE PERMISSIONS
type SpacePermission struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"space_id"`
	UserID      uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	AccessLevel string    `gorm:"type:varchar(20);not null" json:"access_level"`
	JoinedAt    time.Time `gorm:"autoCreateTime" json:"joined_at"`

	CanEditSpaceInfo  bool `gorm:"default:false" json:"can_edit_space_info"`
	CanEditSpaceColor bool `gorm:"default:false" json:"can_edit_space_color"`

	CanCreateContent bool `gorm:"default:false" json:"can_create_content"`
	CanEditContent   bool `gorm:"default:false" json:"can_edit_content"`
	CanDeleteContent bool `gorm:"default:false" json:"can_delete_content"`
	CanManageTags    bool `gorm:"default:false" json:"can_manage_tags"`

	CanManageMembers  bool `gorm:"default:false" json:"can_manage_members"`
	CanSendInvites    bool `gorm:"default:false" json:"can_send_invites"`
	CanSearchContent  bool `gorm:"default:true"  json:"can_search_content"`
	CanChangeSettings bool `gorm:"default:false" json:"can_change_settings"`

	// 🚨 UNIFICADO: CanManageCycles foi removido. CanManagePlans agora governa toda a estratégia.
	CanManagePlans   bool `gorm:"default:false" json:"can_manage_plans"`
	CanManageQuizzes bool `gorm:"default:false" json:"can_manage_quizzes"`
}

// PERMISSÃO GRANULAR NOTEBOOK
type NotebookPermission struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	NotebookID  uuid.UUID `gorm:"type:uuid;index" json:"notebook_id"`
	UserID      uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	AccessLevel string    `gorm:"type:varchar(20);not null" json:"access_level"`
}

// NOTEBOOK
type Notebook struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID  uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	Name     string    `gorm:"size:100;not null" json:"name"`
	ColorHex string    `gorm:"size:7" json:"color_hex"`

	UnlockAt *time.Time `json:"unlock_at"`
	IsLocked bool       `gorm:"-" json:"is_locked"`

	Guides []Guide `gorm:"foreignKey:NotebookID;constraint:OnDelete:CASCADE" json:"guides"`
	Pages  []Page  `gorm:"foreignKey:NotebookID;constraint:OnDelete:CASCADE" json:"pages"`

	OwnerName   string `gorm:"-" json:"owner_name"`
	UpdaterName string `gorm:"-" json:"updater_name"`

	CreatedByID uuid.UUID `gorm:"type:uuid" json:"created_by_id"`
	UpdatedByID uuid.UUID `gorm:"type:uuid" json:"updated_by_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GUIDE
type Guide struct {
	ID            uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	NotebookID    uuid.UUID  `gorm:"type:uuid;index;not null" json:"notebook_id"`
	ParentGuideID *uuid.UUID `gorm:"type:uuid;index" json:"parent_guide_id"`

	Name        string `gorm:"size:255;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	Icon        string `gorm:"size:50" json:"icon"`
	ColorHex    string `gorm:"size:7" json:"color_hex"`
	Order       int    `json:"order"`

	OwnerName   string `gorm:"-" json:"owner_name"`
	UpdaterName string `gorm:"-" json:"updater_name"`

	Pages     []Page  `gorm:"foreignKey:GuideID;constraint:OnDelete:CASCADE" json:"pages"`
	SubGuides []Guide `gorm:"foreignKey:ParentGuideID;constraint:OnDelete:CASCADE" json:"sub_guides"`

	CreatedByID uuid.UUID `gorm:"type:uuid" json:"created_by_id"`
	UpdatedByID uuid.UUID `gorm:"type:uuid" json:"updated_by_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PAGE
type Page struct {
	ID         uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	NotebookID uuid.UUID  `gorm:"type:uuid;index;not null" json:"notebook_id"`
	GuideID    *uuid.UUID `gorm:"type:uuid;index" json:"guide_id"`

	Title   string `gorm:"size:255;not null" json:"title"`
	Content string `gorm:"type:jsonb" json:"content"`
	Order   int    `json:"order"`

	Tags []PageTag `gorm:"type:jsonb;default:'[]'" json:"tags"`

	// 👇 NOVO: Relacionamento com as Notas Privadas do Aluno
	Notes []PageNote `gorm:"foreignKey:PageID;constraint:OnDelete:CASCADE" json:"notes"`

	OwnerName   string `gorm:"-" json:"owner_name"`
	UpdaterName string `gorm:"-" json:"updater_name"`

	CreatedByID uuid.UUID `gorm:"type:uuid" json:"created_by_id"`
	UpdatedByID uuid.UUID `gorm:"type:uuid" json:"updated_by_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ==========================================================
// 📝 NOVO: NOTAS PRIVADAS DA PÁGINA (Anotações do Aluno)
// ==========================================================
type PageNote struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	PageID    uuid.UUID `gorm:"type:uuid;index;not null" json:"page_id"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"` // O dono da anotação
	Content   string    `gorm:"type:text;not null" json:"content"`       // O texto da anotação
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// ==========================================================
// 🧠 NOVO MOTOR UNIFICADO DE ESTUDOS (Substitui Cycle e Plan)
// ==========================================================

// A Configuração Geral (A Estratégia) - Regra: 1 por Space
type StudyStrategy struct {
	ID         uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID    uuid.UUID  `gorm:"type:uuid;uniqueIndex;not null" json:"space_id"` // 1 Space = 1 Estratégia
	Mode       string     `gorm:"size:20;default:'fixed'" json:"mode"`            // 'fixed' (Cronograma) ou 'adaptive' (Ciclo)
	TargetGoal string     `gorm:"size:255" json:"target_goal"`
	TargetDate *time.Time `json:"target_date"`

	// 👇 A SOLUÇÃO DO FURO: A rotina diária flexível (Array de dias) salva como JSON no banco
	DailyAvailability string `gorm:"type:jsonb" json:"daily_availability"`

	// Configurações Globais mantidas
	HoursPerDay        float64 `json:"hours_per_day"` // Ainda útil como meta diária média
	MinSessionMin      int     `gorm:"default:30" json:"min_session_minutes"`
	MaxSessionMin      int     `gorm:"default:90" json:"max_session_minutes"`
	FreeTimePreference int     `gorm:"default:60" json:"free_time_preference"`

	CurrentStep int `gorm:"default:0" json:"current_step"` // Para o modo adaptive saber onde parou

	// A Lista de Matérias/Horários
	Blocks []StudyBlock `gorm:"foreignKey:StrategyID;constraint:OnDelete:CASCADE" json:"blocks"`

	CreatedByID uuid.UUID `gorm:"type:uuid" json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Os Blocos de Estudo (Servem para Fixed ou Adaptive)
type StudyBlock struct {
	ID         uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	StrategyID uuid.UUID  `gorm:"type:uuid;index;not null" json:"strategy_id"`
	NotebookID *uuid.UUID `gorm:"type:uuid" json:"notebook_id"` // Pode ser null se for texto livre

	Activity string `gorm:"size:255;not null" json:"activity"`
	ColorHex string `gorm:"size:7;default:'#3B82F6'" json:"color_hex"`

	// 👉 CAMPOS MODO ADAPTIVE (Ciclo)
	Sequence         int `json:"sequence"`
	Importance       int `gorm:"type:int2;default:3" json:"importance"`
	Performance      int `gorm:"type:int2;default:3" json:"performance"`
	SuggestedMinutes int `json:"suggested_minutes"`

	// 👉 CAMPOS MODO FIXED (Cronograma)
	// São ponteiros (*int, *string) para poderem ser null quando estiver no modo Ciclo
	DayOfWeek *int    `json:"day_of_week"`
	StartTime *string `gorm:"size:5" json:"start_time"` // "08:00"
	EndTime   *string `gorm:"size:5" json:"end_time"`   // "09:00"
}

// ==========================================================
// RESTO DOS MODELOS GERAIS MANTIDOS
// ==========================================================

type QuickNote struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	Title     string    `gorm:"size:255" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	Color     string    `gorm:"size:7" json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

type PomodoroSession struct {
	ID         uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID     uuid.UUID  `gorm:"type:uuid;index" json:"user_id"`
	Duration   int        `json:"duration_minutes"`
	CreatedAt  time.Time  `json:"created_at"`
	SpaceID    *uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	NotebookID *uuid.UUID `gorm:"type:uuid;index" json:"notebook_id"`
}

type MoodCheckIn struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Mood      string    `gorm:"size:50;not null" json:"mood"`
	CreatedAt time.Time `json:"created_at"`
}

type SpaceTag struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index;constraint:OnDelete:CASCADE" json:"space_id"`
	Tag       string    `gorm:"size:50;not null" json:"tag"`
	CreatedAt time.Time `json:"created_at"`
}

type Review struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	NoteID     uuid.UUID `gorm:"type:uuid;index;constraint:OnDelete:CASCADE" json:"note_id"`
	ReviewDate time.Time `json:"review_date"`
	Status     string    `gorm:"size:20;not null" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type Quiz struct {
	ID          uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID     uuid.UUID      `gorm:"type:uuid;index" json:"space_id"`
	Title       string         `gorm:"size:255;not null" json:"title"`
	Description string         `json:"description"`
	Questions   []QuizQuestion `gorm:"foreignKey:QuizID;constraint:OnDelete:CASCADE" json:"questions"`
	CreatedAt   time.Time      `json:"created_at"`
	UnlockAt    *time.Time     `json:"unlock_at"`
	IsLocked    bool           `gorm:"-" json:"is_locked"`
}

type QuizQuestion struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	QuizID        uuid.UUID `gorm:"type:uuid;index" json:"quiz_id"`
	QuestionText  string    `gorm:"type:text;not null" json:"question_text"`
	QuestionType  string    `gorm:"size:50;not null" json:"question_type"`
	Options       string    `gorm:"type:jsonb" json:"options"`
	CorrectAnswer string    `gorm:"type:text" json:"correct_answer"`
	Points        int       `gorm:"default:1" json:"points"`
}

type QuizResult struct {
	ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	QuizID         uuid.UUID `gorm:"type:uuid;index;not null" json:"quiz_id"`
	UserID         uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	SpaceID        uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	Score          float64   `json:"score"`
	TotalQuestions int       `json:"total_questions"`
	Status         string    `gorm:"size:20;default:'completed'" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type ActivityLog struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Action    string    `gorm:"size:100;not null" json:"action"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

type PaymentHistory struct {
	ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID         uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Amount         float64   `json:"amount"`
	Currency       string    `gorm:"size:3;default:'BRL'" json:"currency"`
	Status         string    `gorm:"size:20;not null" json:"status"`
	PaymentDate    time.Time `json:"payment_date"`
	SubscriptionID string    `json:"subscription_id"`
}

type SpaceJoinRequest struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Status    string    `gorm:"size:20;default:'pending'" json:"status"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type GamificationRule struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ActionName  string    `gorm:"size:100;unique;not null" json:"action_name"`
	RewardXP    int       `gorm:"not null;default:0" json:"reward_xp"`
	DailyLimit  int       `gorm:"default:0" json:"daily_limit"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Notification struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Title     string    `gorm:"size:255;not null" json:"title"`
	Message   string    `gorm:"type:text;not null" json:"message"`
	Type      string    `gorm:"size:50;not null" json:"type"`
	Audience  string    `gorm:"size:50;not null" json:"audience"`
	TargetIDs string    `gorm:"type:jsonb" json:"target_ids"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`

	StartAt time.Time  `gorm:"default:now()" json:"start_at"`
	EndAt   *time.Time `json:"end_at"`

	CreatedByID uuid.UUID `gorm:"type:uuid" json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type NotificationRead struct {
	ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	NotificationID uuid.UUID `gorm:"type:uuid;index;not null" json:"notification_id"`
	UserID         uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
	ReadAt         time.Time `gorm:"autoCreateTime" json:"read_at"`
}

type PageTag struct {
	Name     string `json:"name"`
	ColorHex string `json:"color_hex"`
}

type Follower struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	FollowerID  uuid.UUID `gorm:"type:uuid;index;not null" json:"follower_id"`
	FollowingID uuid.UUID `gorm:"type:uuid;index;not null" json:"following_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type StudentDossier struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index;not null" json:"space_id"`
	StudentID uuid.UUID `gorm:"type:uuid;index;not null" json:"student_id"`
	TeacherID uuid.UUID `gorm:"type:uuid;index;not null" json:"teacher_id"`
	Content   string    `gorm:"type:text" json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

type QuestionBankItem struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TeacherID     uuid.UUID `gorm:"type:uuid;index;not null" json:"teacher_id"`
	QuestionText  string    `gorm:"type:text;not null" json:"question_text"`
	QuestionType  string    `gorm:"size:50;not null" json:"question_type"`
	Options       string    `gorm:"type:jsonb" json:"options"`
	CorrectAnswer string    `gorm:"type:text" json:"correct_answer"`
	Points        int       `gorm:"default:1" json:"points"`
	CreatedAt     time.Time `json:"created_at"`
}

type FlashMission struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID     uuid.UUID `gorm:"type:uuid;index;not null" json:"space_id"`
	TeacherID   uuid.UUID `gorm:"type:uuid;not null" json:"teacher_id"`
	Title       string    `gorm:"size:255;not null" json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	RewardXP    int       `gorm:"default:50" json:"reward_xp"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type MissionCompletion struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MissionID   uuid.UUID `gorm:"type:uuid;index;not null" json:"mission_id"`
	UserID      uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
	CompletedAt time.Time `gorm:"autoCreateTime" json:"completed_at"`
}

type Certificate struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID      uuid.UUID `gorm:"type:uuid;index;not null" json:"space_id"`
	UserID       uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
	AverageScore float64   `json:"average_score"`
	IssuedAt     time.Time `gorm:"autoCreateTime" json:"issued_at"`
}

type PageDoubt struct {
	ID         uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID    uuid.UUID  `gorm:"type:uuid;index;not null" json:"space_id"`
	PageID     uuid.UUID  `gorm:"type:uuid;index;not null" json:"page_id"`
	StudentID  uuid.UUID  `gorm:"type:uuid;index;not null" json:"student_id"`
	Content    string     `gorm:"type:text;not null" json:"content"`
	Resolved   bool       `gorm:"default:false" json:"resolved"`
	Answer     string     `gorm:"type:text" json:"answer"`
	AnsweredBy *uuid.UUID `gorm:"type:uuid" json:"answered_by"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type AttendanceSession struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index;not null" json:"space_id"`
	TeacherID uuid.UUID `gorm:"type:uuid;not null" json:"teacher_id"`
	QRCode    string    `gorm:"size:255;uniqueIndex;not null" json:"qr_code"`
	ExpiresAt time.Time `json:"expires_at"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type AttendanceRecord struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SessionID uuid.UUID `gorm:"type:uuid;index;not null" json:"session_id"`
	StudentID uuid.UUID `gorm:"type:uuid;index;not null" json:"student_id"`
	CheckInAt time.Time `gorm:"autoCreateTime" json:"check_in_at"`
}

type Badge struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID     uuid.UUID `gorm:"type:uuid;index;not null" json:"space_id"`
	TeacherID   uuid.UUID `gorm:"type:uuid;not null" json:"teacher_id"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	IconURL     string    `gorm:"type:text;not null" json:"icon_url"`
	CreatedAt   time.Time `json:"created_at"`
}

type UserBadge struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
	BadgeID   uuid.UUID `gorm:"type:uuid;index;not null" json:"badge_id"`
	AwardedBy uuid.UUID `gorm:"type:uuid;not null" json:"awarded_by"`
	AwardedAt time.Time `gorm:"autoCreateTime" json:"awarded_at"`

	Badge Badge `gorm:"foreignKey:BadgeID" json:"badge"`
}

type AutomationRule struct {
	ID              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID         uuid.UUID `gorm:"type:uuid;index;not null" json:"space_id"`
	TeacherID       uuid.UUID `gorm:"type:uuid;not null" json:"teacher_id"`
	ConditionType   string    `gorm:"size:50;not null" json:"condition_type"`
	ConditionValue  float64   `json:"condition_value"`
	ActionType      string    `gorm:"size:50;not null" json:"action_type"`
	TargetContentID uuid.UUID `gorm:"type:uuid;not null" json:"target_content_id"`
	IsActive        bool      `gorm:"default:true" json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
}

type BugReport struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ReporterID  uuid.UUID `gorm:"type:uuid;not null" json:"reporter_id"`
	Title       string    `gorm:"size:255;not null" json:"title"`
	Description string    `gorm:"type:text;not null" json:"description"`
	Status      string    `gorm:"size:50;default:'UNREAD'" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Reporter User `gorm:"foreignKey:ReporterID" json:"reporter"`
}

type VerificationCode struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Email     string    `gorm:"not null;index"`
	Code      string    `gorm:"size:6;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
}

type PasswordReset struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Email     string    `gorm:"not null;index"`
	Token     string    `gorm:"not null;uniqueIndex"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
}

type StudySession struct {
	ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	SpaceID        uuid.UUID `json:"space_id"`
	ActivityName   string    `json:"activity_name"`
	PlannedMinutes int       `json:"planned_minutes"` // Os 60 min que o Back sugeriu
	ActualMinutes  int       `json:"actual_duration"` // Os 180 min que ele fez
	CreatedAt      time.Time `json:"created_at"`
}
