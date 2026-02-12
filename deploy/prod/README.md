# Production Deployment

Раздельная инфраструктура для production:
- **База данных** - запускается один раз вручную, живет постоянно
- **API сервер** - автоматический деплой при каждом push в main

## Первоначальная настройка

### 1. Настройка GitHub Secrets

Добавьте в Settings → Secrets and variables → Actions:

- `VPS_OTUS_HOST` - IP адрес сервера
- `VPS_OTUS_USER` - SSH пользователь (обычно root)
- `VPS_OTUS_SSH_KEY` - SSH приватный ключ
- `SELECTEL_REGISTRY_OTUS_USERNAME_PROD` - username Selectel Registry
- `SELECTEL_REGISTRY_OTUS_TOKEN_PROD` - token Selectel Registry
- **`POSTGRES_PASSWORD`** - пароль для PostgreSQL (НОВЫЙ!)

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

## Миграции

Миграции применяются автоматически при старте API сервера.

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
