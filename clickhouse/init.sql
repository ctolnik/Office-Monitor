-- ClickHouse initialization script for monitoring system

-- Activity events table (time series)
CREATE TABLE IF NOT EXISTS monitoring.activity_events (
    timestamp DateTime64(3),
    computer_name String,
    username String,
    window_title String,
    process_name String,
    duration UInt32,
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

-- Activity segments table (tracks active/idle/offline periods)
CREATE TABLE IF NOT EXISTS monitoring.activity_segments (
    timestamp_start DateTime64(3),
    timestamp_end DateTime64(3),
    duration_sec UInt32,
    state Enum8('active' = 1, 'idle' = 2, 'offline' = 3),
    computer_name String,
    username String,
    process_name String,
    window_title String,
    session_id String,
    event_date Date DEFAULT toDate(timestamp_start)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (computer_name, username, timestamp_start)
TTL event_date + INTERVAL 180 DAY;

-- Process catalog table (friendly names mapping)
CREATE TABLE IF NOT EXISTS monitoring.process_catalog (
    id UUID,
    friendly_name String,
    process_names Array(String),
    window_title_patterns Array(String),
    category Enum8('work' = 1, 'communication' = 2, 'development' = 3, 'browsing' = 4, 'other' = 5),
    is_active UInt8 DEFAULT 1,
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY id;

-- Materialized view for daily activity summary
CREATE MATERIALIZED VIEW IF NOT EXISTS monitoring.daily_activity_summary
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (computer_name, username, event_date, state)
AS SELECT
    computer_name,
    username,
    toDate(timestamp_start) as event_date,
    state,
    count() as segment_count,
    sum(duration_sec) as total_seconds
FROM monitoring.activity_segments
GROUP BY computer_name, username, event_date, state;

-- Materialized view for program usage statistics with friendly names
CREATE MATERIALIZED VIEW IF NOT EXISTS monitoring.program_usage_daily
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (computer_name, username, event_date, process_name)
AS SELECT
    computer_name,
    username,
    toDate(timestamp_start) as event_date,
    process_name,
    state,
    count() as segment_count,
    sum(duration_sec) as total_seconds
FROM monitoring.activity_segments
WHERE state = 'active'
GROUP BY computer_name, username, event_date, process_name, state;

-- Index for fast username search
ALTER TABLE monitoring.activity_events ADD INDEX idx_username username TYPE bloom_filter GRANULARITY 1;
ALTER TABLE monitoring.keyboard_events ADD INDEX idx_username username TYPE bloom_filter GRANULARITY 1;
ALTER TABLE monitoring.file_copy_events ADD INDEX idx_username username TYPE bloom_filter GRANULARITY 1;
ALTER TABLE monitoring.activity_segments ADD INDEX idx_username username TYPE bloom_filter GRANULARITY 1;
