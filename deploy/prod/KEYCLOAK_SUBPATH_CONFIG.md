# Keycloak на подпути - Финальная конфигурация

## 🎯 Что настроено

Keycloak развертывается на **подпути `/auth`** общего домена, а не на отдельном поддомене.

## 📍 URL-адреса

```
Keycloak:        https://yourdomain.com/auth/
Admin Console:   https://yourdomain.com/auth/admin/
Realms:          https://yourdomain.com/auth/realms/my-realm
Account:         https://yourdomain.com/auth/realms/my-realm/account/
Health:          https://yourdomain.com/auth/health/ready
```

## ⚙️ Ключевые изменения

### 1. Docker Compose конфигурация

Добавлен параметр `KC_HTTP_RELATIVE_PATH=/auth`:

```yaml
environment:
  - KC_HOSTNAME=${KEYCLOAK_HOSTNAME}      # yourdomain.com (БЕЗ поддомена)
  - KC_HTTP_RELATIVE_PATH=/auth           # Keycloak на /auth
  - KC_HOSTNAME_STRICT=false
  - KC_HTTP_ENABLED=true
  - KC_PROXY=edge
```

### 2. Health check обновлен

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8080/auth/health/ready"]
  #                                                  ↑ Добавлен /auth
```

### 3. GitHub Secrets

```
KEYCLOAK_HOSTNAME=yourdomain.com  # Основной домен БЕЗ auth.
```

**НЕ используйте:** `auth.yourdomain.com` ❌

### 4. Nginx конфигурация

**Критически важно: `/` в конце proxy_pass!**

```nginx
server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    # SSL сертификаты
    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;

    # Keycloak на /auth
    location /auth/ {
        # ВАЖНО: / в конце обязателен!
        proxy_pass http://127.0.0.1:8080/;
        
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Port $server_port;
        
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # Backend API (если есть)
    location /api/ {
        proxy_pass http://127.0.0.1:38080/;
        # ... headers
    }

    # Frontend (если есть)
    location / {
        root /var/www/frontend;
        try_files $uri $uri/ /index.html;
    }
}
```

## ⚠️ Критически важно

### Слэш в конце proxy_pass

| proxy_pass | Запрос | Проксируется как | Результат |
|------------|--------|------------------|-----------|
| `http://127.0.0.1:8080/` | `/auth/admin` | `http://127.0.0.1:8080/admin` | ✅ Работает |
| `http://127.0.0.1:8080` | `/auth/admin` | `http://127.0.0.1:8080/auth/admin` | ❌ Не работает |

**Правило:** 
```nginx
location /auth/ {
    proxy_pass http://127.0.0.1:8080/;  # / в конце ОБЯЗАТЕЛЕН!
}
```

## 🚀 Развертывание

### Шаг 1: DNS
```
A-запись: yourdomain.com → VPS IP
```

### Шаг 2: GitHub Secrets
```
KEYCLOAK_HOSTNAME=yourdomain.com
```

### Шаг 3: Запуск через GitHub Actions
```
Workflow: Deploy Keycloak to Production (Manual)
Target: all
Action: up
```

### Шаг 4: Настройка Nginx

```bash
# На VPS
ssh root@vps

# Установить nginx и certbot
apt install -y nginx certbot python3-certbot-nginx

# Скопировать конфиг
nano /etc/nginx/sites-available/main
# Вставить содержимое из nginx-keycloak.conf
# Заменить yourdomain.com на ваш домен

# Включить конфиг
ln -s /etc/nginx/sites-available/main /etc/nginx/sites-enabled/
nginx -t

# Получить SSL
certbot --nginx -d yourdomain.com

# Перезагрузить
systemctl reload nginx
```

### Шаг 5: Проверка

```bash
# Health check
curl https://yourdomain.com/auth/health/ready

# OIDC Discovery
curl https://yourdomain.com/auth/realms/master/.well-known/openid-configuration | jq

# Должен вернуть URL с /auth в пути
```

## 🔍 Проверка конфигурации

### Проверка JWT токенов

После получения access token проверьте поле `iss`:

```json
{
  "iss": "https://yourdomain.com/auth/realms/my-realm",
  //     ↑ Должно быть /auth в пути
  "aud": "my-app",
  "sub": "user-id"
}
```

