-- Create application_categories table with correct schema
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

-- Insert default categories
INSERT INTO monitoring.application_categories 
(id, process_name, process_pattern, category, created_by, is_active) 
VALUES
-- Productive applications
(generateUUIDv4(), 'code.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'devenv.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'excel.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'winword.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'outlook.exe', '.*', 'productive', 'system', 1),

-- Communication applications
(generateUUIDv4(), 'teams.exe', '.*', 'communication', 'system', 1),
(generateUUIDv4(), 'slack.exe', '.*', 'communication', 'system', 1),
(generateUUIDv4(), 'telegram.exe', '.*', 'communication', 'system', 1),

-- Neutral (browsers)
(generateUUIDv4(), 'chrome.exe', '.*', 'neutral', 'system', 1),
(generateUUIDv4(), 'firefox.exe', '.*', 'neutral', 'system', 1),
(generateUUIDv4(), 'msedge.exe', '.*', 'neutral', 'system', 1),

-- Unproductive
(generateUUIDv4(), 'youtube.com', '.*', 'unproductive', 'system', 1),
(generateUUIDv4(), 'facebook.com', '.*', 'unproductive', 'system', 1),
(generateUUIDv4(), 'vk.com', '.*', 'unproductive', 'system', 1);
