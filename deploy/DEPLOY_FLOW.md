# Процесс деплоя (Deploy Flow)

## Схема автоматического деплоя

```
┌─────────────────────────────────────────────────────────────────┐
│  Разработчик                                                    │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  │ git push origin main
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│  GitHub Repository                                              │
│  - Код обновлен в ветке main                                    │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  │ webhook trigger
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│  GitHub Actions (ubuntu-latest runner)                          │
│                                                                  │
│  Job 1: Lint                                                    │
│  ├─ Setup Go 1.23                                               │
│  ├─ Checkout code                                               │
│  └─ Run golangci-lint                                           │
│                                                                  │
│  Job 2: Test                                                    │
│  ├─ Setup Go 1.23                                               │
│  ├─ Checkout code                                               │
│  └─ Run unit tests                                              │
│                                                                  │
│  Job 3: Build & Deploy (needs: lint, test)                     │
│  ├─ Checkout code                                               │
│  ├─ Login to Selectel Registry                                 │
│  ├─ Build Docker image                                          │
│  ├─ Push image to Registry ─────────────┐                      │
│  └─ Deploy via SSH ──────────────────┐  │                      │
└──────────────────────────────────────┼──┼──────────────────────┘
                                       │  │
                                       │  │
             ┌─────────────────────────┘  │
             │                            │
             │ SSH Connection             │ Docker Pull
             ▼                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  VPS Server (Production)                                        │
│                                                                  │
│  1. Создание директорий                                         │
│     mkdir -p /root/otus-microservice/prod/be/{configs,logs,...} │
│                                                                  │
│  2. Скачивание docker-compose.yml                               │
│     curl https://raw.githubusercontent.com/.../docker-compose...│
│                                                                  │
│  3. Проверка конфига                                            │
│     if [ ! -f configs/config.prod.yaml ]; exit 1; fi            │
│                                                                  │
│  4. Login в Registry                                            │
│     docker login cr.selcloud.ru                                 │
│                                                                  │
│  5. Graceful shutdown                                           │
│     docker compose down                                         │
│                                                                  │
│  6. Удаление старого образа                                     │
│     docker image rm ...                                         │
│                                                                  │
│  7. Скачивание нового образа                                    │
│     docker compose pull ◄────────────────────────────────────┐  │
│                                                               │  │
│  8. Запуск контейнеров                                        │  │
│     docker compose up -d                                      │  │
│                                                               │  │
│  9. Очистка                                                   │  │
│     docker system prune -f                                    │  │
│                                                               │  │
└───────────────────────────────────────────────────────────────┼──┘
                                                                │
                     ┌──────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│  Selectel Container Registry                                    │
│  cr.selcloud.ru/otus-microservice-be:latest                     │
└─────────────────────────────────────────────────────────────────┘
```

## Детальный процесс

### Этап 1: Проверка качества кода (GitHub Actions)

```yaml
lint:
  runs-on: ubuntu-latest
  steps:
    - Setup Go 1.23
    - Checkout code
    - Run golangci-lint (timeout: 5m)
```

**Проверяет:**
- ✅ Форматирование кода
- ✅ Потенциальные ошибки
- ✅ Best practices
- ✅ Security issues

### Этап 2: Тестирование (GitHub Actions)

```yaml
test:
  runs-on: ubuntu-latest
  steps:
    - Setup Go 1.23
    - Checkout code
    - Run unit tests with race detector
```

**Проверяет:**
- ✅ Unit тесты проходят
- ✅ Нет race conditions
- ✅ Код работает корректно

### Этап 3: Сборка образа (GitHub Actions)

```bash
# На GitHub Actions сервере
docker build -t cr.selcloud.ru/otus-microservice-be:latest -f ./prod.Dockerfile .
docker push cr.selcloud.ru/otus-microservice-be:latest
```

**Создает:**
- 🏗️ Multi-stage Docker образ
- 📦 Optimized Alpine-based image
- 🚀 Готовый к запуску контейнер

### Этап 4: Деплой на VPS (SSH)

