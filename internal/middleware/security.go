package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// ==========================================================
// 🔒 1. CORS BLINDADO (Só o seu Front-end pode chamar a API)
// ==========================================================
func SecureCORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		// ⚠️ Mude aqui para os domínios reais quando for para produção!
		AllowOrigins:     []string{"http://localhost:3000", "https://app.studfy.com", "https://studfy.com"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}

// ==========================================================
// 🛡️ 2. RATE LIMITER (Escudo Anti-DDoS e Anti-Bot)
// Limita cada IP a 10 requisições por segundo.
// ==========================================================

// Criamos um mapa em memória para guardar o limite de cada IP visitante
var visitors = make(map[string]*rate.Limiter)
var mu sync.Mutex

func getVisitor(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	limiter, exists := visitors[ip]
	if !exists {
		// Permite 10 requisições por segundo, com picos (burst) de até 20.
		limiter = rate.NewLimiter(10, 20)
		visitors[ip] = limiter
	}
	return limiter
}

func RateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Pega o IP real de quem está chamando a API
		ip := c.ClientIP()
		limiter := getVisitor(ip)

		// Se o cara ultrapassou o limite, a gente corta a requisição na hora!
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Calma aí, velocista! Muitas requisições. Aguarde um momento.",
			})
			c.Abort() // 👈 Mata a requisição antes de chegar no Banco de Dados
			return
		}

		c.Next()
	}
}
