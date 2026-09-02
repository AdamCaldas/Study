# Rodando os testes

## O básico (não precisa de nada)

```bash
go test ./...
```

Cobre as regras de permissão, a escada de revisão, a paginação e a conversão de
horários. Roda em segundos.

## Com banco de verdade

Alguns testes conferem regras que dependem de JOINs — um banco falso não
provaria nada. Eles são **pulados** se você não passar o banco.

```bash
docker run -d --name pgtest \
  -e POSTGRES_PASSWORD=t -e POSTGRES_USER=studfy -e POSTGRES_DB=studfy \
  -p 55470:5432 postgres:17-alpine

DATABASE_URL_TEST="postgres://studfy:t@localhost:55470/studfy?sslmode=disable" \
  go test ./... -v
```

Para apagar depois: `docker rm -f pgtest`

## Verificação de código

```bash
golangci-lint run ./...
```

Precisa passar com **0 problemas** antes de qualquer commit. Foi ele que
encontrou uma consulta sem parâmetro que deixava um relatório vazio sem dar
erro nenhum.

Instalar: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`

## O que os testes protegem

| Área | Regra travada |
|---|---|
| Permissão de turma | Aluno não apaga turma nem se promove a monitor |
| Leitura de material | Quem não é da turma não lê o caderno |
| Gabarito | Aluno nunca recebe a resposta certa |
| Dados pessoais | `PublicUser` não pode ganhar CPF nem e-mail |
| Revisão espaçada | Intervalos 1→3→7→15→30→60, travando no topo |
| Paginação | Ninguém pede a tabela inteira aumentando o `limit` |
| Horários | Entrada inválida não vira meia-noite silenciosamente |

Antes de mexer em permissão, rode os testes: vários deles existem porque a
falha correspondente **já esteve no ar**.
