# Развертывание Keycloak на Production

## Обзор

Keycloak развертывается на VPS в Docker контейнерах с отдельной PostgreSQL базой данных.

**Архитектура:**
- `otus-keycloak-postgres-prod` - PostgreSQL 16 для Keycloak (порт 5433)
- `otus-keycloak-prod` - Keycloak 26.5.2 (порты 8080, 8443)
- Общая сеть `otus_network` с другими сервисами проекта

## Предварительные требования

### 1. Настройка GitHub Secrets

Добавьте следующие секреты в Settings → Secrets and variables → Actions:

| `KEYCLOAK_DB_PASSWORD` | Пароль для PostgreSQL БД Keycloak | `strong_db_password_123` | При каждом запуске |
| `KEYCLOAK_ADMIN_USER` | Логин администратора Keycloak | `admin` | **Только при первом запуске** |
| `KEYCLOAK_ADMIN_PASSWORD` | Пароль администратора Keycloak | `strong_admin_password_456` | **Только при первом запуске** |
| `KEYCLOAK_HOSTNAME` | Домен или IP для Keycloak | `auth.yourdomain.com` | При каждом запуске |

**⚠️ Важно:** `KEYCLOAK_ADMIN_USER` и `KEYCLOAK_ADMIN_PASSWORD` используются **только при первом запуске** для создания admin пользователя. При последующих запусках эти переменные игнорируются. Для смены пароля используйте Admin Console.

**Существующие секреты** (уже должны быть настроены):
- `VPS_OTUS_HOST` - IP адрес VPS
- `VPS_OTUS_USER` - SSH пользователь (обычно `root`)
- `VPS_OTUS_SSH_KEY` - SSH ключ для подключения

## Развертывание

### Вариант 1: Первоначальное развертывание (рекомендуется)

Запускаем всё сразу - PostgreSQL + Keycloak:

1. Перейдите в GitHub → Actions → "Deploy Keycloak to Production (Manual)"
2. Нажмите "Run workflow"
3. Выберите параметры:
   - **target:** `all`
   - **action:** `up`
4. Нажмите "Run workflow"

**Примечание:** Прямой доступ по IP закрыт. Keycloak доступен только через nginx.

## Управление через GitHub Actions

### Доступные действия

| Action | Описание |
|--------|----------|
| `up` | Запустить сервисы |
| `down` | Остановить сервисы (данные сохраняются) |
| `restart` | Перезапустить сервисы |
| `logs` | Показать последние 200 строк логов |
| `recreate` | Пересоздать контейнеры |

### Доступные цели

| Target | Что управляется |
|--------|----------------|
| `all` | PostgreSQL + Keycloak вместе |
| `keycloak` | Только Keycloak |
| `database` | Только PostgreSQL |

### Примеры использования

**Перезапустить Keycloak:**
- target: `keycloak`
- action: `restart`

**Посмотреть логи БД:**
- target: `database`
- action: `logs`

**Остановить всё:**
- target: `all`
- action: `down`

## Конфигурация Nginx (ОБЯЗАТЕЛЬНО)

Keycloak настроен на работу через nginx с SSL. Прямой доступ закрыт.

## Директории на VPS

```
/root/otus-microservice/prod/keycloak/
├── .env                                        # Переменные окружения (секреты)
├── docker-compose.keycloak-all.prod.yml       # Оба сервиса
├── docker-compose.keycloak.prod.yml           # Только Keycloak
└── docker-compose.keycloak-db.prod.yml        # Только PostgreSQL
```

## Данные и Volumes

### PostgreSQL Data

- Volume: `otus_keycloak_postgres_data`
- Расположение: `/var/lib/docker/volumes/otus_keycloak_postgres_data/`

## Порты

| Сервис | Порт на хосте | Порт в контейнере | Описание |
|--------|---------------|-------------------|----------|
| Nginx | 443 | - | HTTPS (публичный доступ) |
| Nginx | 80 | - | HTTP (редирект на HTTPS) |
| Keycloak | 127.0.0.1:8080 | 8080 | HTTP (только для nginx) |
| PostgreSQL | 127.0.0.1:5433 | 5432 | БД (только для бэкапов) |

**Важно:** Все порты биндятся на `127.0.0.1` для безопасности. Публичный доступ только через nginx.

## Безопасность

1. ✅ **Порты на localhost** - Keycloak и PostgreSQL биндятся на `127.0.0.1`
2. ✅ **HTTPS через nginx** - SSL терминация на nginx
3. ✅ **Firewall настроен:**
   ```bash
   ufw allow 80/tcp    # HTTP (редирект)
   ufw allow 443/tcp   # HTTPS
   # 8080, 5433 закрыты автоматически (биндинг на 127.0.0.1)
   ufw enable
   ```

## Обновление Keycloak

1. Остановите Keycloak:
   - target: `keycloak`
   - action: `down`

2. Обновите версию образа в `docker-compose.keycloak.prod.yml`:
   ```yaml
   image: quay.io/keycloak/keycloak:26.5.2  # → новая версия
   ```

3. Закоммитьте изменения

4. Запустите Keycloak:
   - target: `keycloak`
   - action: `up`
