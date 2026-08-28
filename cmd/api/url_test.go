package main

import "testing"

// Garante que a URL do Keycloak funciona nos formatos que as nuvens entregam.
// Errar aqui significa API no ar sem conseguir validar token nenhum.
func TestNormalizaKeycloakURL(t *testing.T) {
	casos := []struct{ bruta, realm, esperado string }{
		// vazio → padrão local
		{"", "", "http://localhost:8080/realms/studfy"},
		// Render fromService/hostport: só host:porta
		{"studfy-keycloak:10000", "", "http://studfy-keycloak:10000/realms/studfy"},
		// domínio público sem o caminho do realm
		{"https://studfy-keycloak.onrender.com", "", "https://studfy-keycloak.onrender.com/realms/studfy"},
		// barra sobrando no fim
		{"https://kc.exemplo.com/", "", "https://kc.exemplo.com/realms/studfy"},
		// já veio completo: não mexe
		{"https://kc.exemplo.com/realms/outro", "", "https://kc.exemplo.com/realms/outro"},
		// realm customizado
		{"https://kc.exemplo.com", "empresa", "https://kc.exemplo.com/realms/empresa"},
		// espaços acidentais no painel da nuvem
		{"  https://kc.exemplo.com  ", "", "https://kc.exemplo.com/realms/studfy"},

		// 🔑 O CASO QUE QUEBRA TUDO SE ERRAR O ESQUEMA:
		// o Render entrega o issuer como hostname puro. Tem que virar HTTPS,
		// senão o `iss` do token (https) nunca bate e toda requisição é recusada.
		{"studfy-keycloak.onrender.com", "", "https://studfy-keycloak.onrender.com/realms/studfy"},
		// já nome de serviço interno (sem ponto) continua http — rede privada
		{"studfy-keycloak", "", "http://studfy-keycloak/realms/studfy"},
		{"localhost:8080", "", "http://localhost:8080/realms/studfy"},
		{"127.0.0.1:8080", "", "http://127.0.0.1:8080/realms/studfy"},
	}

	for _, c := range casos {
		if got := normalizaKeycloakURL(c.bruta, c.realm); got != c.esperado {
			t.Errorf("normalizaKeycloakURL(%q, %q)\n  deu      %q\n  esperava %q", c.bruta, c.realm, got, c.esperado)
		}
	}
}
