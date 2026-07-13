# ==========================================================
# 🏗️  STAGE 1: BUILD
# ==========================================================
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Cacheia dependências: só re-baixa se go.mod/go.sum mudarem
COPY go.mod go.sum ./
RUN go mod download

# Copia o resto do código e compila um binário estático
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/api-server ./cmd/api

# ==========================================================
# 🚀 STAGE 2: RUNTIME (imagem final enxuta)
# ==========================================================
FROM alpine:3.20

WORKDIR /app

# Certificados raiz (para chamadas HTTPS ex.: Brevo, Google) e timezone
RUN apk add --no-cache ca-certificates tzdata

# Copia apenas o binário compilado
COPY --from=builder /app/api-server /app/api-server

EXPOSE 8080

ENTRYPOINT ["/app/api-server"]
