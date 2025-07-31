# Backup

Simply, run the script, export the tables and store it.
Run a cron job to sync with R2.

## Configs

__R2__:
Uses `rclone` for syncing. You will need the access and secret key that you can get by following [this](https://developers.cloudflare.com/r2/examples/rclone/) guide on Cloudflare website.

You can find the final configuration file in `~/.config/rclone/rclone.conf`

__Telegram__:
We send Telegram notification for events.
Env vars are set in `~/.localrc.sh` and are accessible by the application.

## Crontab

Finally, set the crontab:

`0 3 * * * $HOME/Documents/repos/go-web/scripts/backup/db-backup.sh >> /home/arash/postgres-backups/backup.log 2>&1`
