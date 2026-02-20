# Настройка Keycloak для Auth-Proxy

Этот документ описывает шаги по настройке Keycloak для работы с Auth-Proxy микросервисом.

## Предварительные требования

- Keycloak должен быть запущен и доступен по адресу: https://fishouk-otus-ms.ru
- Доступ к Keycloak Admin Console

## Шаг 1: Доступ к Admin Console

1. Откройте браузер и перейдите по адресу: https://fishouk-otus-ms.ru/auth/admin/
2. Войдите используя admin credentials

## Шаг 2: Создание или выбор Realm

1. В левом верхнем углу нажмите на выпадающий список realms
2. Если realm `otus-ms` уже существует - выберите его
3. Если нет - создайте новый:
   - Нажмите "Create Realm"
   - Name: `otus-ms`
   - Enabled: `ON`
   - Нажмите "Create"

## Шаг 3: Создание Client для Auth-Proxy

1. В левом меню выберите **Clients**
2. Нажмите **Create client**

### 3.1 General Settings

- **Client type**: `OpenID Connect`
- **Client ID**: `auth-proxy`
- Нажмите **Next**

### 3.2 Capability config

- **Client authentication**: `ON` (это сделает client confidential)
- **Authorization**: `OFF`
- **Authentication flow**:
  - ✅ Standard flow
  - ✅ Direct access grants (для Password Grant Flow)
  - ✅ Service accounts roles (опционально, для будущих межсервисных вызовов)
  - ❌ Implicit flow (не используется)
- Нажмите **Next**

### 3.3 Login settings

- **Root URL**: (оставьте пустым)
- **Home URL**: (оставьте пустым)
- **Valid redirect URIs**: 
  - `http://localhost:38081/*` (для локальной разработки)
  - `https://fishouk-otus-ms.ru/*` (для продакшена, если нужно)
- **Valid post logout redirect URIs**: `+` (разрешить все из Valid redirect URIs)
- **Web origins**: `*` (или конкретные домены)

Нажмите **Save**

## Шаг 4: Получение Client Secret

1. Перейдите на вкладку **Credentials**
2. Скопируйте **Client secret**
3. Сохраните его в безопасном месте (понадобится для конфигурации)

**Важно:** Client Secret - это конфиденциальная информация. Не храните его в коде или git репозитории!

## Шаг 5: Настройка конфигурационных файлов

### Для локальной разработки

1. Скопируйте example файл:
```bash
cp configs/config.auth-proxy.local.example.yaml configs/config.auth-proxy.local.yaml
```

2. Откройте `configs/config.auth-proxy.local.yaml` и замените `your-client-secret-here` на реальный Client Secret из Keycloak

```yaml
keycloak:
  url: "https://fishouk-otus-ms.ru/auth"
  realm: "otus-ms"
  client_id: "auth-proxy"
  client_secret: "abc123xyz456..."  # Ваш реальный secret
```

**Важно:** Файл `config.auth-proxy.local.yaml` находится в `.gitignore` - не коммитьте его!

### Для продакшена

1. На VPS создайте директорию для конфигов:
```bash
mkdir -p /root/otus-microservice/prod/auth-proxy/configs
```

2. Создайте файл `config.auth-proxy.prod.yaml` на VPS:
```bash
nano /root/otus-microservice/prod/auth-proxy/configs/config.auth-proxy.prod.yaml
```

3. Скопируйте содержимое из `config.auth-proxy.prod.example.yaml` и замените `your-client-secret-here` на реальный Client Secret

**Важно:** Конфиги с секретами хранятся только на VPS и в вашей локальной машине, НЕ в git!

## Шаг 6: Создание тестового пользователя

Для тестирования Auth-Proxy создайте тестового пользователя:

1. В левом меню выберите **Users**
2. Нажмите **Add user**

### 6.1 User details

- **Username**: `test@example.com`
- **Email**: `test@example.com`
- **Email verified**: `ON`
- **First name**: `Test`
- **Last name**: `User`
- Нажмите **Create**

### 6.2 Установка пароля

1. Перейдите на вкладку **Credentials**
2. Нажмите **Set password**
3. **Password**: `test123`
4. **Password confirmation**: `test123`
5. **Temporary**: `OFF` (чтобы не требовать смену пароля)
6. Нажмите **Save**
7. Подтвердите в диалоге

### 6.3 Назначение роли (опционально)

1. Перейдите на вкладку **Role mappings**
2. Нажмите **Assign role**
3. Выберите роль `user` (если она существует)
4. Нажмите **Assign**

## Шаг 7: Проверка настройки

### Запуск Auth-Proxy локально

```bash
# Убедитесь, что config.auth-proxy.local.yaml создан с реальным client secret
go run cmd/auth-proxy/main.go -config configs/config.auth-proxy.local.yaml
```

### Тест через curl

```bash
# Login
curl -X POST http://localhost:38081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "test@example.com",
    "password": "test123"
  }'
```

