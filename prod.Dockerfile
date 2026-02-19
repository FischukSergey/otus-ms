FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Устанавливаем swag CLI для генерации swagger docs
RUN go install github.com/swaggo/swag/cmd/swag@v1.8.1

# Копируем исходный код
COPY . .

# Генерируем swagger docs для main-service
RUN /go/bin/swag init -g cmd/main-service/main.go -o api/mainservice \
    --parseInternal \
    --parseDependency \
    --exclude internal/handlers/auth

# Собираем бинарник main-service
RUN CGO_ENABLED=0 go build -o main-service ./cmd/main-service

# Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Копируем только исполняемый файл
COPY --from=builder /app/main-service .

# Создаем директории для логов и файлов
RUN mkdir -p logs data/files

# Expose портов
EXPOSE 38080 33000

# Запускаем с указанием на production конфиг
CMD ["./main-service", "-config", "/app/configs/config.prod.yaml"]

