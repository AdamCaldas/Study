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
	// Máximo de conexões abertas ao mesmo tempo (50 para o Supabase free)
	sqlDB.SetMaxOpenConns(50)
	// Máximo de conexões "dormindo" guardadas na memória para respostas super rápidas
	sqlDB.SetMaxIdleConns(10)
	// Tempo máximo que uma conexão pode ficar aberta antes de ser reciclada
	sqlDB.SetConnMaxLifetime(time.Hour)

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
		&models.Notification{},       // Tabela de Notificações Gerais
		&models.NotificationRead{},   // Controle de Lidos do Sino
		&models.NotebookPermission{}, // Permissões de caderno
		&models.GamificationRule{},   // Regras de XP
		&models.Guide{},              // Guias dos Cadernos
		&models.BugReport{},          // 🐛 Sistema de Feedback e Bugs
		&models.BugReport{},          // 🐛 Sistema de Feedback e Bugs
		&models.VerificationCode{},   // 🔐 Tabela de Códigos de E-mail (NOVO)

		// ==========================================
		// 👇 NOVAS TABELAS DAS FASES 1 A 7 👇
		// ==========================================
		&models.Follower{},          // Fase 1: Seguidores do Professor
		&models.StudentDossier{},    // Fase 2: Notas Ocultas do Dossiê
		&models.QuestionBankItem{},  // Fase 3: Banco de Questões Global
		&models.FlashMission{},      // Fase 3: Missões Relâmpago
		&models.MissionCompletion{}, // Fase 3: Controle de quem completou a missão
		&models.Certificate{},       // Fase 4: Certificado de Conclusão (Diploma)
		&models.PageDoubt{},         // Fase 5: Fórum e Plantão de Dúvidas
		&models.AttendanceSession{}, // Fase 5: QR Code (Sessão aberta no Telão)
		&models.AttendanceRecord{},  // Fase 5: QR Code (Lista de Presença do Aluno)
		&models.Badge{},             // Fase 6: Emblemas (Badges) criados pelo prof
		&models.UserBadge{},         // Fase 6: Emblemas ganhos pelo aluno
		&models.AutomationRule{},    // Fase 7: Regras do Robô de Recuperação
	)
	if err != nil {
		log.Fatal("Falha ao rodar as migrations: ", err)
	}
	log.Println("✅ Tabelas sincronizadas com sucesso!")
}
