# AGENTS.md — Office Monitor

## Stack & Architecture

- **Go 1.24.5** monorepo; `server/` and `agent/` are separate Go modules
- **Agent**: Windows-only monitoring service (USB, screenshots, keylogging, activity). Uses build tags + stub files for cross-platform builds
- **Server**: Go + Gin, serves both API and HTML dashboard via `web/templates/` and `web/static/`
- **Database**: ClickHouse (time-series events, monthly partitioning, 180-day TTL)
- **Storage**: MinIO (screenshots, USB copies)
- **Frontend**: React SPA in `frontend/` — **git submodule** pointing to `git@github.com:ctolnik/office-visor-ru.git`

## Dev Commands

```bash
# Root Makefile
make ci              # lint → test (order matters)
make fmt             # format server + agent + zapctx
make lint            # server only (golangci-lint)
make test            # server tests with race detector + coverage

# Server
cd server && go run main.go          # requires config.yaml + ClickHouse locally
make build                             # outputs bin/monitoring-server

# Agent (cross-compile from macOS/Linux)
cd agent && make build-all            # builds agent-svc.exe + agent-sh.exe

# Docker
docker-compose up -d                  # all services (requires submodule init)
docker-compose up clickhouse          # ClickHouse only
make migrate                          # apply clickhouse/migrations.sql
```

## Submodule Flow (Critical)

Frontend is a submodule. Always push frontend changes **before** committing the submodule pointer:

```bash
# 1) Push frontend changes first
cd frontend && git push origin main

# 2) Then update submodule pointer in root
git add frontend && git commit -m "Update frontend submodule" && git push origin main
```

On clone or pull, always init submodules:
```bash
git submodule update --init --recursive
```

## Logging

Always use `c.Request.Context()` in handlers — **never** `context.Background()`:
```go
ctx := c.Request.Context()
zapctx.Info(ctx, "message", zap.String("key", "value"))
```
Functions: `zapctx.Debug()`, `Info()`, `Warn()`, `Error()`. See `docs/LOGGING.md`.

## Key Gotchas

- Server expects `web/templates/` and `web/static/` at relative paths from working dir
- Server Dockerfile copies `./web` into the image — changes to `web/` require rebuild
- Agent builds with `-H=windowsgui` by default (hides console window)
- Config supports `${ENV_VAR}` syntax (expanded before YAML parse)
- Go module path: `github.com/ctolnik/Office-Monitor` with sub-paths `server` and `agent`
- `zapctx/` is a shared package — must be available when building server

## Docs

All project docs are in `docs/`. Entry point: `docs/README.md`.
- `docs/REPOSITORY_FLOW.md` — submodule and deployment flow
- `docs/DEPLOYMENT.md` — server deployment
- `docs/AGENT_SETUP.md` — Windows agent installation
- `docs/LOGGING.md` — logging conventions
