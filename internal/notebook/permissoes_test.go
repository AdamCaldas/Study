package notebook

import (
	"os"
	"testing"

	"studfy-backend/internal/models"
	"studfy-backend/pkg/database"

	"github.com/google/uuid"
)

// ==========================================================
// 🔒 QUEM LÊ E QUEM EDITA O MATERIAL
// ==========================================================
// Estes testes precisam de um PostgreSQL de verdade — as regras dependem de
// JOINs e do comportamento real do banco, então um banco falso não provaria
// nada. Sem DATABASE_URL_TEST eles são pulados.
//
// Para rodar:
//   docker run -d --name pgtest -e POSTGRES_PASSWORD=t -e POSTGRES_USER=studfy \
//     -e POSTGRES_DB=studfy -p 55470:5432 postgres:17-alpine
//   DATABASE_URL_TEST="postgres://studfy:t@localhost:55470/studfy?sslmode=disable" \
//     go test ./internal/notebook/...

func prepararBanco(t *testing.T) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL_TEST")
	if dsn == "" {
		t.Skip("DATABASE_URL_TEST não definida — teste de banco pulado")
	}
	os.Setenv("DATABASE_URL", dsn)
	os.Setenv("AUTO_MIGRATE", "true")
	if database.DB == nil {
		database.ConnectDB()
	}
}

// cenario monta uma turma com dono, um aluno membro e gente de fora.
type cenario struct {
	turma   uuid.UUID
	caderno uuid.UUID
	guia    uuid.UUID
	dono    uuid.UUID
	aluno   uuid.UUID
	deFora  uuid.UUID
}

func montarCenario(t *testing.T) cenario {
	t.Helper()

	novo := func(nome string) uuid.UUID {
		id := uuid.New()
		if err := database.DB.Create(&models.User{
			ID: id, FullName: nome, Email: id.String() + "@t.com",
			CPF: id.String()[:11], Password: "x",
		}).Error; err != nil {
			t.Fatalf("criar usuário %s: %v", nome, err)
		}
		return id
	}

	c := cenario{dono: novo("Dono"), aluno: novo("Aluno"), deFora: novo("De fora")}

	turma := models.Space{
		OwnerID: c.dono, Name: "Turma",
		Slug: "t-" + uuid.NewString()[:8], ShareCode: "C-" + uuid.NewString()[:8],
	}
	if err := database.DB.Create(&turma).Error; err != nil {
		t.Fatalf("criar turma: %v", err)
	}
	c.turma = turma.ID

	if err := database.DB.Create(&models.SpacePermission{
		SpaceID: c.turma, UserID: c.aluno, AccessLevel: "VIEWER",
	}).Error; err != nil {
		t.Fatalf("vincular aluno: %v", err)
	}

	nb := models.Notebook{SpaceID: c.turma, Name: "Caderno", CreatedByID: c.dono, UpdatedByID: c.dono}
	if err := database.DB.Create(&nb).Error; err != nil {
		t.Fatalf("criar caderno: %v", err)
	}
	c.caderno = nb.ID

	g := models.Guide{NotebookID: nb.ID, Name: "Guia", CustomDimensions: "{}", CreatedByID: c.dono, UpdatedByID: c.dono}
	if err := database.DB.Create(&g).Error; err != nil {
		t.Fatalf("criar guia: %v", err)
	}
	c.guia = g.ID

	return c
}

func TestLeituraDeMaterial(t *testing.T) {
	prepararBanco(t)
	c := montarCenario(t)

	casos := []struct {
		quem     string
		id       uuid.UUID
		esperado bool
	}{
		{"dono da turma", c.dono, true},
		{"aluno da turma", c.aluno, true},
		{"pessoa de fora", c.deFora, false},
	}

	for _, caso := range casos {
		if got := canReadNotebook(c.turma, c.caderno, caso.id); got != caso.esperado {
			t.Errorf("caderno — %s: podia ler=%v, esperava %v", caso.quem, got, caso.esperado)
		}
		if got := canReadGuide(c.guia, caso.id); got != caso.esperado {
			t.Errorf("guia — %s: podia ler=%v, esperava %v", caso.quem, got, caso.esperado)
		}
	}
}

func TestAlunoLeMasNaoEdita(t *testing.T) {
	prepararBanco(t)
	c := montarCenario(t)

	if !canReadNotebook(c.turma, c.caderno, c.aluno) {
		t.Fatal("o aluno da turma precisa conseguir LER o material")
	}
	if canEditNotebook(c.turma, c.caderno, c.aluno) {
		t.Fatal("FALHA DE SEGURANÇA: aluno VIEWER conseguiu editar o caderno")
	}
}

func TestAcessoLiberadoSoNaqueleCaderno(t *testing.T) {
	prepararBanco(t)
	c := montarCenario(t)

	// É assim que a automação do professor libera reforço para um aluno.
	if err := database.DB.Create(&models.NotebookPermission{
		NotebookID: c.caderno, UserID: c.deFora, AccessLevel: "VIEWER",
	}).Error; err != nil {
		t.Fatalf("liberar caderno: %v", err)
	}

	if !canReadNotebook(c.turma, c.caderno, c.deFora) {
		t.Error("quem recebeu acesso ao caderno deveria conseguir lê-lo")
	}
	// Mas continua sem poder editar.
	if canEditNotebook(c.turma, c.caderno, c.deFora) {
		t.Error("FALHA DE SEGURANÇA: acesso de leitura virou permissão de edição")
	}
}
