
FROM golang:1.25-alpine AS builder

WORKDIR /app


COPY go.mod go.sum ./
RUN go mod download


COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/api-server ./cmd/api


FROM alpine:3.20

WORKDIR /app

# Certificados raiz (para chamadas HTTPS ex.: Brevo, Google) e timezone
RUN apk add --no-cache ca-certificates tzdata

# Copia apenas o binário compilado
COPY --from=builder /app/api-server /app/api-server

EXPOSE 8080

ENTRYPOINT ["/app/api-server"]
