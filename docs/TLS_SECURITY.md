# TLS Security в микросервисной архитектуре

## Текущая реализация

```
Internet (HTTPS) ─────────────────► Защищено SSL
    ↓ TLS termination
[ Nginx with SSL cert ]
    ↓ HTTP
┌─────────────────────────────────┐
│   Docker Network: otus_network  │ ◄─ Изолированная сеть
│   (Linux network namespace)     │
│                                  │
│   [ Backend Service ]            │
│         ↓                        │
│   [ PostgreSQL ]                 │
│                                  │
│   [ Keycloak ]                   │
│   [ Auth-Proxy ]                 │
└─────────────────────────────────┘
```

**Уровень 1 - Perimeter Security:**
- Nginx с SSL сертификатом
- Защита от внешних угроз
- Автоматическая ротация через Certbot

**Уровень 2 - Network Isolation:**
- Docker network `otus_network`
- Изоляция на уровне Linux namespaces
- Нет прямого доступа из интернета к внутренним сервисам
- Контейнеры видят только друг друга внутри сети

**Уровень 3 - Application Security:**
- JWT токены для аутентификации
- Keycloak для централизованного управления доступом
- Rate limiting и валидация входных данных
