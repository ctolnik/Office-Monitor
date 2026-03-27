# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Project Overview

Office-Monitor is an employee activity monitoring system built with Go. It consists of:
1. **Server** - Backend that receives and stores monitoring data
2. **Agent** - Windows client (service + session-helper) that monitors employee activity (USB, screenshots, keylogging, file operations)
3. **Web UI** - Dashboard for viewing monitoring data

**Architecture:**
- **Server**: Go application using Gin web framework, runs on any platform
- **Agent**: Windows-only Go application (uses Windows API for monitoring)
- **Database**: ClickHouse for time-series event storage
- **Storage**: MinIO for screenshot and file storage (S3-compatible)
- **Frontend**: HTML templates with static assets served by server
- **Logging**: Zap structured logger with custom context integration (`zapctx` package)

## Repository and Submodule Workflow (Critical)

- `frontend/` is a Git submodule pointing to `git@github.com:ctolnik/office-visor-ru.git` (`main`).
- If frontend code changes:
  1. Commit + push inside `frontend/` first.
  2. Then commit updated submodule pointer in superproject (`git add frontend`).
- On `docker-01` after pull, always run:
  - `git submodule sync --recursive`
  - `git submodule update --init --recursive --checkout`

## Legacy Backups (Do Not Delete Blindly)

Check these paths before any cleanup/migration:

- Server backup paths:
  - `/opt/Office-Monitor-submodule-backup-20260327_142149`
  - `/opt/Office-Monitor-metadata-backup-20260327_142906`
  - Patterns: `/opt/Office-Monitor-submodule-backup-*`, `/opt/Office-Monitor-metadata-backup-*`
- Local backup paths:
  - `/Users/ikokv/dev/go/Office-Monitor-metadata-backup-20260327_172906`
  - `/Users/ikokv/dev/go/Office-Monitor-legacy-frontend-backup-20260327_174003`
  - Pattern: `/Users/ikokv/dev/go/Office-Monitor-*-backup-*`

## Repository Structure

```
Office-Monitor/
├── server/               # Backend server (cross-platform)
│   ├── main.go          # Entry point, Gin setup, route registration
│   ├── handlers.go      # HTTP handlers (monolithic, planned refactor)
│   ├── handlers_*.go    # Split handlers (screenshots/settings/categories)
│   ├── config/          # Configuration loading (YAML)
│   ├── database/        # ClickHouse client and queries
│   ├── storage/         # MinIO integration
│   ├── Dockerfile       # Multi-stage build for server
│   └── config.yaml.example # Example config (config.yaml mounted in docker)
├── agent/               # Windows monitoring agent
│   ├── main.go          # Entry point (detect service vs interactive)
│   ├── service_windows.go # Windows Service implementation (Session 0)
│   ├── cmd/             # Helper processes
│   │   ├── session-helper/    # Universal session-helper (runs in user session)
│   │   └── screenshot-helper/ # Legacy helper (planned deprecation)
│   ├── config/          # Agent configuration
│   ├── monitoring/      # Monitoring implementations
│   │   ├── *_windows.go # Windows-specific implementations
│   │   └── *_stub.go    # Non-Windows stubs
│   ├── pkg/             # Shared packages (logger/ipc)
│   └── config.yaml      # Example configuration
├── zapctx/              # Custom zap logger context utility package
├── clickhouse/          # ClickHouse initialization/migrations
│   ├── 01-schema.sql    # Schema (idempotent)
│   ├── 02-seed-data.sql # Seed data (categories/catalog)
│   └── 03-grafana-views.sql # Optional views for Grafana
├── frontend/            # React SPA frontend (git submodule)
│   ├── src/            # TypeScript React components
│   ├── public/         # Static assets
│   └── Dockerfile      # Multi-stage Vite build + nginx
├── web/                 # Legacy frontend assets (deprecated)
│   ├── templates/       # HTML templates
│   └── static/          # CSS, JS, images
├── docs/                # Documentation
│   └── LOGGING.md       # Logging best practices (Russian)
└── docker-compose.yml   # Local development environment
```

**Key architectural notes:**
- Server and agent are separate Go modules with independent `go.mod` files
- Agent uses build tags and stub files for cross-platform compilation
- Server expects `zapctx` package to be available (shared dependency)
- Frontend is a separate git submodule (React + TypeScript + shadcn/ui)
- **Employees page removed** - employee data sourced from Active Directory instead

## Development Commands

### Running the Server

**Local development (from `server/` directory):**
```bash
cd server
go run main.go
```

The server expects:
- `config.yaml` in the current directory
- ClickHouse running on `localhost:9000` (use docker-compose)
- Templates and static files at `../web/templates/*` and `../web/static`

**Using Docker Compose (from project root):**
```bash
# Start only ClickHouse
docker-compose up clickhouse

# Or start all services (uncomment server in docker-compose.yml first)
docker-compose up
```

### Building

**Server - local binary:**
```bash
cd server
go build -o monitoring-server .
```

**Server - Docker image:**
```bash
# Build from project root (Dockerfile expects ../web)
docker build -f server/Dockerfile -t office-monitor-server .
```

**Agent - Windows executable (from macOS/Linux):**
```bash
cd agent

# С консолью (для отладки)
make build-windows

# Без консоли (production, скрытый режим)
make build-service

# Universal session-helper (agent-sh.exe)
make build-helper

# Build both
make build-all

# Или напрямую через go
GOOS=windows GOARCH=amd64 go build -o employee-agent.exe .
```

