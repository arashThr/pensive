#!/bin/bash
# PostgreSQL Status Check Script

set -euo pipefail

usage() {
    echo "Usage: $0 [-c <container>] [-u <user>] [-d <database>]"
    echo ""
    echo "Options:"
    echo "  -c  Postgres container name  (default: go-web-db-1)"
    echo "  -u  Postgres user            (default: POSTGRES_USER from .env)"
    echo "  -d  Database name            (default: DB_NAME from .env)"
    exit 1
}

ENV_FILE="$(dirname "$0")/.env"
if [ -f "$ENV_FILE" ]; then
    source "$ENV_FILE"
fi

POSTGRES_CONTAINER="go-web-db-1"
POSTGRES_USER="${POSTGRES_USER:-prod_user}"
POSTGRES_DB="${DB_NAME:-pensive_prod}"

while getopts "c:u:d:h" opt; do
    case $opt in
        c) POSTGRES_CONTAINER="$OPTARG" ;;
        u) POSTGRES_USER="$OPTARG" ;;
        d) POSTGRES_DB="$OPTARG" ;;
        h) usage ;;
        *) usage ;;
    esac
done

PSQL="docker exec $POSTGRES_CONTAINER psql -U $POSTGRES_USER -d $POSTGRES_DB"

echo "=============================="
echo " DB: $POSTGRES_DB"
echo " Container: $POSTGRES_CONTAINER"
echo "=============================="

echo "--- Row counts ---"
$PSQL -c "
    SELECT schemaname, relname AS tablename, n_live_tup AS rows
    FROM pg_stat_user_tables
    ORDER BY n_live_tup DESC;"

echo "--- Table sizes ---"
$PSQL -c "
    SELECT relname AS tablename,
           pg_size_pretty(pg_total_relation_size(schemaname || '.' || relname)) AS size
    FROM pg_stat_user_tables
    ORDER BY pg_total_relation_size(schemaname || '.' || relname) DESC;"

echo ""
echo "--- Database size ---"
$PSQL -c "
    SELECT pg_size_pretty(pg_database_size('$POSTGRES_DB')) AS total_size;"

echo ""
echo "--- Sequences (last values) ---"
$PSQL -c "
    SELECT sequencename, last_value
    FROM pg_sequences
    WHERE schemaname = 'public'
    ORDER BY sequencename;"
