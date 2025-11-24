#!/bin/bash
# Тест API с правильным username "a-kiv"

echo "═══════════════════════════════════════════════════════════════════"
echo "✅ ТЕСТ API: Правильный username 'a-kiv'"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

echo "1️⃣ Данные в activity_segments для 'a-kiv':"
echo "─────────────────────────────────────────────────────────"
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT 
    state,
    count() as segments,
    sum(duration_sec) as total_seconds,
    round(sum(duration_sec) / 3600, 2) as hours
FROM activity_segments
WHERE username = 'a-kiv'
  AND timestamp_start > now() - INTERVAL 24 HOUR
GROUP BY state
ORDER BY state
FORMAT PrettyCompact
"
echo ""

echo "2️⃣ Прямой запрос к API /api/reports/daily/a-kiv:"
echo "─────────────────────────────────────────────────────────"
TODAY=$(date +%Y-%m-%d)
echo "Дата: $TODAY"
echo ""
curl -s "http://localhost:5000/api/reports/daily/a-kiv?date=$TODAY" | jq '{
  username: .username,
  date: .date,
  activity_events_count: (.activity_events | length),
  applications_count: (.applications | length),
  screenshots_count: (.screenshots | length),
  keyboard_events_count: (.keyboard_periods | length),
  file_events_count: (.file_events | length),
  usb_events_count: (.usb_events | length)
}'
echo ""

echo "3️⃣ Топ приложений из API:"
echo "─────────────────────────────────────────────────────────"
curl -s "http://localhost:5000/api/reports/daily/a-kiv?date=$TODAY" | jq '.applications[] | select(.total_duration > 0) | {process: .process_name, duration_sec: .total_duration}' | head -20
echo ""

echo "4️⃣ Activity events count:"
echo "─────────────────────────────────────────────────────────"
curl -s "http://localhost:5000/api/reports/daily/a-kiv?date=$TODAY" | jq '.activity_events | length'
echo ""

echo "═══════════════════════════════════════════════════════════════════"
echo "✅ ВЫВОДЫ:"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "Если Шаг 1 показал данные (state=active/idle):"
echo "  ✅ Activity segments есть в БД"
echo ""
echo "Если Шаг 2 вернул JSON с данными:"
echo "  ✅ Backend API работает правильно"
echo "  ✅ Проблема ТОЛЬКО в frontend (неправильный username)"
echo ""
echo "Если в JSON пустые массивы []:"
echo "  ❌ GetDailyReport не использует activity_segments"
echo "  → Нужно исправить код сервера"
echo ""
