# RBAC (Role-Based Access Control) - Руководство

Руководство по использованию системы контроля доступа на основе ролей в OtusMS.

## 📋 Обзор

RBAC middleware обеспечивает контроль доступа к API endpoints на основе ролей пользователей из JWT токенов Keycloak.

**Архитектура:**
```
Запрос → ValidateJWT → RequireRole → Handler
         │              │             │
         ├─ 401 (нет токена)
         │              ├─ 403 (нет роли)
         │              │             └─ 200 (успех)
```

---

## 🎭 Доступные роли

| Роль | Описание | Доступ |
|------|----------|--------|
| `user` | Обычный пользователь | Базовые операции с собственными данными |
| `admin` | Администратор | Полный доступ ко всем операциям |

**Примечание:** Администраторы обычно имеют обе роли (`admin` + `user`).

---

## 🔧 Middleware функции

### 1. `RequireRole(roles []string, logger)`

Универсальная проверка наличия любой из указанных ролей.

**Параметры:**
- `roles` - список допустимых ролей (достаточно любой одной)
- `logger` - логгер для audit trail

**Пример:**
```go
// Доступ для user ИЛИ admin
r.With(middleware.RequireRole([]string{"user", "admin"}, logger)).
    Get("/api/v1/profile", handler.GetProfile)

// Доступ только для admin ИЛИ manager
r.With(middleware.RequireRole([]string{"admin", "manager"}, logger)).
    Get("/api/v1/stats", handler.GetStats)
```

### 2. `RequireAdmin(logger)`

Удобная обёртка для проверки роли `admin`.

**Пример:**
```go
r.With(middleware.RequireAdmin(logger)).
    Delete("/api/v1/users/{id}", handler.Delete)
```

### 3. `RequireUser(logger)`

Проверка роли `user` или `admin` (пропускает обе).

**Пример:**
```go
r.With(middleware.RequireUser(logger)).
    Get("/api/v1/users/me", handler.GetMe)
```

---

## 🏗️ Организация роутов

### Подход 1: Группы роутов (рекомендуемый)

```go
router.Route("/api/v1", func(r chi.Router) {
    // JWT для всех защищённых роутов
    r.Group(func(r chi.Router) {
        r.Use(middleware.ValidateJWT(config, jwksManager, logger))

        // Роуты для обычных пользователей
        r.Group(func(r chi.Router) {
            r.Use(middleware.RequireUser(logger))
            
            r.Get("/users/me", userHandler.GetMe)
            r.Put("/users/me", userHandler.UpdateMe)
        })

        // Роуты только для администраторов
        r.Group(func(r chi.Router) {
            r.Use(middleware.RequireAdmin(logger))
            
            r.Post("/users", userHandler.Create)
            r.Get("/users/{uuid}", userHandler.Get)
            r.Delete("/users/{uuid}", userHandler.Delete)
            r.Get("/admin/stats", adminHandler.GetStats)
        })
    })
})
```

**Преимущества:**
- ✅ Наглядная иерархия доступа
- ✅ Легко добавлять новые уровни доступа
- ✅ Один раз указываем JWT middleware для группы

### Подход 2: Per-route middleware

```go
router.Route("/api/v1/users", func(r chi.Router) {
    r.Use(middleware.ValidateJWT(config, jwksManager, logger))

    // Разные роли для разных операций
    r.With(middleware.RequireUser(logger)).Get("/me", userHandler.GetMe)
    r.With(middleware.RequireUser(logger)).Put("/me", userHandler.UpdateMe)
    
    r.With(middleware.RequireAdmin(logger)).Post("/", userHandler.Create)
    r.With(middleware.RequireAdmin(logger)).Get("/{uuid}", userHandler.Get)
    r.With(middleware.RequireAdmin(logger)).Delete("/{uuid}", userHandler.Delete)
})
```

**Преимущества:**
- ✅ Детальный контроль для каждого роута
- ✅ Гибкость для смешанных ролей

---

## 📊 Примеры структуры endpoints

### Сценарий: API управления пользователями

