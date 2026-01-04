# AI Agent Context

This file provides context for AI assistants working on this project.

## Project Overview

Employee activity monitoring system for Windows offices (30 employees).
Tracks computer usage, active windows, screenshots, USB devices, file operations.

## Architecture

```
┌─────────────┐     HTTP      ┌─────────────┐     SQL       ┌─────────────┐
│   Windows   │ ───────────── │  Go Server  │ ───────────── │  ClickHouse │
│   Agent     │               │  (port 5000)│               │             │
└─────────────┘               └──────┬──────┘               └─────────────┘
                                     │
                                     │ S3
                                     ▼
                              ┌─────────────┐
                              │    MinIO    │
                              │ (screenshots│
                              └─────────────┘
```

## Key Directories

| Path | Description |
|------|-------------|
| `server/` | Go backend (Gin framework) |
| `server/database/` | ClickHouse queries |
| `server/handlers/` | HTTP handlers |
| `agent/` | Windows monitoring agent (Go) |
| `agent/monitoring/` | Monitoring modules |
| `clickhouse/` | SQL migrations |
| `grafana/` | Dashboard JSON |
| `docs/` | Documentation |

## Database Schema (ClickHouse)

Main tables:
- `activity_segments` - Time blocks with app/window/category
- `employees` - Employee list (auto-populated from agents)
- `process_catalog` - App friendly names and categories
- `application_categories` - Category definitions
- `screenshot_metadata` - Screenshot info (files in MinIO)
- `usb_events` - USB device connections
- `file_copy_events` - Large file copy detection
- `keyboard_events` - Keystroke logging (optional)
- `alerts` - Security alerts

## Activity States

| State | Description |
|-------|-------------|
| `active` | User actively working |
| `idle` | No input for 5+ minutes |
| `offline` | Agent not reporting |

## Application Categories

| Category | Examples |
|----------|----------|
| `productive` | 1C, Excel, IDE, CAD |
| `unproductive` | Games |
| `neutral` | Explorer, Notepad |
| `communication` | Outlook, Teams, Telegram |
| `entertainment` | YouTube, VK, social media |

Categories are case-sensitive lowercase strings.

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/activity/batch` | Agent sends activity data |
| POST | `/api/screenshots` | Agent uploads screenshot |
| GET | `/api/employees` | List employees |
| GET | `/api/employees/{id}/report` | Daily report |
| GET | `/api/employees/{id}/timeline` | Activity timeline |
| GET | `/health` | Health check |

## Agent Configuration

Key settings in `config.yaml`:
- `agent.server.url` - Server URL
- `activity_monitoring.interval_seconds` - Poll interval (30s)
- `screenshots.enabled` - Enable screenshots
- `screenshots.interval_minutes` - Screenshot interval (15m)

Agent runs as Windows Service (LocalSystem). Uses WTS API for username detection.

## Logging

Server uses structured logging with `zapctx`:
- Request context includes `request_id`
- Log levels: debug, info, warn, error
- Logs in JSON format

## Common Tasks

### Add new category
Edit `clickhouse/02-seed-data.sql`, add to `application_categories` table.

### Add app to catalog
Use API or insert into `process_catalog` table:
```sql
INSERT INTO process_catalog (process_name, friendly_name, category)
VALUES ('notepad++.exe', 'Notepad++', 'productive');
```

### Deploy changes
```bash
git pull
docker-compose build server
docker-compose up -d server
```

### Apply migrations
```bash
docker exec -i monitoring-clickhouse clickhouse-client --database monitoring \
  --multiquery < clickhouse/01-schema.sql
```

## Security Notes

- **API requires authentication**: агент (через сервис) отправляет `X-API-Key`, сервер валидирует ключ
- Agents may auto-register employees on first connection (but access is still protected by API key)

## MinIO Storage

- Bucket: `screenshots`
- Files: `{employee_id}/{date}/{timestamp}.jpg`
- Retention: 30 days (lifecycle policy)
- Access: via presigned URLs from server

## Alerts

Alerts generated automatically for:
- Mass file copy (>100MB in short time)
- USB device connections (when shadow_copy enabled)
- Suspicious process names (configurable)

Stored in `alerts` table, no push notifications.

## Known Limitations

1. No RDS/Terminal Services support - one agent per physical PC
2. No real-time push (polling-based)
3. Agent requires Windows 10+ (uses modern APIs)
4. No authentication layer (internal network only)

## Environment Variables

Server reads from environment:
- `CLICKHOUSE_HOST`, `CLICKHOUSE_USER`, `CLICKHOUSE_PASSWORD`
- `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`
- See `.env.example` for full list

## Files to Update

When making changes, update:
1. Code in `server/` or `agent/`
2. `replit.md` - project overview (canonical architecture doc)
3. `docs/AI_CONTEXT.md` - this file
4. `clickhouse/*.sql` - if schema changes