**Agent - local (Windows only):**
```bash
cd agent
go build -o employee-agent.exe .
```

### Testing

Limited test coverage exists. Tests are primarily in:
- `zapctx/zapctx_test.go` - Context logger tests
- `agent/monitoring/*_test_windows.go` - Agent monitoring tests (Windows only)

**Run all tests:**
```bash
go test ./...
```

**Run tests in specific package:**
```bash
go test ./zapctx
go test ./server/database
```

**Run with verbose output:**
```bash
go test -v ./...
```

**Run agent tests (Windows only):**
```bash
cd agent
go test ./monitoring
```

## Configuration

Configuration is loaded from `server/config.yaml` with environment variable expansion support.

Key configuration sections:
- `server`: HTTP server settings (host, port, mode, API key)
- `database`: ClickHouse connection parameters
- `storage`: MinIO settings (currently commented out)
- `monitoring`: Feature flags for activity tracking, screenshots, keylogger, USB, file copy
- `alerts`: Notification settings
- `security`: Authentication and CORS settings
- `logging`: Log level and file settings

Environment variables can be used with `${VAR_NAME}` syntax, e.g., `${CLICKHOUSE_PASSWORD}`.

## Database Schema

ClickHouse tables (defined in `clickhouse/01-schema.sql`):
- `activity_events` - User window/process activity with duration
- `keyboard_events` - Keylogger data (sensitive, requires consent)
- `file_copy_events` - File operations tracking
- `usb_events` - USB device connection/disconnection
- `screenshot_metadata` - Screenshot references (images in MinIO)
- `alerts` - Suspicious activity alerts
- `agent_configs` - Per-agent configuration
- `employees` - Employee registry with consent tracking (deprecated - data from AD)

All tables use:
- `MergeTree` engine with monthly partitioning
- 180-day TTL for automatic data retention
- Bloom filter indexes on usernames for fast lookups

## API Endpoints

Current active endpoints (in `server/main.go`):

**Frontend:**
- `GET /` - Dashboard HTML page

**API:**
- `POST /api/events/batch` - Receive batch events from agent service (includes `activity_segment`)
- `POST /api/screenshot` - Receive screenshots (metadata + image data)
- `GET /api/employees` - List active employees (last hour)
- `GET /api/recent` - Get recent activity (last 100 events)

**Commented out endpoints** (not currently active):
- USB events, file events, screenshots, keyboard events

All handlers use JSON for request/response bodies.


## Code Patterns

**Handler structure:**
```go
func handlerName(c *gin.Context) {
    ctx := c.Request.Context()  // IMPORTANT: Use Gin's context, NOT context.Background()
    var event database.EventType
    if err := c.ShouldBindJSON(&event); err != nil {
        zapctx.Warn(ctx, "Invalid request", zap.Error(err))
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }
    
    if err := db.InsertEvent(ctx, event); err != nil {
        zapctx.Error(ctx, "Failed to save event", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
        return
    }
    
    zapctx.Info(ctx, "Event saved successfully", zap.String("computer", event.ComputerName))
    c.JSON(http.StatusOK, gin.H{"status": "success"})
}
```

**Database operations:**
- All database methods accept `context.Context` as first parameter
- Use parameterized queries with `?` placeholders
- ClickHouse driver: `github.com/ClickHouse/clickhouse-go/v2`
- Log slow queries with duration and context

**Logging patterns (see `docs/LOGGING.md` for details):**
- **ALWAYS use `c.Request.Context()` in handlers**, NEVER `context.Background()`
- Use structured logging: `zapctx.Info(ctx, "Message", zap.String("key", "value"))`
- Levels: Debug (development), Info (operations), Warn (suspicious), Error (failures)
- Every request has a `request_id` for tracing
- Functions: `zapctx.Debug()`, `zapctx.Info()`, `zapctx.Warn()`, `zapctx.Error()`

## Important Notes

1. **The server and web directories are tightly coupled**: The server expects web templates/static files at relative paths (`../web/`)

2. **ClickHouse is the primary data store**: No traditional relational database. All event data is time-series with automatic partitioning and TTL.

3. **Agent is Windows-only**: Uses Windows API for monitoring. Has stub implementations for non-Windows platforms.

4. **Agent runs hidden**: Built with `-H=windowsgui` flag to hide console window. Uses mutex to prevent multiple instances.

5. **Module path**: The project uses `github.com/ctolnik/Office-Monitor` with separate modules for server and agent.

6. **Config loading**: Uses `os.ExpandEnv()` to support environment variables in `config.yaml` before YAML parsing.

7. **Graceful shutdown**: Agent uses context.Context for proper cleanup with 10-second timeout.

## Environment Setup

**Required services for development:**
- ClickHouse 25.9+ (port 9000 for native protocol, 8123 for HTTP)
- MinIO (optional, only if enabling screenshot storage)

**Go version:** 1.24.5 (as specified in `go.mod`)

**Key dependencies:**
- `github.com/gin-gonic/gin` - Web framework
- `github.com/ClickHouse/clickhouse-go/v2` - ClickHouse driver
- `go.uber.org/zap` - Structured logging
- `github.com/minio/minio-go/v7` - MinIO client
- `gopkg.in/yaml.v3` - YAML parsing
