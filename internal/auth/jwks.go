package auth

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
)

// ==========================================================
// 🔑 CHAVES DO KEYCLOAK — carregadas sem derrubar a API
// ==========================================================
// Antes, se o Keycloak não respondesse no boot, a API chamava log.Fatal e o
// processo morria. Em nuvem (Render, Railway...) isso vira um ciclo de deploy
// falhando para sempre: o container nunca sobe, então nem /health responde e
// não há como diagnosticar nada.
//
// Agora a API sobe do mesmo jeito e fica tentando buscar as chaves em segundo
// plano. Enquanto elas não chegam, TODA rota protegida responde 401 — ou seja,
// falha fechada: sem chave não se valida token, e sem token validado ninguém
// entra. Nada é liberado por engano.
type JWKSProvider struct {
	jwks     atomic.Pointer[keyfunc.JWKS]
	url      string
	carregou atomic.Bool
}

var ErrKeycloakIndisponivel = fmt.Errorf("chaves do Keycloak ainda não carregadas")

func NewJWKSProvider(jwksURL string) *JWKSProvider {
	return &JWKSProvider{url: jwksURL}
}

// Keyfunc é o que o parser de JWT usa para achar a chave do token.
func (p *JWKSProvider) Keyfunc(token *jwt.Token) (interface{}, error) {
	j := p.jwks.Load()
	if j == nil {
		return nil, ErrKeycloakIndisponivel
	}
	return j.Keyfunc(token)
}

// Pronto diz se as chaves já foram carregadas (usado pelo /health).
func (p *JWKSProvider) Pronto() bool { return p.carregou.Load() }

// URL do JWKS, para aparecer nos diagnósticos.
func (p *JWKSProvider) URL() string { return p.url }

// Carregar tenta buscar as chaves algumas vezes. Se conseguir, devolve true.
// Se não, devolve false SEM derrubar nada — quem chama decide seguir em frente.
func (p *JWKSProvider) Carregar(tentativas int, espera time.Duration) bool {
	opts := keyfunc.Options{
		RefreshInterval:   time.Hour, // recarrega as chaves de hora em hora
		RefreshUnknownKID: true,      // busca a chave na hora se o `kid` for novo (rotação)
		RefreshTimeout:    10 * time.Second,
		RefreshErrorHandler: func(err error) {
			log.Printf("Keycloak: falha ao atualizar as chaves: %v", err)
		},
	}

	for i := 1; i <= tentativas; i++ {
		j, err := keyfunc.Get(p.url, opts)
		if err == nil {
			p.jwks.Store(j)
			p.carregou.Store(true)
			log.Printf("✅ Keycloak: chaves públicas carregadas de %s", p.url)
			return true
		}
		log.Printf("Keycloak: tentativa %d/%d falhou em %s (%v)", i, tentativas, p.url, err)
		if i < tentativas {
			time.Sleep(espera)
		}
	}
	return false
}

// CarregarEmSegundoPlano insiste para sempre, com espera crescente até 5 min.
// Assim, no minuto em que o Keycloak subir, a API passa a autenticar sozinha —
// sem precisar de um novo deploy.
func (p *JWKSProvider) CarregarEmSegundoPlano() {
	go func() {
		espera := 15 * time.Second
		for {
			if p.Carregar(1, 0) {
				return
			}
			time.Sleep(espera)
			if espera < 5*time.Minute {
				espera *= 2
			}
		}
	}()
}
