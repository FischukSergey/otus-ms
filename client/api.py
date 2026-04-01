"""
Клиент для Auth-Proxy и Main-service API.
"""
import os
import time
from dataclasses import dataclass
from typing import Any

import requests


def _base_url(env_key: str, default: str) -> str:
    url = os.getenv(env_key, default).rstrip("/")
    return url


def auth_proxy_url() -> str:
    return _base_url("AUTH_PROXY_URL", "http://localhost:38081")


def main_service_url() -> str:
    return _base_url("MAIN_SERVICE_URL", "http://localhost:38080")


def loki_url() -> str:
    return _base_url("LOKI_URL", "http://localhost:3100")


def news_collector_url() -> str:
    return _base_url("NEWS_COLLECTOR_URL", "http://localhost:38082")


def news_processor_url() -> str:
    return _base_url("NEWS_PROCESSOR_URL", "http://localhost:38083")


@dataclass
class TokenResponse:
    access_token: str
    refresh_token: str
    expires_in: int
    token_type: str = "Bearer"

    @property
    def expires_at(self) -> float:
        return time.time() + self.expires_in


# --- Auth-Proxy ---


def login(username: str, password: str) -> TokenResponse | str:
    """Логин. Возвращает TokenResponse или строку с ошибкой."""
    url = f"{auth_proxy_url()}/api/v1/auth/login"
    try:
        r = requests.post(
            url,
            json={"username": username, "password": password},
            headers={"Content-Type": "application/json"},
            timeout=10,
        )
    except requests.RequestException as e:
        return f"Ошибка сети: {e}"
    if r.status_code == 200:
        data = r.json()
        return TokenResponse(
            access_token=data["access_token"],
            refresh_token=data["refresh_token"],
            expires_in=data.get("expires_in", 300),
            token_type=data.get("token_type", "Bearer"),
        )
    try:
        err = r.json().get("error", r.text)
    except Exception:
        err = r.text or str(r.status_code)
    return f"{r.status_code}: {err}"


def refresh_token(refresh_token: str) -> TokenResponse | str:
    """Обновление access token. Возвращает TokenResponse или строку ошибки."""
    url = f"{auth_proxy_url()}/api/v1/auth/refresh"
    try:
        r = requests.post(
            url,
            json={"refresh_token": refresh_token},
            headers={"Content-Type": "application/json"},
            timeout=10,
        )
    except requests.RequestException as e:
        return f"Ошибка сети: {e}"
    if r.status_code == 200:
        data = r.json()
        return TokenResponse(
            access_token=data["access_token"],
            refresh_token=data["refresh_token"],
            expires_in=data.get("expires_in", 300),
            token_type=data.get("token_type", "Bearer"),
        )
    try:
        err = r.json().get("error", r.text)
    except Exception:
        err = r.text or str(r.status_code)
    return f"{r.status_code}: {err}"


def logout(refresh_token: str) -> str | None:
    """Logout. Возвращает None при успехе или строку с ошибкой."""
    url = f"{auth_proxy_url()}/api/v1/auth/logout"
    try:
        r = requests.post(
            url,
            json={"refresh_token": refresh_token},
            headers={"Content-Type": "application/json"},
            timeout=10,
        )
    except requests.RequestException as e:
        return f"Ошибка сети: {e}"
    if r.status_code == 204:
        return None
    try:
        err = r.json().get("error", r.text)
    except Exception:
        err = r.text or str(r.status_code)
    return f"{r.status_code}: {err}"


# --- Health ---


def health_check(base_url: str, name: str) -> dict[str, Any]:
    """GET /health. Возвращает dict: ok, status, time, error."""
    url = f"{base_url.rstrip('/')}/health"
    try:
        r = requests.get(url, timeout=5)
        ok = r.status_code == 200
        data = r.json() if ok else {}
        return {
            "ok": ok,
            "status": data.get("status", "unknown"),
            "time": data.get("time", ""),
            "error": None if ok else (r.text or str(r.status_code)),
        }
    except requests.RequestException as e:
        return {"ok": False, "status": "error", "time": "", "error": str(e)}


# --- Main-service: users (требуют Bearer) ---


