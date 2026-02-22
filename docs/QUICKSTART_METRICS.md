# Быстрый старт - Prometheus метрики

### Собираемые метрики:
- `http_requests_total` - количество запросов (counter)
- `http_request_duration_seconds` - время выполнения (histogram)
- `http_request_size_bytes` - размер запроса (histogram)
- `http_response_size_bytes` - размер ответа (histogram)

### 2. Запустите приложение

```bash
go run cmd/main-service/*.go -config configs/config.local.yaml
```

### 3. Запустите Prometheus

```bash
docker compose -f deploy/local/docker-compose.local.yml --profile monitoring up -d
```

Откройте Prometheus UI: http://localhost:39000

## Проверка работы

### Тест 1: Метрики собираются
```bash
# Создайте несколько запросов
curl http://localhost:38080/
curl http://localhost:38080/health

# Проверьте метрики
curl http://localhost:39090/metrics | grep http_requests_total
```

Должны увидеть:
```
http_requests_total{method="GET",path="/",status="200"} 1
http_requests_total{method="GET",path="/health",status="200"} 1
```

### Тест 2: Prometheus scraping работает

В Prometheus UI (http://localhost:39000) выполните запрос:
```promql
http_requests_total
```

Или проверьте targets: http://localhost:39000/targets
- Должен быть `otusms-api` со статусом **UP**

### Тест 3: Графики работают

Создайте нагрузку:
```bash
for i in {1..50}; do curl -s http://localhost:38080/health > /dev/null; done
```

## Полезные PromQL запросы

```promql
# RPS (requests per second)
rate(http_requests_total[5m])

# Процент ошибок
sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) * 100

# 95-й перцентиль времени ответа
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Топ эндпоинтов по количеству запросов
topk(5, sum by (path) (http_requests_total))
```
