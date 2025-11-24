#!/bin/bash
# Диагностика: почему данные не попадают в БД

echo "═══════════════════════════════════════════════════════════════════"
echo "🔍 ДИАГНОСТИКА: Данные не попадают в БД"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

echo "1️⃣ Схема таблицы activity_events:"
echo "─────────────────────────────────────────────────────────"
docker exec clickhouse clickhouse-client --database=monitoring --query="
DESC activity_events
FORMAT PrettyCompact
"
echo ""

echo "2️⃣ Последние ошибки сервера за 10 минут:"
echo "─────────────────────────────────────────────────────────"
journalctl -u monitoring-server --since "10 minutes ago" | grep -i "error\|failed\|exception" | tail -30
echo ""

echo "3️⃣ Запросы к /api/events/batch (последние 10):"
echo "─────────────────────────────────────────────────────────"
journalctl -u monitoring-server --since "10 minutes ago" | grep "events/batch" | tail -10
echo ""

echo "4️⃣ Проверка: Работает ли ClickHouse?"
echo "─────────────────────────────────────────────────────────"
docker exec clickhouse clickhouse-client --query="SELECT 1 AS test" 2>&1
echo ""

echo "5️⃣ Ручная вставка тестовых данных:"
echo "─────────────────────────────────────────────────────────"
docker exec clickhouse clickhouse-client --database=monitoring --query="
INSERT INTO activity_events 
(timestamp, computer_name, username, window_title, process_name, duration)
VALUES 
(now(), 'TEST-PC', 'test-user', 'Test Window', 'test.exe', 10)
" 2>&1

echo ""
echo "Проверка что тест вставился:"
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT * FROM activity_events WHERE computer_name = 'TEST-PC' LIMIT 1
FORMAT PrettyCompact
"
echo ""

echo "6️⃣ Логи сервера с keyword 'activity' (последние 20):"
echo "─────────────────────────────────────────────────────────"
journalctl -u monitoring-server --since "30 minutes ago" | grep -i "activity" | tail -20
echo ""

echo "═══════════════════════════════════════════════════════════════════"
echo "✅ ИТОГИ:"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "Если Шаг 5 показал ошибку:"
echo "  → Проблема в схеме таблицы или правах доступа"
echo ""
echo "Если Шаг 5 успешен, но данные агента не попадают:"
echo "  → Проблема в коде receiveBatchEventsHandler"
echo "  → Сервер получает, но не вызывает InsertActivityEvent"
echo ""
echo "Если Шаг 2 показал ошибки ClickHouse:"
echo "  → Нужно исправить подключение к БД или SQL запросы"
echo ""
