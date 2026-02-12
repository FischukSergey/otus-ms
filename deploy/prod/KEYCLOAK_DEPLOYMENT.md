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

### 2. Проверка сети на VPS

Убедитесь, что сеть `otus_network` существует:

```bash
ssh root@your-vps-ip
docker network ls | grep otus_network
```

Если сети нет, она будет создана автоматически при первом запуске основной БД.

## Развертывание

### Вариант 1: Первоначальное развертывание (рекомендуется)

Запускаем всё сразу - PostgreSQL + Keycloak:

1. Перейдите в GitHub → Actions → "Deploy Keycloak to Production (Manual)"
2. Нажмите "Run workflow"
3. Выберите параметры:
   - **target:** `all`
   - **action:** `up`
4. Нажмите "Run workflow"

### Вариант 2: Поэтапное развертывание

#### Шаг 1: Запустить PostgreSQL

1. GitHub → Actions → "Deploy Keycloak to Production (Manual)"
2. Run workflow:
   - **target:** `database`
   - **action:** `up`

#### Шаг 2: Запустить Keycloak

1. GitHub → Actions → "Deploy Keycloak to Production (Manual)"
2. Run workflow:
   - **target:** `keycloak`
   - **action:** `up`

## Проверка развертывания

### На VPS

```bash
# Подключиться к VPS
ssh root@your-vps-ip

# Проверить контейнеры
docker ps | grep keycloak

# Должны быть запущены:
# otus-keycloak-postgres-prod
# otus-keycloak-prod

# Проверить логи PostgreSQL
docker logs otus-keycloak-postgres-prod --tail 50

# Проверить логи Keycloak
docker logs otus-keycloak-prod --tail 50

# Проверить health
docker exec otus-keycloak-prod curl -f http://localhost:8080/health/ready
```

### Веб интерфейс

1. Откройте браузер: `https://auth.yourdomain.com` (ваш домен)
2. Нажмите "Administration Console"
3. Войдите с credentials из секретов:
   - Username: значение `KEYCLOAK_ADMIN_USER`
   - Password: значение `KEYCLOAK_ADMIN_PASSWORD`

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

## Настройка после развертывания

### 1. Создание первого Realm

1. Войдите в Admin Console
2. Наведите на "Master" → "Create realm"
3. Введите имя realm (например, `my-application`)
4. Нажмите "Create"

### 2. Создание клиента

1. В вашем realm: Clients → "Create client"
2. Укажите Client ID (например, `my-app`)
3. Настройте Valid redirect URIs
4. Сохраните

### 3. Создание пользователей

1. Users → "Add user"
2. Заполните данные
3. Credentials → Set password
4. Сохраните

## Конфигурация Nginx (ОБЯЗАТЕЛЬНО)

Keycloak настроен на работу через nginx с SSL. Прямой доступ закрыт.

### Keycloak не запускается

```bash
# Проверить логи
docker logs otus-keycloak-prod

# Проверить health БД
docker exec otus-keycloak-postgres-prod pg_isready -U keycloak_user

# Проверить переменные окружения
docker exec otus-keycloak-prod env | grep KC_
```

### Не могу войти в Admin Console

**Причина:** Пароль из .env используется только при первом запуске

**Решение 1: Вспомнить/найти правильный пароль**
```bash
# Пароль хранится в БД, а не в .env
# Попробуйте пароль, который использовали при первом запуске
```

**Решение 2: Сбросить пароль через CLI**
```bash
docker exec -it otus-keycloak-prod bash

# Войти в kcadm (используйте текущий пароль)
/opt/keycloak/bin/kcadm.sh config credentials \
  --server http://localhost:8080 \
  --realm master \
  --user admin \
  --password CURRENT_PASSWORD

# Установить новый пароль
/opt/keycloak/bin/kcadm.sh set-password \
  --username admin \
  --new-password NEW_PASSWORD
```

**Решение 3: Пересоздать всё (потеря данных!)** ⚠️
```bash
# ВНИМАНИЕ: Удаляет ВСЕ данные Keycloak!
docker compose -f docker-compose.keycloak.prod.yml down -v
# Обновите пароль в .env
docker compose -f docker-compose.keycloak.prod.yml up -d
# Теперь будет использован пароль из .env
```

### Ошибка подключения к БД

```bash
# Проверить, что PostgreSQL запущен
docker ps | grep keycloak-postgres

# Проверить сеть
docker network inspect otus_network

# Проверить connectivity
docker exec otus-keycloak-prod ping otus-keycloak-postgres-prod
```

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

### Проверка volumes

```bash
# Список volumes
docker volume ls | grep keycloak

# Информация о volume
docker volume inspect otus_keycloak_postgres_data
```

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
4. **Измените пароли** в GitHub Secrets перед развертыванием
5. **Регулярно обновляйте** Keycloak до последней версии
6. **Мониторьте логи** nginx и Keycloak

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

## Полезные команды

```bash
# Подключиться к PostgreSQL
docker exec -it otus-keycloak-postgres-prod psql -U keycloak_user -d keycloak_db

# Посмотреть список таблиц
docker exec -it otus-keycloak-postgres-prod psql -U keycloak_user -d keycloak_db -c "\dt"

# Проверить размер БД
docker exec -it otus-keycloak-postgres-prod psql -U keycloak_user -d keycloak_db -c "SELECT pg_size_pretty(pg_database_size('keycloak_db'));"

# Войти в контейнер Keycloak
docker exec -it otus-keycloak-prod bash

# Проверить метрики Keycloak
curl http://localhost:8080/metrics

# Проверить health
curl http://localhost:8080/health
curl http://localhost:8080/health/ready
curl http://localhost:8080/health/live
```

## Следующие шаги

После успешного развертывания:

1. ✅ Настройте realm для вашего приложения
2. ✅ Создайте клиентов (clients)
3. ✅ Настройте роли и группы
4. ✅ Добавьте пользователей
5. ✅ Интегрируйте с вашим backend
6. ⏳ Настройте резервное копирование (следующая итерация)
7. ⏳ Настройте мониторинг (следующая итерация)
