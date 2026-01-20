# OtusMS - Микросервисная архитектура на Go

> Production: **[https://fishouk-otus-ms.ru/](https://fishouk-otus-ms.ru/)**

Проект разработан в рамках курса OTUS "Микросервисы на GO".

## О проекте

OtusMS - это шаблон микросервиса с полным CI/CD циклом, демонстрирующий:

- **Архитектуру** - разделение на слои (handlers, services, repositories)
- **Контейнеризацию** - Docker multi-stage builds для оптимизации образов
- **Автоматизацию** - полный CI/CD pipeline с GitHub Actions
- **Качество кода** - линтинг, тестирование, форматирование
- **Production-ready** - graceful shutdown, health checks, логирование

**Production версия доступна по адресу:** [https://fishouk-otus-ms.ru/](https://fishouk-otus-ms.ru/)

### Текущая структура кода

```
OtusMS/
├── cmd/
│   └── main-service/          # Точка входа приложения
│       ├── main.go            # Инициализация и запуск
│       └── api-server.go      # HTTP сервер с роутами
│
├── internal/                  # Внутренняя бизнес-логика
│   └── config/               # Конфигурация
│       ├── config.go         # Структуры конфигурации
│       ├── parse.go          # Парсинг YAML
│       └── parse_test.go     # Unit тесты
│
├── pkg/                      # Публичные библиотеки (переиспользуемые)
│
├── configs/                  # Файлы конфигурации
│   ├── config.local.yaml    # Для разработки (не в git)
│   ├── config.prod.yaml     # Для production (не в git)
│   └── *.example.yaml       # Примеры конфигов
│
├── deploy/                   # Инфраструктура и деплой
│   ├── local/               # Docker Compose для разработки
│   └── prod/                # Production конфигурация
│
├── tests/                    # Тесты
│   ├── integration/         # Интеграционные тесты
│   └── unit/                # Unit тесты
│
├── .github/
│   └── workflows/
│       └── ci.yml           # GitHub Actions CI/CD
│
├── prod.Dockerfile          # Multi-stage build для production
├── Taskfile.yml             # Автоматизация задач разработки
└── go.mod                   # Go зависимости
```

## Технологический стек

### Backend
- **Go 1.23.8**
- **chi/v5** - HTTP роутер
- **slog** - структурированное логирование
- **cleanenv** - парсинг конфигурации
- **validator/v10** - валидация данных

### DevOps & Infrastructure
- **Docker** - контейнеризация приложения
- **Docker BuildKit** - оптимизация сборки образов
- **Selectel Container Registry** - хранение Docker образов
- **GitHub Actions** - CI/CD автоматизация
- **Nginx** - reverse proxy, SSL termination
- **Let's Encrypt** - бесплатные SSL сертификаты
- **Task** - автоматизация локальной разработки

### Code Quality
- **golangci-lint** - комплексный линтер для Go
- **gofumpt** - строгое форматирование кода
- **gci** - организация импортов
- **go test** - unit тестирование

### VPS & Hosting
- **Selectel Cloud** - VPS хостинг
- **Ubuntu 22.04** - операционная система
- **UFW** - firewall

## CI/CD Pipeline

### Автоматический процесс при push в `main` или в ручную:

```
 GitHub Actions (ubuntu-latest)                         
                                                         
 1️⃣ Lint                                                
    ├─ Setup Go 1.23                                    
    ├─ Checkout code                                    
    └─ golangci-lint (68 linters)                       
                                                         
 2️⃣ Test                                                
    ├─ Setup Go 1.23                                    
    ├─ Checkout code                                    
    └─ go test -race -count=1 -v ./...                  
                                                         
 3️⃣ Build & Push                                        
    ├─ Setup Docker Buildx                              
    ├─ Login to Selectel Registry                       
    └─ Build & Push image                               
       └─ cr.selcloud.ru/otus-microservice-be/backend   
                                                         
 4️⃣ Deploy to Production                               
    ├─ Copy docker-compose via SCP                      
    └─ Deploy via SSH                                   
       ├─ Stop old containers                           
       ├─ Pull new image                                
       ├─ Start new containers                          
       └─ Health check                                  

       │
       ▼

 Production Server (Selectel VPS)                       
 https://fishouk-otus-ms.ru/                            

```

### Детали CI/CD:

**Lint Stage** 
- Проверка форматирования кода
- Статический анализ (68 линтеров)

**Test Stage** 
- Unit тесты с race detector
- В дальнейшем добавим параллельное выполнение интеграционных тестов

**Build Stage** 
- Multi-stage Docker build
- Оптимизация размера образа
- Push в Selectel Container Registry
- Тег: `latest` (пока без версий для ролбэков и вообще без версий)

**Deploy Stage** 
- Копирование конфигурации через SCP
- Graceful shutdown старой версии
- Pull нового образа из registry
- Zero-downtime deployment
- Автоматическая очистка старых образов

**Total pipeline time:** ~2-3 минуты от commit до production

![GitHub Actions Pipeline](docs/images/github-actions.png)

### Docker образ

**Multi-stage build для минимального размера:**
**Результат:**
- Builder stage: ~500 MB (отбрасывается)
- Final image: ~15 MB (только бинарник + Alpine)

### Selectel Container Registry

**Registry:** `cr.selcloud.ru/otus-microservice-be/backend`

Образы хранятся в приватном registry Selectel с автоматической очисткой старых версий.

![Selectel Container Registry](docs/images/selectel-registry.png)

**Характеристики образов:**
- Размер production образа: ~12.51 MB
- Manifest размер: ~19.16 KB
- Автоматическое версионирование по тегу `latest`
- История всех сборок доступна в registry

## 💻 Локальная разработка

### Требования

- **Go 1.23.8+**
- **Docker** (опционально)
- **Task** (опционально, для автоматизации)

### Установка Task

```bash
# macOS
brew install go-task

# Linux
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin

# Windows
choco install go-task
```

### Быстрый старт

```bash
# Клонировать репозиторий
git clone https://github.com/FischukSergey/otus-ms.git
cd otus-ms

# Установить зависимости
go mod download

# Запустить полный цикл разработки
task

# Или запустить напрямую
go run ./cmd/main-service -config configs/config.local.yaml
```

### Доступные Task команды

```bash
# Полный цикл (tidy, fmt, lint, test, build)
task

# Отдельные команды
task tidy           # go mod tidy + vendor
task fmt            # Форматирование кода (gofumpt + gci)
task lint           # Линтинг (golangci-lint в Docker)
task tests          # Запуск тестов
task build          # Сборка бинарника

# С Docker Compose
docker compose -f deploy/local/docker-compose.local.yml up -d
docker compose -f deploy/local/docker-compose.local.yml logs -f
docker compose -f deploy/local/docker-compose.local.yml down
```

### Проверка работы

```bash
# Healthcheck
curl http://localhost:38080/health

# Главная страница
curl http://localhost:38080/
```

## Production деплой

### Production окружение

- **URL:** https://fishouk-otus-ms.ru/
- **Server:** Selectel Cloud VPS
- **OS:** Ubuntu 22.04 LTS
- **Reverse Proxy:** Nginx
- **SSL:** Let's Encrypt (автообновление)
- **Container Runtime:** Docker
- **Registry:** Selectel Container Registry

### Автоматический деплой

При push в `main` ветку автоматически:
1. Проходит все проверки (lint, test)
2. Собирается Docker образ
3. Публикуется в Selectel Container Registry
4. Деплоится на production сервер
5. Выполняется graceful restart


## 🌐 API Endpoints

### `GET /`
Приветственное сообщение с информацией о сервисе

**Response:**
```json
{
  "message": "Welcome to OtusMS Microservice!",
  "version": "1.0.0",
  "status": "running"
}
```

**Example:**
```bash
curl https://fishouk-otus-ms.ru/
```

### `GET /health`
Health check endpoint для мониторинга и балансировщиков

**Response:**
```json
{
  "status": "ok",
  "time": "2026-01-17T20:30:59+03:00"
}
```

**Example:**
```bash
curl https://fishouk-otus-ms.ru/health
```

## ⚙️ Конфигурация

Конфигурация загружается из YAML файлов с валидацией через `validator/v10`.

### Файлы конфигурации

- `configs/config.local.yaml` - для локальной разработки (не в git)
- `configs/config.prod.yaml` - для production (не в git)
- `configs/*.example.yaml` - примеры конфигураций

### Пример конфигурации

```yaml
global:
  env: prod  # local, dev, prod

log:
  level: info  # debug, info, warn, error

servers:
  debug:
    addr: 0.0.0.0:33000  # pprof/debug endpoints
  client:
    addr: 0.0.0.0:38080  # main API
    allow_origins:
      - "https://fishouk-otus-ms.ru"
```


## 🔗 Ссылки

- **Production:** https://fishouk-otus-ms.ru/
- **Repository:** https://github.com/FischukSergey/otus-ms
- **CI/CD:** https://github.com/FischukSergey/otus-ms/actions

## 👨‍💻 Автор

Проект разработан в рамках курса OTUS "Микросервисы на GO"

## 📄 Лицензия

MIT License - см. [LICENSE](LICENSE)

---
