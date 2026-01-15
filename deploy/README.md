# Деплой OtusMS

## Локальная разработка

### С Docker Compose

```bash
# Запуск
docker compose -f deploy/local/docker-compose.local.yml up -d

# Просмотр логов
docker compose -f deploy/local/docker-compose.local.yml logs -f

# Остановка
docker compose -f deploy/local/docker-compose.local.yml down
```

### Без Docker (для разработки)

```bash
# Запуск сервиса
go run ./cmd/main-service -config configs/config.local.yaml

# Или через Taskfile
task build
./main-service -config configs/config.local.yaml
```

## Production деплой

### Предварительная настройка на сервере

1. **Создайте структуру директорий:**

```bash
sudo mkdir -p /root/otus-microservice/prod/be/{configs,logs,data/files}
```

2. **Создайте конфигурационный файл:**

```bash
# Скопируйте пример конфига
sudo nano /root/otus-microservice/prod/be/configs/config.prod.yaml
```

Используйте `configs/config.prod.example.yaml` как шаблон.

3. **Настройте GitHub Actions Runner** (для self-hosted):

```bash
# Установите и настройте runner на сервере
# https://github.com/your-repo/settings/actions/runners/new
```

### Автоматический деплой через GitHub Actions

При пуше в ветку `main` автоматически:
1. ✅ Запускается линтер
2. ✅ Выполняются unit тесты
3. ✅ Собирается Docker образ
4. ✅ Образ публикуется в Selectel Container Registry
5. ✅ Деплоится на production сервер

### Ручной деплой

```bash
# На сервере
cd /root/otus-microservice/prod/be

# Скачать новую версию docker-compose (если изменился)
# обычно делается через CI/CD

# Перезапустить контейнеры
sudo docker compose -f docker-compose.be.prod.yml down
sudo docker compose -f docker-compose.be.prod.yml pull
sudo docker compose -f docker-compose.be.prod.yml up -d

# Проверить статус
sudo docker ps | grep otus-microservice
sudo docker logs otus-microservice-be-prod -f
```

## Полезные команды

### Просмотр логов

```bash
# Production
sudo docker logs otus-microservice-be-prod -f

# Local
docker logs otus-microservice-be-local -f
```

### Проверка health

```bash
# Production (если healthcheck настроен)
curl http://localhost:38080/health

# Или через docker
sudo docker inspect otus-microservice-be-prod | grep Health -A 10
```

### Очистка Docker

```bash
# Удалить неиспользуемые образы
sudo docker system prune -f

# Удалить все (включая volumes) - ОСТОРОЖНО!
sudo docker system prune -af --volumes
```

## Структура проекта

```
/root/otus-microservice/prod/be/
├── configs/
│   └── config.prod.yaml      # Production конфиг (не в git!)
├── logs/                     # Логи приложения
├── data/
│   └── files/               # Загруженные файлы
└── docker-compose.be.prod.yml
```

## Переменные окружения (GitHub Secrets)

Необходимо настроить в GitHub:
- `SELECTEL_REGISTRY_USERNAME_PROD` - логин для Selectel Container Registry
- `SELECTEL_REGISTRY_TOKEN_PROD` - токен для Selectel Container Registry

## Порты

- `38080` - HTTP API
- `33000` - Debug/pprof

