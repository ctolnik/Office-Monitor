#!/bin/bash
# Скрипт для проверки данных в ClickHouse на production

echo "🔍 Проверка данных в ClickHouse"
echo "================================"
echo ""

echo "1️⃣ Проверка activity_events (последний час):"
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT count() as total_events, 
       min(timestamp) as first_event,
       max(timestamp) as last_event
FROM activity_events 
WHERE timestamp > now() - INTERVAL 1 HOUR
"
echo ""

echo "2️⃣ Проверка activity_segments (последний час):"
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT count() as total_segments,
       min(timestamp_start) as first_segment,
       max(timestamp_start) as last_segment
FROM activity_segments 
WHERE timestamp_start > now() - INTERVAL 1 HOUR
"
echo ""

echo "3️⃣ Проверка keyboard_events (последний час):"
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT count() FROM keyboard_events WHERE timestamp > now() - INTERVAL 1 HOUR
"
echo ""

echo "4️⃣ Проверка file_copy_events (последний час):"
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT count() FROM file_copy_events WHERE timestamp > now() - INTERVAL 1 HOUR
"
echo ""

echo "5️⃣ Проверка usb_events (последний час):"
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT count() FROM usb_events WHERE timestamp > now() - INTERVAL 1 HOUR
"
echo ""

echo "6️⃣ Проверка screenshot_metadata (последний час):"
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT count() FROM screenshot_metadata WHERE timestamp > now() - INTERVAL 1 HOUR
"
echo ""

echo "7️⃣ Список всех пользователей с данными:"
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT DISTINCT username, computer_name, count() as events
FROM activity_events
WHERE timestamp > now() - INTERVAL 24 HOUR
GROUP BY username, computer_name
ORDER BY events DESC
LIMIT 10
"
echo ""

echo "8️⃣ Последние 5 событий:"
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT 
    timestamp,
    computer_name,
    username,
    process_name,
    window_title
FROM activity_events
ORDER BY timestamp DESC
LIMIT 5
FORMAT Vertical
"

