#!/bin/bash

# Run this script first time before adding db-backup.sh to crontab
# 0 0 * * * /home/ubuntu/scripts/backup/db-backup.sh > /dev/null 2>&1

set -euo pipefail

# Check if .env file exists
if [ ! -f .env ]; then
    echo "❌ ERROR: .env file not found!" >&2
    exit 1
fi

# Set permissions
chmod 600 .env
chown $USER .env

# Check if rclone is configured
rclone config file | grep -q "\[$RCLONE_REMOTE\]"
if [ $? -ne 0 ]; then
    echo "❌ ERROR: rclone is not configured!" >&2
    exit 1
fi

chmod +x db-backup.sh
