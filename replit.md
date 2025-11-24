# Office-Monitor

## Overview

Office-Monitor is an employee activity monitoring system consisting of a Windows agent that collects activity data and a centralized web-based management platform. The agent monitors user activity, screenshots, USB connections, file operations, and optionally keyboard input. The web platform provides real-time dashboards, analytics, and activity reports for administrators.

The system follows a distributed architecture with agents reporting to a central API server, which stores events in ClickHouse and screenshots/files in MinIO object storage. A React-based frontend provides the administrative interface through an Nginx reverse proxy.

## User Preferences

Preferred communication style: Simple, everyday language.

## System Architecture

### Frontend Architecture
- **Technology**: React with TypeScript and Vite build system
- **Deployment**: Static files served through Nginx reverse proxy
- **API Communication**: REST API calls to `/api` endpoint (proxied by Nginx)
- **Configuration**: Environment-based feature flags for screenshots, keylogger, DLP monitoring
- **Auto-refresh**: Configurable intervals for agents list and dashboard data

### Backend Architecture
- **Language**: Go with Gin web framework
- **Logging**: Structured logging using Zap with context propagation (zapctx package)
- **Request Tracing**: Unique request IDs for correlation across logs
- **API Design**: RESTful endpoints for employee management and activity data
- **Middleware**: Gin-contrib/zap integration for HTTP request logging

### Agent Architecture
- **Platform**: Windows-only monitoring agent
- **Build Modes**: 
  - Debug mode with console window
  - Production mode (`-H=windowsgui`) as hidden background service
- **Features**:
  - Active window and process monitoring
  - USB device detection
  - Periodic screenshot capture
  - File operation monitoring for data loss prevention
  - Optional keylogger functionality
  - Mutex-based single instance enforcement
- **Deployment**: Manual installation or Windows Service via `sc` command

### Data Storage
- **ClickHouse**: Primary database for storing monitoring events and activity logs
- **MinIO**: S3-compatible object storage for screenshots and file artifacts
- **Configuration**: Password-based authentication via environment variables

### Reverse Proxy Architecture
- **Nginx**: Single entry point on port 80
- **Routing**:
  - `/` routes to React frontend static files
  - `/api` proxies to backend Go API server
- **Benefits**: Simplified CORS handling, single domain for frontend/backend

### Authentication & Security
- **API Key**: Frontend uses configurable API key for backend authentication
- **Agent Security**: Single-instance protection via Windows mutex
- **Data Protection**: DLP monitoring for large file copy operations

## External Dependencies

### Databases
- **ClickHouse**: High-performance columnar database for time-series activity events
  - Configured via `CLICKHOUSE_PASSWORD` environment variable
  - Used for storing all monitoring events and analytics queries

### Object Storage
- **MinIO**: Self-hosted S3-compatible storage
  - Authentication: `MINIO_ROOT_USER` and `MINIO_ROOT_PASSWORD`
  - Stores screenshots and monitored file artifacts

### Third-Party Libraries

**Backend (Go):**
- `gin-gonic/gin`: HTTP web framework
- `uber-go/zap`: Structured logging library
- `gin-contrib/zap`: Gin framework integration for Zap logger
- Custom `zapctx` package: Context-aware logging utilities

**Frontend:**
- `react`: UI framework
- `typescript`: Type safety
- `vite`: Build tool and dev server

### Infrastructure
- **Docker Compose**: Container orchestration for all services
- **Nginx**: Reverse proxy and static file server
- **Git Submodules**: Frontend repository included as submodule

### Windows Agent Dependencies
- Windows-specific APIs for:
  - Process and window monitoring
  - USB device detection
  - Screenshot capture
  - Keyboard input capture
  - File system operations monitoring