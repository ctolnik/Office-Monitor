-- Миграции для Office-Monitor Backend
-- Дата создания: 2025-01-30
-- Описание: Добавление таблиц и полей для управления категориями приложений,
--           контекстной информации клавиатуры и системных настроек

-- ============================================================================
-- 1. Таблица для категорий приложений (Application Categories)
-- ============================================================================

CREATE TABLE IF NOT EXISTS monitoring.application_categories (
    id UUID DEFAULT generateUUIDv4(),
    process_name String,
    process_pattern String, -- wildcard pattern: *.exe, chrome*
    category Enum8(
        'productive' = 1, 
        'unproductive' = 2, 
        'neutral' = 3, 
        'communication' = 4, 
        'system' = 5
    ),
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now(),
    created_by String DEFAULT '',
    updated_by String DEFAULT '',
    is_active UInt8 DEFAULT 1
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (process_name, id)
SETTINGS index_granularity = 8192;

-- Индекс для быстрого поиска по категории
ALTER TABLE monitoring.application_categories 
ADD INDEX IF NOT EXISTS idx_category category TYPE set(0) GRANULARITY 4;

-- Индекс для поиска активных записей
ALTER TABLE monitoring.application_categories 
ADD INDEX IF NOT EXISTS idx_is_active is_active TYPE set(0) GRANULARITY 4;

-- ============================================================================
-- 2. Добавление поля context_info в keyboard_events
-- ============================================================================

ALTER TABLE monitoring.keyboard_events 
ADD COLUMN IF NOT EXISTS context_info String DEFAULT '';

-- ============================================================================
-- 3. Таблица для системных настроек (System Settings)
-- ============================================================================

CREATE TABLE IF NOT EXISTS monitoring.system_settings (
    key String,
    value String,
    type Enum8(
        'string' = 1, 
        'number' = 2, 
        'boolean' = 3, 
        'json' = 4
    ) DEFAULT 'string',
    description String DEFAULT '',
    updated_at DateTime DEFAULT now(),
    updated_by String DEFAULT 'system'
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY key
SETTINGS index_granularity = 8192;

-- ============================================================================
-- 4. Оптимизационные индексы для существующих таблиц
-- ============================================================================

-- Индексы для activity_events
ALTER TABLE monitoring.activity_events 
ADD INDEX IF NOT EXISTS idx_username_timestamp (username, timestamp) TYPE minmax GRANULARITY 4;

-- Индексы для screenshot_metadata
ALTER TABLE monitoring.screenshot_metadata 
ADD INDEX IF NOT EXISTS idx_username_timestamp (username, timestamp) TYPE minmax GRANULARITY 4;

-- Индексы для keyboard_events
ALTER TABLE monitoring.keyboard_events 
ADD INDEX IF NOT EXISTS idx_username_timestamp (username, timestamp) TYPE minmax GRANULARITY 4;

-- Индексы для usb_events
ALTER TABLE monitoring.usb_events 
ADD INDEX IF NOT EXISTS idx_username_timestamp (username, timestamp) TYPE minmax GRANULARITY 4;

-- Индексы для file_copy_events
ALTER TABLE monitoring.file_copy_events 
ADD INDEX IF NOT EXISTS idx_username_timestamp (username, timestamp) TYPE minmax GRANULARITY 4;

-- ============================================================================
-- 5. Начальные данные для application_categories (seed data)
-- ============================================================================

-- Продуктивные приложения
INSERT INTO monitoring.application_categories (process_name, process_pattern, category, created_by, updated_by) VALUES
-- IDE и редакторы кода
('code.exe', 'code*', 'productive', 'system', 'system'),
('idea64.exe', 'idea*', 'productive', 'system', 'system'),
('pycharm64.exe', 'pycharm*', 'productive', 'system', 'system'),
('goland64.exe', 'goland*', 'productive', 'system', 'system'),
('webstorm64.exe', 'webstorm*', 'productive', 'system', 'system'),
('notepad++.exe', 'notepad++*', 'productive', 'system', 'system'),
('sublime_text.exe', 'sublime*', 'productive', 'system', 'system'),

-- Терминалы и командная строка
('powershell.exe', 'powershell*', 'productive', 'system', 'system'),
('cmd.exe', '', 'productive', 'system', 'system'),
('terminal.exe', 'terminal*', 'productive', 'system', 'system'),
('wt.exe', '', 'productive', 'system', 'system'),

-- Базы данных
('datagrip64.exe', 'datagrip*', 'productive', 'system', 'system'),
('ssms.exe', '', 'productive', 'system', 'system'),

-- Microsoft Office
('excel.exe', '', 'productive', 'system', 'system'),
('winword.exe', '', 'productive', 'system', 'system'),
('powerpnt.exe', '', 'productive', 'system', 'system'),

-- Git clients
('git.exe', 'git*', 'productive', 'system', 'system'),
('gitkraken.exe', 'gitkraken*', 'productive', 'system', 'system'),

-- Непродуктивные приложения
('steam.exe', 'steam*', 'unproductive', 'system', 'system'),
('spotify.exe', 'spotify*', 'unproductive', 'system', 'system'),
('vlc.exe', '', 'unproductive', 'system', 'system'),

-- Мессенджеры (Communication)
('slack.exe', 'slack*', 'communication', 'system', 'system'),
('teams.exe', '', 'communication', 'system', 'system'),
('telegram.exe', '', 'communication', 'system', 'system'),
('discord.exe', '', 'communication', 'system', 'system'),
('outlook.exe', '', 'communication', 'system', 'system'),
('zoom.exe', '', 'communication', 'system', 'system'),

-- Браузеры (Neutral - категория определяется по URL)
('chrome.exe', '', 'neutral', 'system', 'system'),
('firefox.exe', '', 'neutral', 'system', 'system'),
('msedge.exe', '', 'neutral', 'system', 'system'),

-- Системные процессы
('explorer.exe', '', 'system', 'system', 'system'),
('taskmgr.exe', '', 'system', 'system', 'system');

-- ============================================================================
-- 6. Начальные данные для system_settings (seed data)
-- ============================================================================

INSERT INTO monitoring.system_settings (key, value, type, description, updated_by) VALUES
('org_name', 'GSL-Audit', 'string', 'Название организации', 'system'),
('org_tagline', 'Система мониторинга активности', 'string', 'Слоган организации', 'system'),
('org_logo', '', 'string', 'URL логотипа организации', 'system'),
('timezone', 'Europe/Moscow', 'string', 'Часовой пояс', 'system'),
('date_format', 'DD/MM/YYYY', 'string', 'Формат даты', 'system'),
('time_format', '24h', 'string', 'Формат времени (12h/24h)', 'system'),
('theme', 'light', 'string', 'Тема интерфейса (light/dark)', 'system'),
('language', 'ru', 'string', 'Язык интерфейса', 'system');

-- ============================================================================
-- Примечания по применению миграций:
-- ============================================================================

-- Для применения миграций выполните:
-- clickhouse-client --host=localhost --port=9000 --user=monitor_user --password=change_me_in_production --database=monitoring < migrations.sql

-- Для проверки созданных таблиц:
-- SHOW TABLES FROM monitoring;

-- Для проверки структуры таблицы:
-- DESCRIBE monitoring.application_categories;
-- DESCRIBE monitoring.system_settings;

-- Для проверки индексов:
-- SHOW CREATE TABLE monitoring.activity_events;
