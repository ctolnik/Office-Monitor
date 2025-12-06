-- ============================================================================
-- Grafana Views for Employee Activity Monitoring
-- ============================================================================

-- View: Application usage by user with friendly names and categories
CREATE OR REPLACE VIEW monitoring.v_app_usage_daily AS
SELECT
    pud.event_date,
    pud.username,
    pud.computer_name,
    pud.process_name,
    COALESCE(pc.friendly_name, pud.process_name) AS app_name,
    COALESCE(toString(pc.category), toString(ac.category), 'neutral') AS category,
    sum(pud.total_seconds) AS duration_seconds,
    round(sum(pud.total_seconds) / 60, 1) AS duration_minutes,
    round(sum(pud.total_seconds) / 3600, 2) AS duration_hours
FROM monitoring.program_usage_daily AS pud
LEFT JOIN (
    SELECT 
        arrayJoin(process_names) AS process_name,
        friendly_name,
        category
    FROM monitoring.process_catalog
    WHERE is_active = 1
) AS pc ON lower(pud.process_name) = lower(pc.process_name)
LEFT JOIN monitoring.application_categories AS ac ON lower(pud.process_name) = lower(ac.process_name)
GROUP BY pud.event_date, pud.username, pud.computer_name, pud.process_name, app_name, category
ORDER BY pud.event_date DESC, duration_seconds DESC;

-- View: Daily summary by state (active/idle/offline)
CREATE OR REPLACE VIEW monitoring.v_daily_state_summary AS
SELECT
    event_date,
    username,
    computer_name,
    state,
    sum(total_seconds) AS duration_seconds,
    round(sum(total_seconds) / 60, 1) AS duration_minutes,
    round(sum(total_seconds) / 3600, 2) AS duration_hours
FROM monitoring.daily_activity_summary
GROUP BY event_date, username, computer_name, state
ORDER BY event_date DESC, username;

-- View: Work hours (first and last activity per day)
CREATE OR REPLACE VIEW monitoring.v_work_hours AS
SELECT
    toDate(timestamp_start) AS event_date,
    username,
    computer_name,
    min(timestamp_start) AS first_activity,
    max(timestamp_end) AS last_activity,
    dateDiff('minute', min(timestamp_start), max(timestamp_end)) AS work_span_minutes,
    round(dateDiff('minute', min(timestamp_start), max(timestamp_end)) / 60, 2) AS work_span_hours
FROM monitoring.activity_segments
WHERE state = 'active'
GROUP BY event_date, username, computer_name
ORDER BY event_date DESC;

-- View: Productivity by category per day
CREATE OR REPLACE VIEW monitoring.v_productivity_daily AS
SELECT
    toDate(timestamp_start) AS event_date,
    username,
    computer_name,
    sumIf(duration_sec, category = 'productive') AS productive_seconds,
    sumIf(duration_sec, category = 'unproductive') AS unproductive_seconds,
    sumIf(duration_sec, category = 'neutral') AS neutral_seconds,
    sumIf(duration_sec, category = 'communication') AS communication_seconds,
    sumIf(duration_sec, category = 'entertainment') AS entertainment_seconds,
    sum(duration_sec) AS total_active_seconds,
    round(sumIf(duration_sec, category = 'productive') * 100.0 / nullIf(sum(duration_sec), 0), 1) AS productivity_percent
FROM monitoring.activity_segments
WHERE state = 'active'
GROUP BY event_date, username, computer_name
ORDER BY event_date DESC;

-- View: Hourly activity heatmap
CREATE OR REPLACE VIEW monitoring.v_hourly_activity AS
SELECT
    toDate(timestamp_start) AS event_date,
    toHour(timestamp_start) AS hour,
    username,
    computer_name,
    state,
    sum(duration_sec) AS duration_seconds
FROM monitoring.activity_segments
GROUP BY event_date, hour, username, computer_name, state
ORDER BY event_date DESC, hour;

-- View: Top applications by user
CREATE OR REPLACE VIEW monitoring.v_top_apps AS
SELECT
    toDate(timestamp_start) AS event_date,
    username,
    process_name,
    COALESCE(pc.friendly_name, process_name) AS app_name,
    COALESCE(toString(pc.category), 'neutral') AS category,
    sum(duration_sec) AS duration_seconds,
    round(sum(duration_sec) / 60, 1) AS duration_minutes,
    count() AS segment_count
FROM monitoring.activity_segments
LEFT JOIN (
    SELECT 
        arrayJoin(process_names) AS pn,
        friendly_name,
        category
    FROM monitoring.process_catalog
    WHERE is_active = 1
) AS pc ON lower(process_name) = lower(pc.pn)
WHERE state = 'active'
GROUP BY event_date, username, process_name, app_name, category
ORDER BY event_date DESC, duration_seconds DESC;

-- View: List of employees for dropdown
CREATE OR REPLACE VIEW monitoring.v_employees_list AS
SELECT DISTINCT
    username,
    COALESCE(e.full_name, username) AS display_name,
    e.department,
    e.position
FROM monitoring.activity_segments AS s
LEFT JOIN monitoring.employees AS e ON s.username = e.username
ORDER BY display_name;

-- View: Activity timeline (chronological periods)
CREATE OR REPLACE VIEW monitoring.v_activity_timeline AS
SELECT
    timestamp_start,
    timestamp_end,
    duration_sec,
    state,
    username,
    computer_name,
    process_name,
    COALESCE(pc.friendly_name, process_name) AS app_name,
    window_title,
    COALESCE(toString(pc.category), 'neutral') AS category
FROM monitoring.activity_segments
LEFT JOIN (
    SELECT 
        arrayJoin(process_names) AS pn,
        friendly_name,
        category
    FROM monitoring.process_catalog
    WHERE is_active = 1
) AS pc ON lower(process_name) = lower(pc.pn)
ORDER BY timestamp_start;
