#!/bin/bash
# Скрипт для исправления "справочник программ" на production

echo "=== СОЗДАНИЕ ТАБЛИЦЫ application_categories ==="
echo ""
echo "Выполняем SQL..."

docker exec -i clickhouse clickhouse-client --database monitoring << 'SQL'
-- Создать таблицу application_categories с правильными категориями
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
ORDER BY (id);

-- Добавить базовые категории приложений
INSERT INTO monitoring.application_categories 
(id, process_name, process_pattern, category, created_by, is_active) 
VALUES
-- Productive applications
(generateUUIDv4(), 'code.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'devenv.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'excel.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'winword.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'outlook.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), '1cv8.exe', '.*', 'productive', 'system', 1),

-- Communication applications
(generateUUIDv4(), 'teams.exe', '.*', 'communication', 'system', 1),
(generateUUIDv4(), 'slack.exe', '.*', 'communication', 'system', 1),
(generateUUIDv4(), 'telegram.exe', '.*', 'communication', 'system', 1),
(generateUUIDv4(), 'skype.exe', '.*', 'communication', 'system', 1),

-- Neutral (browsers)
(generateUUIDv4(), 'chrome.exe', '.*', 'neutral', 'system', 1),
(generateUUIDv4(), 'firefox.exe', '.*', 'neutral', 'system', 1),
(generateUUIDv4(), 'msedge.exe', '.*', 'neutral', 'system', 1),
(generateUUIDv4(), 'opera.exe', '.*', 'neutral', 'system', 1),

-- Unproductive
(generateUUIDv4(), 'youtube.com', '.*', 'unproductive', 'system', 1),
(generateUUIDv4(), 'facebook.com', '.*', 'unproductive', 'system', 1),
(generateUUIDv4(), 'vk.com', '.*', 'unproductive', 'system', 1),
(generateUUIDv4(), 'instagram.com', '.*', 'unproductive', 'system', 1);
SQL

echo ""
echo "=== ПРОВЕРКА ==="
echo ""

# Проверить количество записей
COUNT=$(docker exec clickhouse clickhouse-client --database monitoring -q "SELECT count(*) FROM monitoring.application_categories")
echo "✅ Количество записей: $COUNT"

if [ "$COUNT" -gt 0 ]; then
    echo ""
    echo "=== ПРИМЕРЫ ЗАПИСЕЙ ==="
    docker exec clickhouse clickhouse-client --database monitoring -q "SELECT process_name, category FROM monitoring.application_categories FINAL LIMIT 5"
    echo ""
    echo "✅ ГОТОВО! Таблица создана и заполнена."
    echo ""
    echo "Теперь откройте веб-интерфейс и проверьте 'Справочник программ'"
else
    echo "❌ ОШИБКА: Таблица пустая или не создалась!"
fi
