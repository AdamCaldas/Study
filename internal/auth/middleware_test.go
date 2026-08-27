package auth

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"studfy-backend/internal/models"

	"github.com/gin-gonic/gin"
)

// reflectTypeOf devolve o conjunto de nomes de campo de uma struct.
func reflectTypeOf(v any) map[string]struct{} {
	t := reflect.TypeOf(v)
	campos := make(map[string]struct{}, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		campos[t.Field(i).Name] = struct{}{}
	}
	return campos
}

// ==========================================================
// 🛡️ TESTES DAS REGRAS DE PERMISSÃO DA TURMA
// ==========================================================
// Estes testes existem por um motivo concreto: a auditoria encontrou rotas em
// que QUALQUER aluno conseguia apagar a turma ou se promover a monitor. Sem
// teste, uma verificação de cargo esquecida não faz barulho nenhum.
//
// Não precisam de banco: os guardas leem o que o CheckSpaceAccess deixou no
// contexto, então dá para montar o contexto na mão e verificar a decisão.

// ctxWith monta uma requisição falsa com o papel do usuário já resolvido.
func ctxWith(isOwner bool, perm *models.SpacePermission) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	c.Set("spaceIsOwner", isOwner)
	if perm != nil {
		c.Set("spacePermission", *perm)
		c.Set("spaceRole", perm.AccessLevel)
	} else if isOwner {
		c.Set("spaceRole", "OWNER")
	}
	return c, rec
}

// aluno comum: entrou na turma, sem nenhuma permissão especial.
func aluno() *models.SpacePermission {
	return &models.SpacePermission{AccessLevel: "VIEWER"}
}

// run executa o guarda e devolve o status e se a requisição seguiu adiante.
func run(guard gin.HandlerFunc, c *gin.Context, rec *httptest.ResponseRecorder) (int, bool) {
	passou := false
	c.Next() // garante índice inicial
	guard(c)
	if !c.IsAborted() {
		passou = true
	}
	return rec.Code, passou
}

func TestAlunoNaoPodeAcoesDeProfessor(t *testing.T) {
	casos := []struct {
		nome   string
		guarda gin.HandlerFunc
	}{
		{"apagar a turma", RequireSpaceOwner()},
		{"gerenciar membros (auto-promoção)", RequireSpaceManageMembers()},
		{"criar conteúdo", RequireSpaceCreateContent()},
		{"editar conteúdo", RequireSpaceEditContent()},
		{"apagar conteúdo", RequireSpaceDeleteContent()},
		{"gerenciar simulados", RequireSpaceManageQuizzes()},
		{"gerenciar planos", RequireSpaceManagePlans()},
		{"alterar configurações", RequireSpaceEditInfo()},
		{"ver telas de professor", RequireSpaceStaff()},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c, rec := ctxWith(false, aluno())
			status, passou := run(caso.guarda, c, rec)

			if passou {
				t.Fatalf("FALHA DE SEGURANÇA: um aluno VIEWER conseguiu %q", caso.nome)
			}
			if status != http.StatusForbidden {
				t.Errorf("esperava 403 ao bloquear %q, veio %d", caso.nome, status)
			}
		})
	}
}

func TestDonoPodeTudo(t *testing.T) {
	guardas := map[string]gin.HandlerFunc{
		"dono":               RequireSpaceOwner(),
		"membros":            RequireSpaceManageMembers(),
		"criar conteúdo":     RequireSpaceCreateContent(),
		"apagar conteúdo":    RequireSpaceDeleteContent(),
		"simulados":          RequireSpaceManageQuizzes(),
		"planos":             RequireSpaceManagePlans(),
		"configurações":      RequireSpaceEditInfo(),
		"telas de professor": RequireSpaceStaff(),
	}

	for nome, guarda := range guardas {
		t.Run(nome, func(t *testing.T) {
			c, rec := ctxWith(true, nil)
			_, passou := run(guarda, c, rec)
			if !passou {
				t.Fatalf("o dono da turma foi bloqueado em %q", nome)
			}
		})
	}
}

