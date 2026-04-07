"""
Streamlit-клиент OtusMS: авторизация, дашборд сервисов, пользователи.
Запуск: streamlit run app.py
"""
import base64
import json
import time

import streamlit as st

from api import (
    alert_worker_url,
    auth_proxy_url,
    get_all_users,
    get_news,
    get_personalized_feed,
    get_user_preferences,
    get_logs,
    health_check,
    login,
    loki_health,
    loki_url,
    logout,
    main_service_url,
    news_collector_url,
    news_processor_url,
    refresh_token,
    update_user_preferences,
)

# Загрузка .env (python-dotenv)
try:
    from dotenv import load_dotenv
    load_dotenv()
except ImportError:
    pass

# Константы сессии
ACCESS_TOKEN = "access_token"
REFRESH_TOKEN = "refresh_token"
EXPIRES_AT = "expires_at"
PAGE = "page"

# Страницы
PAGE_DASHBOARD = "Дашборд"
PAGE_USERS = "Пользователи"
PAGE_NEWS = "Новости"
PAGE_PERSONALIZATION = "Personalization"
PAGE_LOGS = "Логи"


def _decode_jwt_payload(token: str) -> dict:
    """Декодирует payload JWT без проверки подписи (для чтения claims)."""
    try:
        parts = token.split(".")
        if len(parts) != 3:
            return {}
        padding = 4 - len(parts[1]) % 4
        payload_b64 = parts[1] + "=" * (padding % 4)
        return json.loads(base64.b64decode(payload_b64).decode("utf-8"))
    except Exception:
        return {}


def get_user_roles() -> list[str]:
    """Возвращает список ролей текущего пользователя из JWT."""
    token = st.session_state.get(ACCESS_TOKEN) or ""
    payload = _decode_jwt_payload(token)
    return payload.get("realm_access", {}).get("roles", [])


def is_admin() -> bool:
    """Проверяет, имеет ли текущий пользователь роль admin."""
    return "admin" in get_user_roles()


def ensure_session():
    if ACCESS_TOKEN not in st.session_state:
        st.session_state[ACCESS_TOKEN] = None
    if REFRESH_TOKEN not in st.session_state:
        st.session_state[REFRESH_TOKEN] = None
    if EXPIRES_AT not in st.session_state:
        st.session_state[EXPIRES_AT] = None
    if PAGE not in st.session_state:
        st.session_state[PAGE] = PAGE_DASHBOARD


def is_logged_in() -> bool:
    return bool(
        st.session_state.get(ACCESS_TOKEN)
        and st.session_state.get(REFRESH_TOKEN)
    )


def maybe_refresh_token() -> bool:
    """Обновляет access token при необходимости.

    Возвращает True если токен валиден.
    """
    if not st.session_state.get(REFRESH_TOKEN):
        return False
    expires_at = st.session_state.get(EXPIRES_AT) or 0
    if time.time() < expires_at - 60:  # обновить за минуту до истечения
        return True
    result = refresh_token(st.session_state[REFRESH_TOKEN])
    if isinstance(result, str):
        st.session_state[ACCESS_TOKEN] = None
        st.session_state[REFRESH_TOKEN] = None
        st.session_state[EXPIRES_AT] = None
        return False
    st.session_state[ACCESS_TOKEN] = result.access_token
    st.session_state[REFRESH_TOKEN] = result.refresh_token
    st.session_state[EXPIRES_AT] = result.expires_at
    return True


def render_login():
    st.title("Вход")
    with st.form("login"):
        username = st.text_input("Username / Email")
        password = st.text_input("Пароль", type="password")
        submitted = st.form_submit_button("Войти")
        if submitted:
            if not username or not password:
                st.error("Введите username и пароль")
                return
            result = login(username, password)
            if isinstance(result, str):
                st.error(result)
                return
            st.session_state[ACCESS_TOKEN] = result.access_token
            st.session_state[REFRESH_TOKEN] = result.refresh_token
            st.session_state[EXPIRES_AT] = result.expires_at
            st.success("Вход выполнен")
            st.rerun()


def render_sidebar():
    st.sidebar.title("OtusMS Admin")
    pages = [
        PAGE_DASHBOARD,
        PAGE_USERS,
        PAGE_NEWS,
        PAGE_PERSONALIZATION,
        PAGE_LOGS,
    ]
    current = st.session_state.get(PAGE, PAGE_DASHBOARD)
    idx = pages.index(current) if current in pages else 0
    page = st.sidebar.radio(
        "Раздел", pages, index=idx, label_visibility="collapsed"
    )
    st.session_state[PAGE] = page
    st.sidebar.divider()
    if st.sidebar.button("Выход", type="primary"):
        ref = st.session_state.get(REFRESH_TOKEN)
        if ref:
            logout(ref)
        st.session_state[ACCESS_TOKEN] = None
        st.session_state[REFRESH_TOKEN] = None
        st.session_state[EXPIRES_AT] = None
        st.rerun()


