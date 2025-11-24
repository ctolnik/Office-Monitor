#!/bin/bash
# Диагностика пустого отчёта на production

echo "═══════════════════════════════════════════════════════════════════"
echo "🔍 ДИАГНОСТИКА ПУСТОГО ОТЧЁТА"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

echo "1️⃣ Проверка: Есть ли вообще данные в activity_events?"
echo "─────────────────────────────────────────────────────────"
TOTAL=$(docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT count() FROM activity_events WHERE timestamp > now() - INTERVAL 24 HOUR
" 2>/dev/null)
echo "Всего событий за 24 часа: $TOTAL"
echo ""

if [ "$TOTAL" -eq 0 ]; then
    echo "❌ ПРОБЛЕМА: Нет данных в activity_events!"
    echo "   Агент не отправляет данные или сервер не сохраняет их в БД"
    echo ""
    echo "   Проверьте логи сервера:"
    echo "   journalctl -u monitoring-server -n 100 | grep -i error"
    exit 1
fi

echo "✅ Данные в БД есть ($TOTAL событий)"
echo ""

echo "2️⃣ Проверка: Какие username есть в базе?"
echo "─────────────────────────────────────────────────────────"
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT 
    username,
    computer_name,
    count() as events,
    min(timestamp) as first_event,
    max(timestamp) as last_event
FROM activity_events
WHERE timestamp > now() - INTERVAL 24 HOUR
GROUP BY username, computer_name
ORDER BY events DESC
FORMAT PrettyCompact
"
echo ""

echo "3️⃣ Проверка: Последние события"
echo "─────────────────────────────────────────────────────────"
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT 
    timestamp,
    username,
    computer_name,
    process_name,
    left(window_title, 50) as window_title
FROM activity_events
ORDER BY timestamp DESC
LIMIT 5
FORMAT PrettyCompact
"
echo ""

echo "4️⃣ Проверка: Данные в activity_segments"
echo "─────────────────────────────────────────────────────────"
SEGMENTS=$(docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT count() FROM activity_segments WHERE timestamp_start > now() - INTERVAL 24 HOUR
" 2>/dev/null || echo "TABLE_NOT_EXISTS")

if [ "$SEGMENTS" = "TABLE_NOT_EXISTS" ]; then
    echo "❌ ПРОБЛЕМА: Таблица activity_segments не существует!"
    echo "   Временная шкала активности не будет работать"
else
    echo "Сегментов активности за 24 часа: $SEGMENTS"
    
    if [ "$SEGMENTS" -gt 0 ]; then
        docker exec clickhouse clickhouse-client --database=monitoring --query="
        SELECT 
            username,
            computer_name,
            state,
            count() as segments,
            sum(duration_sec) as total_seconds
        FROM activity_segments
        WHERE timestamp_start > now() - INTERVAL 24 HOUR
        GROUP BY username, computer_name, state
        ORDER BY total_seconds DESC
        FORMAT PrettyCompact
        "
    fi
fi
echo ""

echo "5️⃣ Тест API: Запрос отчёта для username"
echo "─────────────────────────────────────────────────────────"
echo "Выберите username из списка выше и замените в команде:"
echo ""
echo "curl -s 'http://localhost:5000/api/reports/daily/USERNAME?date=2025-11-25' | jq '.applications[] | {process: .process_name, duration: .total_duration}'"
echo ""

echo "═══════════════════════════════════════════════════════════════════"
echo "✅ ДИАГНОСТИКА ЗАВЕРШЕНА"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "📋 Следующие шаги:"
echo "1. Скопируйте РЕАЛЬНЫЙ username из 'Шаг 2' выше"
echo "2. Проверьте что frontend использует ТАКОЙ ЖЕ username в URL"
echo "3. Если username не совпадает - исправьте в frontend"
echo "4. Запустите тестовый API запрос из 'Шаг 5'"
echo ""
