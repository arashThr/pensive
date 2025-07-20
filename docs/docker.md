# Docker Production Operations Cheat Sheet

Define an aliase: `alias dockerc=docker compose -f compose.yaml -f compose.production.yaml`

## Basic Service Management

### Start/Stop Services
```bash
# Start all services
dockerc up -d

# Stop all services
dockerc down

# Stop services gracefully (allows cleanup)
dockerc stop

# Start specific service
dockerc up -d db

# Restart specific service
dockerc restart server

# Stop accepting new requests (scale to 0)
dockerc scale server=0

# Scale a service (e.g., run multiple server instances):
dockerc up -d --scale server=3

```

### Service Status & Health
```bash
# View running containers
dockerc ps

# View detailed status
dockerc ps -a

# View resource usage
docker stats

# Check service health
dockerc top
```

## Database Operations

### Connect to Database
```bash
# Connect to PostgreSQL container
dockerc exec db psql -U ${POSTGRES_USER} -d ${DB_NAME}

# Alternative with environment variables
dockerc exec db psql -U your_username -d your_database

# Run single SQL command
dockerc exec db psql -U ${POSTGRES_USER} -d ${DB_NAME} -c "SELECT version();"
```

### Database Backup & Restore
```bash
# Create backup
dockerc exec db pg_dump -U ${POSTGRES_USER} ${DB_NAME} > backup_$(date +%Y%m%d_%H%M%S).sql

# Restore from backup
dockerc exec -T db psql -U ${POSTGRES_USER} -d ${DB_NAME} < backup_file.sql

# Create compressed backup
dockerc exec db pg_dump -U ${POSTGRES_USER} -d ${DB_NAME} | gzip > backup_$(date +%Y%m%d_%H%M%S).sql.gz
```

## Logs & Debugging

### View Logs
```bash
# View all logs
dockerc logs

# Follow logs in real-time
dockerc logs -f

# View specific service logs
dockerc logs -f server

# View last N lines
dockerc logs --tail=50 server

# View logs with timestamps
dockerc logs -t server

# View logs for specific time range
dockerc logs --since="2025-07-20T10:00:00" --until="2025-07-20T11:00:00"
```

### Debug Container Issues
```bash
# Execute shell in running container
dockerc exec server /bin/bash
dockerc exec server /bin/sh  # for Alpine

# Run temporary container for debugging
docker run -it --rm postgres:15-alpine /bin/sh

# Inspect container details
docker inspect container_name

# View container processes
dockerc exec server ps aux
```

## Updates & Maintenance

### Update Services
```bash
# Pull latest images
dockerc pull

# Rebuild and update (for custom images)
dockerc build --no-cache

# Update with zero downtime (rolling update)
dockerc up -d --no-deps server

# Update all services
dockerc up -d --build
```

### Cleanup
```bash
# Remove stopped containers
docker container prune

# Remove unused images
docker image prune

# Remove unused volumes (CAREFUL!)
docker volume prune

# Remove everything unused
docker system prune -a

# View disk usage
docker system df
```

## Emergency Operations

### Stop Accepting New Requests
```bash
# Scale service to 0 (immediate)
dockerc scale server=0

# Or stop the load balancer
dockerc stop caddy
```

### Quick Recovery
```bash
# Restart everything
dockerc restart

# Force recreate containers
dockerc up -d --force-recreate

# Recreate specific service
dockerc up -d --force-recreate server
```

### Database Emergency
```bash
# Create emergency backup before fixes
dockerc exec db pg_dump -U ${POSTGRES_USER} ${DB_NAME} > emergency_backup_$(date +%Y%m%d_%H%M%S).sql

# Restart database only
dockerc restart db

# View database logs
dockerc logs -f db
```

## Environment & Configuration

### Environment Variables
```bash
# View environment variables in container
dockerc exec server env

# Load new environment file
dockerc --env-file .env.production up -d
```

### Configuration Reload
```bash
# Reload Caddy configuration
dockerc exec caddy caddy reload --config /etc/caddy/Caddyfile
```

## Monitoring Commands

### Real-time Monitoring
```bash
# Monitor resource usage
docker stats

# Monitor logs across all services
dockerc logs -f | grep -i error

# Check container health
dockerc ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}"
```