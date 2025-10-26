package database

import "time"

type ActivityEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	ComputerName string    `json:"computer_name"`
	Username     string    `json:"username"`
	WindowTitle  string    `json:"window_title"`
	ProcessName  string    `json:"process_name"`
	Duration     uint32    `json:"duration"`
}

type KeyboardEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	ComputerName string    `json:"computer_name"`
	Username     string    `json:"username"`
	WindowTitle  string    `json:"window_title"`
	ProcessName  string    `json:"process_name"`
	TextContent  string    `json:"text_content"`
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
	Timestamp       time.Time `json:"timestamp"`
	ComputerName    string    `json:"computer_name"`
	Username        string    `json:"username"`
	AlertType       string    `json:"alert_type"`
	Severity        string    `json:"severity"`
	Description     string    `json:"description"`
	Metadata        string    `json:"metadata"`
	IsAcknowledged  bool      `json:"is_acknowledged"`
}

type Employee struct {
	ComputerName string    `json:"computer_name"`
	Username     string    `json:"username"`
	LastSeen     time.Time `json:"last_seen"`
	Status       string    `json:"status"`
}

type AgentConfig struct {
	ComputerName                 string    `json:"computer_name"`
	APIKey                       string    `json:"api_key"`
	ScreenshotEnabled            bool      `json:"screenshot_enabled"`
	ScreenshotIntervalMinutes    int       `json:"screenshot_interval_minutes"`
	KeyloggerEnabled             bool      `json:"keylogger_enabled"`
	USBMonitoringEnabled         bool      `json:"usb_monitoring_enabled"`
	FileCopyMonitoringEnabled    bool      `json:"file_copy_monitoring_enabled"`
	LargeCopyThresholdMB         int       `json:"large_copy_threshold_mb"`
	LastSeen                     time.Time `json:"last_seen"`
	AgentVersion                 string    `json:"agent_version"`
}
