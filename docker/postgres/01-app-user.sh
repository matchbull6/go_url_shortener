#!/bin/sh
# Runs once, on an empty data volume. The app gets its own non-superuser role
# that owns the shortener database, so it can run migrations but nothing more.
set -eu

: "${APP_DB_PASSWORD:?APP_DB_PASSWORD is required}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
	-v app_password="$APP_DB_PASSWORD" <<'EOSQL'
CREATE ROLE shortener LOGIN PASSWORD :'app_password';
ALTER DATABASE shortener OWNER TO shortener;
EOSQL
