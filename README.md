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
│   ├── main-service/          # Основной микросервис
│   │   ├── main.go            # Инициализация и запуск
│   │   └── api-server.go      # HTTP сервер с роутами
│   └── auth-proxy/            # Auth-Proxy микросервис
│       ├── main.go            # Инициализация и запуск
│       └── api-server.go      # HTTP сервер для авторизации
│
├── internal/                  # Внутренняя бизнес-логика
│   ├── config/               # Конфигурация
│   │   ├── config.go         # Структуры конфигурации
│   │   ├── parse.go          # Парсинг YAML
│   │   └── parse_test.go     # Unit тесты
│   ├── keycloak/             # Keycloak интеграция
│   │   ├── client.go         # Клиент для Keycloak
│   │   └── models.go         # Модели данных
│   └── handlers/
│       ├── user/             # User handlers
│       └── auth/             # Auth handlers (login/refresh/logout)
│
├── pkg/                      # Публичные библиотеки (переиспользуемые)
│
├── configs/                  # Файлы конфигурации
│   ├── config.local.yaml    # Main-service (не в git)
│   ├── config.prod.yaml     # Main-service production (не в git)
│   ├── config.auth-proxy.local.yaml   # Auth-Proxy локально
│   ├── config.auth-proxy.prod.yaml    # Auth-Proxy production
│   └── *.example.yaml       # Примеры конфигов
│
├── deploy/                   # Инфраструктура и деплой
│   ├── local/               # Docker Compose для разработки
│   └── prod/                # Production конфигурация
│       ├── docker-compose.auth-proxy.prod.yml
│       ├── docker-compose.keycloak*.prod.yml
│       └── KEYCLOAK_AUTH_PROXY_SETUP.md
│
├── docs/                     # Документация
│   └── AUTH_PROXY_API.md    # API документация Auth-Proxy
│
├── tests/                    # Тесты
│   ├── integration/         # Интеграционные тесты
│   │   ├── README.md        # 📖 Документация по интеграционным тестам
│   │   ├── user_test.go
│   │   └── auth_test.go
│   └── unit/                # Unit тесты
├── TESTING.md               # 📖 Руководство по тестированию
├── README.md                # Этот файл
├── Taskfile.yml             # Task автоматизация
│
├── .github/
│   └── workflows/
│       ├── ci.yml           # GitHub Actions CI/CD
│       └── deploy-keycloak.yml  # Деплой Keycloak
│
├── prod.Dockerfile          # Main-service Dockerfile
├── auth-proxy.Dockerfile    # Auth-Proxy Dockerfile
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
- **gocloak/v13** - Keycloak клиент для авторизации
- **golang-jwt/jwt/v5** - JWT токены и валидация
- **lestrrat-go/jwx/v2** - JWKS (публичные ключи для JWT)

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

**GitHub Secrets:** 📖 **[Документация по настройке секретов для CI/CD](.github/workflows/SECRETS.md)**

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
task tidy           # go mod tidy
task fmt            # Форматирование кода (gofumpt + gci)
task lint           # Линтинг (golangci-lint в Docker)
task tests          # Запуск тестов
task build          # Сборка бинарника

# С Docker Compose
docker compose -f deploy/local/docker-compose.local.yml up -d
docker compose -f deploy/local/docker-compose.local.yml logs -f
docker compose -f deploy/local/docker-compose.local.yml down

# Запуск Auth-Proxy с профилем
docker compose -f deploy/local/docker-compose.local.yml --profile auth up -d
```

### Настройка Auth-Proxy (если нужен)

Перед запуском Auth-Proxy создайте конфиг с реальным Client Secret:

```bash
# 1. Скопируйте example файл
cp configs/config.auth-proxy.local.example.yaml configs/config.auth-proxy.local.yaml

# 2. Откройте файл и замените 'your-client-secret-here' на реальный secret из Keycloak
nano configs/config.auth-proxy.local.yaml
```

### Проверка работы

```bash
# Main Service health check
curl http://localhost:38080/health

# Auth-Proxy health check (если запущен с --profile auth)
curl http://localhost:38081/health

# Главная страница
curl http://localhost:38080/
```

## Тестирование

📖 **[Полное руководство по тестированию](TESTING.md)**

### Быстрый старт

```bash
# Unit тесты
task test:unit

# Интеграционные тесты (требуют подготовки)
cp configs/config.auth-proxy.test.example.yaml configs/config.auth-proxy.test.yaml
# Отредактируйте config.auth-proxy.test.yaml - добавьте client_secret
task test:integration