### Проверка Discovery endpoint

```bash
curl https://yourdomain.com/auth/realms/master/.well-known/openid-configuration | jq .issuer

# Должно вернуть:
"https://yourdomain.com/auth/realms/master"
```

## 🔧 Интеграция с приложениями

### Backend (Go)

```go
keycloakURL := "https://yourdomain.com/auth"
issuer := "https://yourdomain.com/auth/realms/my-realm"

// Keycloak middleware
middleware := keycloak.NewMiddleware(keycloak.Config{
    URL:    keycloakURL,
    Realm:  "my-realm",
    Issuer: issuer,
})
```

### Frontend (JavaScript)

```javascript
const keycloak = new Keycloak({
  url: 'https://yourdomain.com/auth',
  realm: 'my-realm',
  clientId: 'my-app'
});
```

### Python

```python
from keycloak import KeycloakOpenID

keycloak_openid = KeycloakOpenID(
    server_url="https://yourdomain.com/auth/",
    client_id="my-app",
    realm_name="my-realm"
)
```

## 📊 Мониторинг

```bash
# Health check
watch -n 5 'curl -s https://yourdomain.com/auth/health | jq'

# Логи Keycloak
docker logs -f otus-keycloak-prod

# Логи Nginx
tail -f /var/log/nginx/main-access.log
tail -f /var/log/nginx/main-error.log
```

## 🛠️ Troubleshooting

### 404 Not Found при доступе к /auth

**Причина:** Неправильный proxy_pass в nginx

**Решение:**
```nginx
# Проверьте наличие / в конце
location /auth/ {
    proxy_pass http://127.0.0.1:8080/;  # ← / в конце!
}
```

### 502 Bad Gateway

**Причина:** Keycloak не запущен или healthcheck не проходит

**Решение:**
```bash
docker ps | grep keycloak
docker logs otus-keycloak-prod
# Проверить healthcheck
docker exec otus-keycloak-prod curl http://localhost:8080/auth/health/ready
```

### Неправильный issuer в токенах

**Причина:** KC_HOSTNAME неправильный

**Решение:**
```bash
# Проверить .env
cat /root/otus-microservice/prod/keycloak/.env | grep KEYCLOAK_HOSTNAME
# Должно быть: KEYCLOAK_HOSTNAME=yourdomain.com (БЕЗ auth.)

# Пересоздать контейнер
cd /root/otus-microservice/prod/keycloak
docker compose -f docker-compose.keycloak.prod.yml restart
```

### Редирект на неправильный URL

**Причина:** KC_HTTP_RELATIVE_PATH не установлен

**Решение:**
```bash
# Проверить, что в docker-compose есть:
# - KC_HTTP_RELATIVE_PATH=/auth

docker compose -f docker-compose.keycloak.prod.yml down
docker compose -f docker-compose.keycloak.prod.yml up -d
```

## 📝 Резюме конфигурации

| Параметр | Значение | Комментарий |
|----------|----------|-------------|
| KC_HOSTNAME | `yourdomain.com` | Основной домен БЕЗ поддомена |
| KC_HTTP_RELATIVE_PATH | `/auth` | Подпуть для Keycloak |
| URL Keycloak | `https://yourdomain.com/auth/` | Публичный доступ |
| Nginx location | `/auth/` | Проксирование на Keycloak |
| proxy_pass | `http://127.0.0.1:8080/` | **Обязательно / в конце!** |
| SSL сертификат | `yourdomain.com` | Один сертификат на весь домен |

## ✅ Преимущества подхода

- ✅ Один домен для всех сервисов
- ✅ Один SSL сертификат
- ✅ Простая DNS конфигурация
- ✅ Можно добавлять другие сервисы на том же домене

## ⚠️ Важные моменты

1. **Слэш в proxy_pass обязателен** - без него не работает
2. **KC_HOSTNAME без поддомена** - только основной домен
3. **Healthcheck с /auth** - путь изменился
4. **Issuer содержит /auth** - это нормально
5. **Все URL с /auth** - admin, realms, account и т.д.

---

**Конфигурация готова к развертыванию!** 🚀
