package database

import (
	"log"
	"os"
	"time"

	"studfy-backend/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("Erro: DATABASE_URL não encontrada no arquivo .env")
	}

	// Iniciamos a conexão com o logger apenas para avisos críticos para não poluir o terminal
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		PrepareStmt: false,
		Logger:      logger.Default.LogMode(logger.Warn),
	})

	if err != nil {
		log.Fatal("Falha ao conectar no banco de dados: ", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Falha ao pegar a instância genérica do banco: ", err)
	}

	// 🛡️ BLINDAGEM DO POOL DE CONEXÕES
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	DB = db
	log.Println("✅ Conexão com PostgreSQL estabelecida com sucesso!")

	// ==========================================================
	// 🚀 AUTOMIGRATE
	// Como agora usamos um PostgreSQL próprio (container), o banco começa VAZIO.
	// Rodamos o AutoMigrate para criar/atualizar todas as tabelas a partir dos Models.
	// Controlado pela env AUTO_MIGRATE ("true" para rodar). Depois que o schema já
	// estiver criado você pode setar AUTO_MIGRATE=false para a API ligar mais rápido.
	// ==========================================================
	if os.Getenv("AUTO_MIGRATE") == "true" {
		log.Println("⏳ Rodando AutoMigrate (criando/atualizando tabelas)...")
		if err := runMigrations(db); err != nil {
			log.Fatal("Falha ao rodar as migrations: ", err)
		}
		log.Println("✅ AutoMigrate concluído com sucesso!")
	}
}

// runMigrations cria/atualiza todas as tabelas do domínio a partir dos Models.
func runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.VerificationCode{},
		&models.PasswordReset{},
		&models.AvailabilityProfile{},
		&models.Follower{},
		&models.PaymentHistory{},
		&models.Space{},
		&models.SpacePermission{},
		&models.SpaceJoinRequest{},
		&models.SpaceTag{},
		&models.SpaceQuestion{},
		&models.QuestionGroup{},
		&models.StudfyQuestion{},
		&models.Notebook{},
		&models.NotebookPermission{},
		&models.Guide{},
		&models.Page{},
		&models.PageNote{},
		&models.PageTag{},
		&models.PageDoubt{},
		&models.QuickNote{},
		&models.StudyStrategy{},
		&models.StudyBlock{},
		&models.StudySession{},
		&models.ScheduleLog{},
		&models.ScheduleLogBlock{},
		&models.CycleLog{},
		&models.CycleLogBlock{},
		&models.Review{},
		&models.Quiz{},
		&models.QuizQuestion{},
		&models.QuizResult{},
		&models.Flashcard{},
		&models.FlashcardCategory{},
		&models.FlashcardTag{},
		&models.Certificate{},
		&models.StudentDossier{},
		&models.AttendanceSession{},
		&models.AttendanceRecord{},
		&models.PomodoroSession{},
		&models.MoodCheckIn{},
		&models.ActivityLog{},
		&models.AutomationRule{},
		&models.GamificationRule{},
		&models.Badge{},
		&models.UserBadge{},
		&models.FlashMission{},
		&models.MissionCompletion{},
		&models.ArenaMatch{},
		&models.Notification{},
		&models.NotificationRead{},
		&models.BugReport{},
		&models.HelpCategory{},
		&models.HelpArticle{},
	)
}