Ожидаемый ответ:

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 300,
  "refresh_expires_in": 1800,
  "token_type": "Bearer",
  "scope": "openid profile email"
}
```

### Тест Refresh Token

```bash
# Refresh (используйте refresh_token из предыдущего ответа)
curl -X POST http://localhost:38081/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }'
```

### Тест Logout

```bash
# Logout
curl -X POST http://localhost:38081/api/v1/auth/logout \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }'
```

Ожидаемый ответ: `204 No Content`

## Шаг 8: Настройка Token Settings (опционально)

Для настройки времени жизни токенов:

1. Перейдите в **Realm settings** → **Tokens**
2. Настройте следующие параметры:
   - **Access Token Lifespan**: `5m` (5 минут)
   - **Refresh Token Lifespan**: `30d` (30 дней)
   - **Client Session Idle**: `30m` (30 минут)
   - **Client Session Max**: `10h` (10 часов)
3. Нажмите **Save**

## Создание тестового пользователя для интеграционных тестов

Для запуска интеграционных тестов Auth-Proxy нужен тестовый пользователь.

### Шаги создания:

1. **Откройте Keycloak Admin Console:**
   ```
   https://fishouk-otus-ms.ru/auth/admin/
   ```

2. **Переключитесь на realm `otus-ms`** (выпадающий список в левом верхнем углу)

3. **Создайте тестового пользователя:**
   - В левом меню: **Users** → **Add user**
   - **Username**: `test@example.com`
   - **Email**: `test@example.com`
   - **Email Verified**: `ON` ✅
   - **Enabled**: `ON` ✅
   - Нажмите **Create**

4. **Установите пароль:**
   - После создания откройте вкладку **Credentials**
   - Нажмите **Set password**
   - **Password**: `test123`
   - **Temporary**: `OFF` (важно!)
   - Нажмите **Save**
   - Подтвердите в диалоге

5. **Проверьте пользователя:**
   ```bash
   curl -X POST https://fishouk-otus-ms.ru/auth/realms/otus-ms/protocol/openid-connect/token \
     -H "Content-Type: application/x-www-form-urlencoded" \
     -d "client_id=auth-proxy" \
     -d "client_secret=ваш-client-secret" \
     -d "grant_type=password" \
     -d "username=test@example.com" \
     -d "password=test123"
   ```

   Должны получить JSON с `access_token`, `refresh_token` и т.д.

### Использование в тестах

**Локально:**

1. Создайте конфиг и добавьте в него секрет:
   ```bash
   cp configs/config.auth-proxy.test.example.yaml configs/config.auth-proxy.test.yaml
   nano configs/config.auth-proxy.test.yaml
   # Замените client_secret на реальный
   ```

2. Запустите тесты:
   ```bash
   task test:integration
   ```

**В CI/CD (GitHub Actions):**

Добавьте в GitHub Secrets:
- `KEYCLOAK_CLIENT_SECRET` - client secret из Keycloak
- `TEST_KEYCLOAK_USERNAME` - username тестового пользователя (по умолчанию: `test@example.com`)
- `TEST_KEYCLOAK_PASSWORD` - пароль тестового пользователя (по умолчанию: `test123`)

Workflow автоматически подставит секреты через переменные окружения.

## Troubleshooting

### Ошибка "Invalid credentials"

- Проверьте, что пользователь существует и пароль установлен
- Убедитесь, что Email verified = ON
- Проверьте, что пользователь не заблокирован

### Ошибка "Client not found"

- Проверьте Client ID в конфигурации (`auth-proxy`)
- Убедитесь, что вы используете правильный realm (`otus-ms`)

### Ошибка "Invalid client secret"

- Проверьте, что Client Secret скопирован правильно
- Убедитесь, что переменная окружения `KEYCLOAK_CLIENT_SECRET` установлена
- Проверьте, что в конфигурации нет лишних пробелов

### Ошибка "Refresh token expired"

- Refresh token действителен только 30 дней
- После logout refresh token становится невалидным
- Нужно выполнить новый login

## Дополнительная информация

### Полезные ссылки

- Keycloak Admin Console: https://fishouk-otus-ms.ru/auth/admin/
- Keycloak Documentation: https://www.keycloak.org/documentation

### Структура токенов

#### Access Token (JWT)

```json
{
  "exp": 1707567890,
  "iat": 1707567590,
  "jti": "abc123...",
  "iss": "https://fishouk-otus-ms.ru/auth/realms/otus-ms",
  "aud": "account",
  "sub": "user-uuid-123",
  "typ": "Bearer",
  "azp": "auth-proxy",
  "preferred_username": "test@example.com",
  "email": "test@example.com",
  "email_verified": true,
  "realm_access": {
    "roles": ["user"]
  }
}
```

## Security Best Practices

1. **Никогда не коммитьте** Client Secret в git - используйте файлы из `.gitignore`
2. **Используйте HTTPS** в продакшене
3. **Ротируйте секреты** регулярно
4. **Минимальные права** - давайте только необходимые роли
5. **Мониторинг** - логируйте все попытки входа
6. **Rate Limiting** - ограничивайте количество попыток логина
7. **Конфиги на VPS** - храните файлы с секретами только на production сервере
8. **Права доступа** - `chmod 600` для конфигов с секретами на VPS

## Статус

✅ Документация готова  
✅ Тестовый пользователь создан  
✅ Client настроен