func TestMonitorPodeConteudoMasNaoApagarATurma(t *testing.T) {
	monitor := &models.SpacePermission{AccessLevel: "MONITOR"}

	// Monitor cuida do dia a dia da turma...
	podem := map[string]gin.HandlerFunc{
		"criar conteúdo":      RequireSpaceCreateContent(),
		"editar conteúdo":     RequireSpaceEditContent(),
		"gerenciar simulados": RequireSpaceManageQuizzes(),
		"telas de professor":  RequireSpaceStaff(),
	}
	for nome, guarda := range podem {
		t.Run("monitor pode "+nome, func(t *testing.T) {
			c, rec := ctxWith(false, monitor)
			if _, passou := run(guarda, c, rec); !passou {
				t.Fatalf("monitor deveria poder %q", nome)
			}
		})
	}

	// ...mas apagar a turma é só do dono.
	t.Run("monitor NÃO pode apagar a turma", func(t *testing.T) {
		c, rec := ctxWith(false, monitor)
		if _, passou := run(RequireSpaceOwner(), c, rec); passou {
			t.Fatal("FALHA DE SEGURANÇA: monitor conseguiu apagar a turma")
		}
	})
}

func TestPermissaoGranularEhRespeitada(t *testing.T) {
	// Aluno a quem o professor deu explicitamente o direito de criar conteúdo.
	colaborador := &models.SpacePermission{AccessLevel: "VIEWER", CanCreateContent: true}

	c, rec := ctxWith(false, colaborador)
	if _, passou := run(RequireSpaceCreateContent(), c, rec); !passou {
		t.Fatal("quem tem can_create_content deveria poder criar conteúdo")
	}

	// Mas isso não lhe dá poder sobre os membros da turma.
	c2, rec2 := ctxWith(false, colaborador)
	if _, passou := run(RequireSpaceManageMembers(), c2, rec2); passou {
		t.Fatal("FALHA DE SEGURANÇA: can_create_content virou poder de gerenciar membros")
	}
}

func TestSemContextoNaoPassa(t *testing.T) {
	// Se o CheckSpaceAccess não rodou antes, nada pode passar.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	if _, passou := run(RequireSpaceOwner(), c, rec); passou {
		t.Fatal("FALHA DE SEGURANÇA: passou sem o contexto de acesso da turma")
	}
}

// ==========================================================
// 🔒 GABARITO DO SIMULADO
// ==========================================================
func TestAlunoNaoVeGabarito(t *testing.T) {
	c, _ := ctxWith(false, aluno())
	if CanSeeQuizAnswers(c) {
		t.Fatal("FALHA DE SEGURANÇA: aluno consegue ver a resposta certa da prova")
	}
}

func TestProfessorEMonitorVeemGabarito(t *testing.T) {
	dono, _ := ctxWith(true, nil)
	if !CanSeeQuizAnswers(dono) {
		t.Error("o dono da turma precisa ver o gabarito para corrigir")
	}

	monitor, _ := ctxWith(false, &models.SpacePermission{AccessLevel: "MONITOR"})
	if !CanSeeQuizAnswers(monitor) {
		t.Error("o monitor precisa ver o gabarito para corrigir")
	}
}

// ==========================================================
// 👤 PERFIL PÚBLICO NÃO PODE CARREGAR DADO SENSÍVEL
// ==========================================================
// Trava de projeto: se alguém adicionar CPF/e-mail ao PublicUser um dia, este
// teste quebra — foi exatamente esse vazamento que a auditoria encontrou.
func TestPublicUserNaoExpoeDadoSensivel(t *testing.T) {
	proibidos := []string{"CPF", "CNPJ", "Email", "Password", "BirthDate"}

	tipo := reflectTypeOf(models.PublicUser{})
	for _, campo := range proibidos {
		if _, existe := tipo[campo]; existe {
			t.Fatalf("PublicUser expõe o campo sensível %q — ele vai vazar para todos os alunos da turma", campo)
		}
	}
}
