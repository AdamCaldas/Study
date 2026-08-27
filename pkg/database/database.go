package database

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"studfy-backend/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// withStatementTimeout injeta o statement_timeout do Postgres na string de
// conexão, respeitando o formato usado (URL ou "chave=valor").
func withStatementTimeout(dsn string, ms int) string {
	if ms <= 0 || strings.Contains(dsn, "statement_timeout") {
		return dsn
	}

	// Formato URL: postgres://user:pass@host/db?sslmode=disable
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		return fmt.Sprintf("%s%soptions=-c%%20statement_timeout%%3D%d", dsn, sep, ms)
	}

	// Formato chave=valor: host=... user=... dbname=...
	return fmt.Sprintf("%s options='-c statement_timeout=%d'", dsn, ms)
}

// envInt lê um inteiro do ambiente, com padrão seguro.
func envInt(key string, def int) int {
	if s := os.Getenv(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return def
}

var DB *gorm.DB

func ConnectDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("Erro: DATABASE_URL não encontrada no arquivo .env")
	}

	// ⏱️ PRAZO MÁXIMO POR CONSULTA (protege o pool inteiro)
	// Sem isto, UMA consulta lenta segura a conexão indefinidamente; num pico,
	// poucas travadas consomem o pool e todos os alunos recebem erro.
	// O próprio Postgres derruba a consulta passando do limite.
	dsn = withStatementTimeout(dsn, envInt("DB_STATEMENT_TIMEOUT_MS", 15000))

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

	// 🛡️ BLINDAGEM DO POOL DE CONEXÕES (configurável por env)
	// Padrões pensados para 1000 alunos em UMA instância. Se rodar várias
	// réplicas, lembre que cada uma abre até DB_MAX_OPEN_CONNS conexões —
	// some tudo e mantenha abaixo do max_connections do Postgres.
	maxOpen := envInt("DB_MAX_OPEN_CONNS", 50)
	maxIdle := envInt("DB_MAX_IDLE_CONNS", 15)
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute) // devolve conexões ociosas
	log.Printf("Pool do banco: maxOpen=%d maxIdle=%d", maxOpen, maxIdle)

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
