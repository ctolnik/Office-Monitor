package database

import "time"

type ActivityEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	ComputerName string    `json:"computer_name"`
	Username     string    `json:"username"`
	WindowTitle  string    `json:"window_title"`
	ProcessName  string    `json:"process_name"`
	ProcessPath  string    `json:"process_path"`
	Duration     uint32    `json:"duration"`
	IdleTime     uint32    `json:"idle_time"`
	Category     string    `json:"category,omitempty"` // Computed field, not stored in DB
}

type KeyboardEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	ComputerName string    `json:"computer_name"`
	Username     string    `json:"username"`
	WindowTitle  string    `json:"window_title"`
	ProcessName  string    `json:"process_name"`
	TextContent  string    `json:"text_content"`
	ContextInfo  string    `json:"context_info,omitempty" db:"context_info"` // Added field
}

type FileCopyEvent struct {
	Timestamp       time.Time `json:"timestamp"`
	ComputerName    string    `json:"computer_name"`
	Username        string    `json:"username"`
	SourcePath      string    `json:"source_path"`
	DestinationPath string    `json:"destination_path"`
	FileSize        uint64    `json:"file_size"`
	FileCount       uint32    `json:"file_count"`
	OperationType   string    `json:"operation_type"`
	IsUSBTarget     uint8     `json:"is_usb_target"`
}

type USBEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	ComputerName string    `json:"computer_name"`
	Username     string    `json:"username"`
	DeviceID     string    `json:"device_id"`
	DeviceName   string    `json:"device_name"`
	DeviceType   string    `json:"device_type"`
	EventType    string    `json:"event_type"`
	VolumeSerial string    `json:"volume_serial"`
}

type ScreenshotMetadata struct {
	Timestamp    time.Time `json:"timestamp"`
	ComputerName string    `json:"computer_name"`
	Username     string    `json:"username"`
	ScreenshotID string    `json:"screenshot_id"`
	MinIOPath    string    `json:"minio_path"`
	FileSize     uint64    `json:"file_size"`
	WindowTitle  string    `json:"window_title"`
	ProcessName  string    `json:"process_name"`
}

type Alert struct {
	Timestamp      time.Time `json:"timestamp"`
	ComputerName   string    `json:"computer_name"`
	Username       string    `json:"username"`
	AlertType      string    `json:"alert_type"`
	Severity       string    `json:"severity"`
	Description    string    `json:"description"`
	Metadata       string    `json:"metadata"`
	IsAcknowledged bool      `json:"is_acknowledged"`
}

type Employee struct {
	ComputerName string    `json:"computer_name"`
	Username     string    `json:"username"`
	LastSeen     time.Time `json:"last_seen"`
	Status       string    `json:"status"`
}

type AgentConfig struct {
	ComputerName              string    `json:"computer_name"`
	APIKey                    string    `json:"api_key"`
	ScreenshotEnabled         bool      `json:"screenshot_enabled"`
	ScreenshotIntervalMinutes int       `json:"screenshot_interval_minutes"`
	KeyloggerEnabled          bool      `json:"keylogger_enabled"`
	USBMonitoringEnabled      bool      `json:"usb_monitoring_enabled"`
	FileCopyMonitoringEnabled bool      `json:"file_copy_monitoring_enabled"`
	LargeCopyThresholdMB      int       `json:"large_copy_threshold_mb"`
	LastSeen                  time.Time `json:"last_seen"`
	AgentVersion              string    `json:"agent_version"`
}

// Frontend API models
type Agent struct {
	ComputerName string       `json:"computer_name"`
	Username     string       `json:"username"`
	LastSeen     string       `json:"last_seen"`
	Status       string       `json:"status"` // online, offline, idle
	IPAddress    string       `json:"ip_address"`
	OSVersion    string       `json:"os_version"`
	AgentVersion string       `json:"agent_version"`
	Config       ConfigUpdate `json:"config"`
}

type ConfigUpdate struct {
	ScreenshotInterval int  `json:"screenshot_interval"` // seconds
	ActivityTracking   bool `json:"activity_tracking"`
	KeyloggerEnabled   bool `json:"keylogger_enabled"`
	USBMonitoring      bool `json:"usb_monitoring"`
	FileMonitoring     bool `json:"file_monitoring"`
	DLPEnabled         bool `json:"dlp_enabled"`
}

type EmployeeFull struct {
	ID           string  `json:"id"`
	Username     string  `json:"username"`
	FullName     string  `json:"full_name"`
	Department   string  `json:"department"`
	Position     string  `json:"position"`
	Email        string  `json:"email"`
	ConsentGiven bool    `json:"consent_given"`
	ConsentDate  *string `json:"consent_date"`
	CreatedAt    string  `json:"created_at"`
	IsActive     bool    `json:"is_active"`
}

