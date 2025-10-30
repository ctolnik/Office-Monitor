package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Storage  StorageConfig  `yaml:"storage"`
	// Monitoring MonitoringConfig `yaml:"monitoring"`
}

type ServerConfig struct {
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	Mode   string `yaml:"mode"`
	APIKey string `yaml:"api_key"`
}

// type ServerConfig struct {
// 	Host   string `yaml:"host" env: "OM_HOST" envDefault: "0.0.0.0"`
// 	Port   int    `yaml:"port" env: "OM_PORT" envDefault:"5000"`
// 	Mode   string `yaml:"mode" env: "OM_MODE" envDefault: "prod"`
// 	APIKey string `yaml:"api_key" env: "OM_API_KEY"envDefault: ""`
// }

type DatabaseConfig struct {
	// ClickHouse ClickHouseConfig `yaml:"clickhouse"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Database     string `yaml:"database"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	Timezone     string `yaml:"timezone"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

type ClickHouseConfig struct {
}

type StorageConfig struct {
	Endpoint  string        `yaml:"endpoint"`
	AccessKey string        `yaml:"access_key"`
	SecretKey string        `yaml:"secret_key"`
	UseSSL    bool          `yaml:"use_ssl"`
	Buckets   BucketsConfig `yaml:"buckets"`
}

type BucketsConfig struct {
	Screenshots string `yaml:"screenshots"`
	USBCopies   string `yaml:"usb_copies"`
}

type MonitoringConfig struct {
	Activity    ActivityConfig   `yaml:"activity"`
	Screenshots ScreenshotConfig `yaml:"screenshots"`
	USB         USBConfig        `yaml:"usb"`
	FileCopy    FileCopyConfig   `yaml:"file_copy"`
}

type ActivityConfig struct {
	StatusActiveThresholdMinutes int `yaml:"status_active_threshold_minutes"`
	StatusIdleThresholdMinutes   int `yaml:"status_idle_threshold_minutes"`
}

type ScreenshotConfig struct {
	Enabled            bool `yaml:"enabled"`
	RetentionDays      int  `yaml:"retention_days"`
	MaxSizeMB          int  `yaml:"max_size_mb"`
	CompressionQuality int  `yaml:"compression_quality"`
}

type USBConfig struct {
	Enabled           bool   `yaml:"enabled"`
	ShadowCopyEnabled bool   `yaml:"shadow_copy_enabled"`
	ShadowCopyShare   string `yaml:"shadow_copy_share"`
}

type FileCopyConfig struct {
	Enabled                 bool `yaml:"enabled"`
	LargeCopyThresholdMB    int  `yaml:"large_copy_threshold_mb"`
	AlertFileCountThreshold int  `yaml:"alert_file_count_threshold"`
	AlertTimeWindowSeconds  int  `yaml:"alert_time_window_seconds"`
}

func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 5000
	}

	return &cfg, nil
}
