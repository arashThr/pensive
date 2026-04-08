#!/bin/bash
# Simple PostgreSQL Backup Script

set -euo pipefail

# Load environment variables
ENV_FILE="$(dirname "$0")/.env"
if [ -f "$ENV_FILE" ]; then
    source "$ENV_FILE"
else
    echo "❌ ERROR: Environment file $ENV_FILE not found!" >&2
    exit 1
fi

# Validate required environment variables
REQUIRED_VARS=("TELEGRAM_BOT_TOKEN" "TELEGRAM_CHAT_ID")
for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        echo "❌ ERROR: Required environment variable $var is not set!" >&2
        exit 1
    fi
done

# Configuration
BACKUP_DIR="$HOME/backups/postgres-backups"
POSTGRES_CONTAINER=pensive-db-1
KEEP_DAYS=14  # Keep backups for 14 days

# Database credentials (adjust these)
POSTGRES_USER="${POSTGRES_USER:-prod_user}"
POSTGRES_DB="${DB_NAME:-pensive_prod}"

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Generate timestamp
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="db_${TIMESTAMP}.sql.gz"
BACKUP_PATH="$BACKUP_DIR/$BACKUP_FILE"

# Create a function to send a message to telegram
function send_telegram_message {
    curl -s -X POST "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/sendMessage" \
    		-d "chat_id=$TELEGRAM_CHAT_ID" \
            -d "text=$1" >/dev/null || {
            echo "⚠️ WARNING: Failed to send Telegram message!" >&2
        }
}

echo "🚀 Starting backup at $(date)"
send_telegram_message "Backup started at $(date)"
echo "📦 Container: $POSTGRES_CONTAINER"

# Create database dump
echo "💾 Creating database backup..."
docker exec "$POSTGRES_CONTAINER" pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" | gzip > "$BACKUP_PATH"

# Check if backup was successful
if [ -f "$BACKUP_PATH" ]; then
	BACKUP_SIZE=$(du -h "$BACKUP_PATH" | cut -f1)
	echo "✅ Backup created: $BACKUP_FILE (Size: $BACKUP_SIZE)"
else
	echo "❌ ERROR: Backup failed!"
	send_telegram_message "Backup failed at $(date)"
	exit 1
fi

# Clean up old backups
echo "🧹 Cleaning up old backups (older than $KEEP_DAYS days)..."
find "$BACKUP_DIR" -type f -name 'db_*.sql.gz' -mtime +"$KEEP_DAYS" -delete

# Count remaining backups
BACKUP_COUNT=$(ls -1 "$BACKUP_DIR"/db_*.sql.gz 2>/dev/null | wc -l)
echo "📊 Total backups stored: $BACKUP_COUNT"

echo "🎉 Backup completed successfully at $(date) - Backup location: $BACKUP_PATH"

rclone copy "$BACKUP_PATH" "r2_backup_server:pensive"

send_telegram_message "Backup completed successfully at $(date)"
