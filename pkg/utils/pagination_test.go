package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// A paginação é a proteção contra devolver a tabela inteira. Se o teto parar
// de valer, uma turma grande derruba o app do aluno sem dar erro nenhum.

func pedido(query string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
	return c
}

func TestPaginaPadrao(t *testing.T) {
	p := GetPage(pedido(""))
	if p.Page != 1 {
		t.Errorf("sem ?page= deveria começar na 1, veio %d", p.Page)
	}
	if p.Limit != defaultPageSize {
		t.Errorf("sem ?limit= deveria usar %d, veio %d", defaultPageSize, p.Limit)
	}
}

func TestLimiteNaoPassaDoTeto(t *testing.T) {
	// Alguém pedindo 100 mil registros não pode conseguir.
	p := GetPage(pedido("limit=100000"))
	if p.Limit != maxPageSize {
		t.Fatalf("FALHA: pediu 100000 e recebeu limite %d (o teto é %d)", p.Limit, maxPageSize)
	}
}

func TestValoresInvalidosCaemNoPadrao(t *testing.T) {
	casos := []string{"page=0", "page=-5", "page=abc", "limit=0", "limit=-1", "limit=xyz"}
	for _, q := range casos {
		p := GetPage(pedido(q))
		if p.Page < 1 {
			t.Errorf("%q: página virou %d (não pode ser menor que 1)", q, p.Page)
		}
		if p.Limit < 1 || p.Limit > maxPageSize {
			t.Errorf("%q: limite virou %d (fora do intervalo permitido)", q, p.Limit)
		}
	}
}

func TestOffsetAcompanhaAPagina(t *testing.T) {
	casos := []struct {
		query    string
		esperado int
	}{
		{"page=1&limit=30", 0},
		{"page=2&limit=30", 30},
		{"page=5&limit=10", 40},
	}
	for _, c := range casos {
		if got := GetPage(pedido(c.query)).Offset(); got != c.esperado {
			t.Errorf("%q: offset %d, esperava %d", c.query, got, c.esperado)
		}
	}
}

func TestMetaDizSeTemMaisPagina(t *testing.T) {
	p := GetPage(pedido("page=1&limit=10"))

	m := p.Meta(35) // 35 itens, 10 por página → 4 páginas, tem mais
	if m["total_pages"] != int64(4) {
		t.Errorf("35 itens com 10 por página deveria dar 4 páginas, deu %v", m["total_pages"])
	}
	if m["has_more"] != true {
		t.Error("na página 1 de 4 deveria indicar que há mais")
	}

	// Página única: não pode dizer que tem mais.
	if m2 := p.Meta(7); m2["has_more"] != false {
		t.Error("com 7 itens em uma página só não deveria indicar mais páginas")
	}

	// Lista vazia não pode quebrar a conta.
	if m3 := p.Meta(0); m3["total_pages"] != int64(0) || m3["has_more"] != false {
		t.Errorf("lista vazia deveria dar 0 páginas e has_more falso, deu %v", m3)
	}
}
