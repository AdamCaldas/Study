package study

import "testing"

// ==========================================================
// ⏱️ DISTRIBUIÇÃO DE TEMPO ENTRE AS MATÉRIAS
// ==========================================================
// Este cálculo já gerou -9223372036854775808 minutos em produção: importância 0
// e desempenho 6 zeravam o peso de todas as matérias, a soma dava zero, a
// divisão virava NaN e a conversão para inteiro estourava. O número ia para o
// banco e para a tela do aluno.

// minutosValidos falha se algum bloco receber tempo impossível.

func TestNuncaGeraMinutosAbsurdos(t *testing.T) {
	casos := []struct {
		nome  string
		discs []DisciplineInput
		dia   float64
		min   int
		max   int
	}{
		{"peso zero em todas (o bug original)",
			[]DisciplineInput{{Name: "A", Importance: 0, Performance: 6}, {Name: "B", Importance: 0, Performance: 6}}, 240, 0, 0},
		{"uma matéria com peso zero",
			[]DisciplineInput{{Name: "Sozinha", Importance: 0, Performance: 6}}, 300, 0, 0},
		{"valores negativos",
			[]DisciplineInput{{Name: "A", Importance: -10, Performance: -10}}, 240, 0, 0},
		{"valores acima da escala",
			[]DisciplineInput{{Name: "A", Importance: 999, Performance: 999}}, 240, 0, 0},
		{"sem horas por dia",
			[]DisciplineInput{{Name: "A", Importance: 3, Performance: 3}}, 0, 0, 0},
		{"horas negativas",
			[]DisciplineInput{{Name: "A", Importance: 3, Performance: 3}}, -50, 0, 0},
		{"uso normal",
			[]DisciplineInput{{Name: "Mat", Importance: 5, Performance: 2}, {Name: "Port", Importance: 3, Performance: 4}}, 240, 30, 50},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			blocos := calculateDistribution(c.discs, c.dia, c.min, c.max)
			if len(blocos) == 0 {
				t.Fatal("nenhum bloco gerado")
			}
			for _, b := range blocos {
				if b.SuggestedMinutes < 1 {
					t.Errorf("%s ficou com %d minutos — bloco sem tempo não faz sentido", b.Activity, b.SuggestedMinutes)
				}
				if b.SuggestedMinutes > 24*60 {
					t.Errorf("%s ficou com %d minutos — mais que um dia inteiro", b.Activity, b.SuggestedMinutes)
				}
				if b.Importance < 1 || b.Importance > 5 {
					t.Errorf("importância %d saiu da escala 1..5", b.Importance)
				}
				if b.Performance < 1 || b.Performance > 5 {
					t.Errorf("desempenho %d saiu da escala 1..5", b.Performance)
				}
			}
		})
	}
}

func TestListaVaziaNaoQuebra(t *testing.T) {
	if b := calculateDistribution(nil, 240, 30, 50); len(b) != 0 {
		t.Errorf("sem matérias deveria devolver lista vazia, veio %d blocos", len(b))
	}
}

func TestQuemVaiPiorGanhaMaisTempo(t *testing.T) {
	// A regra do produto: desempenho baixo = precisa estudar mais.
	discs := []DisciplineInput{
		{Name: "Indo mal", Importance: 3, Performance: 1},
		{Name: "Indo bem", Importance: 3, Performance: 5},
	}
	blocos := calculateDistribution(discs, 300, 0, 0)

	var mal, bem int
	for _, b := range blocos {
		if b.Activity == "Indo mal" {
			mal += b.SuggestedMinutes
		} else {
			bem += b.SuggestedMinutes
		}
	}
	if mal <= bem {
		t.Errorf("quem vai mal (%d min) deveria receber MAIS tempo que quem vai bem (%d min)", mal, bem)
	}
}

func TestImportanciaAumentaOTempo(t *testing.T) {
	discs := []DisciplineInput{
		{Name: "Essencial", Importance: 5, Performance: 3},
		{Name: "Secundária", Importance: 1, Performance: 3},
	}
	blocos := calculateDistribution(discs, 300, 0, 0)

	var ess, sec int
	for _, b := range blocos {
		if b.Activity == "Essencial" {
			ess += b.SuggestedMinutes
		} else {
			sec += b.SuggestedMinutes
		}
	}
	if ess <= sec {
		t.Errorf("a matéria essencial (%d min) deveria receber mais tempo que a secundária (%d min)", ess, sec)
	}
}
