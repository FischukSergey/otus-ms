# GitHub Secrets для CI/CD

## Необходимые секреты

### 1. Selectel Container Registry (для деплоя)

| Секрет | Описание | Пример |
|--------|----------|--------|
| `SELECTEL_REGISTRY_OTUS_USERNAME_PROD` | Username от Selectel Registry | `otus-microservice-be` |
| `SELECTEL_REGISTRY_OTUS_TOKEN_PROD` | Token от Selectel Registry | `docker.******************` |

### 2. VPS Доступ (для деплоя)

| Секрет | Описание | Пример |
|--------|----------|--------|
| `VPS_OTUS_HOST` | IP адрес или домен VPS | `185.123.45.67` |
| `VPS_OTUS_USER` | SSH пользователь | `root` |
| `VPS_OTUS_SSH_KEY` | Приватный SSH ключ | `-----BEGIN RSA PRIVATE KEY-----...` |

### 3. Keycloak (для интеграционных тестов Auth-Proxy)

| Секрет | Описание | Обязательный? |
|--------|----------|---------------|
| `KEYCLOAK_CLIENT_SECRET` | Client Secret из Keycloak для `auth-proxy` клиента | ✅ Да |
| `TEST_KEYCLOAK_USERNAME` | Username тестового пользователя | ⚠️ Опционально (default: `test@example.com`) |
| `TEST_KEYCLOAK_PASSWORD` | Пароль тестового пользователя | ⚠️ Опционально (default: `test123`) |