def render_dashboard():
    st.header("Состояние сервисов")
    col1, col2, col3, col4, col5, col6 = st.columns(6)
    with col1:
        st.subheader("Auth-Proxy")
        info = health_check(auth_proxy_url(), "Auth-Proxy")
        if info["ok"]:
            st.success(f"Статус: {info['status']}")
            st.caption(f"Время: {info['time']}")
        else:
            st.error(info.get("error", "Недоступен"))
    with col2:
        st.subheader("Main-service")
        info = health_check(main_service_url(), "Main-service")
        if info["ok"]:
            st.success(f"Статус: {info['status']}")
            st.caption(f"Время: {info['time']}")
        else:
            st.error(info.get("error", "Недоступен"))
    with col3:
        st.subheader("News-collector")
        info = health_check(news_collector_url(), "News-collector")
        if info["ok"]:
            st.success(f"Статус: {info['status']}")
            st.caption(f"Время: {info['time']}")
        else:
            st.error(info.get("error", "Недоступен"))
    with col4:
        st.subheader("News-processor")
        info = health_check(news_processor_url(), "News-processor")
        if info["ok"]:
            st.success(f"Статус: {info['status']}")
            st.caption(f"Время: {info['time']}")
        else:
            st.error(info.get("error", "Недоступен"))
    with col5:
        st.subheader("Alert-worker")
        info = health_check(alert_worker_url(), "Alert-worker")
        if info["ok"]:
            st.success(f"Статус: {info['status']}")
            st.caption(f"Время: {info['time']}")
        else:
            st.error(info.get("error", "Недоступен"))
    with col6:
        st.subheader("Loki")
        if loki_health():
            st.success("Статус: ok")
        else:
            st.error("Недоступен")
    st.caption(
        f"Auth-Proxy: {auth_proxy_url()} | "
        f"Main: {main_service_url()} | "
        f"News-collector: {news_collector_url()} | "
        f"News-processor: {news_processor_url()} | "
        f"Alert-worker: {alert_worker_url()} | "
        f"Loki: {loki_url()}"
    )


_ROLE_LABELS = {
    "admin": "🔑 admin",
    "user1C": "👤 user1C",
    "user":   "👤 user",
}


def render_users():
    st.header("Пользователи")
    token = st.session_state.get(ACCESS_TOKEN)
    if not token:
        st.warning("Нет токена")
        return

    if not is_admin():
        st.warning("Доступ только для пользователей с ролью **admin**.")
        return

    st.caption("Все пользователи системы (включая мягко удалённых).")
    if st.button("Загрузить пользователей", key="btn_load_users"):
        users, err = get_all_users(token)
        if err:
            st.error(err)
        elif not users:
            st.info("Пользователей не найдено.")
        else:
            st.success(f"Найдено: {len(users)}")
            rows = []
            for u in users:
                rows.append({
                    "UUID":       u.get("uuid", ""),
                    "Email":      u.get("email", ""),
                    "Имя":        u.get("firstName", ""),
                    "Фамилия":    u.get("lastName", ""),
                    "Роль":    _ROLE_LABELS.get(
                        u.get("role", ""), u.get("role", "")
                    ),
                    "Удалён":  "🗑 да" if u.get("deleted") else "нет",
                    "Создан":  u.get("createdAt", "")[:19].replace("T", " "),
                })
            st.dataframe(rows, width="stretch")


def render_news():
    st.header("Новости")
    token = st.session_state.get(ACCESS_TOKEN)
    if not token:
        st.warning("Нет токена")
        return

    if not is_admin():
        st.warning("Доступ только для пользователей с ролью **admin**.")
        return

    limit = st.number_input(
        "Сколько записей показать",
        min_value=10,
        max_value=500,
        value=50,
        step=10,
        key="news_limit",
    )

    if st.button("Загрузить новости", key="btn_load_news"):
        rows, err = get_news(token, limit=int(limit))
        if err:
            st.error(err)
            return
        if not rows:
            st.info("Новости не найдены.")
            return

        st.success(f"Найдено: {len(rows)}")
        for idx, row in enumerate(rows, start=1):
            topic = row.get("topic", "").strip() or "(без заголовка)"
            source = row.get("source", "").strip() or "(неизвестный источник)"
            url = row.get("url", "").strip()
            created_at = row.get("createdAt", "")[:19].replace("T", " ")
            st.markdown(f"**{idx}. {topic}**")
            st.caption(f"Источник: {source} | Создана: {created_at or '—'}")
            if url:
                st.markdown(f"[Открыть новость]({url})")
            else:
                st.caption("Ссылка отсутствует")
            st.divider()
    else:
        st.caption("Нажмите «Загрузить новости» для получения данных.")


