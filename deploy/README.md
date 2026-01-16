# Деплой OtusMS

## 📚 Документация

- 📋 [CHECKLIST.md](CHECKLIST.md) - Пошаговый чек-лист настройки
- 🔐 [SSH_SETUP.md](SSH_SETUP.md) - Детальная настройка SSH
- 🔄 [DEPLOY_FLOW.md](DEPLOY_FLOW.md) - Схема процесса деплоя

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

> 📖 **Подробная инструкция:** См. [SSH_SETUP.md](SSH_SETUP.md)

1. **Настройте SSH доступ:**

```bash
# На вашем локальном компьютере
ssh-keygen -t ed25519 -C "github-actions-deploy" -f ~/.ssh/vps_deploy

# Скопируйте публичный ключ на VPS
ssh-copy-id -i ~/.ssh/vps_deploy.pub root@ваш_vps_ip

# Проверьте подключение
ssh -i ~/.ssh/vps_deploy root@ваш_vps_ip

# Скопируйте приватный ключ для GitHub Secrets
cat ~/.ssh/vps_deploy
# Скопируйте весь вывод включая -----BEGIN/END-----
```

2. **Создайте структуру директорий на VPS:**

```bash
# На VPS
mkdir -p /root/otus-microservice/prod/be/{configs,logs,data/files}
chmod -R 755 /root/otus-microservice/prod/be
```

3. **Создайте конфигурационный файл на VPS:**

```bash
# На VPS
nano /root/otus-microservice/prod/be/configs/config.prod.yaml
```

Используйте `configs/config.prod.example.yaml` как шаблон.

4. **Настройте GitHub Secrets:**

Перейдите в `Settings` → `Secrets and variables` → `Actions` → `New repository secret`

Добавьте:
- `VPS_OTUS_HOST` = IP адрес вашего VPS
- `VPS_OTUS_USER` = `root`
- `VPS_OTUS_SSH_KEY` = содержимое файла `~/.ssh/vps_deploy` (приватный ключ)
- `SELECTEL_REGISTRY_OTUS_USERNAME_PROD` = логин в Selectel Registry
- `SELECTEL_REGISTRY_OTUS_TOKEN_PROD` = токен Selectel Registry

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

# Скачать последнюю версию docker-compose файла
curl -o docker-compose.be.prod.yml https://raw.githubusercontent.com/ВАШ_USERNAME/OtusMS/main/deploy/prod/docker-compose.be.prod.yml

# Логин в registry
echo "ваш_токен" | docker login cr.selcloud.ru -u "ваш_логин" --password-stdin

# Перезапустить контейнеры
docker compose -f docker-compose.be.prod.yml down
docker compose -f docker-compose.be.prod.yml pull
docker compose -f docker-compose.be.prod.yml up -d

# Проверить статус
docker ps | grep otus-microservice
docker logs otus-microservice-be-prod -f
```

### Rollback к предыдущей версии

Каждая сборка создает образ с тегом SHA коммита. Для отката:

```bash
# Найдите SHA коммита на GitHub:
# https://github.com/YOUR_USERNAME/OtusMS/commits/main

# Откатитесь к конкретной версии
docker compose down
docker pull cr.selcloud.ru/otus-microservice-be:abc123def456
docker tag cr.selcloud.ru/otus-microservice-be:abc123def456 cr.selcloud.ru/otus-microservice-be:latest
docker compose up -d

# Или прямо запустите нужную версию:
docker run -d --name otus-microservice-be-prod \
  -p 38080:38080 -p 33000:33000 \
  -v $(pwd)/configs:/app/configs:ro \
  -v $(pwd)/logs:/app/logs \
  -v $(pwd)/data/files:/app/data/files \
  --restart always \
  cr.selcloud.ru/otus-microservice-be:abc123def456
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

Необходимо настроить в GitHub (`Settings` → `Secrets and variables` → `Actions`):

### Доступ к Container Registry:
- `SELECTEL_REGISTRY_OTUS_USERNAME_PROD` - логин для Selectel Container Registry
- `SELECTEL_REGISTRY_OTUS_TOKEN_PROD` - токен для Selectel Container Registry

### Доступ к VPS (SSH):
- `VPS_OTUS_HOST` - IP адрес или домен вашего VPS
- `VPS_OTUS_USER` - пользователь для SSH (обычно `root`)
- `VPS_OTUS_SSH_KEY` - приватный SSH ключ для доступа к серверу

## Порты

- `38080` - HTTP API
- `33000` - Debug/pprof

