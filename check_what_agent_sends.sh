#!/bin/bash
# Проверка: что именно агент отправляет на /api/events/batch

echo "═══════════════════════════════════════════════════════════════════"
echo "🔍 ПРОВЕРКА: Какие события отправляет агент"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

echo "1️⃣ Логи сервера: обработка events/batch (последние 50 строк):"
echo "─────────────────────────────────────────────────────────"
journalctl -u monitoring-server --since "1 hour ago" | grep -A 5 -B 5 "events/batch\|activity event\|keyboard event\|usb event\|file event" | tail -50
echo ""

echo "2️⃣ Ошибки unmarshal или insert (последние 30):"
echo "─────────────────────────────────────────────────────────"
journalctl -u monitoring-server --since "1 hour ago" | grep -i "failed.*unmarshal\|failed.*insert" | tail -30
echo ""

echo "3️⃣ Validation failures (ComputerName пустой, Duration > 86400):"
echo "─────────────────────────────────────────────────────────"
echo "(Эти ошибки не логируются - код просто делает continue)"
echo "Проверим через добавление debug логов..."
echo ""

echo "4️⃣ Все логи с 'activity' за последний час:"
echo "─────────────────────────────────────────────────────────"
journalctl -u monitoring-server --since "1 hour ago" | grep -i "activity" | tail -40
echo ""

echo "5️⃣ Статистика ответов сервера:"
echo "─────────────────────────────────────────────────────────"
journalctl -u monitoring-server --since "1 hour ago" | grep "POST /api/events/batch" | tail -20
echo ""

echo "═══════════════════════════════════════════════════════════════════"
echo "💡 ПОДСКАЗКА:"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "Если видите ошибки 'Failed to unmarshal activity event':"
echo "  → Агент отправляет данные в неправильном формате"
echo ""
echo "Если видите ошибки 'Failed to insert activity event':"
echo "  → Проблема в InsertActivityEvent (ClickHouse)"
echo ""
echo "Если НЕТ никаких ошибок:"
echo "  → Агент не отправляет type='activity' события"
echo "  → Или validation fails (ComputerName пустой)"
echo ""
