# Patroni — отказоустойчивый PostgreSQL (лайт: 1 primary + 1 replica + HAProxy)

Локальный кластер для демонстрации: один etcd, два узла Patroni и HAProxy как единая точка входа.

**Топология портов на хосте:**

| Порт    | Назначение |
|---------|------------|
| `38432` | HAProxy → PostgreSQL primary (используется приложением) |
| `38480` | HAProxy stats: http://localhost:38480/ |
| `38008` | Patroni REST API node1 (patronictl, мониторинг) |
| `38009` | Patroni REST API node2 (patronictl, мониторинг) |

HAProxy определяет, кто сейчас primary, через HTTP-check на Patroni REST API: `GET /primary` возвращает **200** только на текущем primary. При failover трафик автоматически переключается на новую ноду — приложение продолжает работать через тот же порт **38432**.

## Предварительные условия

1. **Один раз создать пользователя репликации** в текущей БД (пока работает обычный postgres):

   Подключитесь к postgres (например: `psql -h localhost -p 38432 -U otus_ms -d otus_ms` или через контейнер) и выполните:

   ```sql
   CREATE USER replicator WITH REPLICATION ENCRYPTED PASSWORD 'replicate';
   ```

   Если пользователь уже есть — шаг можно пропустить.

2. **Обязательно остановить обычный postgres** (иначе порт 38432 будет занят):

   ```bash
   docker compose -f deploy/local/docker-compose.local.yml --profile db stop postgres
   ```

   Проверить, что порт свободен: `lsof -i :38432`. Убедитесь, что volume не удалялся (`docker volume ls | grep otusms_postgres_data_local`).

## Запуск кластера

Из корня проекта:

```bash
# Сборка образа (один раз)
docker compose -f deploy/local/docker-compose.patroni.yml build

# Запуск (etcd → node1 → node2 → haproxy)
docker compose -f deploy/local/docker-compose.patroni.yml up -d

# Проверка состояния контейнеров
docker compose -f deploy/local/docker-compose.patroni.yml ps
```

Приложение (main-service) использует `configs/config.local.yaml`: `host: 127.0.0.1`, `port: "38432"` — трафик идёт через HAProxy. Данные те же, что были в обычном postgres.

Если main-service запущен в Docker (из `docker-compose.local.yml`), запускайте его на хосте или подключите к сети `otusms_patroni_network`.

## Проверка кластера

```bash
# Роли нод (node1 = primary, node2 = replica)
docker exec otusms-patroni-node1-local patronictl -c /etc/patroni/patroni.yml list

# Статус репликации (подключение к node1)
docker exec otusms-patroni-node1-local psql -U otus_ms -d otus_ms -c "SELECT * FROM pg_stat_replication;"

# Проверка подключения через HAProxy
psql -h 127.0.0.1 -p 38432 -U otus_ms -d otus_ms -c "SELECT 1"
```

Страница статуса HAProxy: http://localhost:38480/
- Зелёный (`UP`) — нода принимает трафик (это текущий primary).
- Красный (`DOWN`) — нода недоступна или это replica.

## Сценарий «нагрузка + failover PostgreSQL»

Показывает автоматическое переключение на реплику при падении primary:

```bash
# Терминал 1: нагрузка на /ready (проверка БД)
while true; do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:38080/ready; sleep 0.2; done

# Терминал 2: в момент нагрузки остановить primary (node1)
docker stop otusms-patroni-node1-local

# Через 10–30 секунд — Patroni переключает node2 в primary,
# HAProxy начинает направлять трафик на node2.
# Ответы в терминале 1 снова станут 200.

# Убедиться, что node2 стал primary:
docker exec otusms-patroni-node2-local patronictl -c /etc/patroni/patroni.yml list

# Восстановление: поднять node1 обратно (станет replica)
docker start otusms-patroni-node1-local
```

**Ожидаемые фазы в терминале 1:**
1. Первые ~30 с: 200 (нагрузка на primary node1 через HAProxy).
2. После `docker stop` ~10–30 с: 503/ошибки (failover в процессе).
3. После переключения: снова 200 (трафик идёт на node2 через тот же 38432).

## Откат к обычному Postgres (обратимость)

1. Остановить кластер **без удаления volume**:

   ```bash
   docker compose -f deploy/local/docker-compose.patroni.yml down
   ```

   Не используйте `down -v`: volume реплики будет удалён, но volume primary (`otusms_postgres_data_local`) объявлен как `external` — `down` его не трогает.

2. Запустить обычный postgres с тем же volume:

   ```bash
   docker compose -f deploy/local/docker-compose.local.yml --profile db up -d postgres
   ```

3. Проверить, что приложение видит те же данные.

## Файлы

| Файл | Назначение |
|------|------------|
| `deploy/local/patroni/Dockerfile` | Образ: postgres:16-alpine + Patroni + python3 + etcd-клиент |
| `deploy/local/patroni/patroni-node1.yml` | Конфиг node1 (bootstrap existing_data) |
| `deploy/local/patroni/patroni-node2.yml` | Конфиг node2 (replica) |
| `deploy/local/patroni/haproxy.cfg` | Конфиг HAProxy (единая точка входа на primary) |
| `deploy/local/docker-compose.patroni.yml` | Compose: etcd, node1, node2, haproxy |

Существующие файлы (`docker-compose.local.yml`, `configs/config.local.yaml`) не изменяются.

## Если порт 38432 уже занят

Ошибка `Bind for 0.0.0.0:38432 failed` — обычный postgres ещё запущен или остался контейнер Patroni. Остановите и перезапустите:

```bash
docker compose -f deploy/local/docker-compose.local.yml --profile db stop postgres
docker compose -f deploy/local/docker-compose.patroni.yml down
docker compose -f deploy/local/docker-compose.patroni.yml up -d
```

## Если main-service выдаёт «connection reset by peer»

Такое бывает, пока HAProxy или Patroni ещё в фазе запуска.

1. **Проверить состояние контейнеров:**
   ```bash
   docker compose -f deploy/local/docker-compose.patroni.yml ps
   ```
   `otusms-patroni-haproxy-local` должен быть `Up (healthy)`.

2. **Проверить подключение через HAProxy:**
   ```bash
   psql -h 127.0.0.1 -p 38432 -U otus_ms -d otus_ms -c "SELECT 1"
   ```

3. **Проверить, кто слушает порт 38432:**
   ```bash
   lsof -i :38432
   ```
   Должен быть только процесс Docker (контейнер HAProxy).

4. **Проверить статус HAProxy:** открыть http://localhost:38480/ — хотя бы одна нода должна быть зелёной.
