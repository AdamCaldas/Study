# Subir o StudFy no Render

Passo a passo do zero até alguém conseguir logar e navegar no sistema.
Tempo estimado: **10 a 15 minutos**, quase tudo esperando build.

---

## 1. Subir o Keycloak (quem faz o login)

Render → **New** → **Web Service** → conecte o repositório `AdamCaldas/Study`.

| Campo | Valor |
|---|---|
| Name | `studfy-keycloak` |
| Language / Runtime | **Docker** |
| Dockerfile Path | `./deploy/keycloak/Dockerfile` |
| Docker Build Context | `./deploy/keycloak` |
| Health Check Path | `/realms/studfy` |
| Instance Type | Free |

**Environment Variables:**

| Chave | Valor |
|---|---|
| `KC_BOOTSTRAP_ADMIN_USERNAME` | `admin` |
| `KC_BOOTSTRAP_ADMIN_PASSWORD` | uma senha forte sua (console do Keycloak) |
| `DEMO_PASSWORD` | a senha dos 3 usuários de teste |
| `KC_HOSTNAME_STRICT` | `false` |
| `KC_DB` | `dev-file` |

> `DEMO_PASSWORD` fica **só aqui**. O repositório é público, então nenhuma
> senha vai no código — o Keycloak troca `${DEMO_PASSWORD}` na importação.

Clique em **Create**. O primeiro boot demora alguns minutos (ele importa o realm).

**Confira:** abra `https://studfy-keycloak.onrender.com/realms/studfy`.
Tem que voltar um JSON com `"realm": "studfy"`. Se voltar, está pronto.

---

## 2. Apontar a API para ele

No serviço `studfy-api` que já existe → **Environment** → adicione:

| Chave | Valor |
|---|---|
| `KEYCLOAK_URL` | `studfy-keycloak.onrender.com` |
| `KEYCLOAK_ISSUER` | `studfy-keycloak.onrender.com` |
| `KEYCLOAK_REALM` | `studfy` |
| `KEYCLOAK_ONLY` | `false` |
| `ENABLE_LEGACY_AUTH` | `false` |
| `AUTO_MIGRATE` | `true` &nbsp;← só no primeiro deploy, depois volte para `false` |
| `DB_MAX_OPEN_CONNS` | `10` |
| `DB_MAX_IDLE_CONNS` | `4` |
| `FRONTEND_URL` | a URL do front |

> Pode informar só o hostname: a API acrescenta `https://` e `/realms/studfy`
> sozinha. **`KEYCLOAK_ONLY` precisa ser `false`** — com `true` nada é
> espelhado na tabela `users` e o `/v1/app/bootstrap` responde 404.

Salve. O Render reinicia sozinho.

**Confira:** `https://studfy-api.onrender.com/health` deve responder:

```json
{ "status": "ok", "database": "ok", "keycloak": "ok" }
```

Se `keycloak` vier `indisponível`, espere 1 minuto — a API tenta de novo
sozinha, sem precisar de redeploy.

---

## 3. Banco: rodar a migração

⚠️ **Só se o banco já tiver dados.** Banco novo pode pular.

```bash
pg_dump "$DATABASE_URL" > backup.sql                                   # backup primeiro
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/001_pre_migracao.sql
```

Depois suba a API uma vez com `AUTO_MIGRATE=true` e volte para `false`.

---

## 4. Entrar no sistema

Usuários já criados pelo realm (senha = o seu `DEMO_PASSWORD`):

| Login | Cargo | Enxerga |
|---|---|---|
| `admin@studfy.com` | admin | tudo, inclusive o painel `/v1/admin` |
| `professor@studfy.com` | teacher | turmas dele nascem como Sala de Aula |
| `aluno@studfy.com` | user | visão de aluno |

Quem quiser criar a própria conta pode: o cadastro está aberto na tela do
Keycloak (o novo usuário entra como aluno).

### Pegar um token para testar sem front

```bash
curl -X POST "https://studfy-keycloak.onrender.com/realms/studfy/protocol/openid-connect/token" \
  -d "client_id=studfy-app" \
  -d "username=aluno@studfy.com" \
  -d "password=SUA_DEMO_PASSWORD" \
  -d "grant_type=password"
```

Use o `access_token` que voltar:

```bash
curl https://studfy-api.onrender.com/v1/app/bootstrap \
  -H "Authorization: Bearer SEU_TOKEN"
```

---

## 5. O que o front precisa saber

| Item | Valor |
|---|---|
| API | `https://studfy-api.onrender.com` |
| Keycloak | `https://studfy-keycloak.onrender.com` |
| Realm | `studfy` |
| Client ID | `studfy-app` (público, PKCE — **sem** client secret) |
| Fluxo | Authorization Code + PKCE |
| Enviar token | `Authorization: Bearer <access_token>` |
| Primeira chamada | `GET /v1/app/bootstrap` |

Antes de publicar de verdade, troque `redirectUris` e `webOrigins` de `*` para
o domínio do front (em `deploy/keycloak/realm-studfy.json`, ou no console).

---

## Quando algo não funciona

| Sintoma | Causa quase certa |
|---|---|
| Deploy falha, nada responde | Faltou `DATABASE_URL` |
| `/health` diz `keycloak: indisponível` | `KEYCLOAK_URL` errada, ou o Keycloak ainda subindo |
| Todo token dá 401 "issuer diferente" | `KEYCLOAK_ISSUER` não é a URL **pública** |
| `/bootstrap` dá 404 com token válido | `KEYCLOAK_ONLY` está `true` — mude para `false` |
| Rotas protegidas dão 503 | Keycloak fora; a API se reconecta sozinha |
| Login entra em loop de redirect | Falta `KC_PROXY_HEADERS` (já vem no Dockerfile) |
| Primeira chamada demora ~50s | Plano free hiberna. É normal. |

> **Plano free:** Render e Supabase hibernam sem uso. Antes de mostrar para
> alguém, abra o `/health` uma vez para "acordar" os serviços.
