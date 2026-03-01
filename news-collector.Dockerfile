FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем бинарник news-collector
RUN CGO_ENABLED=0 go build -o news-collector ./cmd/news-collector

# Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Копируем только исполняемый файл
COPY --from=builder /app/news-collector .

# Expose порт для HTTP health check
EXPOSE 38082

# Запускаем с указанием на production конфиг
CMD ["./news-collector", "-config", "/app/configs/config.news-collector.prod.yaml"]
