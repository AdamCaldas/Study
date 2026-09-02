package utils

import (
	"log"
	"os"
	"sync"
	"time"
)

// ==========================================================
// 🕐 O DIA DO ALUNO, NÃO O DIA DO SERVIDOR
// ==========================================================
// Servidores em nuvem rodam em UTC. Sem tratar isso, um aluno no Brasil que
// estuda às 22h tem o registro contado no DIA SEGUINTE (01h UTC):
//
//   para ele    : 02/09, 22h
//   no servidor : 03/09, 01h  ← cai no dia errado
//
// O estrago é direto no produto: a ofensiva quebra sem motivo (o dia "pulou"),
// o log diário de estudo vai para a data errada e os relatórios agrupados por
// dia ficam deslocados.
//
// Defina TZ_APP com o fuso do público (ex.: America/Sao_Paulo).

var carregaFuso = sync.OnceValue(func() *time.Location {
	nome := os.Getenv("TZ_APP")
	if nome == "" {
		nome = "America/Sao_Paulo" // o público do StudFy é brasileiro
	}

	loc, err := time.LoadLocation(nome)
	if err != nil {
		log.Printf("⚠️  Fuso %q não encontrado (%v). Usando o do servidor — as datas podem cair no dia errado.", nome, err)
		return time.Local
	}
	return loc
})

// Fuso devolve o fuso em que "o dia" é contado para o usuário.
func Fuso() *time.Location { return carregaFuso() }

// Agora é a hora atual no fuso do usuário.
func Agora() time.Time { return time.Now().In(Fuso()) }

// InicioDoDia devolve a meia-noite do dia em que `t` cai, no fuso do usuário.
// É o que define a qual dia um registro pertence.
func InicioDoDia(t time.Time) time.Time {
	l := Fuso()
	t = t.In(l)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, l)
}

// Hoje é a meia-noite de hoje no fuso do usuário.
func Hoje() time.Time { return InicioDoDia(time.Now()) }

// MesmoDia diz se duas datas caem no mesmo dia para o usuário.
func MesmoDia(a, b time.Time) bool { return InicioDoDia(a).Equal(InicioDoDia(b)) }

// DiasEntre conta quantos dias separam duas datas (pelo dia, não pelas horas).
func DiasEntre(de, ate time.Time) int {
	return int(InicioDoDia(ate).Sub(InicioDoDia(de)).Hours() / 24)
}
