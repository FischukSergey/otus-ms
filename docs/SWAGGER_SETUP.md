# Swagger UI — настройка и деплой

Swagger UI показывает API обоих микросервисов в одном интерфейсе с переключением через dropdown.

## Архитектура

```
Browser → nginx /swagger/          → swagger-ui container  (:38090)
Browser → nginx /api/main-service/ → main-service           (:38080/swagger/*)
Browser → nginx /api/auth-proxy/   → auth-proxy             (:38081/swagger/*)
```

Swagger UI контейнер запрашивает `doc.json` из браузера пользователя,
поэтому URL spec-файлов должны быть **публичными** (через nginx).

## 1. Деплой Swagger UI

Запустить через GitHub Actions: **Deploy Swagger UI to Production (Manual)** → action: `up`

Или вручную на VPS:
```bash
cd /root/otus-microservice/prod/swagger
docker compose -f docker-compose.swagger.prod.yml pull
docker compose -f docker-compose.swagger.prod.yml up -d
```

## 2. Конфигурация Nginx

Добавить в секцию `server { ... }` вашего nginx.conf:

```nginx
# Swagger UI
location /swagger/ {
    proxy_pass         http://127.0.0.1:38090/swagger/;
    proxy_set_header   Host              $host;
    proxy_set_header   X-Real-IP         $remote_addr;
    proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header   X-Forwarded-Proto $scheme;
}

# doc.json для main-service (запрашивается браузером из Swagger UI)
location /api/main-service/swagger/ {
    proxy_pass         http://127.0.0.1:38080/swagger/;
    proxy_set_header   Host              $host;
    proxy_set_header   X-Real-IP         $remote_addr;
    proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header   X-Forwarded-Proto $scheme;
}

# doc.json для auth-proxy (запрашивается браузером из Swagger UI)
location /api/auth-proxy/swagger/ {
    proxy_pass         http://127.0.0.1:38081/swagger/;
    proxy_set_header   Host              $host;
    proxy_set_header   X-Real-IP         $remote_addr;
    proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header   X-Forwarded-Proto $scheme;
}
```

После добавления — перезагрузить nginx:
```bash
nginx -t && systemctl reload nginx
```

## 3. URL для доступа

- Swagger UI:    https://fishouk-otus-ms.ru/swagger/
- Main Service spec:  https://fishouk-otus-ms.ru/api/main-service/swagger/doc.json
- Auth Proxy spec:    https://fishouk-otus-ms.ru/api/auth-proxy/swagger/doc.json

## 4. Обновление swagger docs локально

При изменении хендлеров нужно пересгенерировать docs:

```bash
# Main Service
swag init -g cmd/main-service/main.go -o api/mainservice \
  --parseInternal --parseDependency --exclude internal/handlers/auth

# Auth Proxy  
swag init -g cmd/auth-proxy/main.go -o api/authproxy \
  --parseInternal --parseDependency --exclude internal/handlers/user
```

Установка swag CLI (один раз):
```bash
go install github.com/swaggo/swag/cmd/swag@v1.8.1
```

Сгенерированные файлы (`api/mainservice/` и `api/authproxy/`) нужно коммитить.
В Dockerfile docs пересоздаются автоматически при каждом билде.
