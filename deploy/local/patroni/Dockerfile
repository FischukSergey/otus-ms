# Patroni + PostgreSQL 16 на базе postgres:16-alpine.
# Для локального отказоустойчивого кластера (1 primary + 1 replica).
FROM postgres:16-alpine

# Патрони и etcd-клиент работают под пользователем postgres
USER root

# Python3 и зависимости для Patroni (psycopg2-binary чтобы не тянуть postgresql-dev)
RUN apk add --no-cache \
    python3 \
    py3-pip \
    py3-setuptools \
    curl \
    && pip3 install --no-cache-dir --break-system-packages \
    psycopg2-binary \
    'patroni[etcd]'

# Скрипт-заглушка для bootstrap existing_data: не делать initdb, только вернуть 0
RUN echo '#!/bin/sh' > /bootstrap-existing-data.sh \
    && echo 'exit 0' >> /bootstrap-existing-data.sh \
    && chmod +x /bootstrap-existing-data.sh

# Каталог для конфигов (переопределяется через volume или копирование)
RUN mkdir -p /etc/patroni && chown -R postgres:postgres /etc/patroni

# Patroni будет запускать PostgreSQL; PGDATA по умолчанию как в образе postgres
ENV PGDATA=/var/lib/postgresql/data
ENV PATRONI_CONFIG_PATH=/etc/patroni/patroni.yml
# Чтобы patroni и patronictl были в PATH (pip ставит в /usr/local/bin или ~/.local/bin)
ENV PATH="${PATH}:/usr/local/bin"

# Entrypoint-обёртка: исправляет права на PGDATA (Docker volume создаёт 0777, PG требует 0700)
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER postgres

# Подключаем конфиг при старте (в compose передаём разный конфиг для node1/node2)
ENTRYPOINT ["/entrypoint.sh"]
CMD ["/etc/patroni/patroni.yml"]
