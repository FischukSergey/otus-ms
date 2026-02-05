# Production Deployment

Раздельная инфраструктура для production:
- **База данных** - запускается один раз вручную, живет постоянно
- **API сервер** - автоматический деплой при каждом push в main

## Архитектура

```
┌─────────────────────────────────────────┐
│ Production Server                       │
├─────────────────────────────────────────┤
│                                         │
│  Docker Network: otus_network           │
│  ├── otus-postgres-prod (постоянно)    │
│  │   └── Volume: postgres_data_prod    │
│  │                                     │
│  └── otus-microservice-be-prod         │
│      └── Обновляется при каждом push   │
│                                         │
└─────────────────────────────────────────┘
```

## Первоначальная настройка

### 1. Настройка GitHub Secrets

Добавьте в Settings → Secrets and variables → Actions:

- `VPS_OTUS_HOST` - IP адрес сервера
- `VPS_OTUS_USER` - SSH пользователь (обычно root)
- `VPS_OTUS_SSH_KEY` - SSH приватный ключ
- `SELECTEL_REGISTRY_OTUS_USERNAME_PROD` - username Selectel Registry
- `SELECTEL_REGISTRY_OTUS_TOKEN_PROD` - token Selectel Registry
- **`POSTGRES_PASSWORD`** - пароль для PostgreSQL (НОВЫЙ!)

### 2. Создание конфигурации на сервере

На VPS сервере создайте:

```bash
# Директория для БД
mkdir -p /root/otus-microservice/prod/db

# Директория для API
mkdir -p /root/otus-microservice/prod/be/configs

# Создайте config.prod.yaml
nano /root/otus-microservice/prod/be/configs/config.prod.yaml
```

**Пример config.prod.yaml:**
```yaml
global:
  env: prod

log:
  level: info

servers:
  debug:
    addr: 0.0.0.0:33000
  client:
    addr: 0.0.0.0:38080
    allow_origins:
      - "https://fishouk-otus-ms.ru"

db:
  name: otus_ms_prod
  user: otus_ms_user
  password: ВАШ_ПАРОЛЬ_ИЗ_SECRETS  # Тот же что в POSTGRES_PASSWORD
  host: postgres  # Имя контейнера PostgreSQL
  port: "5432"
  ssl_mode: disable
```

## Деплой

### Первый запуск

1. **Запустите PostgreSQL** (один раз):
   - Перейдите: Actions → Deploy PostgreSQL to Production
   - Run workflow
   - Выберите: `up`
   - Нажмите: Run workflow

2. **Проверьте БД**:
   ```bash
   # На сервере
   docker ps | grep postgres
   docker logs otus-postgres-prod
   ```

3. **Задеплойте API** (автоматически при push в main):
   - Push в main триггерит CI/CD
   - Или вручную: Actions → CI & Deploy to Production → Run workflow

### Повседневное использование

**API сервер** деплоится автоматически при каждом push в `main`.

**PostgreSQL** управляется вручную через GitHub Actions:

| Действие | Когда использовать |
|----------|-------------------|
| `up` | Первый запуск или после остановки |
| `down` | Остановить БД (данные сохраняются!) |
| `restart` | Перезагрузить БД (при проблемах) |
| `logs` | Посмотреть логи БД |

## Проверка работы

На сервере:

```bash
# Статус контейнеров
docker ps

# Должны быть запущены:
# - otus-postgres-prod
# - otus-microservice-be-prod

# Логи API
docker logs otus-microservice-be-prod -f

# Логи БД
docker logs otus-postgres-prod -f

# Проверка API
curl http://localhost:38080/health

# Проверка подключения к БД из API контейнера
docker exec otus-microservice-be-prod nc -zv postgres 5432
```

## Миграции

Миграции применяются автоматически при старте API сервера.

Для проверки:

```bash
# Подключиться к БД
docker exec -it otus-postgres-prod psql -U otus_ms_user -d otus_ms_prod

# Проверить примененные миграции
SELECT * FROM schema_migrations;

# Проверить таблицы
\dt
```

## Backup и восстановление

### Backup

```bash
# На сервере
docker exec otus-postgres-prod pg_dump -U otus_ms_user otus_ms_prod > backup_$(date +%Y%m%d).sql
```

### Восстановление

```bash
# На сервере
cat backup_20260124.sql | docker exec -i otus-postgres-prod psql -U otus_ms_user -d otus_ms_prod
```

## Troubleshooting

### API не может подключиться к БД

1. Проверьте что PostgreSQL запущен:
   ```bash
   docker ps | grep postgres
   ```

2. Проверьте что оба контейнера в одной сети:
   ```bash
   docker network inspect otus_network
   ```

3. Проверьте пароль в `/root/otus-microservice/prod/be/configs/config.prod.yaml`

### PostgreSQL не запускается

1. Проверьте логи:
   ```bash
   docker logs otus-postgres-prod
   ```

2. Проверьте порт 5432:
   ```bash
   lsof -i :5432
   ```

3. Проверьте volume:
   ```bash
   docker volume ls | grep postgres
   ```

## Файлы

- `docker-compose.db.prod.yml` - PostgreSQL (запускается вручную)
- `docker-compose.be.prod.yml` - API сервер (автодеплой)
- `.github/workflows/deploy-database.yml` - ручной деплой БД
- `.github/workflows/ci.yml` - автодеплой API

## Security

⚠️ **Важно:**
- Пароль PostgreSQL хранится в GitHub Secrets
- БД доступна только внутри Docker сети
- Для внешнего доступа к БД используйте SSH tunnel

