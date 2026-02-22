# Быстрая настройка Keycloak для регистрации пользователей

Краткая инструкция по включению возможности создания пользователей через Auth-Proxy.

## Предварительные требования

- ✅ Keycloak запущен и доступен
- ✅ Realm `otus-ms` создан
- ✅ Client `auth-proxy` настроен

## Шаги настройки (5 минут)

### 1. Откройте Keycloak Admin Console

```
https://fishouk-otus-ms.ru/auth/admin/
```

Войдите с admin credentials.

### 2. Выберите realm `otus-ms`

В левом верхнем углу в выпадающем списке выберите **otus-ms**.

### 3. Откройте настройки client

**Clients** → **auth-proxy**

### 4. Включите Service Accounts (если не включен)

1. Перейдите на вкладку **Settings**
2. Найдите **Service accounts roles**: установите **ON** ✅
3. Нажмите **Save**

### 5. Назначьте роли управления пользователями

1. Перейдите на вкладку **Service account roles**
2. В поле **Client roles** выберите из выпадающего списка: **realm-management**
3. В списке **Available roles** найдите и выберите (Ctrl/Cmd + Click):
   - ✅ **manage-users**
   - ✅ **view-users**
4. Нажмите кнопку **Add selected** (стрелка вправо →)

### 6. Проверьте результат

В разделе **Assigned roles** должны появиться:

| Client | Role | Описание |
|--------|------|----------|
| realm-management | manage-users | Создание и управление пользователями |
| realm-management | view-users | Просмотр пользователей |

---

## ✅ Готово!

Теперь `auth-proxy` client имеет права для:

- ✅ Создания новых пользователей в Keycloak
- ✅ Установки паролей
- ✅ Назначения ролей (admin, user)
- ✅ Проверки существования пользователей
- ✅ Удаления пользователей (для rollback при ошибках)

---

## Проверка настройки

После реализации кода регистрации можно проверить работу:

```bash
# Тест создания пользователя
curl -X POST http://localhost:38081/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newuser@example.com",
    "password": "SecurePass123!",
    "firstName": "Test",
    "lastName": "User"
  }'
```

**Ожидаемый ответ:**
- `201 Created` - пользователь успешно создан
- `409 Conflict` - пользователь уже существует
- `400 Bad Request` - ошибка валидации данных

---

## Troubleshooting

### Ошибка: "User registration failed"

**Проверьте:**
1. Service account roles включен в настройках client
2. Роли `manage-users` и `view-users` назначены
3. Client secret корректен в конфигурации Auth-Proxy

### Проверка прав через API

Получите токен service account:

```bash
curl -X POST https://fishouk-otus-ms.ru/auth/realms/otus-ms/protocol/openid-connect/token \
  -d "client_id=auth-proxy" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  -d "grant_type=client_credentials"
```

Если получили токен - service account работает корректно.

---

## Дополнительная информация

- 📚 **Полная документация**: [KEYCLOAK_AUTH_PROXY_SETUP.md](./KEYCLOAK_AUTH_PROXY_SETUP.md) → Шаг 5
- 🔐 **RBAC настройка**: [KEYCLOAK_AUTH_PROXY_SETUP.md](./KEYCLOAK_AUTH_PROXY_SETUP.md) → Настройка ролей
- 🧪 **Тестирование**: [../../../TESTING.md](../../../TESTING.md)

---

## Следующие шаги

1. ✅ Настройка Keycloak завершена
2. 🔄 Реализация кода регистрации в Auth-Proxy
3. 🔄 Создание HTTP client для Main Service
4. 🔄 Добавление handler для /api/v1/auth/register
5. 🔄 Написание тестов
6. 🔄 Обновление Swagger документации
