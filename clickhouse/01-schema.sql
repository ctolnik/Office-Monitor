-- ============================================================================
-- Office Monitor - Database Schema
-- ============================================================================
-- This migration creates all database tables, views, and indexes
-- Safe to run multiple times (idempotent)
-- ============================================================================

-- ============================================================================
-- 1. Core Activity Tables
-- ============================================================================

CREATE TABLE IF NOT EXISTS monitoring.activity_events (
    timestamp DateTime64(3),
    computer_name String,
    username String,
    window_title String,
    process_name String,
    process_path String DEFAULT '',
    duration UInt32,
    idle_time UInt32 DEFAULT 0,
    event_date Date DEFAULT toDate(timestamp)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (computer_name, username, timestamp)
TTL event_date + INTERVAL 180 DAY;

ALTER TABLE monitoring.activity_events 
ADD COLUMN IF NOT EXISTS process_path String DEFAULT '' AFTER process_name;

ALTER TABLE monitoring.activity_events 
ADD COLUMN IF NOT EXISTS idle_time UInt32 DEFAULT 0 AFTER duration;

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

CREATE TABLE IF NOT EXISTS monitoring.keyboard_events (
    timestamp DateTime64(3),
    computer_name String,
    username String,
    window_title String,
    process_name String,
    text_content String,
    context_info String DEFAULT '',
    event_date Date DEFAULT toDate(timestamp)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (computer_name, username, timestamp)
TTL event_date + INTERVAL 180 DAY;

ALTER TABLE monitoring.keyboard_events 
ADD COLUMN IF NOT EXISTS context_info String DEFAULT '';

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

-- ============================================================================
-- 2. Configuration Tables
-- ============================================================================

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

CREATE TABLE IF NOT EXISTS monitoring.employees (
    username String,
    full_name String,
    department String,
    position String,
    email String,
    is_active UInt8 DEFAULT 1,
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY username;

CREATE TABLE IF NOT EXISTS monitoring.process_catalog (
    id String,
    friendly_name String,
    process_names Array(String),
    window_title_patterns Array(String),
    category Enum8('productive' = 1, 'unproductive' = 2, 'neutral' = 3, 'communication' = 4, 'entertainment' = 5),
    is_active UInt8 DEFAULT 1,
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY id;

CREATE TABLE IF NOT EXISTS monitoring.application_categories (
    id UUID DEFAULT generateUUIDv4(),
    process_name String,
    process_pattern String,
    category Enum8(
        'productive' = 1, 
        'unproductive' = 2, 
        'neutral' = 3, 
        'communication' = 4, 
        'entertainment' = 5
    ),
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now(),
    created_by String DEFAULT '',
    updated_by String DEFAULT '',
    is_active UInt8 DEFAULT 1
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (process_name, id)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS monitoring.system_settings (
    key String,
    value String,
    type Enum8(
        'string' = 1, 
        'number' = 2, 
        'boolean' = 3, 
        'json' = 4
    ) DEFAULT 'string',
    description String DEFAULT '',
    updated_at DateTime DEFAULT now(),
    updated_by String DEFAULT 'system'
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY key
SETTINGS index_granularity = 8192;

-- ============================================================================
-- 3. Materialized Views
-- ============================================================================

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

-- ============================================================================
-- 4. Indexes
-- ============================================================================

ALTER TABLE monitoring.application_categories 
ADD INDEX IF NOT EXISTS idx_category category TYPE set(0) GRANULARITY 4;

ALTER TABLE monitoring.application_categories 
ADD INDEX IF NOT EXISTS idx_is_active is_active TYPE set(0) GRANULARITY 4;

ALTER TABLE monitoring.activity_events 
ADD INDEX IF NOT EXISTS idx_username_timestamp (username, timestamp) TYPE minmax GRANULARITY 4;

ALTER TABLE monitoring.screenshot_metadata 
ADD INDEX IF NOT EXISTS idx_username_timestamp (username, timestamp) TYPE minmax GRANULARITY 4;

ALTER TABLE monitoring.keyboard_events 
ADD INDEX IF NOT EXISTS idx_username_timestamp (username, timestamp) TYPE minmax GRANULARITY 4;

ALTER TABLE monitoring.usb_events 
ADD INDEX IF NOT EXISTS idx_username_timestamp (username, timestamp) TYPE minmax GRANULARITY 4;

ALTER TABLE monitoring.file_copy_events 
ADD INDEX IF NOT EXISTS idx_username_timestamp (username, timestamp) TYPE minmax GRANULARITY 4;
