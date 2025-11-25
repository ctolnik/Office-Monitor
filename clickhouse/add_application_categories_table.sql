-- Add application_categories table for categorizing applications (productive/unproductive/neutral)
-- This table allows admins to classify applications and processes for productivity reporting

CREATE TABLE IF NOT EXISTS monitoring.application_categories (
    id UUID DEFAULT generateUUIDv4(),
    process_name String,
    process_pattern String,
    category Enum8('productive' = 1, 'neutral' = 2, 'unproductive' = 3),
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now(),
    created_by String DEFAULT 'system',
    updated_by String DEFAULT 'system',
    is_active UInt8 DEFAULT 1
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY id;

-- Insert some default categories
INSERT INTO monitoring.application_categories 
(id, process_name, process_pattern, category, created_by) 
VALUES
(generateUUIDv4(), 'chrome.exe', '.*', 'neutral', 'system'),
(generateUUIDv4(), 'firefox.exe', '.*', 'neutral', 'system'),
(generateUUIDv4(), 'msedge.exe', '.*', 'neutral', 'system'),
(generateUUIDv4(), 'code.exe', '.*', 'productive', 'system'),
(generateUUIDv4(), 'devenv.exe', '.*', 'productive', 'system'),
(generateUUIDv4(), 'excel.exe', '.*', 'productive', 'system'),
(generateUUIDv4(), 'winword.exe', '.*', 'productive', 'system'),
(generateUUIDv4(), 'outlook.exe', '.*', 'productive', 'system'),
(generateUUIDv4(), 'teams.exe', '.*', 'productive', 'system'),
(generateUUIDv4(), 'slack.exe', '.*', 'productive', 'system'),
(generateUUIDv4(), 'youtube.com', '.*', 'unproductive', 'system'),
(generateUUIDv4(), 'facebook.com', '.*', 'unproductive', 'system'),
(generateUUIDv4(), 'vk.com', '.*', 'unproductive', 'system'),
(generateUUIDv4(), 'telegram.exe', '.*', 'neutral', 'system');
