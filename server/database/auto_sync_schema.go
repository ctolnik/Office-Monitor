package database

import (
        "context"

        "github.com/ClickHouse/clickhouse-go/v2"
        "github.com/ctolnik/Office-Monitor/zapctx"
        "go.uber.org/zap"
)

// AutoSyncApplicationCategoriesTable creates or updates the application_categories table schema
// This runs automatically on server startup to ensure table exists
func (db *Database) AutoSyncApplicationCategoriesTable(ctx context.Context) error {
        zapctx.Info(ctx, "🔄 Auto-syncing application_categories table schema...")

        // Create table if not exists (idempotent)
        createTableSQL := `
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
ORDER BY (process_name, id)
SETTINGS index_granularity = 8192`

        err := db.conn.Exec(ctx, createTableSQL)
        if err != nil {
                zapctx.Error(ctx, "Failed to create application_categories table", zap.Error(err))
                return err
        }

        // Add indexes if not exist (idempotent)
        indexSQL := []string{
                `ALTER TABLE monitoring.application_categories ADD INDEX IF NOT EXISTS idx_category category TYPE set(0) GRANULARITY 4`,
                `ALTER TABLE monitoring.application_categories ADD INDEX IF NOT EXISTS idx_is_active is_active TYPE set(0) GRANULARITY 4`,
        }

        for _, sql := range indexSQL {
                if err := db.conn.Exec(ctx, sql); err != nil {
                        // Indexes might already exist, log but don't fail
                        if exception, ok := err.(*clickhouse.Exception); ok && exception.Code != 44 { // Code 44 = index already exists
                                zapctx.Warn(ctx, "Failed to add index", zap.Error(err))
                        }
                }
        }

        zapctx.Info(ctx, "✅ application_categories table schema is up to date")
        return nil
}

// AutoLoadDefaultCategories loads default seed data if table is empty
// This ensures basic categories are always available
func (db *Database) AutoLoadDefaultCategories(ctx context.Context) error {
        // Check if table has data
        var count uint64
        err := db.conn.QueryRow(ctx, "SELECT count(*) FROM monitoring.application_categories").Scan(&count)
        if err != nil {
                zapctx.Error(ctx, "Failed to check categories count", zap.Error(err))
                return err
        }

        if count > 0 {
                zapctx.Info(ctx, "✅ Application categories already loaded", zap.Uint64("count", count))
                return nil
        }

        zapctx.Info(ctx, "📥 Loading default application categories...")

        // Load basic seed data
        seedSQL := `
INSERT INTO monitoring.application_categories 
(process_name, process_pattern, category, created_by, updated_by, is_active) 
VALUES
-- Productive
('code.exe', 'code*', 'productive', 'system', 'system', 1),
('chrome.exe', 'chrome*', 'neutral', 'system', 'system', 1),
('firefox.exe', 'firefox*', 'neutral', 'system', 'system', 1),
('excel.exe', '', 'productive', 'system', 'system', 1),
('winword.exe', '', 'productive', 'system', 'system', 1),
('powerpnt.exe', '', 'productive', 'system', 'system', 1),
('powershell.exe', 'powershell*', 'productive', 'system', 'system', 1),
('cmd.exe', '', 'productive', 'system', 'system', 1),
-- Communication
('teams.exe', 'teams*', 'communication', 'system', 'system', 1),
('slack.exe', 'slack*', 'communication', 'system', 'system', 1),
('telegram.exe', 'telegram*', 'communication', 'system', 'system', 1),
('outlook.exe', '', 'communication', 'system', 'system', 1),
-- Unproductive
('steam.exe', 'steam*', 'unproductive', 'system', 'system', 1),
('spotify.exe', 'spotify*', 'unproductive', 'system', 'system', 1)
`

        err = db.conn.Exec(ctx, seedSQL)
        if err != nil {
                zapctx.Error(ctx, "Failed to load seed data", zap.Error(err))
                return err
        }

        zapctx.Info(ctx, "✅ Default categories loaded successfully")
        return nil
}

// AutoSyncProcessCatalogTable creates the process_catalog table if it doesn't exist
// This is used by /api/process-catalog endpoints
func (db *Database) AutoSyncProcessCatalogTable(ctx context.Context) error {
        zapctx.Info(ctx, "🔄 Auto-syncing process_catalog table schema...")

        createTableSQL := `
CREATE TABLE IF NOT EXISTS monitoring.process_catalog (
    id String,
    friendly_name String,
    process_names Array(String),
    window_title_patterns Array(String),
    category Enum8('productive' = 1, 'unproductive' = 2, 'neutral' = 3, 'communication' = 4, 'entertainment' = 5),
    is_active UInt8 DEFAULT 1,
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY id`

        err := db.conn.Exec(ctx, createTableSQL)
        if err != nil {
                zapctx.Error(ctx, "Failed to create process_catalog table", zap.Error(err))
                return err
        }

        zapctx.Info(ctx, "✅ process_catalog table schema is up to date")
        return nil
}
