package database

import (
	"context"

	"github.com/ctolnik/Office-Monitor/zapctx"
	"go.uber.org/zap"
)

// AutoSyncProcessCatalogTable creates the process_catalog_v2 table if it doesn't exist.
// This table is the single source of truth for application categorization rules.
// Note: Uses category_id UUID referencing monitoring.categories.
func (db *Database) AutoSyncProcessCatalogTable(ctx context.Context) error {
	zapctx.Info(ctx, "🔄 Auto-syncing process_catalog_v2 table schema...")

	createTableSQL := `
CREATE TABLE IF NOT EXISTS monitoring.process_catalog_v2 (
    id UUID DEFAULT generateUUIDv4(),
    friendly_name String,
    process_names Array(String),
    window_title_patterns Array(String),
    category_id UUID,
    is_active UInt8 DEFAULT 1,
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY id`

	if err := db.conn.Exec(ctx, createTableSQL); err != nil {
		zapctx.Error(ctx, "Failed to create process_catalog_v2 table", zap.Error(err))
		return err
	}

	zapctx.Info(ctx, "✅ process_catalog_v2 table schema is up to date")
	return nil
}
