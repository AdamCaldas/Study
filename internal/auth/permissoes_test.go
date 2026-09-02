package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"studfy-backend/internal/models"

	"github.com/gin-gonic/gin"
)

// ==========================================================
// 🔒 LER NÃO É EDITAR
// ==========================================================
// A auditoria encontrou rotas de LEITURA sem checagem nenhuma: qualquer pessoa
// logada abria o caderno de qualquer turma sabendo o id. A escrita estava
// protegida, a leitura não.
//
// Estes testes travam a regra: quem não é da turma não lê, e quem é da turma
// lê mas não necessariamente edita.

func TestQuemNaoEhDaTurmaNaoLe(t *testing.T) {
	// Sem contexto de turma (o middleware não deixou nada), nada pode passar.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	if IsSpaceOwner(c) {
		t.Fatal("FALHA: sem contexto de turma o código achou que é dono")
	}
	if _, ok := SpacePermissionFromCtx(c); ok {
		t.Fatal("FALHA: sem contexto de turma apareceu uma permissão do nada")
	}
	if CanSeeQuizAnswers(c) {
		t.Fatal("FALHA DE SEGURANÇA: sem contexto de turma liberou o gabarito")
	}
}

func TestAlunoDaTurmaLeMasNaoEdita(t *testing.T) {
	aluno := &models.SpacePermission{AccessLevel: "VIEWER"}

	// Ele É da turma: o CheckSpaceAccess deixa passar (é o que permite ler).
	c, _ := ctxWith(false, aluno)
	if _, ok := SpacePermissionFromCtx(c); !ok {
		t.Fatal("o aluno da turma deveria ter permissão no contexto")
	}

	// Mas não pode escrever nem ver gabarito.
	casos := map[string]gin.HandlerFunc{
		"criar conteúdo":  RequireSpaceCreateContent(),
		"editar conteúdo": RequireSpaceEditContent(),
		"apagar conteúdo": RequireSpaceDeleteContent(),
	}
	for nome, guarda := range casos {
		c2, rec2 := ctxWith(false, aluno)
		if _, passou := run(guarda, c2, rec2); passou {
			t.Errorf("FALHA DE SEGURANÇA: aluno VIEWER conseguiu %q", nome)
		}
	}
	if c3, _ := ctxWith(false, aluno); CanSeeQuizAnswers(c3) {
		t.Error("FALHA DE SEGURANÇA: aluno viu o gabarito")
	}
}

func TestDonoLeEEdita(t *testing.T) {
	c, rec := ctxWith(true, nil)
	if _, passou := run(RequireSpaceCreateContent(), c, rec); !passou {
		t.Fatal("o dono da turma deveria poder criar conteúdo")
	}
	if c2, _ := ctxWith(true, nil); !CanSeeQuizAnswers(c2) {
		t.Fatal("o dono precisa ver o gabarito para corrigir")
	}
}
