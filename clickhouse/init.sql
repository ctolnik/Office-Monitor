-- ClickHouse initialization script for monitoring system

-- Activity events table (time series)
CREATE TABLE IF NOT EXISTS monitoring.activity_events (
    timestamp DateTime64(3),
    computer_name String,
    username String,
    window_title String,
    process_name String,
    process_path String,
    duration UInt32,
    idle_time UInt32,
    event_date Date DEFAULT toDate(timestamp)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (computer_name, username, timestamp)
TTL event_date + INTERVAL 180 DAY;

-- Keyboard events table (keylogger data)
CREATE TABLE IF NOT EXISTS monitoring.keyboard_events (
    timestamp DateTime64(3),
    computer_name String,
    username String,
    window_title String,
    process_name String,
    text_content String,
    event_date Date DEFAULT toDate(timestamp)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (computer_name, username, timestamp)
TTL event_date + INTERVAL 180 DAY;

-- File copy events table
CREATE TABLE IF NOT EXISTS monitoring.file_copy_events (
    timestamp DateTime64(3),
    computer_name String,
    username String,
    source_path String,
    destination_path String,
    file_size UInt64,
    file_count UInt32,
    operation_type Enum('copy', 'move', 'delete'),
    is_usb_target UInt8,
    event_date Date DEFAULT toDate(timestamp)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (computer_name, username, timestamp)
TTL event_date + INTERVAL 180 DAY;

-- USB device events table
CREATE TABLE IF NOT EXISTS monitoring.usb_events (
    timestamp DateTime64(3),
    computer_name String,
    username String,
    device_id String,
    device_name String,
    device_type String,
    event_type Enum('connected', 'disconnected'),
    volume_serial String,
    event_date Date DEFAULT toDate(timestamp)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (computer_name, username, timestamp)
TTL event_date + INTERVAL 180 DAY;

-- Screenshot metadata table (actual images stored in MinIO)
CREATE TABLE IF NOT EXISTS monitoring.screenshot_metadata (
    timestamp DateTime64(3),
    computer_name String,
    username String,
    screenshot_id String,
    minio_path String,
    file_size UInt64,
    window_title String,
    process_name String,
    event_date Date DEFAULT toDate(timestamp)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (computer_name, username, timestamp)
TTL event_date + INTERVAL 180 DAY;

-- Alerts table (suspicious activity)
CREATE TABLE IF NOT EXISTS monitoring.alerts (
    timestamp DateTime64(3),
    computer_name String,
    username String,
    alert_type Enum('mass_file_copy', 'usb_connection', 'suspicious_process', 'large_upload'),
    severity Enum('low', 'medium', 'high', 'critical'),
    description String,
    metadata String,
    is_acknowledged UInt8 DEFAULT 0,
    event_date Date DEFAULT toDate(timestamp)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (timestamp, severity)
TTL event_date + INTERVAL 180 DAY;

-- Materialized view for activity statistics
CREATE MATERIALIZED VIEW IF NOT EXISTS monitoring.activity_stats_hourly
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (computer_name, username, event_date, hour, process_name)
AS SELECT
    computer_name,
    username,
    toDate(timestamp) as event_date,
    toHour(timestamp) as hour,
    process_name,
    count() as event_count,
    sum(duration) as total_duration
FROM monitoring.activity_events
GROUP BY computer_name, username, event_date, hour, process_name;

-- Agent configurations table (stored in ClickHouse instead of PostgreSQL)
CREATE TABLE IF NOT EXISTS monitoring.agent_configs (
    computer_name String,
    api_key String,
    screenshot_enabled UInt8 DEFAULT 0,
    screenshot_interval_minutes UInt32 DEFAULT 15,
    keylogger_enabled UInt8 DEFAULT 0,
    usb_monitoring_enabled UInt8 DEFAULT 1,
    file_copy_monitoring_enabled UInt8 DEFAULT 1,
    large_copy_threshold_mb UInt32 DEFAULT 100,
    last_seen DateTime,
    agent_version String,
    updated_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY computer_name;

-- Employees/consent table
CREATE TABLE IF NOT EXISTS monitoring.employees (
    computer_name String,
    username String,
    full_name String,
    department String,
    email String,
    is_active UInt8 DEFAULT 1,
    monitoring_enabled UInt8 DEFAULT 1,
    consent_date DateTime,
    created_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(created_at)
ORDER BY (computer_name, username);

-- Index for fast username search
ALTER TABLE monitoring.activity_events ADD INDEX idx_username username TYPE bloom_filter GRANULARITY 1;
ALTER TABLE monitoring.keyboard_events ADD INDEX idx_username username TYPE bloom_filter GRANULARITY 1;
ALTER TABLE monitoring.file_copy_events ADD INDEX idx_username username TYPE bloom_filter GRANULARITY 1;
