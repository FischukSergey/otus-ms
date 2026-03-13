# OtusMS Streamlit Admin

Небольшой веб-клиент на Streamlit для авторизации, просмотра состояния микросервисов и работы с пользователями.

## Возможности

- **Вход** — логин через Auth-Proxy (Keycloak), хранение и автообновление токенов
- **Дашборд** — health check Auth-Proxy, Main-service, News-collector, News-processor и Loki
- **Пользователи** — создание, получение по UUID, мягкое удаление
- **Логи** — просмотр логов из Loki с фильтрами по сервису и уровню

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
| `NEWS_COLLECTOR_URL` | `http://localhost:38082` | URL News-collector (health) |
| `NEWS_PROCESSOR_URL` | `http://localhost:38083` | URL News-processor (health) |
| `LOKI_URL` | `http://localhost:3100` | URL Loki (логи) |

## Замечания

- Если Loki недоступен, раздел логов покажет ошибку подключения.
