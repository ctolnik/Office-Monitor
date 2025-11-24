# Office-Monitor

Comprehensive employee activity monitoring system with DLP capabilities for Windows environments.

## Project Structure

- **server/** - Go REST API server with ClickHouse + MinIO backend
- **agent/** - Windows monitoring agent (cross-compiled from Linux)
- **clickhouse/** - Database initialization scripts
- **docker-compose.yml** - Full stack deployment configuration

## Quick Build

```bash
# Build server
cd server && go build -o monitoring-server .

# Build Windows agent (cross-compile)
cd agent && GOOS=windows GOARCH=amd64 go build -o employee-agent.exe .
```

## Docker Deployment

```bash
docker-compose up -d
```

Server runs on port 5000. See `replit.md` for full architecture documentation.

## Features

- Real-time activity tracking (active/idle/offline states)
- Screenshot capture with MinIO storage
- USB device monitoring with shadow copying
- File copy detection and DLP alerts
- Keyboard activity logging (optional, compliance required)
- Process catalog with friendly names
- Daily productivity reports with scoring

## Documentation

- **replit.md** - Complete system architecture and technical details
- **agent/README.md** - Agent configuration and deployment guide
- **FRONTEND_SPECIFICATION.md** - Frontend API specification (for separate React dashboard)
