# Employee Activity Monitoring System

## Overview

This is a comprehensive employee activity monitoring system designed to track and analyze employee computer activity in an office environment. The system consists of three main components:

1. **Server** - A Go-based web server providing REST API endpoints and an admin dashboard
2. **Agent** - A Windows desktop application that collects activity data from employee computers
3. **Web Interface** - A real-time administrative dashboard for monitoring and analyzing employee activity

The system tracks active windows, application usage, and employee status (active/idle/offline), providing administrators with real-time insights and historical activity reports.

## User Preferences

Preferred communication style: Simple, everyday language.

## System Architecture

### Backend Architecture

**Technology Stack**: Go (Golang) web server with built-in HTTP routing

**Design Pattern**: Monolithic architecture with REST API endpoints

**Core Components**:
- `main.go` - Primary application entry point and HTTP server setup
- `database.go` - Data persistence layer and database operations
- Built-in HTTP server running on port 5000

**API Structure**: RESTful endpoints for:
- Employee management (`/api/employees`)
- Activity tracking (`/api/activity/recent`)
- Real-time status updates

**Data Model**:
- Employee records with status tracking (active/idle/offline)
- Activity logs capturing window/application usage with timestamps
- Duration tracking for application usage sessions

**Rationale**: Go was chosen for its performance, simple deployment (single binary), and excellent concurrency support for handling multiple simultaneous agent connections.

### Frontend Architecture

**Technology Stack**: Vanilla JavaScript with server-side HTML templates

**Structure**:
- Template-based rendering (`/web/templates/index.html`)
- Client-side JavaScript for dynamic updates (`/web/static/app.js`)
- CSS styling (`/web/static/style.css`)

**Key Features**:
- Real-time dashboard with polling mechanism
- Employee list with filtering and search
- Activity timeline and statistics visualization
- Tabbed interface (Overview, Activity, Statistics)

**Rationale**: Vanilla JavaScript keeps the frontend lightweight and eliminates build tool dependencies, making deployment simpler on Replit.

### Windows Agent Architecture

**Technology Stack**: Go compiled to Windows executable

**Functionality**:
- Monitors active windows and application focus
- Collects activity data locally
- Periodically sends data to server via HTTP API
- Runs as background process

**Reliability Features**:
- **Circuit Breaker**: Sony/gobreaker library prevents overwhelming unavailable server
  - Opens after 60% failure rate (min 3 requests)
  - Auto-recovery after 30s timeout
  - Protects server from spike after downtime
- **Retry Logic**: 3 attempts with 5s delay for transient errors
- **Event Buffer**: 1000-event memory buffer + disk persistence for offline operation
- **Graceful Shutdown**: No data loss on restart

**Build Process**: Cross-compilation support (GOOS=windows GOARCH=amd64) allows building Windows executables from Linux/Mac environments

**Rationale**: Go enables building a single executable without runtime dependencies, simplifying deployment to employee workstations.

### Data Storage

**Current Implementation**: ClickHouse database for time-series data + MinIO for file storage

**Database Structure**:
- **ClickHouse tables**:
  - `activity_segments`: Time-based activity tracking (state, timestamps, duration, process, window title)
  - `screenshots`: Screenshot metadata with MinIO object references
  - `usb_events`: USB device connection/disconnection events
  - `file_events`: File copy operations tracking
  - `keyboard_events`: Keyboard activity periods (optional, requires legal compliance)
  - `alerts`: DLP alerts and security events
- **Indexes**: Optimized for time-range queries on computer_name, username, and timestamp

**Implementation Details**:
- Activity states: active (< 5min idle), idle (< 30min), offline (> 30min)
- Window title parsing for browser URLs (Chrome, Firefox, Edge)
- Process catalog for friendly names (chrome.exe → "Google Chrome")
- Daily activity summaries with productivity scoring
- Empty arrays returned instead of null for consistent API responses

**MinIO Storage**:
- Screenshots stored with retention policy (configurable, default 30 days)
- USB shadow copy files stored with configurable retention
- Automatic cleanup based on retention policies

### Authentication and Authorization

**Current State**: No authentication layer implemented (designed for internal network use)

