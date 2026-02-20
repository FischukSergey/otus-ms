"""
Streamlit-клиент OtusMS: авторизация, дашборд сервисов, пользователи.
Запуск: streamlit run app.py
"""
import time

import streamlit as st

from api import (
    auth_proxy_url,
    create_user,
    get_user,
    delete_user,
    get_logs,
    health_check,
    login,
    loki_health,
    loki_url,
    logout,
    main_service_url,
    refresh_token,
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
PAGE_LOGS = "Логи"


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
    return bool(st.session_state.get(ACCESS_TOKEN) and st.session_state.get(REFRESH_TOKEN))


def maybe_refresh_token() -> bool:
    """Обновляет access token при необходимости. Возвращает True если токен валиден."""
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
    pages = [PAGE_DASHBOARD, PAGE_USERS, PAGE_LOGS]
    current = st.session_state.get(PAGE, PAGE_DASHBOARD)
    idx = pages.index(current) if current in pages else 0
    page = st.sidebar.radio("Раздел", pages, index=idx, label_visibility="collapsed")
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
    col1, col2, col3 = st.columns(3)
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
        st.subheader("Loki")
        if loki_health():
            st.success("Статус: ok")
        else:
            st.error("Недоступен")
    st.caption(
        f"Auth-Proxy: {auth_proxy_url()} | "
        f"Main: {main_service_url()} | "
        f"Loki: {loki_url()}"
    )


def render_users():
    st.header("Пользователи")
    token = st.session_state.get(ACCESS_TOKEN)
    if not token:
        st.warning("Нет токена")
        return

    tab_create, tab_get, tab_delete = st.tabs(["Создать", "Получить по UUID", "Удалить"])

    with tab_create:
        with st.form("create_user"):
            uuid = st.text_input("UUID")
            email = st.text_input("Email")
            first_name = st.text_input("Имя", key="fn")
            last_name = st.text_input("Фамилия", key="ln")
            middle_name = st.text_input("Отчество (необязательно)", key="mn")
            if st.form_submit_button("Создать"):
                if not uuid or not email:
                    st.error("UUID и Email обязательны")
                else:
                    payload = {
                        "uuid": uuid.strip(),
                        "email": email.strip(),
                        "firstName": first_name.strip() or "",
                        "lastName": last_name.strip() or "",
                    }
                    if middle_name.strip():
                        payload["middleName"] = middle_name.strip()
                    ok, msg = create_user(token, payload)
                    if ok:
                        st.success(msg)
                    else:
                        st.error(msg)

    with tab_get:
        uuid_get = st.text_input("UUID пользователя", key="get_uuid")
        if st.button("Загрузить", key="btn_get"):
            if not uuid_get or not uuid_get.strip():
                st.warning("Введите UUID")
            else:
                data, err = get_user(token, uuid_get.strip())
                if err:
                    st.error(err)
                else:
                    st.json(data)

    with tab_delete:
        uuid_del = st.text_input("UUID для удаления (мягкое)", key="del_uuid")
        if st.button("Удалить", key="btn_del", type="secondary"):
            if not uuid_del or not uuid_del.strip():
                st.warning("Введите UUID")
            else:
                ok, msg = delete_user(token, uuid_del.strip())
                if ok:
                    st.success(msg)
                else:
                    st.error(msg)


_LEVEL_COLORS = {
    "ERROR": "🔴",
    "WARN":  "🟡",
    "WARNING": "🟡",
    "INFO":  "🔵",
    "DEBUG": "⚪",
}

_SERVICES = [
    ("Все сервисы", None),
    ("main-service", "otus-microservice-be-prod"),
    ("auth-proxy",   "otus-microservice-auth-proxy-prod"),
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
            "Период", [1, 3, 6, 12, 24], format_func=lambda h: f"Последние {h}ч",
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
    elif page == PAGE_LOGS:
        render_logs()
    else:
        render_dashboard()


if __name__ == "__main__":
    main()
