-- Показать сегменты с process_name = 'unknown' или пустым
SELECT 
    process_name,
    window_title,
    state,
    duration_sec,
    timestamp_start
FROM monitoring.activity_segments
WHERE username = 'a-kiv'
  AND toDate(timestamp_start) = '2025-11-25'
  AND (process_name = 'unknown' OR process_name = '' OR process_name IS NULL)
ORDER BY duration_sec DESC
LIMIT 20;