```bash
# Подключение к VPS через SSH
ssh root@VPS_IP

# Подготовка окружения
mkdir -p /root/otus-microservice/prod/be/{configs,logs,data/files}
cd /root/otus-microservice/prod/be

# Скачивание конфигурации
curl -o docker-compose.be.prod.yml https://raw.githubusercontent.com/.../

# Проверка production конфига
if [ ! -f configs/config.prod.yaml ]; then exit 1; fi

# Авторизация в Registry
echo "TOKEN" | docker login cr.selcloud.ru -u "USERNAME" --password-stdin

# Остановка старой версии
docker compose -f docker-compose.be.prod.yml down

# Удаление старого образа
docker image rm cr.selcloud.ru/otus-microservice-be:latest

# Скачивание нового образа
docker compose -f docker-compose.be.prod.yml pull

# Запуск новой версии
docker compose -f docker-compose.be.prod.yml up -d

# Проверка
docker ps | grep otus-microservice

# Очистка
docker system prune -f
```

## Временная диаграмма

```
0s    ─────► [Lint Job] ─────────────────────► ~30s
                                                  │
0s    ─────► [Test Job] ─────────────────────► ~20s
                                                  │
                                                  ▼
~30s  ─────────────────────► [Build & Deploy] ───────────► ~3min
                             ├─ Build image: ~2min
                             └─ Deploy via SSH: ~1min
                                ├─ Stop old: ~10s
                                ├─ Pull new: ~30s
                                ├─ Start new: ~10s
                                └─ Cleanup: ~10s

Общее время: ~3-4 минуты
```

## Мониторинг процесса

### На GitHub

```
https://github.com/YOUR_USERNAME/OtusMS/actions
```

Здесь видно:
- ✅ Статус каждого job
- ⏱️ Время выполнения
- 📋 Логи каждого шага
- ❌ Ошибки если что-то пошло не так

### На VPS

```bash
# Просмотр запущенных контейнеров
docker ps

# Логи приложения
docker logs otus-microservice-be-prod -f

# Healthcheck
curl http://localhost:38080/health
```

## Rollback (откат изменений)

Если что-то пошло не так:

### Вариант 1: Откат через Git

```bash
# На локальном компьютере
git revert HEAD
git push origin main
# Автоматически запустится новый деплой
```

### Вариант 2: Ручной откат на VPS

```bash
# На VPS
cd /root/otus-microservice/prod/be

# Запустить предыдущую версию образа
docker compose pull
docker compose up -d
```

### Вариант 3: Запуск конкретной версии

```bash
# В docker-compose.be.prod.yml измените
image: cr.selcloud.ru/otus-microservice-be:v1.0.0  # вместо :latest

# Затем
docker compose pull
docker compose up -d
```

## Zero Downtime Deployment (будущее улучшение)

Для деплоя без простоя можно:

1. **Использовать health checks:**
   ```yaml
   healthcheck:
     test: ["CMD", "curl", "-f", "http://localhost:38080/health"]
     interval: 30s
     timeout: 10s
     retries: 3
   ```

2. **Blue-Green deployment:**
   - Запускать новую версию на другом порту
   - Переключать Nginx после проверки
   - Останавливать старую версию

3. **Rolling update:**
   - Использовать Docker Swarm или Kubernetes
   - Постепенная замена контейнеров

## Безопасность

### Защита SSH ключа

- ✅ Приватный ключ только в GitHub Secrets
- ✅ Публичный ключ только на VPS
- ✅ Ключ используется только для деплоя
- ✅ Регулярная ротация ключей

### Защита Registry токенов

- ✅ Токены только в GitHub Secrets
- ✅ Read-only доступ где возможно
- ✅ Срок действия токенов ограничен

### Защита VPS

- ✅ Firewall настроен
- ✅ Только SSH ключи (пароли отключены)
- ✅ Регулярные обновления системы
- ✅ Мониторинг логов

## Troubleshooting

См. [SSH_SETUP.md](SSH_SETUP.md#troubleshooting) для решения проблем.

