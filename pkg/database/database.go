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

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		PrepareStmt: false,
		Logger:      logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatal("Falha ao conectar no banco de dados: ", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Falha ao pegar a instância genérica do banco: ", err)
	}

	// 🛡️ BLINDAGEM DO POOL DE CONEXÕES (Turbo no Banco)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	DB = db
	log.Println("✅ Conexão com PostgreSQL (Pooler) estabelecida com sucesso!")

	// ==========================================================
	// 🧹 LIXEIRA TEMPORÁRIA (Limpeza da Unificação)
	// ==========================================================
	log.Println("Limpando tabelas antigas de ciclo e plano...")
	db.Migrator().DropTable("study_cycles", "study_cycle_items", "study_plans")

	log.Println("Destruindo o cadeado do banco para permitir Ciclo + Plano juntos...")
	db.Exec("DROP INDEX IF EXISTS idx_study_strategies_space_id;")

	// ==========================================================
	// 🚀 RODANDO AS MIGRATIONS (Sincronizando Tabelas)
	// ==========================================================
	log.Println("Rodando as Migrations do banco de dados...")
	err = db.AutoMigrate(
		&models.User{},
		&models.Space{},
		&models.SpacePermission{},
		&models.Notebook{},
		&models.Guide{}, // 👈 Essencial para Paginação Dinâmica
		&models.Page{},
		&models.PageNote{},
		&models.QuickNote{},
		&models.StudyStrategy{},
		&models.StudyBlock{},
		&models.PomodoroSession{},
		&models.MoodCheckIn{},
		&models.SpaceTag{},
		&models.Review{},
		&models.QuizResult{},
		&models.ActivityLog{},
		&models.PaymentHistory{},
		&models.Quiz{},
		&models.QuizQuestion{},
		&models.SpaceJoinRequest{},
		&models.Notification{},
		&models.NotificationRead{},
		&models.NotebookPermission{},
		&models.GamificationRule{},
		&models.BugReport{},
		&models.VerificationCode{},
		&models.PasswordReset{},
		&models.StudySession{},
		&models.AvailabilityProfile{},

		// 📊 TABELAS DE LOGS (Execução Diária)
		&models.CycleLog{},
		&models.CycleLogBlock{},
		&models.ScheduleLog{},
		&models.ScheduleLogBlock{},

		// 📚 BANCO DE QUESTÕES E EDITAIS
		&models.StudfyQuestion{},
		&models.SpaceQuestion{},
		&models.QuestionGroup{}, // 👈 NOVO: As pastas dos Editais!

		// 🃏 FLASHCARDS E GAMIFICAÇÃO
		&models.Flashcard{},  // 👈 AQUI ESTÁ A SOLUÇÃO DO ERRO 500!
		&models.ArenaMatch{}, // 👈 ⚔️ Arena 1x1
		&models.Follower{},
		&models.Badge{},
		&models.UserBadge{},
		&models.FlashMission{},
		&models.MissionCompletion{},

		// 🏫 GESTÃO ACADÊMICA
		&models.StudentDossier{},
		&models.Certificate{},
		&models.PageDoubt{},
		&models.AttendanceSession{},
		&models.AttendanceRecord{},
		&models.AutomationRule{},

		// 💡 CENTRAL DE AJUDA (Academy)
		&models.HelpCategory{},
		&models.HelpArticle{},

		// ==========================================================
		// 🏷️ FILTROS DOS FLASHCARDS (Pro Front-end novo)
		// ==========================================================
		&models.FlashcardCategory{},
		&models.FlashcardTag{},
	)

	if err != nil {
		log.Fatal("Falha ao rodar as migrations: ", err)
	}
	log.Println("✅ Todas as tabelas (Fases 1 a 7 + Extras) sincronizadas com sucesso!")
}s