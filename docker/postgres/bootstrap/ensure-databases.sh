#!/bin/sh
set -eu
POSTGRES_HOST="${POSTGRES_HOST:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-postgres}"
export PGPASSWORD="${POSTGRES_PASSWORD:-postgres}"
APP_DB_USER="${APP_DB_USER:-admin}"
APP_DB_PASSWORD="${APP_DB_PASSWORD:-admin}"
AUTH_DB_NAME="${AUTH_DB_NAME:-admin}"
GAME_DB_NAME="${GAME_DB_NAME:-game_db}"
[ "$AUTH_DB_NAME" != "$GAME_DB_NAME" ] || { echo 'Auth and game databases must be distinct.' >&2; exit 1; }
until pg_isready -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d postgres >/dev/null 2>&1; do sleep 2; done
# Quote identifiers/literals on the server. No data reset or historical dump import.
psql -X -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 \
 -v app_role="$APP_DB_USER" -v app_password="$APP_DB_PASSWORD" \
 -v auth_db="$AUTH_DB_NAME" -v game_db="$GAME_DB_NAME" <<'SQL'
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'app_role', :'app_password')
WHERE NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=:'app_role') \gexec
SELECT format('CREATE DATABASE %I OWNER %I', :'auth_db', :'app_role')
WHERE NOT EXISTS(SELECT 1 FROM pg_database WHERE datname=:'auth_db') \gexec
SELECT format('CREATE DATABASE %I OWNER %I', :'game_db', :'app_role')
WHERE NOT EXISTS(SELECT 1 FROM pg_database WHERE datname=:'game_db') \gexec
SELECT format('GRANT ALL PRIVILEGES ON DATABASE %I TO %I', :'auth_db', :'app_role') \gexec
SELECT format('GRANT ALL PRIVILEGES ON DATABASE %I TO %I', :'game_db', :'app_role') \gexec
SQL
echo 'Database provisioning complete. Services apply their own schema/content bootstrap.'
