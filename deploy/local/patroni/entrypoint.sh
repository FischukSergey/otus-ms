#!/bin/sh
# Entrypoint для Patroni: исправляет права на PGDATA перед запуском.
# Docker volume создаёт каталог с правами 0777, PostgreSQL требует 0700/0750.
set -e

if [ -d "$PGDATA" ]; then
  chmod 0700 "$PGDATA"
fi

exec patroni "$@"