type DashboardStats struct {
	TotalEmployees    uint64  `json:"total_employees"`
	ActiveNow         uint64  `json:"active_now"`
	Offline           uint64  `json:"offline"`
	TotalAlerts       uint64  `json:"total_alerts"`
	UnresolvedAlerts  uint64  `json:"unresolved_alerts"`
	AvgProductivity   float64 `json:"avg_productivity"`
	TodayScreenshots  uint64  `json:"today_screenshots"`
	TodayUSBEvents    uint64  `json:"today_usb_events"`
	TodayFileEvents   uint64  `json:"today_file_events"`
}

type ApplicationUsage struct {
	ProcessName  string  `json:"process_name"`
	WindowTitle  string  `json:"window_title"`
	Duration     uint64  `json:"duration"` // seconds
	Count        uint64  `json:"count"`
	Category     string  `json:"category"`
	Percentage   float64 `json:"percentage"`
}

type ActivitySummary struct {
	Username          string  `json:"username"`
	StartDate         string  `json:"start_date"`
	EndDate           string  `json:"end_date"`
	TotalActiveTime   uint64  `json:"total_active_time"`
	TotalIdleTime     uint64  `json:"total_idle_time"`
	ProductiveTime    uint64  `json:"productive_time"`
	UnproductiveTime  uint64  `json:"unproductive_time"`
	NeutralTime       uint64  `json:"neutral_time"`
	FirstActivity     string  `json:"first_activity"`
	LastActivity      string  `json:"last_activity"`
	ProductivityScore float64 `json:"productivity_score"`
}

type KeyboardPeriod struct {
	Start        string `json:"start"`
	End          string `json:"end"`
	Application  string `json:"application"`
	WindowTitle  string `json:"window_title"`
	FormattedText string `json:"formatted_text"`
	RawKeys      string `json:"raw_keys"` // JSON
}

type DailyReport struct {
	Username        string               `json:"username"`
	Date            string               `json:"date"`
	Summary         ActivitySummary      `json:"summary"`
	ActivityEvents  []ActivityEvent      `json:"activity_events"`
	Applications    []ApplicationUsage   `json:"applications"`
	Screenshots     []ScreenshotMetadata `json:"screenshots"`
	USBEvents       []USBEvent           `json:"usb_events"`
	FileEvents      []FileCopyEvent      `json:"file_events"`
	KeyboardPeriods []KeyboardPeriod     `json:"keyboard_periods"`
	DLPAlerts       []Alert              `json:"dlp_alerts"`
}

type AlertFull struct {
	ID          string  `json:"id"`
	Timestamp   string  `json:"timestamp"`
	ComputerName string `json:"computer_name"`
	Username    string  `json:"username"`
	AlertType   string  `json:"alert_type"`
	Severity    string  `json:"severity"`
	Description string  `json:"description"`
	Details     string  `json:"details"` // JSON
	IsResolved  bool    `json:"is_resolved"`
	ResolvedAt  *string `json:"resolved_at"`
	ResolvedBy  *string `json:"resolved_by"`
}

// ApplicationCategory represents application category for productivity calculation
type ApplicationCategory struct {
	ID             string    `json:"id" db:"id"`
	ProcessName    string    `json:"process_name" db:"process_name" binding:"required"`
	ProcessPattern string    `json:"process_pattern" db:"process_pattern"`
	Category       string    `json:"category" db:"category" binding:"required,oneof=productive unproductive neutral communication system"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
	CreatedBy      string    `json:"created_by" db:"created_by"`
	UpdatedBy      string    `json:"updated_by" db:"updated_by"`
	IsActive       bool      `json:"is_active" db:"is_active"`
}

// SystemSetting represents system configuration settings
type SystemSetting struct {
	Key         string    `json:"key" db:"key"`
	Value       string    `json:"value" db:"value"`
	Type        string    `json:"type" db:"type"`
	Description string    `json:"description" db:"description"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	UpdatedBy   string    `json:"updated_by" db:"updated_by"`
}

// ImportResult represents the result of bulk import operation
type ImportResult struct {
	Imported int                 `json:"imported"`
	Skipped  int                 `json:"skipped"`
	Errors   []ImportError       `json:"errors,omitempty"`
}

// ImportError represents an error during import
type ImportError struct {
	Line        int    `json:"line"`
	ProcessName string `json:"process_name"`
	Error       string `json:"error"`
}