**Security Model**: Assumes deployment in trusted office network environment where all agents have legitimate access

**Future Consideration**: Authentication can be added at the API level for multi-tenant scenarios or external access.

## External Dependencies

### Go Standard Library
- `net/http` - HTTP server and client functionality
- `encoding/json` - JSON serialization for API responses
- `html/template` - Server-side HTML rendering

### Windows API (Agent Only)
- `golang.org/x/sys/windows` - Windows system calls
- `user32.dll` - GetForegroundWindow, GetWindowTextW, GetWindowThreadProcessId
- `kernel32.dll` - OpenProcess, QueryFullProcessImageNameW for process name resolution
- Real-time window and process tracking without external dependencies

### Browser APIs (Frontend)
- Fetch API for async HTTP requests
- DOM manipulation APIs
- LocalStorage for client-side preferences (if implemented)

### Deployment Platform
- **Replit**: Primary hosting platform with automatic server startup on port 5000
- Environment supports Go runtime and static file serving

### Go Libraries (Server)
- `github.com/gin-gonic/gin` - Web framework for HTTP routing and middleware
- ClickHouse Go driver - Time-series database connectivity
- MinIO Go SDK - Object storage for screenshots and files

### No External Services
- No third-party APIs or cloud services
- Self-contained system designed for on-premise deployment
- Uses ClickHouse and MinIO containers (included in docker-compose.yml)

## Recent Changes

**2025-11-24 (late evening)**: Added Circuit Breaker to agent
- **Library**: Integrated sony/gobreaker v1.0.0 for production-grade circuit breaker
- **Configuration**:
  - Opens after 60% failure rate (minimum 3 requests)
  - Half-open after 30s timeout
  - Max 3 requests in half-open state
  - 60s interval for clearing failure counts
- **Integration**: Wrapped all HTTP client methods (PostJSON, PostMultipart)
- **Benefits**: 
  - Fast-fail when server unavailable (no timeout waits)
  - Auto-recovery when server returns
  - Protects server from spike after downtime
  - Events buffered to disk during outage
- **Logging**: Circuit breaker state changes logged for monitoring

**2025-11-24 (evening)**: Fixed zapctx panic - comprehensive solution
- **Critical bug fix**: Server was panicking on `/api/dashboard/stats` with "context without logger"
- **Root cause**: `zapctx.Logger()` called on contexts without initialized zap logger
- **Multi-layer solution**:
  1. **HTTP layer**: `logger_middleware.go` injects zap logger into all HTTP request contexts
  2. **Handler layer**: Replaced all `context.Background()` with `c.Request.Context()` in 30+ handlers
  3. **Safety layer**: `safe_context.go` provides `withLogger()` wrapper with fallback to global logger
  4. **Critical paths**: Cache and other non-HTTP paths use `withLogger()` for panic prevention
- **Implementation details**:
  - `logger_middleware.go`: Middleware that enriches every HTTP request context
  - `safe_context.go`: Panic-safe wrapper using defer/recover to detect missing logger
  - Global `logger *zap.Logger` initialized with `zap.NewProduction()`
  - All handlers: `context.Background()` → `c.Request.Context()`
- **Coverage**: 
  - HTTP requests: 100% protected (middleware + handler fixes)
  - Cache operations: Protected with withLogger() wrapper
  - Background jobs: Not yet implemented (future consideration)
- **Known limitations**: 
  - Tests using context.Background() may still panic (not production code)
  - Future non-HTTP contexts need withLogger() wrapper
  - Gin recovery middleware prevents server crashes from any panics
- **Production readiness**: Full HTTP API protection + panic recovery = production-safe
- **Future improvements**: Wrap all DB methods or fork zapctx for safe fallback

**2025-11-24 (afternoon)**: Merge conflict recovery and complete API restoration
- **CRITICAL FIX**: Recovered 31 API endpoints lost in merge conflict 3db80ad
  - Batch events API: `POST /api/events/batch` with `InsertActivityEventsBatch()` for bulk agent uploads
  - Dashboard API (4): stats, active-now, daily reports, unresolved alerts
  - Agents management (4): CRUD operations for agent configs
  - Employees CRUD (4): full employee management API
  - Reports per user (5): applications, keyboard, USB, files, screenshots
  - Alerts (2): list all, resolve by ID
  - Categories (7): CRUD, bulk update, import/export for app categorization
  - Settings (3): general settings CRUD, logo upload
  - Screenshot (1): file download endpoint
