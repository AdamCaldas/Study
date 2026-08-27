package middleware

import (
	"net/http"
	"os"
	"strconv"
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
// Limita cada IP a N requisições por segundo (padrão 10, burst 20).
//
// ⚠️ ESCALA: este limitador vive na MEMÓRIA deste processo. Ele é leve e
// aguenta muito bem 1000 alunos numa única instância. Se um dia você subir
// VÁRIAS réplicas da API atrás de um load balancer, troque este mapa por um
// limitador compartilhado (Redis), senão cada réplica conta separado.
// ==========================================================

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	visitors = make(map[string]*visitor)
	mu       sync.Mutex

	// Configuráveis por env (com padrões seguros).
	rlPerSecond = envFloat("RATE_LIMIT_RPS", 10)
	rlBurst     = envInt("RATE_LIMIT_BURST", 20)
)

func init() {
	// 🧹 Faxineiro: a cada minuto remove IPs inativos há mais de 5 min.
	// Sem isso, o mapa cresceria para sempre (memory leak) sob tráfego real.
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, v := range visitors {
				if time.Since(v.lastSeen) > 5*time.Minute {
					delete(visitors, ip)
				}
			}
			mu.Unlock()
		}
	}()
}

func getVisitor(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	v, exists := visitors[ip]
	if !exists {
		v = &visitor{limiter: rate.NewLimiter(rate.Limit(rlPerSecond), rlBurst)}
		visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
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

func envInt(key string, def int) int {
	if s := os.Getenv(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if s := os.Getenv(key); s != "" {
		if n, err := strconv.ParseFloat(s, 64); err == nil && n > 0 {
			return n
		}
	}
	return def
}