def _headers(access_token: str) -> dict[str, str]:
    return {
        "Authorization": f"Bearer {access_token}",
        "Content-Type": "application/json",
    }


def _extract_error(response: requests.Response) -> str:
    try:
        body = response.json()
        return body.get("error", body.get("message", response.text))
    except Exception:
        return response.text or str(response.status_code)


def create_user(
    access_token: str, payload: dict[str, Any]
) -> tuple[bool, str]:
    """POST /api/v1/users. Возвращает (success, message)."""
    url = f"{main_service_url()}/api/v1/users"
    try:
        r = requests.post(
            url,
            json=payload,
            headers=_headers(access_token),
            timeout=10,
        )
    except requests.RequestException as e:
        return False, f"Ошибка сети: {e}"
    if r.status_code == 201:
        return True, "Пользователь создан"
    return False, f"{r.status_code}: {_extract_error(r)}"


def get_user(access_token: str, uuid: str) -> tuple[dict | None, str | None]:
    """GET /api/v1/users/{uuid}. Возвращает (data, error)."""
    url = f"{main_service_url()}/api/v1/users/{uuid}"
    try:
        r = requests.get(url, headers=_headers(access_token), timeout=10)
    except requests.RequestException as e:
        return None, f"Ошибка сети: {e}"
    if r.status_code == 200:
        return r.json(), None
    return None, f"{r.status_code}: {_extract_error(r)}"


# --- Loki: логи ---


def get_logs(
    service: str | None = None,
    level: str | None = None,
    limit: int = 100,
    hours: int = 1,
) -> tuple[list[dict], str | None]:
    """
    Запрос логов из Loki за последние N часов.
    Возвращает (список записей, ошибка или None).
    Запись: {ts, service, level, msg, container, raw}
    """
    import time as _time

    label_sel = '{container=~"otus-(microservice|news)-.*"}'
    filters = []
    if service:
        label_sel = f'{{container="{service}"}}'
    if level:
        filters.append(f'level="{level}"')

    query = label_sel
    if filters:
        query += " | " + " | ".join(filters)

    end_ns = int(_time.time() * 1e9)
    start_ns = end_ns - hours * 3600 * int(1e9)

    url = f"{loki_url()}/loki/api/v1/query_range"
    try:
        r = requests.get(
            url,
            params={
                "query": query,
                "limit": limit,
                "start": start_ns,
                "end": end_ns,
                "direction": "backward",
            },
            timeout=10,
        )
    except requests.RequestException as e:
        return [], f"Ошибка сети: {e}"

    if r.status_code != 200:
        return [], f"{r.status_code}: {r.text[:200]}"

    rows: list[dict] = []
    try:
        data = r.json()
        for stream in data.get("data", {}).get("result", []):
            labels = stream.get("stream", {})
            for ts_ns, line in stream.get("values", []):
                import json as _json
                import datetime as _dt

                ts_sec = int(ts_ns) / 1e9
                ts_str = _dt.datetime.utcfromtimestamp(
                    ts_sec
                ).strftime("%Y-%m-%d %H:%M:%S")
                try:
                    parsed = _json.loads(line)
                    rows.append({
                        "ts":          ts_str,
                        "service":     parsed.get(
                            "service", labels.get("service", "")
                        ),
                        "level":       parsed.get(
                            "level", labels.get("level", "")
                        ).upper(),
                        "msg":         parsed.get("msg", ""),
                        "method":      parsed.get("method", ""),
                        "path":        parsed.get("path", ""),
                        "status":      parsed.get("status", ""),
                        "duration_ms": parsed.get("duration_ms", ""),
                        "remote_addr": parsed.get("remote_addr", ""),
                        "request_id":  parsed.get("request_id", ""),
                        "raw":         line,
                    })
                except Exception:
                    rows.append({
                        "ts":          ts_str,
                        "service":     labels.get("service", ""),
                        "level":       labels.get("level", "").upper(),
                        "msg":         line,
                        "method":      "",
                        "path":        "",
                        "status":      "",
                        "duration_ms": "",
                        "remote_addr": "",
                        "request_id":  "",
                        "raw":         line,
                    })
    except Exception as e:
        return [], f"Ошибка разбора ответа: {e}"

    return rows, None


