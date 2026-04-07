FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем бинарник alert-worker
RUN CGO_ENABLED=0 go build -o alert-worker ./cmd/alert-worker

# Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Копируем только исполняемый файл
COPY --from=builder /app/alert-worker .

# Expose порт для HTTP health check
EXPOSE 38084

# Запускаем с указанием на production конфиг
CMD ["./alert-worker", "-config", "/app/configs/config.alert-worker.prod.yaml"]