def _current_user_uuid() -> str:
    token = st.session_state.get(ACCESS_TOKEN) or ""
    payload = _decode_jwt_payload(token)
    return payload.get("sub", "")


def _split_csv(value: str) -> list[str]:
    return [item.strip() for item in value.split(",") if item.strip()]


def render_personalization():
    st.header("Personalization")
    token = st.session_state.get(ACCESS_TOKEN)
    if not token:
        st.warning("Нет токена")
        return

    current_uuid = _current_user_uuid()
    st.caption(f"JWT user UUID: `{current_uuid or 'unknown'}`")

    default_target = st.session_state.get("p13n_target_uuid", current_uuid)
    target_uuid = st.text_input(
        "Target user UUID",
        value=default_target,
        key="p13n_target_uuid",
        help="Для admin можно указать UUID другого пользователя.",
    ).strip()

    st.subheader("Preferences")
    if st.button("Загрузить preferences", key="btn_load_prefs"):
        prefs, err = get_user_preferences(token, user_uuid=target_uuid or None)
        if err:
            st.error(err)
        else:
            st.session_state["p13n_categories"] = ", ".join(
                prefs.get("preferredCategories", [])
            )
            st.session_state["p13n_sources"] = ", ".join(
                prefs.get("preferredSources", [])
            )
            st.session_state["p13n_keywords"] = ", ".join(
                prefs.get("preferredKeywords", [])
            )
            st.session_state["p13n_language"] = prefs.get(
                "preferredLanguage", ""
            )
            st.session_state["p13n_from_hours"] = int(
                prefs.get("fromHours", 168)
            )
            st.success("Preferences загружены")

    col1, col2 = st.columns(2)
    with col1:
        categories = st.text_input(
            "preferredCategories (через запятую)",
            key="p13n_categories",
            placeholder="tech, science",
        )
        sources = st.text_input(
            "preferredSources (через запятую)",
            key="p13n_sources",
            placeholder="source_3, source_2",
        )
        keywords = st.text_input(
            "preferredKeywords (через запятую)",
            key="p13n_keywords",
            placeholder="ai, golang, llm",
        )
    with col2:
        language = st.selectbox(
            "preferredLanguage",
            options=["", "ru", "en"],
            key="p13n_language",
            format_func=lambda x: x or "(пусто)",
        )
        from_hours = st.number_input(
            "fromHours",
            min_value=1,
            max_value=720,
            value=int(st.session_state.get("p13n_from_hours", 168)),
            step=1,
            key="p13n_from_hours",
        )

    if st.button(
        "Сохранить preferences",
        key="btn_save_prefs",
        type="primary",
    ):
        ok, msg = update_user_preferences(
            token,
            {
                "preferredCategories": _split_csv(categories),
                "preferredSources": _split_csv(sources),
                "preferredKeywords": _split_csv(keywords),
                "preferredLanguage": language,
                "fromHours": int(from_hours),
            },
            user_uuid=target_uuid or None,
        )
        if ok:
            st.success(msg)
        else:
            st.error(msg)

    st.divider()
    st.subheader("Feed")
    fcol1, fcol2, fcol3, fcol4 = st.columns([2, 1, 1, 1])
    with fcol1:
        query = st.text_input("q (FTS query)", key="p13n_q")
    with fcol2:
        limit = st.number_input(
            "limit",
            min_value=1,
            max_value=100,
            value=20,
            step=1,
            key="p13n_limit",
        )
    with fcol3:
        offset = st.number_input(
            "offset",
            min_value=0,
            max_value=10000,
            value=0,
            step=1,
            key="p13n_offset",
        )
    with fcol4:
        feed_from_hours = st.number_input(
            "fromHours",
            min_value=0,
            max_value=720,
            value=0,
            step=1,
            key="p13n_feed_from_hours",
            help="0 = использовать fromHours из preferences",
        )

    if st.button("Загрузить feed", key="btn_load_p13n_feed"):
        rows, err = get_personalized_feed(
            token,
            limit=int(limit),
            offset=int(offset),
            from_hours=int(feed_from_hours),
            q=query.strip() or None,
            user_uuid=target_uuid or None,
        )
        if err:
            st.error(err)
            return
        if not rows:
            st.info("Лента пустая.")
            return

        st.success(f"Найдено: {len(rows)}")
        table_rows = []
        for row in rows:
            table_rows.append(
                {
                    "Score": round(float(row.get("score", 0.0)), 4),
                    "Topic": row.get("topic", ""),
                    "Source": row.get("source", ""),
                    "Category": row.get("category", ""),
                    "URL": row.get("url", ""),
                    "ProcessedAt": (
                        row.get("processedAt", "")[:19].replace("T", " ")
                    ),
                }
            )
        st.dataframe(table_rows, width="stretch")


