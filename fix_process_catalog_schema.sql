-- Fix process_catalog table schema to match application code

-- Drop old table if exists
DROP TABLE IF EXISTS monitoring.process_catalog;

-- Create with correct category enum matching application_categories
CREATE TABLE IF NOT EXISTS monitoring.process_catalog (
    id String,  -- Changed from UUID to String
    friendly_name String,
    process_names Array(String),
    window_title_patterns Array(String),
    category Enum8('productive' = 1, 'unproductive' = 2, 'neutral' = 3, 'communication' = 4, 'entertainment' = 5),
    is_active UInt8 DEFAULT 1,
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY id;

-- Add some default entries
INSERT INTO monitoring.process_catalog 
(id, friendly_name, process_names, window_title_patterns, category, is_active, created_at, updated_at) 
VALUES
('1', 'Google Chrome', ['chrome.exe'], [], 'neutral', 1, now(), now()),
('2', 'Visual Studio Code', ['code.exe'], [], 'productive', 1, now(), now()),
('3', 'Microsoft Word', ['winword.exe'], [], 'productive', 1, now(), now()),
('4', 'Microsoft Teams', ['teams.exe'], [], 'communication', 1, now(), now());
