#!/bin/sh
set -eu

POSTGRES_HOST="${POSTGRES_HOST:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-postgres}"
APP_DB_USER="${APP_DB_USER:-admin}"
APP_DB_PASSWORD="${APP_DB_PASSWORD:-admin}"
AUTH_DB_NAME="${AUTH_DB_NAME:-admin}"
GAME_DB_NAME="${GAME_DB_NAME:-game_db}"

export PGPASSWORD="${POSTGRES_PASSWORD}"

echo "Waiting for postgres at ${POSTGRES_HOST}..."
until pg_isready -h "${POSTGRES_HOST}" -U "${POSTGRES_USER}" -d postgres >/dev/null 2>&1; do
  sleep 2
done

psql_base() {
  psql -h "${POSTGRES_HOST}" -U "${POSTGRES_USER}" -d postgres -v ON_ERROR_STOP=1 "$@"
}

role_exists="$(psql_base -tAc "SELECT 1 FROM pg_roles WHERE rolname = '${APP_DB_USER}'")"
if [ "${role_exists}" != "1" ]; then
  psql_base -c "CREATE ROLE ${APP_DB_USER} LOGIN PASSWORD '${APP_DB_PASSWORD}'"
fi

ensure_database() {
  db_name="$1"
  db_exists="$(psql_base -tAc "SELECT 1 FROM pg_database WHERE datname = '${db_name}'")"
  if [ "${db_exists}" != "1" ]; then
    psql_base -c "CREATE DATABASE ${db_name} OWNER ${APP_DB_USER}"
  fi
}

ensure_database "${AUTH_DB_NAME}"
ensure_database "${GAME_DB_NAME}"

psql_base -c "ALTER ROLE ${APP_DB_USER} WITH LOGIN PASSWORD '${APP_DB_PASSWORD}'"
psql_base -c "GRANT ALL PRIVILEGES ON DATABASE ${AUTH_DB_NAME} TO ${APP_DB_USER}"
psql_base -c "GRANT ALL PRIVILEGES ON DATABASE ${GAME_DB_NAME} TO ${APP_DB_USER}"

echo "Database bootstrap finished successfully."
