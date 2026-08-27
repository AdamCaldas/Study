package middleware

import (
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==========================================================
// 🔭 OBSERVABILIDADE
// ==========================================================
// Antes só existia log.Printf solto: sem identificador de requisição, sem tempo
// de resposta, sem contagem de erro. Com mil alunos reclamando de lentidão, não
// havia como descobrir QUAL rota era a culpada.
//
// Usa log/slog, que já vem na linguagem — nenhuma dependência nova.

var Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// RequestLogger registra uma linha estruturada por requisição.
// Cada requisição ganha um request_id que também volta no cabeçalho
// X-Request-ID — quando um aluno relata um erro, dá para achar a linha exata.
func RequestLogger() gin.HandlerFunc {
	// Rotas de infraestrutura não poluem o log.
	ignorar := map[string]bool{"/ping": true, "/metrics": true}

	return func(c *gin.Context) {
		if ignorar[c.Request.URL.Path] {
			c.Next()
			return
		}

		start := time.Now()

		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set("requestID", requestID)
		c.Header("X-Request-ID", requestID)

		c.Next()

		status := c.Writer.Status()
		durationMs := float64(time.Since(start).Microseconds()) / 1000.0

		// `route` é o padrão da rota (/v1/app/spaces/:space_id), não a URL com o
		// id preenchido: é assim que dá para agrupar e achar a rota lenta.
		route := c.FullPath()
		if route == "" {
			route = "desconhecida"
		}

		attrs := []any{
			slog.String("request_id", requestID),
			slog.String("method", c.Request.Method),
			slog.String("route", route),
			slog.Int("status", status),
			slog.Float64("duration_ms", durationMs),
		}

		// Quem fez a requisição (quando já passou pela autenticação).
		if v, ok := c.Get("userID"); ok {
			attrs = append(attrs, slog.String("user_id", toString(v)))
		}

		switch {
		case status >= 500:
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
			Logger.Error("erro no servidor", attrs...)
		case status >= 400:
			Logger.Warn("requisição recusada", attrs...)
		case durationMs > 1000:
			// Alvo direto para caçar gargalo em produção.
			Logger.Warn("requisição lenta", attrs...)
		default:
			Logger.Info("requisição", attrs...)
		}
	}
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case uuid.UUID:
		return t.String()
	default:
		return ""
	}
}

// Recovery evita que um panic derrube o processo inteiro e registra o ocorrido
// com o request_id, para dar pra rastrear depois.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		requestID, _ := c.Get("requestID")
		Logger.Error("panic capturado",
			slog.Any("request_id", requestID),
			slog.String("route", c.FullPath()),
			slog.Any("panic", recovered),
		)
		c.AbortWithStatusJSON(500, gin.H{
			"error":      "Erro interno no servidor.",
			"request_id": requestID,
		})
	})
}
