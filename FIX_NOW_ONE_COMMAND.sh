#!/bin/bash
# КРИТИЧНО: Выполните эту команду на production сервере!

cat << 'SQL' | docker exec -i clickhouse clickhouse-client --database monitoring
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

INSERT INTO monitoring.application_categories 
(id, process_name, process_pattern, category, created_by, is_active) 
VALUES
(generateUUIDv4(), 'code.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'excel.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'winword.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'teams.exe', '.*', 'communication', 'system', 1),
(generateUUIDv4(), 'chrome.exe', '.*', 'neutral', 'system', 1),
(generateUUIDv4(), '1cv8.exe', '.*', 'productive', 'system', 1);
SQL

# Проверить
docker exec clickhouse clickhouse-client --database monitoring -q "SELECT count(*) as total FROM monitoring.application_categories"

