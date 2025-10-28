package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Agent              AgentConfig              `yaml:"agent"`
	ActivityMonitoring ActivityMonitoringConfig `yaml:"activity_monitoring"`
	Screenshots        ScreenshotsConfig        `yaml:"screenshots"`
	Keylogger          KeyloggerConfig          `yaml:"keylogger"`
	USBMonitoring      USBMonitoringConfig      `yaml:"usb_monitoring"`
	FileMonitoring     FileMonitoringConfig     `yaml:"file_monitoring"`
	Performance        PerformanceConfig        `yaml:"performance"`
	Logging            LoggingConfig            `yaml:"logging"`
}

type AgentConfig struct {
	ComputerName string       `yaml:"computer_name"`
	APIKey       string       `yaml:"api_key"`
	Server       ServerConfig `yaml:"server"`
}

type ServerConfig struct {
	URL            string `yaml:"url"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	RetryAttempts  int    `yaml:"retry_attempts"`
	RetryDelay     int    `yaml:"retry_delay_seconds"`
}

type ActivityMonitoringConfig struct {
	Enabled               bool `yaml:"enabled"`
	IntervalSeconds       int  `yaml:"interval_seconds"`
	TrackWindowTitles     bool `yaml:"track_window_titles"`
	TrackProcessNames     bool `yaml:"track_process_names"`
	IdleThresholdSeconds  int  `yaml:"idle_threshold_seconds"`
}

type ScreenshotsConfig struct {
	Enabled             bool `yaml:"enabled"`
	IntervalMinutes     int  `yaml:"interval_minutes"`
	CaptureOnlyActive   bool `yaml:"capture_on_activity_only"`
	Quality             int  `yaml:"quality"`
	MaxSizeKB           int  `yaml:"max_size_kb"`
	UploadImmediately   bool `yaml:"upload_immediately"`
}

type KeyloggerConfig struct {
	Enabled            bool     `yaml:"enabled"`
	MonitoredProcesses []string `yaml:"monitored_processes"`
	BufferSizeChars    int      `yaml:"buffer_size_chars"`
	SendIntervalMin    int      `yaml:"send_interval_minutes"`
}

type USBMonitoringConfig struct {
	Enabled              bool     `yaml:"enabled"`
	DetectNewDevices     bool     `yaml:"detect_new_devices"`
	ShadowCopyEnabled    bool     `yaml:"shadow_copy_enabled"`
	ShadowCopyDest       string   `yaml:"shadow_copy_destination"`
	CopyFileExtensions   []string `yaml:"copy_file_extensions"`
	ExcludePatterns      []string `yaml:"exclude_patterns"`
}

type FileMonitoringConfig struct {
	Enabled              bool     `yaml:"enabled"`
	AlertOnLargeCopy     bool     `yaml:"alert_on_large_copy"`
	LargeCopyThresholdMB int      `yaml:"large_copy_threshold_mb"`
	LargeCopyFileCount   int      `yaml:"large_copy_file_count"`
	MonitoredLocations   []string `yaml:"monitored_locations"`
	DetectExternalCopy   bool     `yaml:"detect_external_copy"`
}

type PerformanceConfig struct {
	MaxMemoryMB         int `yaml:"max_memory_mb"`
	MaxCPUPercent       int `yaml:"max_cpu_percent"`
	ScreenshotMaxQueue  int `yaml:"screenshot_max_queue"`
	EventBufferSize     int `yaml:"event_buffer_size"`
}

type LoggingConfig struct {
	Level       string `yaml:"level"`
	File        string `yaml:"file"`
	MaxSizeMB   int    `yaml:"max_size_mb"`
	MaxBackups  int    `yaml:"max_backups"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if cfg.Agent.ComputerName == "" {
		cfg.Agent.ComputerName = os.Getenv("COMPUTERNAME")
	}
	if cfg.ActivityMonitoring.IntervalSeconds == 0 {
		cfg.ActivityMonitoring.IntervalSeconds = 30
	}

	return &cfg, nil
}
