# Employee Activity Monitoring System

## Overview

This project is a comprehensive employee activity monitoring system designed to track and analyze computer usage in an office environment. It comprises a Go-based web server, a Windows desktop agent, and a real-time web interface. The system monitors active windows, application usage, and employee status (active/idle/offline), providing administrators with real-time insights and historical activity reports. The business vision is to enhance productivity and security within internal networks.

## User Preferences

Preferred communication style: Simple, everyday language.

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