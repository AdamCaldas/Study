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
	// DICA DE SÊNIOR: Deixe esse código rodar UMA VEZ quando você iniciar o servidor.
	// Ele vai limpar a "sujeira" das tabelas antigas do banco de dados.
	// Depois que rodar com sucesso a primeira vez, você pode apagar esta linha se quiser.
	log.Println("Limpando tabelas antigas de ciclo e plano...")
	db.Migrator().DropTable("study_cycles", "study_cycle_items", "study_plans")

	// 👇 COLOQUE A BOMBA AQUI 👇
	log.Println("Destruindo o cadeado do banco para permitir Ciclo + Plano juntos...")
	db.Exec("DROP INDEX IF EXISTS idx_study_strategies_space_id;")

	// --- MIGRATIONS ---
	log.Println("Rodando as Migrations do banco de dados...")
	err = db.AutoMigrate(
		&models.User{},
		&models.Space{},
		&models.SpacePermission{},
		&models.Notebook{},
		&models.Page{},
		&models.PageNote{}, // 📝 NOVO: Notas da Página
		&models.QuickNote{},
		&models.StudyStrategy{}, // 🧠 NOVO: Motor Unificado (A Estratégia Mestra)
		&models.StudyBlock{},    // 🧱 NOVO: Blocos de Estudo (Os horários/roleta)
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
		&models.Guide{},
		&models.BugReport{},
		&models.VerificationCode{},
		&models.PasswordReset{},
		&models.StudySession{},
		&models.AvailabilityProfile{},

		// ==========================================
		// 👇 FASES 1 A 7 👇
		// ==========================================
		&models.Follower{},
		&models.StudentDossier{},
		&models.QuestionBankItem{},
		&models.FlashMission{},
		&models.MissionCompletion{},
		&models.Certificate{},
		&models.PageDoubt{},
		&models.AttendanceSession{},
		&models.AttendanceRecord{},
		&models.Badge{},
		&models.UserBadge{},
		&models.AutomationRule{},
	)
	if err != nil {
		log.Fatal("Falha ao rodar as migrations: ", err)
	}
	log.Println("✅ Tabelas sincronizadas com sucesso!")
}