```
/api/v1/users
├─ [ValidateJWT]           → Все роуты требуют JWT
│  │
│  ├─ GET /me              → RequireUser (user или admin)
│  ├─ PUT /me              → RequireUser (user или admin)
│  │
│  ├─ POST /               → RequireAdmin (только admin)
│  ├─ GET /{uuid}          → RequireAdmin (только admin)
│  └─ DELETE /{uuid}       → RequireAdmin (только admin)
```

### Код реализации:

```go
router.Route("/api/v1/users", func(r chi.Router) {
    // JWT для всех роутов
    r.Use(middleware.ValidateJWT(config, jwksManager, logger))

    // Endpoints для обычных пользователей
    r.Group(func(r chi.Router) {
        r.Use(middleware.RequireUser(logger))
        r.Get("/me", userHandler.GetMe)       // Свой профиль
        r.Put("/me", userHandler.UpdateMe)    // Обновить свой профиль
    })

    // Admin endpoints
    r.Group(func(r chi.Router) {
        r.Use(middleware.RequireAdmin(logger))
        r.Post("/", userHandler.Create)        // Создать пользователя
        r.Get("/{uuid}", userHandler.Get)      // Любой пользователь
        r.Delete("/{uuid}", userHandler.Delete) // Удалить пользователя
    })
})
```

---

## 🧪 Тестирование

### Unit тесты

```go
func TestAdminEndpoint(t *testing.T) {
    // Создаём claims с ролью admin
    claims := &middleware.JWTClaims{
        Sub:   "admin-id",
        Email: "admin@example.com",
    }
    claims.RealmAccess.Roles = []string{"admin"}

    // Добавляем в контекст
    ctx := context.WithValue(context.Background(), 
        middleware.ContextKeyClaims, claims)
    req := httptest.NewRequest("DELETE", "/api/v1/users/123", nil).
        WithContext(ctx)
    rec := httptest.NewRecorder()

    // Тестируем handler с RBAC
    handler := middleware.RequireAdmin(logger)(yourHandler)
    handler.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
}
```

### Integration тесты

```bash
# Получить токен
TOKEN=$(curl -s -X POST http://localhost:38081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin@example.com","password":"password"}' \
  | jq -r '.access_token')

# Использовать токен для admin endpoint
curl -X DELETE http://localhost:8080/api/v1/users/123 \
  -H "Authorization: Bearer $TOKEN"

# Ожидаем:
# - 200 OK если пользователь admin
# - 403 Forbidden если пользователь user
# - 401 Unauthorized если токен невалиден
```

---

## 🔒 Безопасность

### ⚠️ Важно!

1. **Всегда используйте ValidateJWT перед RequireRole**
   ```go
   // ✅ Правильно
   r.Use(middleware.ValidateJWT(...))
   r.Use(middleware.RequireRole(...))
   
   // ❌ Неправильно - RequireRole не найдет claims
   r.Use(middleware.RequireRole(...))
   ```

2. **Не пропускайте JWT валидацию**
   ```go
   // ❌ Опасно! Endpoint без JWT
   r.Delete("/users/{id}", handler.Delete)
   
   // ✅ Безопасно
   r.With(middleware.ValidateJWT(...)).
     With(middleware.RequireAdmin(...)).
     Delete("/users/{id}", handler.Delete)
   ```

3. **Проверяйте роли в JWT токене**
   - JWT подписан Keycloak → нельзя подделать роли
   - Но убедитесь что middleware применен к нужным роутам

### Audit Logging

RBAC middleware автоматически логирует:
- ✅ Успешные проверки ролей (`level=DEBUG`)
- ⚠️ Отказы в доступе (`level=WARN`) с деталями:
  - User ID и email
  - Требуемые роли
  - Роли пользователя
  - Путь и метод запроса

**Пример лога:**
```
level=WARN msg="access denied - missing required role" 
  user_id=123 
  email=user@example.com 
  required_roles=[admin] 
  user_roles=[user] 
  path=/api/v1/admin/stats 
  method=GET
```

---

## 📈 Мониторинг

