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

	// Configurações de performance da fila de conexões
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	DB = db
	log.Println("✅ Conexão com PostgreSQL (Pooler) estabelecida com sucesso!")

	// --- MIGRATIONS ---
	log.Println("Rodando as Migrations do banco de dados...")
	err = db.AutoMigrate(
		&models.User{},
		&models.Space{},
		&models.SpacePermission{},
		&models.Notebook{},
		&models.Page{},
		&models.QuickNote{},
		&models.StudyCycle{},
		&models.StudyCycleItem{},
		&models.StudyPlan{},
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
		&models.Notification{},       // 👈 ADICIONE ESSA!
		&models.NotificationRead{},   // 👈 E ADICIONE ESSA!
		&models.NotebookPermission{}, // 👈 Cria a tabela de permissões de caderno
		&models.GamificationRule{},   // 👈 Cria a tabela de regras de XP
	)
	if err != nil {
		log.Fatal("Falha ao rodar as migrations: ", err)
	}
	log.Println("✅ Tabelas sincronizadas com sucesso!")
}
