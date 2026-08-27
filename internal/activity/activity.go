package activity

import (
	"log"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/google/uuid"
)

// ==========================================================
// 📜 HISTÓRICO DA TURMA
// ==========================================================
// A tela de histórico do professor existia, mas a única coisa que escrevia em
// `activity_logs` era o alerta anti-cola — então a tela vivia praticamente
// vazia. Log() é o ponto único por onde as ações relevantes passam a ser
// registradas.
//
// Nunca derruba a requisição: histórico é informativo, se falhar a gravação a
// ação principal do usuário continua valendo. Por isso o erro só vai para o log
// do servidor.
func Log(spaceID, userID uuid.UUID, action string) {
	if spaceID == uuid.Nil || userID == uuid.Nil || action == "" {
		return
	}

	entry := models.ActivityLog{
		SpaceID: spaceID,
		UserID:  userID,
		Action:  action,
	}

	if err := database.DB.Create(&entry).Error; err != nil {
		log.Printf("histórico: falha ao registrar \"%s\" na turma %s: %v", action, spaceID, err)
	}
}