### Prometheus метрики (рекомендуется добавить)

```go
// Пример метрик для RBAC
var (
    rbacChecksTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "rbac_checks_total",
            Help: "Total RBAC checks",
        },
        []string{"role", "status"}, // status: allowed, denied
    )
)
```

### Grafana Dashboard

Рекомендуемые графики:
- Количество denied access по ролям
- Top endpoints с 403 ошибками
- Users с частыми 403 (возможно неправильные роли)

---

## 🚀 Расширение системы ролей

### Добавление новой роли

1. **Создайте роль в Keycloak:**
   ```
   Realm roles → Create role
   Name: manager
   Description: Manager with extended access
   ```

2. **Добавьте helper функцию (опционально):**
   ```go
   // internal/middleware/rbac.go
   func RequireManager(logger *slog.Logger) func(next http.Handler) http.Handler {
       return RequireRole([]string{"manager"}, logger)
   }
   ```

3. **Используйте в роутах:**
   ```go
   r.Group(func(r chi.Router) {
       r.Use(middleware.RequireManager(logger))
       r.Get("/reports", reportHandler.GetAll)
   })
   ```

### Комбинированные роли

```go
// Доступ для manager ИЛИ admin
r.With(middleware.RequireRole([]string{"manager", "admin"}, logger)).
    Get("/reports", handler.Get)

// Можно создать helper
func RequireManagerOrAdmin(logger *slog.Logger) func(next http.Handler) http.Handler {
    return RequireRole([]string{"manager", "admin"}, logger)
}
```

---

## 🎯 Best Practices

1. **Минимальные привилегии**
   - Назначайте только необходимые роли
   - Разделяйте admin функции на более мелкие роли если нужно

2. **Документируйте endpoints**
   - В Swagger указывайте требуемые роли
   - В README описывайте модель доступа

3. **Тестируйте RBAC**
   - Unit тесты для каждой роли
   - Integration тесты для важных workflows
   - Negative тесты (нет роли → 403)

4. **Мониторьте отказы**
   - Настройте алерты на частые 403
   - Проверяйте логи на подозрительную активность

---

## 🔍 Troubleshooting

### Проблема: "claims not found in context"

**Причина:** RequireRole вызван без ValidateJWT

**Решение:**
```go
// Добавьте ValidateJWT перед RequireRole
r.Use(middleware.ValidateJWT(...))
r.Use(middleware.RequireRole(...))
```

### Проблема: 403 Forbidden для admin пользователя

**Диагностика:**
```bash
# Проверьте токен
echo "$TOKEN" | cut -d'.' -f2 | base64 -d | jq '.realm_access.roles'

# Должно быть:
["admin", "user"]
```

**Возможные причины:**
1. Роль не назначена в Keycloak → назначьте через Admin Console
2. Mapper не настроен → проверьте Client Scopes
3. Токен устарел → получите новый через /auth/login

### Проблема: 401 Unauthorized на защищённом endpoint

**Причина:** JWT токен невалиден или отсутствует

**Решение:**
1. Проверьте что токен передан: `Authorization: Bearer <token>`
2. Проверьте что токен не истек (expires_in: 300 секунд)
3. Получите новый токен если истек

---

## 📚 См. также

- [Настройка ролей в Keycloak](../deploy/prod/KEYCLOAK_AUTH_PROXY_SETUP.md)
- [JWT Middleware](../internal/middleware/jwt.go)
- [Архитектура авторизации](../Feat_Authorization.md)
- [API документация](../api/mainservice/)

---

## ✅ Чеклист внедрения RBAC

- [ ] Роли созданы в Keycloak
- [ ] Mapper настроен для включения ролей в токены
- [ ] ValidateJWT middleware применен к защищённым роутам
- [ ] RequireRole/RequireAdmin/RequireUser применены к нужным endpoints
- [ ] Unit тесты написаны для RBAC middleware
- [ ] Integration тесты покрывают основные сценарии
- [ ] Документация обновлена (Swagger, README)
- [ ] Логирование и мониторинг настроены
- [ ] Проверена работа на production-like окружении
