# Руководство по тестированию

Быстрый старт для запуска всех типов тестов в проекте.

## 🚀 Быстрый запуск

### Unit тесты

```bash
task test:unit
```

Или напрямую:
```bash
go test -race -count=1 -short ./...
```

### Интеграционные тесты

```bash
# 1. Подготовка (только первый раз)
cp configs/config.auth-proxy.test.example.yaml configs/config.auth-proxy.test.yaml

# 2. Отредактируйте config.auth-proxy.test.yaml:
#    - Замените client_secret на реальный из Keycloak
nano configs/config.auth-proxy.test.yaml

# 3. Запуск всех интеграционных тестов
task test:integration
```

### Все тесты разом

```bash
task tests
```

## 📝 Подробности

### Интеграционные тесты - что проверяется?

#### Main Service (`user_test.go`)
- ✅ Создание пользователя (POST /api/v1/users) с JWT токеном
- ✅ Получение пользователя (GET /api/v1/users/{uuid}) с JWT токеном
- ✅ Мягкое удаление (DELETE /api/v1/users/{uuid}) с JWT токеном
- ✅ Валидация запросов
- ✅ Health check
- ✅ **RBAC проверки (6 тестов):**
  - Админ может создавать пользователей (200)
  - User не может создавать (403 Forbidden)
  - User не может читать других (403 Forbidden)
  - User не может удалять (403 Forbidden)
  - Без токена 401 Unauthorized
  - Невалидный токен 401 Unauthorized

#### Auth-Proxy (`auth_test.go`)
- ✅ Login - получение токенов
- ✅ Refresh - обновление access token
- ✅ Logout - инвалидация токенов
- ✅ Валидация credentials
- ✅ Полный флоу: Login → Refresh → Logout
- ✅ Health check

### Как работают интеграционные тесты?

1. **Автоматически поднимаются Docker контейнеры:**
   - PostgreSQL (порт 38433)
   - Main Service API (порт 8081)
   - Auth-Proxy (порт 38081)

2. **JWT токены для тестов:**
   - Используется тестовый режим с `skip_verify: true` в конфигах
   - Генерируются HMAC JWT токены с ролями (admin/user)
   - Все защищённые endpoints требуют JWT токен

3. **Запускаются тесты** с тегом `integration`:
   ```bash
   go test -tags=integration -v ./tests/integration/...
   ```

4. **Контейнеры останавливаются** после тестов

### Ручной запуск для отладки

```bash
# Поднять тестовое окружение
task test:env:up

# Проверить что сервисы запустились
curl http://localhost:8081/health      # Main Service
curl http://localhost:38081/health     # Auth-Proxy

# Запустить тесты
go test -tags=integration -v ./tests/integration/...

# Или только Auth тесты
go test -tags=integration -v ./tests/integration -run TestAuth

# Посмотреть логи
docker logs otusms-api-test
docker logs otusms-auth-proxy-test

# Остановить окружение
task test:env:down
```

## 🔐 Настройка Auth-Proxy для тестов

### Локальная разработка

**Секреты хранятся в файлах (в .gitignore):**

```yaml
# configs/config.auth-proxy.test.yaml
keycloak:
  client_secret: "ваш-реальный-client-secret"
```

**Никаких `export` не нужно!** Просто отредактируйте файл.

### CI/CD (GitHub Actions)

**Секреты хранятся в GitHub Secrets:**

1. Откройте: **Settings** → **Secrets and variables** → **Actions**
2. Добавьте:
   - `KEYCLOAK_CLIENT_SECRET` - client secret из Keycloak
   - `TEST_KEYCLOAK_USERNAME` (опционально, default: `test@example.com`)
   - `TEST_KEYCLOAK_PASSWORD` (опционально, default: `test123`)

Workflow автоматически использует эти секреты через переменные окружения.

📖 **[Подробная документация по GitHub Secrets](.github/workflows/SECRETS.md)**

## 📋 Создание тестового пользователя в Keycloak

Для Auth-Proxy тестов нужен тестовый пользователь:

1. **Откройте Keycloak Admin Console:**
   ```
   https://fishouk-otus-ms.ru/auth/admin/
   ```

2. **Переключитесь на realm `otus-ms`**

3. **Создайте пользователя:**
   - **Users** → **Add user**
   - **Username**: `test@example.com`
   - **Email**: `test@example.com`
   - **Email Verified**: `ON` ✅
   - **Enabled**: `ON` ✅

4. **Установите пароль:**
   - **Credentials** → **Set password**
   - **Password**: `test123`
   - **Temporary**: `OFF`

5. **Проверьте:**
   ```bash
   curl -X POST http://localhost:38081/api/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"test@example.com","password":"test123"}'
   ```

## 🐛 Troubleshooting

### "Auth-Proxy недоступен"
- Проверьте что `config.auth-proxy.test.yaml` содержит правильный client_secret
- Убедитесь что тестовое окружение поднято: `task test:env:up`
- Проверьте логи: `docker logs otusms-auth-proxy-test`

### "Invalid credentials"
- Убедитесь что тестовый пользователь создан в Keycloak
- Проверьте что Email Verified = ON
- Проверьте username и password

### "Realm does not exist"
- Убедитесь что realm `otus-ms` создан в Keycloak
- Проверьте URL в конфиге (должен включать `/auth`)

## 📚 Дополнительные ресурсы

- [Документация по интеграционным тестам](tests/integration/README.md)
- [Настройка Keycloak для Auth-Proxy](deploy/prod/KEYCLOAK_AUTH_PROXY_SETUP.md)
- [Auth-Proxy API документация](cmd/auth-proxy/README.md)
- [Настройка GitHub Secrets](.github/workflows/SECRETS.md)
