# Employee Activity Monitoring System

## Overview

Employee activity monitoring system for 30-person office with Active Directory and Windows 10+. Features activity tracking (active/idle/offline states), periodic screenshots, USB monitoring, mass file copy detection, and optional keyboard logging. Application categorization (productive/unproductive/neutral/communication/entertainment) with friendly names and chronological timeline. Data retained 6 months with employee consent.

## Documentation

- `docs/DEPLOYMENT.md` - Server deployment and migrations
- `docs/AGENT_SETUP.md` - Windows agent installation
- `docs/AI_CONTEXT.md` - Context for AI agents
- `docs/LOGGING.md` - Logging configuration
- `grafana/README.md` - Grafana dashboard setup

## User Preferences

Preferred communication style: Simple, everyday language (Russian).

## System Architecture

### Backend Architecture

The backend is a monolithic Go (Golang) web server utilizing its built-in HTTP routing for REST API endpoints. It uses ClickHouse for time-series data and MinIO for object storage. Key components include `main.go` for server setup and `database.go` for data persistence. The API supports employee management, activity tracking, and real-time status updates. Go was chosen for its performance, single-binary deployment, and concurrency.

### Frontend Architecture

The frontend uses vanilla JavaScript with server-side HTML templates (`/web/templates/index.html`). Dynamic updates are handled by client-side JavaScript (`/web/static/app.js`) and styled with CSS (`/web/static/style.css`). It features a real-time dashboard with polling, employee lists with filtering, activity timelines, statistics visualization, and a tabbed interface. This approach prioritizes a lightweight frontend without complex build tools.

### Windows Agent Architecture

The agent is a Go-compiled Windows executable that monitors active windows and application focus, collects activity data locally, and periodically sends it to the server via HTTP. It runs as a background process and includes robustness features such as:
- **Circuit Breaker**: `sony/gobreaker` for preventing server overload.
- **Retry Logic**: 3 attempts with 5-second delays for transient errors.
- **Event Buffer**: 1000-event memory buffer with disk persistence for offline operation.
- **Graceful Shutdown**: Ensures no data loss on restart.
It uses Windows API calls for real-time monitoring and is cross-compiled for easy deployment.

### Data Storage

The system primarily uses **ClickHouse** for time-series data and **MinIO** for file storage.
- **ClickHouse tables** store `activity_segments`, `screenshots` metadata, `usb_events`, `file_events`, `keyboard_events`, and `alerts`. Activity states are categorized as active, idle, or offline.
- **MinIO** stores actual screenshots and USB shadow copy files with configurable retention policies (e.g., 30 days for screenshots).

### Authentication and Authorization

Currently, no authentication layer is implemented, assuming deployment within a trusted internal network. Future considerations include adding API-level authentication for multi-tenant or external access.

## External Dependencies

- **Go Standard Library**: `net/http`, `encoding/json`, `html/template`.
- **Windows API (Agent Only)**: `golang.org/x/sys/windows`, `user32.dll`, `kernel32.dll` for system interactions.
- **Browser APIs (Frontend)**: Fetch API, DOM manipulation.
- **Deployment Platform**: Replit (for hosting and auto-startup).
- **Go Libraries (Server)**:
    - `github.com/gin-gonic/gin` (web framework)
    - ClickHouse Go driver
    - MinIO Go SDK
- **No External Services**: The system is self-contained and designed for on-premise deployment, utilizing ClickHouse and MinIO as local services (via `docker-compose.yml`).

### Grafana Analytics (Added December 2025)

Grafana dashboards for advanced analytics (uses external Grafana instance):
- **Dashboard JSON**: `grafana/dashboards/employee-activity.json` — import into existing Grafana
- **Setup Guide**: `grafana/README.md` — instructions for datasource and dashboard import
- **SQL Views**: `clickhouse/03-grafana-views.sql` — helper views for Grafana queries
- **Required Plugin**: `grafana-clickhouse-datasource`
- **Dashboard "Активность сотрудника"** includes:
  - Stat panels: active time, idle time, productive time, app count
  - Pie charts: time by applications (with friendly names), time by categories
  - Bar chart: top applications by usage
  - Table: chronological activity timeline with window titles
  - Time series: hourly activity breakdown
  - Variable `$employee` for employee selection

### Frontend Reports (Updated January 2026)

Enhanced reporting UI with category visualization:
- **Tabs**: Overview, Activity, Statistics, Daily Report
- **Categories**: productive (green), unproductive (red), neutral (gray), communication (blue), system (dark-gray), entertainment (orange)
- **Daily Report** includes:
  - Day summary (events, screenshots, USB, files)
  - Chronological timeline with categories
  - Top applications with usage bars
  - Screenshots grid with preview
  - DLP/Security alerts
- **Statistics** includes:
  - Category distribution with percentages
  - Application usage with progress bars
  - Color-coded category badges

### API Endpoints

Key reporting endpoints:
- `GET /api/activity/:username` - activity events with categories
- `GET /api/stats/:username` - application statistics
- `GET /api/reports/daily/:username` - comprehensive daily report
- `GET /api/activity/applications/:username` - application usage with categories