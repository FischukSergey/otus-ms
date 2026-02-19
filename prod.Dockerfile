FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем бинарник main-service
# Swagger docs берутся из закоммиченных api/mainservice/ (генерируются локально через swag init)
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

