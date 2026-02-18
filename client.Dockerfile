FROM python:3.11-slim

# Системные зависимости (curl для healthcheck)
RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Устанавливаем зависимости (кэшируется отдельно от кода)
COPY client/requirements.txt ./requirements.txt
RUN pip3 install --no-cache-dir -r requirements.txt

# Копируем исходный код клиента
COPY client/app.py ./app.py
COPY client/api.py ./api.py

# Streamlit config: headless mode, фиксированный порт, без телеметрии
ENV STREAMLIT_SERVER_PORT=8501 \
    STREAMLIT_SERVER_ADDRESS=0.0.0.0 \
    STREAMLIT_SERVER_HEADLESS=true \
    STREAMLIT_BROWSER_GATHER_USAGE_STATS=false

EXPOSE 8501

HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
    CMD curl -f http://localhost:8501/_stcore/health || exit 1

CMD ["streamlit", "run", "app.py"]
