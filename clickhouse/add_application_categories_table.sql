-- Add default application categories for productivity tracking
-- Compatible with schema from migrations.sql (5 categories)
-- This migration only inserts default data, table is created by migrations.sql

-- Note: Table application_categories is created by 02-migrations.sql
-- This file (03-categories.sql) only adds default application data

-- Insert some default categories (using 5-category schema from migrations.sql)
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

-- Neutral (browsers - depends on usage)
(generateUUIDv4(), 'chrome.exe', '.*', 'neutral', 'system', 1),
(generateUUIDv4(), 'firefox.exe', '.*', 'neutral', 'system', 1),
(generateUUIDv4(), 'msedge.exe', '.*', 'neutral', 'system', 1),

-- Unproductive websites/apps
(generateUUIDv4(), 'youtube.com', '.*', 'unproductive', 'system', 1),
(generateUUIDv4(), 'facebook.com', '.*', 'unproductive', 'system', 1),
(generateUUIDv4(), 'vk.com', '.*', 'unproductive', 'system', 1)
ON CONFLICT DO NOTHING;
