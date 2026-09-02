package utils

import (
	"os"
	"testing"
	"time"
)

// ==========================================================
// 🕐 O DIA É O DO ALUNO
// ==========================================================
// Em UTC, quem estuda às 22h no Brasil já está no dia seguinte. Isso quebrava a
// ofensiva (o dia "pulava"), mandava o log diário para a data errada e
// deslocava os relatórios por dia.

func TestEstudoDaNoiteContaNoDiaCerto(t *testing.T) {
	os.Setenv("TZ_APP", "America/Sao_Paulo")

	br, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skip("fuso do Brasil indisponível nesta máquina")
	}

	// Aluno estudou 02/09 às 22h (horário dele).
	estudo := time.Date(2026, 9, 2, 22, 0, 0, 0, br)

	// No servidor isso é 03/09 01h UTC — o dia errado.
	if estudo.UTC().Day() != 3 {
		t.Fatalf("o teste pressupõe que 22h BRT vira dia 3 em UTC, deu dia %d", estudo.UTC().Day())
	}

	dia := InicioDoDia(estudo)
	if dia.Day() != 2 {
		t.Errorf("o estudo das 22h deveria contar no dia 2 (o dia do aluno), contou no dia %d", dia.Day())
	}
}

func TestDoisEstudosNaMesmaNoiteSaoOMesmoDia(t *testing.T) {
	os.Setenv("TZ_APP", "America/Sao_Paulo")
	br, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skip("fuso indisponível")
	}

	// 19h e 23h do MESMO dia para o aluno.
	cedo := time.Date(2026, 9, 2, 19, 0, 0, 0, br)
	tarde := time.Date(2026, 9, 2, 23, 0, 0, 0, br)

	if !MesmoDia(cedo, tarde) {
		t.Error("19h e 23h do mesmo dia deveriam contar como um dia só — em UTC viravam dois")
	}
}

func TestOfensivaNaoQuebraPorCausaDoFuso(t *testing.T) {
	os.Setenv("TZ_APP", "America/Sao_Paulo")
	br, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skip("fuso indisponível")
	}

	// Estudou ontem à noite e hoje de manhã: são dias CONSECUTIVOS,
	// então a ofensiva tem que continuar (diferença de 1 dia).
	ontemNoite := time.Date(2026, 9, 1, 22, 30, 0, 0, br)
	hojeManha := time.Date(2026, 9, 2, 8, 0, 0, 0, br)

	if d := DiasEntre(ontemNoite, hojeManha); d != 1 {
		t.Errorf("de ontem 22h30 para hoje 8h deveria ser 1 dia, deu %d — a ofensiva quebraria", d)
	}
}