# Все тесты
task tests
```

### Что тестируется?

**Unit тесты:**
- Конфигурация и валидация
- Бизнес-логика

**Интеграционные тесты (в Docker):**
- Main Service API (users CRUD)
- Auth-Proxy (login/refresh/logout)
- PostgreSQL интеграция

### Особенности

- **Локально:** секреты в файлах (в .gitignore), никаких `export` не нужно
- **CI/CD:** секреты через GitHub Secrets → [документация](.github/workflows/SECRETS.md)
- **Auth-Proxy тесты:** требуют настройки Keycloak client и тестового пользователя

📖 Подробности: [TESTING.md](TESTING.md) | [tests/integration/README.md](tests/integration/README.md)

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


## 🌐 Микросервисы

### Main Service (порт 38080)

Основной микросервис для работы с пользователями.

**Endpoints:**

- `GET /` - Приветственное сообщение
- `GET /health` - Health check
- `POST /api/v1/users` - Создание пользователя
- `GET /api/v1/users` - Список пользователей (admin)
- `GET /api/v1/users/{uuid}` - Получение пользователя
- `DELETE /api/v1/users/{uuid}` - Удаление пользователя
- `GET /api/v1/users/me/preferences` - Получение персональных предпочтений
- `PUT /api/v1/users/me/preferences` - Обновление персональных предпочтений
- `GET /api/v1/news/feed` - Персонализированная лента новостей
- `POST /api/v1/news/events` - Отправка user events (view/click/like/dislike/hide)

**Example:**
```bash
curl https://fishouk-otus-ms.ru/health
```

### Auth-Proxy Service (порт 38081)

Микросервис для централизованной авторизации через Keycloak.

**Endpoints:**

- `GET /health` - Health check
- `POST /api/v1/auth/login` - Логин пользователя
- `POST /api/v1/auth/refresh` - Обновление токена
- `POST /api/v1/auth/logout` - Logout пользователя

**Подробная документация:** [AUTH_PROXY_API.md](docs/AUTH_PROXY_API.md)

**Example:**
```bash
# Health check
curl http://localhost:38081/health

# Login
curl -X POST http://localhost:38081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"test@example.com","password":"test123"}'
```

**Архитектура авторизации:**
- Централизованная аутентификация через Keycloak
- JWT токены для API доступа
- Refresh token для обновления
- Логирование всех попыток авторизации

См. полную документацию: [Feat_Authorization.md](Feat_Authorization.md)

### Streamlit Admin (client/)

Веб-клиент на Python/Streamlit для входа, дашборда сервисов и работы с пользователями.

```bash
cd client && pip3 install -r requirements.txt && streamlit run app.py
```

Подробнее: [client/README.md](client/README.md)

### 🔐 RBAC (Role-Based Access Control)

Контроль доступа на основе ролей из JWT токенов Keycloak.

**Доступные роли:**
- `user` - обычный пользователь с базовыми правами
- `admin` - администратор с полным доступом

**Защищённые endpoints (Main Service):**
```
POST   /api/v1/users                  → service-account, admin
GET    /api/v1/users                  → admin
GET    /api/v1/users/{id}             → admin
DELETE /api/v1/users/{id}             → admin
GET    /api/v1/news                   → admin
GET    /api/v1/users/me/preferences   → user, admin
PUT    /api/v1/users/me/preferences   → user, admin
GET    /api/v1/news/feed              → user, admin
POST   /api/v1/news/events            → user, admin
```

**Middleware цепочка:**
```
Request → ValidateJWT → RequireRole → Handler
          ↓              ↓             ↓
          401           403           200
```

📖 [Полное руководство по RBAC](docs/RBAC_GUIDE.md) | [Настройка ролей](deploy/prod/KEYCLOAK_AUTH_PROXY_SETUP.md)

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


## 📋 Планируемые доработки news-processor

| # | Задача | Описание |
|---|--------|----------|
| 1 | **Теги** | Реализовать `ExtractTags` в `pipeline.go` — возвращать слова из `categoryKeywords`, совпавшие с текстом новости (до 10 штук) |
| 2 | **Словари категорий в БД** | Перенести `categoryKeywords` из кода в `main-service` (таблица `category_keywords`), загружать через gRPC при старте `news-processor` и обновлять по расписанию. TODO уже есть в `pipeline.go` |
| 3 | **Сброс оффсетов** | Добавить локальную задачу `kafka:consumer:reset` в `Taskfile.yml` для сброса оффсетов consumer group при разработке |
| 4 | **Длинные URL** | Пропускать новости с `url > 1000` символов с предупреждением в лог вместо падения INSERT |
| 5 | **Тесты** | Написать unit-тесты для `pipeline.go`: `StripHTML`, `ExtractSummary`, `DetectCategory`, `truncate` |
| 6 | **Retention policy** | Настроить автоудаление новостей старше N дней: `pg_cron` на уровне БД или cron-задача в `main-service`. Добавить параметр `news_retention_days` в конфиг |

## 🔗 Ссылки

- **Production:** https://fishouk-otus-ms.ru/
- **Repository:** https://github.com/FischukSergey/otus-ms
- **CI/CD:** https://github.com/FischukSergey/otus-ms/actions

## 👨‍💻 Автор

Проект разработан в рамках курса OTUS "Микросервисы на GO"

## 📄 Лицензия

MIT License - см. [LICENSE](LICENSE)

---
