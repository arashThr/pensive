#!/bin/bash
# Simple PostgreSQL Backup Script

set -euo pipefail

# Configuration
BACKUP_DIR="$HOME/postgres-backups"
POSTGRES_CONTAINER=go-web-db-1
KEEP_DAYS=14  # Keep backups for 14 days

# Database credentials (adjust these)
POSTGRES_USER="${POSTGRES_USER:-prod_user}"
POSTGRES_DB="${DB_NAME:-pensive_prod}"

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Generate timestamp
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="postgres_backup_${TIMESTAMP}.sql.gz"
BACKUP_PATH="$BACKUP_DIR/$BACKUP_FILE"

echo "🚀 Starting backup at $(date)"
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
	exit 1
fi

# Clean up old backups
echo "🧹 Cleaning up old backups (older than $KEEP_DAYS days)..."
find "$BACKUP_DIR" -name "postgres_backup_*.sql.gz" -mtime +$KEEP_DAYS -delete

# Count remaining backups
BACKUP_COUNT=$(ls -1 "$BACKUP_DIR"/postgres_backup_*.sql.gz 2>/dev/null | wc -l)
echo "📊 Total backups stored: $BACKUP_COUNT"

echo "🎉 Backup completed successfully at $(date)"
echo "📁 Backup location: $BACKUP_PATH"

# Optional: Simple email notification (if mail is configured)
# echo "PostgreSQL backup completed: $BACKUP_FILE" | mail -s "DB Backup OK" your-email@example.com