- **Total API surface**: 47 REST endpoints (from 16 after merge conflict)
- **Code quality**: 
  - Formatted all files with gofmt/goimports
  - Fixed unchecked errors (errcheck)
  - Removed unused functions and ineffectual assignments
  - All handlers properly registered in routes
- **Build verification**: Server compiles successfully (43MB binary)
- **Previous fixes maintained**:
  - Cross-platform build tags for Windows/non-Windows
  - Model completeness (ActivitySummary, DailyReport, etc.)
  - Database and MinIO storage initialization
  - Agent dependencies (httpclient, event buffer)

**2025-11-23**: Added activity tracking and reporting features
- **Activity segments tracking**: Implemented active/idle/offline state detection using GetLastInputInfo API
  - Agent detects user activity state (active < 5min idle, idle < 30min, offline > 30min)
  - Creates time segments with start/end timestamps and duration
  - Automatic window title parsing for browser URLs extraction (Chrome, Firefox, Edge)
  - Configurable idle threshold and poll interval in agent config
- **Process catalog (Friendly names)**: Admin-managed mapping of process names to logical program names
  - Database table: monitoring.process_catalog with process_names arrays and categories
  - CRUD API endpoints: GET/POST/PUT/DELETE /api/process-catalog
  - Allows grouping processes (chrome.exe → "Google Chrome", 1C.exe + 1Cvs.exe → "1С")
- **Daily activity summaries**: Aggregated reports with ClickHouse materialized views
  - API endpoint: GET /api/activity/summary?computer_name=X&date=2025-11-23
  - Returns: active/idle/offline seconds, top 10 programs with friendly names, window titles
  - Materialized views: daily_activity_summary, program_usage_daily
- **Agent integration**: activity_tracker_windows.go integrated into main.go with idle detection
- **Database schema**: activity_segments table with state enum (active/idle/offline) and session tracking

**2025-10-21**: Major DLP architecture upgrade and monitoring features
- **Simplified architecture**: Removed PostgreSQL and Redis, now using only ClickHouse (time series) + MinIO (file storage)
- **USB monitoring**: Fully implemented Windows USB device tracking with shadow copying capability
  - Real-time detection of USB device connections/disconnections
  - Automatic shadow copying of files from USB drives to network share
  - Configurable file filtering by extensions
  - Event logging to ClickHouse database
- **File monitoring**: Mass file copy detection with configurable thresholds
  - Windows ReadDirectoryChanges API for real-time monitoring
  - Alert on large copy operations (configurable MB/file count thresholds)
  - Monitored locations configurable via YAML
- **Screenshot capture**: Periodic screen capture with intelligent buffering
  - Windows GDI32/User32 API for screen capture
  - Configurable interval, quality, and size limits
  - Two modes: immediate upload or buffered with batching
  - Graceful shutdown with queue draining (no data loss)
  - Storage in MinIO with metadata in ClickHouse
- **Configuration system**: YAML-based config for both server and agent with environment variable expansion
- **Legal compliance**: Created GDPR/Russian law templates and employee consent forms (LEGAL_COMPLIANCE.md)
- **Frontend specification**: Complete technical specification for React/TypeScript dashboard (FRONTEND_SPECIFICATION.md)
- **Data retention**: 180 days for ClickHouse, 30 days for MinIO screenshots/files
- **Build system**: Successfully compiled Windows agent (9.6MB executable)
- **Testing**: Unit tests for all monitoring modules (Windows-specific with build tags)

**2025-10-20**: Initial implementation complete
- Created full-stack monitoring system with Go backend and vanilla JS frontend
- Implemented SQLite database with proper schema and indexes
- Built Windows agent with real process name resolution using Windows API
- Added REST API endpoints for activity tracking and statistics
- Created responsive admin dashboard with real-time updates