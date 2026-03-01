# Patroni — отказоустойчивый PostgreSQL (лайт: 1 primary + 1 replica)

Локальный кластер для демонстрации: один etcd, два узла Patroni (node1 = primary с существующими данными, node2 = replica). Существующие данные БД сохраняются; приложение подключается к primary по порту 38432 (как и к обычному postgres).

## Предварительные условия

1. **Один раз создать пользователя репликации** в текущей БД (пока работает обычный postgres):

   Подключитесь к postgres (например: `psql -h localhost -p 38432 -U otus_ms -d otus_ms` или через контейнер) и выполните:

   ```sql
   CREATE USER replicator WITH REPLICATION ENCRYPTED PASSWORD 'replicate';
   ```

   Если пользователь уже есть — шаг можно пропустить.

2. **Обязательно остановить обычный postgres** (иначе порт 38432 будет занят и Patroni не поднимется):

   ```bash
   docker compose -f deploy/local/docker-compose.local.yml --profile db stop postgres
   ```

   Проверить, что порт свободен: `lsof -i :38432` или `docker ps -a | grep postgres`. Убедитесь, что volume не удалялся (`docker volume ls | grep otusms_postgres_data_local`).

## Запуск кластера Patroni

Из корня проекта:

```bash
# Сборка образа (один раз)
docker compose -f deploy/local/docker-compose.patroni.yml build

# Запуск (etcd → node1 → node2)
docker compose -f deploy/local/docker-compose.patroni.yml up -d

# Проверка
docker compose -f deploy/local/docker-compose.patroni.yml ps
```

Приложение (main-service) продолжает использовать `configs/config.local.yaml`: host `localhost`, port `38432` — порт проброшен с node1. Данные те же, что были в обычном postgres.

Если main-service запущен в Docker (из `docker-compose.local.yml`), по умолчанию он в другой сети и не увидит `patroni-node1`. Либо запускайте main-service на хосте (подключение к localhost:38432), либо подключите сервис к сети `otusms_patroni_network`.

Проверка репликации (в контейнере node1):

```bash
docker exec -it otusms-patroni-node1-local psql -U otus_ms -d otus_ms -c "SELECT * FROM pg_stat_replication;"
```

## Откат к обычному Postgres (обратимость)

1. Остановить кластер Patroni **без удаления volume**:

   ```bash
   docker compose -f deploy/local/docker-compose.patroni.yml down
   ```

   Не используйте `down -v`: иначе будет удалён только volume реплики (`otusms_patroni_replica_data_local`). Volume с данными primary (`otusms_postgres_data_local`) в этом compose объявлен как `external`, поэтому `down` его не трогает.

2. Запустить обычный postgres с тем же volume:

   ```bash
   docker compose -f deploy/local/docker-compose.local.yml --profile db up -d postgres
   ```

3. Проверить, что приложение видит те же данные.

После отката в каталоге данных primary могут остаться параметры, добавленные Patroni (например `wal_level`, `max_wal_senders`). Для одиночного инстанса они не мешают.

## Файлы

| Файл | Назначение |
|------|------------|
| `deploy/local/patroni/Dockerfile` | Образ: postgres:16-alpine + Patroni + python3 + etcd-клиент |
| `deploy/local/patroni/patroni-node1.yml` | Конфиг node1 (bootstrap existing_data) |
| `deploy/local/patroni/patroni-node2.yml` | Конфиг node2 (replica) |
| `deploy/local/docker-compose.patroni.yml` | Compose: etcd, node1, node2 |

Существующие файлы (`docker-compose.local.yml`, `configs/config.local.yaml`) не изменяются.

## Если порт 38432 уже занят

Ошибка `Bind for 0.0.0.0:38432 failed: port is already allocated` значит, что обычный postgres ещё запущен. Остановите его и при необходимости уберите контейнеры Patroni, затем запустите снова:

```bash
docker compose -f deploy/local/docker-compose.local.yml --profile db stop postgres
docker compose -f deploy/local/docker-compose.patroni.yml down
docker compose -f deploy/local/docker-compose.patroni.yml up -d
```

## Если main-service выдаёт «connection reset by peer»

Такое бывает, если до порта 38432 ещё не поднялся настоящий Postgres (например, node1 ещё в bootstrap) или на порту слушает другой процесс.

1. **Убедиться, что поднят именно Patroni node1 и он healthy:**
   ```bash
   docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "patroni|postgres"
   ```
   У `otusms-patroni-node1-local` должно быть `Up (healthy)` и `0.0.0.0:38432->5432/tcp`.

2. **Проверить подключение с хоста:**
   ```bash
   psql -h 127.0.0.1 -p 38432 -U otus_ms -d otus_ms -c "SELECT 1"
   ```
   Если здесь тоже «connection reset» — проблема на стороне контейнера (логи node1: `docker logs otusms-patroni-node1-local`). Если psql подключается, а приложение нет — попробуйте в `configs/config.local.yaml` явно задать `host: 127.0.0.1` вместо `localhost`, чтобы не ходить по IPv6.

3. **Проверить, кто слушает порт:**
   ```bash
   lsof -i :38432
   ```
   Должен быть только процесс Docker (проброс в контейнер node1).
