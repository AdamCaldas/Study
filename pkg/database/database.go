package database

import (
	"log"
	"os"
	"time"

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

	// 🛡️ BLINDAGEM DO POOL DE CONEXÕES (Essencial para não derrubar o Supabase)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	DB = db
	log.Println("✅ Conexão com PostgreSQL estabelecida com sucesso! (Pooler Ativado)")

	// ==========================================================
	// 🚀 FASE 4: AUTOMIGRATE DESATIVADO PARA PERFORMANCE MAXIMA!
	// O Supabase já tem as tabelas. Desligar isto faz a API ligar 10x mais rápido.
	// Se criar um Model novo no futuro, descomente a linha abaixo, rode uma vez, e comente de novo.
	// ==========================================================

	/*
		err = db.AutoMigrate(
			&models.User{},
			// ... adicione os models aqui se precisar migrar algo novo
		)
		if err != nil {
			log.Fatal("Falha ao rodar as migrations: ", err)
		}
	*/
}
