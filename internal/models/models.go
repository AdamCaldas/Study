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

	// Relacionamentos em Cascata
	Notebooks []Notebook `gorm:"foreignKey:SpaceID;constraint:OnDelete:CASCADE" json:"notebooks"`

	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastActivity time.Time `json:"last_activity"`
}

// SPACE PERMISSIONS (Amigos)
type SpacePermission struct {
	SpaceID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"space_id"`
	UserID      uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	AccessLevel string    `gorm:"type:varchar(20);not null" json:"access_level"` // VIEWER, EDITOR
}

// NOTEBOOK
type Notebook struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID  uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	Name     string    `gorm:"size:100;not null" json:"name"`
	ColorHex string    `gorm:"size:7" json:"color_hex"`
	Pages    []Page    `gorm:"foreignKey:NotebookID;constraint:OnDelete:CASCADE" json:"pages"`
}

// PAGE (O Arquivo de Texto JSONB)
type Page struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	NotebookID uuid.UUID `gorm:"type:uuid;index" json:"notebook_id"`
	Title      string    `gorm:"size:255;not null" json:"title"`
	Content    string    `gorm:"type:jsonb" json:"content"`
	Order      int       `json:"order"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// QUICK NOTE
type QuickNote struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID   uuid.UUID `gorm:"type:uuid;index" json:"space_id"`
	Title     string    `gorm:"size:255" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	Color     string    `gorm:"size:7" json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

// STUDY CYCLE
type StudyCycle struct {
	ID          uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SpaceID     uuid.UUID        `gorm:"type:uuid;index" json:"space_id"`
	CurrentStep int              `gorm:"default:0" json:"current_step"`
	Items       []StudyCycleItem `gorm:"foreignKey:CycleID;constraint:OnDelete:CASCADE" json:"items"`
}

type StudyCycleItem struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CycleID     uuid.UUID `gorm:"type:uuid;index" json:"cycle_id"`
	NotebookID  uuid.UUID `gorm:"type:uuid" json:"notebook_id"`
	Sequence    int       `json:"sequence"`
	DurationMin int       `json:"duration_min"`
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
}

// POMODORO SESSION
type PomodoroSession struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Duration  int       `json:"duration_minutes"`
	CreatedAt time.Time `json:"created_at"`
}

// MOOD CHECK-IN (Humor)
type MoodCheckIn struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Mood      string    `gorm:"size:50;not null" json:"mood"` // Ex: "Focado", "Cansado", "Animado"
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
