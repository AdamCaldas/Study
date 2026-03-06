package database

import (
	"log"
	"os"
	"time"

	"studfy-backend/internal/models" // Importa nossas Structs

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
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatal("Falha ao conectar no banco de dados: ", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Falha ao pegar a instância genérica do banco: ", err)
	}

	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	DB = db
	log.Println("✅ Conexão com PostgreSQL estabelecida com sucesso!")

	// --- A MÁGICA ACONTECE AQUI ---
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
	)
	if err != nil {
		log.Fatal("Falha ao rodar as migrations: ", err)
	}
	log.Println("✅ Tabelas criadas/atualizadas com sucesso!")
}
