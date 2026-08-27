package study

import "testing"

// ==========================================================
// 🧠 ESCADA DA REPETIÇÃO ESPAÇADA
// ==========================================================
// A curva de esquecimento é o coração de um app de estudos. Se os intervalos
// pararem de crescer (ou estourarem o fim da lista), o aluno revisa na hora
// errada e a funcionalidade perde o sentido — sem dar nenhum erro visível.

func TestIntervalosCrescemAteOTeto(t *testing.T) {
	esperado := []int{1, 3, 7, 15, 30, 60}

	for degrau, dias := range esperado {
		data, aplicado := nextReviewDate(degrau)
		if aplicado != degrau {
			t.Errorf("degrau %d foi ajustado para %d sem motivo", degrau, aplicado)
		}
		if data.IsZero() {
			t.Fatalf("degrau %d devolveu data zerada", degrau)
		}
		if reviewIntervals[aplicado] != dias {
			t.Errorf("degrau %d: esperava %d dias, veio %d", degrau, dias, reviewIntervals[aplicado])
		}
	}
}

func TestDegrauAcimaDoTetoNaoEstoura(t *testing.T) {
	// Um aluno muito consistente passa do último degrau: tem que travar no
	// maior intervalo, e não explodir com índice fora da lista.
	_, aplicado := nextReviewDate(99)

	ultimo := len(reviewIntervals) - 1
	if aplicado != ultimo {
		t.Fatalf("esperava travar no degrau %d (60 dias), veio %d", ultimo, aplicado)
	}
}

func TestDegrauNegativoVoltaParaOComeco(t *testing.T) {
	_, aplicado := nextReviewDate(-5)
	if aplicado != 0 {
		t.Fatalf("degrau negativo deveria virar 0, veio %d", aplicado)
	}
}

func TestEsquecerVoltaParaUmDia(t *testing.T) {
	// Regra: errou, recomeça do degrau 0 (revisar amanhã).
	_, aplicado := nextReviewDate(0)
	if reviewIntervals[aplicado] != 1 {
		t.Fatalf("ao esquecer, a revisão deveria voltar para 1 dia, veio %d", reviewIntervals[aplicado])
	}
}

// ==========================================================
// ⏰ CONVERSÃO DE HORÁRIO DO CRONOGRAMA
// ==========================================================
// timeToMinutes ignorava o erro do Sscanf: horário malformado virava 0
// silenciosamente e o bloco de estudo ia parar na meia-noite.

func TestTimeToMinutes(t *testing.T) {
	casos := []struct {
		entrada  string
		esperado int
	}{
		{"00:00", 0},
		{"08:30", 510},
		{"23:59", 1439},
		{"08:30:00", 510}, // aceita o formato com segundos
		{"", 0},
		{"abc", 0},   // lixo não vira meia-noite por acidente
		{"99:99", 0}, // fora do intervalo válido
		{"25:00", 0}, // hora inexistente
	}

	for _, caso := range casos {
		if got := timeToMinutes(caso.entrada); got != caso.esperado {
			t.Errorf("timeToMinutes(%q) = %d, esperava %d", caso.entrada, got, caso.esperado)
		}
	}
}

func TestMinutesToHHMMCabeNaColuna(t *testing.T) {
	// A coluna start_time/end_time é varchar(5): "HH:MM".
	// O AutoFit gravava "HH:MM:00" (8 chars) e o valor era truncado/rejeitado.
	for _, min := range []int{0, 510, 1439, 60} {
		got := minutesToHHMM(min)
		if len(got) != 5 {
			t.Errorf("minutesToHHMM(%d) = %q tem %d caracteres; a coluna aceita 5", min, got, len(got))
		}
	}
}
