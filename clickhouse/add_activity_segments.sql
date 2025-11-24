-- Migration: Add activity_segments table and related views
-- Safe to run multiple times (uses IF NOT EXISTS)

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

-- Materialized view for program usage statistics
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