_LEVEL_COLORS = {
    "ERROR": "🔴",
    "WARN":  "🟡",
    "WARNING": "🟡",
    "INFO":  "🔵",
    "DEBUG": "⚪",
}

_SERVICES = [
    ("Все сервисы",    None),
    ("main-service",   "otus-microservice-be-prod"),
    ("auth-proxy",     "otus-microservice-auth-proxy-prod"),
    ("news-collector", "otus-news-collector-prod"),
    ("news-processor", "otus-news-processor-prod"),
    ("alert-worker",   "otus-alert-worker-prod"),
]

_LEVELS = ["Все уровни", "ERROR", "WARN", "INFO", "DEBUG"]


def render_logs():
    st.header("Логи")

    if not loki_health():
        st.error(
            f"Loki недоступен ({loki_url()}). "
            "Убедитесь, что стек Loki + Promtail запущен."
        )
        return

    # --- Фильтры ---
    col1, col2, col3, col4 = st.columns([3, 2, 2, 1])
    with col1:
        svc_label = st.selectbox(
            "Сервис", [s[0] for s in _SERVICES], key="log_svc"
        )
        container = next(s[1] for s in _SERVICES if s[0] == svc_label)
    with col2:
        level_label = st.selectbox("Уровень", _LEVELS, key="log_lvl")
        level = None if level_label == "Все уровни" else level_label
    with col3:
        hours = st.selectbox(
            "Период", [1, 3, 6, 12, 24],
            format_func=lambda h: f"Последние {h}ч",
            key="log_hours",
        )
    with col4:
        limit = st.number_input("Строк", min_value=10, max_value=500,
                                value=100, step=10, key="log_limit")

    if st.button("Обновить", key="btn_logs"):
        rows, err = get_logs(
            service=container, level=level,
            limit=int(limit), hours=int(hours),
        )
        if err:
            st.error(err)
        elif not rows:
            st.info("Логов не найдено за выбранный период.")
        else:
            st.caption(f"Найдено записей: {len(rows)}")
            for row in rows:
                icon = _LEVEL_COLORS.get(row["level"], "⚫")
                # Краткая строка для заголовка expander
                http_hint = ""
                if row["method"] and row["path"]:
                    http_hint = (
                        f" | `{row['method']} {row['path']}`"
                        + (f" → {row['status']}" if row["status"] else "")
                    )
                summary = (
                    f"`{row['ts']}` {icon} **{row['level']}**"
                    f" [{row['service']}] — {row['msg']}{http_hint}"
                )
                with st.expander(summary, expanded=False):
                    # Доп. поля — только если есть значение
                    detail_cols = [
                        ("Method",   row["method"]),
                        ("Path",     row["path"]),
                        ("Status",   str(row["status"])),
                        ("Duration", f"{row['duration_ms']} ms"
                         if row["duration_ms"] != "" else ""),
                        ("IP",       row["remote_addr"]),
                    ]
                    visible = [(k, v) for k, v in detail_cols if v]
                    if visible:
                        cols = st.columns(len(visible))
                        for col, (k, v) in zip(cols, visible):
                            col.metric(k, v)
                    if row["request_id"]:
                        st.caption(
                            f"request_id: `{row['request_id']}`"
                        )
                    st.code(row["raw"], language="json")
    else:
        st.caption("Нажмите «Обновить» для загрузки логов.")


def main():
    ensure_session()

    if not is_logged_in():
        render_login()
        return

    if not maybe_refresh_token():
        st.error("Сессия истекла. Войдите снова.")
        st.session_state[ACCESS_TOKEN] = None
        st.session_state[REFRESH_TOKEN] = None
        st.session_state[EXPIRES_AT] = None
        render_login()
        return

    render_sidebar()

    page = st.session_state.get(PAGE, PAGE_DASHBOARD)
    if page == PAGE_USERS:
        render_users()
    elif page == PAGE_NEWS:
        render_news()
    elif page == PAGE_PERSONALIZATION:
        render_personalization()
    elif page == PAGE_LOGS:
        render_logs()
    else:
        render_dashboard()


if __name__ == "__main__":
    main()
