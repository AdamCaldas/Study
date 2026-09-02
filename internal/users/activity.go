package users

import (
	"log"
	"time"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/cache"
	"studfy-backend/pkg/database"
	"studfy-backend/pkg/utils"

	"github.com/google/uuid"
)

// ==========================================================
// 🔥 PRESENÇA DIÁRIA E OFENSIVA (STREAK)
// ==========================================================
// Antes, `last_login_at`, `current_streak` e `highest_streak` NUNCA eram
// escritos: a ofensiva do aluno ficava travada em zero e o relatório de
// retenção do admin contava zero usuários ativos.
//
// TouchDailyActivity é chamada pelo middleware de autenticação em toda
// requisição, mas só encosta no banco UMA VEZ POR HORA por usuário (trava no
// cache em memória). Ou seja: com 1000 alunos online o custo é praticamente
// zero — uma consulta de mapa em memória na esmagadora maioria das chamadas.
func TouchDailyActivity(userID uuid.UUID) {
	seenKey := "seen:" + userID.String()
	if _, found := cache.AppCache.Get(seenKey); found {
		return
	}
	// Marca ANTES de ir ao banco: se 50 requisições do mesmo aluno chegarem
	// juntas, só a primeira faz o trabalho (evita estouro de conexões).
	cache.AppCache.Set(seenKey, true, time.Hour)

	// Find (e não First) porque "sem linha" é normal quando KEYCLOAK_ONLY está
	// ligado — nesse caso não há o que atualizar.
	var found []models.User
	if err := database.DB.Model(&models.User{}).
		Select("id", "last_login_at", "current_streak", "highest_streak").
		Where("id = ?", userID).
		Limit(1).
		Find(&found).Error; err != nil {
		log.Printf("presença diária: erro ao ler o usuário %s: %v", userID, err)
		return
	}
	if len(found) == 0 {
		return
	}
	user := found[0]

	// O dia é o do ALUNO, não o do servidor. Em UTC, quem estuda às 22h no
	// Brasil já está no dia seguinte — e perdia a ofensiva sem motivo.
	now := time.Now()
	today := utils.InicioDoDia(now)
	last := utils.InicioDoDia(user.LastLoginAt)

	// Já contabilizamos a presença de hoje: nada a fazer.
	if !user.LastLoginAt.IsZero() && last.Equal(today) {
		return
	}

	var streak int
	switch {
	case user.LastLoginAt.IsZero():
		streak = 1 // primeiro acesso registrado
	case last.Equal(today.AddDate(0, 0, -1)):
		streak = user.CurrentStreak + 1 // veio ontem também: a ofensiva continua
	default:
		streak = 1 // faltou pelo menos um dia: recomeça
	}

	highest := user.HighestStreak
	if streak > highest {
		highest = streak
	}

	if err := database.DB.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"last_login_at":  now,
			"current_streak": streak,
			"highest_streak": highest,
		}).Error; err != nil {
		log.Printf("presença diária: erro ao atualizar o usuário %s: %v", userID, err)
		return
	}

	// O painel mostra a ofensiva: limpa o cache para o aluno ver o número novo.
	cache.AppCache.Delete("dashboard_" + userID.String())
}