def loki_health() -> bool:
    """Проверка доступности Loki."""
    try:
        r = requests.get(f"{loki_url()}/ready", timeout=3)
        return r.status_code == 200
    except requests.RequestException:
        return False


def get_all_users(access_token: str) -> tuple[list[dict] | None, str | None]:
    """
    GET /api/v1/users.
    Возвращает (список пользователей, ошибка или None).
    """
    url = f"{main_service_url()}/api/v1/users"
    try:
        r = requests.get(url, headers=_headers(access_token), timeout=10)
    except requests.RequestException as e:
        return None, f"Ошибка сети: {e}"
    if r.status_code == 200:
        return r.json(), None
    return None, f"{r.status_code}: {_extract_error(r)}"


def get_news(
    access_token: str, limit: int = 50
) -> tuple[list[dict] | None, str | None]:
    """GET /api/v1/news. Возвращает (список новостей, ошибка или None)."""
    url = f"{main_service_url()}/api/v1/news"
    try:
        r = requests.get(
            url,
            params={"limit": limit},
            headers=_headers(access_token),
            timeout=10,
        )
    except requests.RequestException as e:
        return None, f"Ошибка сети: {e}"
    if r.status_code == 200:
        return r.json(), None
    return None, f"{r.status_code}: {_extract_error(r)}"


def delete_user(access_token: str, uuid: str) -> tuple[bool, str]:
    """DELETE /api/v1/users/{uuid}. Возвращает (success, message)."""
    url = f"{main_service_url()}/api/v1/users/{uuid}"
    try:
        r = requests.delete(url, headers=_headers(access_token), timeout=10)
    except requests.RequestException as e:
        return False, f"Ошибка сети: {e}"
    if r.status_code in (200, 204):
        return True, "Пользователь удалён (мягкое удаление)"
    return False, f"{r.status_code}: {_extract_error(r)}"


def get_user_preferences(
    access_token: str, user_uuid: str | None = None
) -> tuple[dict | None, str | None]:
    """GET /api/v1/users/me/preferences."""
    url = f"{main_service_url()}/api/v1/users/me/preferences"
    params = {"userUuid": user_uuid} if user_uuid else None
    try:
        r = requests.get(
            url,
            params=params,
            headers=_headers(access_token),
            timeout=10,
        )
    except requests.RequestException as e:
        return None, f"Ошибка сети: {e}"
    if r.status_code == 200:
        return r.json(), None
    return None, f"{r.status_code}: {_extract_error(r)}"


def update_user_preferences(
    access_token: str,
    payload: dict[str, Any],
    user_uuid: str | None = None,
) -> tuple[bool, str]:
    """PUT /api/v1/users/me/preferences."""
    url = f"{main_service_url()}/api/v1/users/me/preferences"
    params = {"userUuid": user_uuid} if user_uuid else None
    try:
        r = requests.put(
            url,
            params=params,
            json=payload,
            headers=_headers(access_token),
            timeout=10,
        )
    except requests.RequestException as e:
        return False, f"Ошибка сети: {e}"
    if r.status_code == 204:
        return True, "Предпочтения сохранены"
    return False, f"{r.status_code}: {_extract_error(r)}"


def get_personalized_feed(
    access_token: str,
    limit: int = 50,
    offset: int = 0,
    from_hours: int | None = None,
    q: str | None = None,
    user_uuid: str | None = None,
) -> tuple[list[dict] | None, str | None]:
    """GET /api/v1/news/feed."""
    url = f"{main_service_url()}/api/v1/news/feed"
    params: dict[str, Any] = {
        "limit": limit,
        "offset": offset,
    }
    if from_hours is not None and from_hours > 0:
        params["fromHours"] = from_hours
    if q:
        params["q"] = q
    if user_uuid:
        params["userUuid"] = user_uuid

    try:
        r = requests.get(
            url,
            params=params,
            headers=_headers(access_token),
            timeout=15,
        )
    except requests.RequestException as e:
        return None, f"Ошибка сети: {e}"
    if r.status_code == 200:
        return r.json(), None
    return None, f"{r.status_code}: {_extract_error(r)}"
