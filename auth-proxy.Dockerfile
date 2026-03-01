FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем бинарник auth-proxy
# Swagger docs берутся из закоммиченных api/authproxy/ (генерируются локально через swag init)
RUN CGO_ENABLED=0 go build -o auth-proxy ./cmd/auth-proxy

# Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Копируем только исполняемый файл
COPY --from=builder /app/auth-proxy .

# Создаем директорию для логов
RUN mkdir -p logs

# Expose порт для API
EXPOSE 38081

# Запускаем с указанием на production конфиг
CMD ["./auth-proxy", "-config", "/app/configs/config.auth-proxy.prod.yaml"]
