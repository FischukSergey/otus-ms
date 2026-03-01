# OtusMS Streamlit Admin

Небольшой веб-клиент на Streamlit для авторизации, просмотра состояния микросервисов и работы с пользователями.

## Возможности

- **Вход** — логин через Auth-Proxy (Keycloak), хранение и автообновление токенов
- **Дашборд** — health check Auth-Proxy и Main-service
- **Пользователи** — создание, получение по UUID, мягкое удаление
- **Логи** — заглушка (API логов на бэкенде нет)

## Требования

- Python 3.10+
- Запущенные сервисы: Auth-Proxy (38081), Main-service (38080)

## Установка и запуск

```bash
cd client
python3 -m venv .venv
source .venv/bin/activate   # Windows: .venv\Scripts\activate
pip3 install -r requirements.txt
cp .env.example .env       # при необходимости отредактировать URL
streamlit run app.py
```

Откроется браузер на `http://localhost:8501`.

## Конфигурация

Переменные окружения (или файл `.env`):

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `AUTH_PROXY_URL` | `http://localhost:38081` | URL Auth-Proxy (логин, refresh, logout) |
| `MAIN_SERVICE_URL` | `http://localhost:38080` | URL Main-service (пользователи, health) |

## Замечания

- Список пользователей (GET /api/v1/users) в текущем API отсутствует — доступны только создание и получение/удаление по UUID.
- Логи сервисов можно смотреть через `docker compose logs` или Prometheus (порт 39000 при профиле monitoring).
