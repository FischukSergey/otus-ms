# Keycloak - Быстрый старт 🚀

## Шаг 2: Настройка GitHub Secrets

Добавьте в GitHub → Settings → Secrets:

```
KEYCLOAK_DB_PASSWORD=your_strong_db_password
KEYCLOAK_ADMIN_USER=admin
KEYCLOAK_ADMIN_PASSWORD=your_strong_admin_password
KEYCLOAK_HOSTNAME=yourdomain.com
```

**Важно:** Указывайте основной домен БЕЗ поддомена (Keycloak будет на `/auth`)

## Шаг 3: Запуск Keycloak

1. GitHub → Actions → "Deploy Keycloak to Production (Manual)"
2. Run workflow:
   - **target:** `all`
   - **action:** `up`

## Шаг 4: Настройка Nginx

**ВАЖНО:** В nginx конфиге обязательно `/` в конце:
```nginx
location /auth/ {
    proxy_pass http://127.0.0.1:8080/;  # Обязательно / в конце!
}
```

## Шаг 5: Проверка

Откройте в браузере:
```
https://yourdomain.com/auth/
```

Войдите с credentials из секретов:
- Username: `admin` (или ваше значение)
- Password: из `KEYCLOAK_ADMIN_PASSWORD`

1. Создайте realm для вашего приложения
2. Создайте клиента (client)
3. Добавьте пользователей

### Управление

Через GitHub Actions workflow:
- **Перезапустить:** target=`keycloak`, action=`restart`
- **Логи:** target=`all`, action=`logs`
- **Остановить:** target=`all`, action=`down`

### Troubleshooting

**502 Bad Gateway от nginx?**
```bash
ssh root@your-vps
docker ps | grep keycloak  # Проверить, что контейнер запущен
docker logs otus-keycloak-prod  # Логи Keycloak
tail -f /var/log/nginx/keycloak-error.log  # Логи nginx
```
