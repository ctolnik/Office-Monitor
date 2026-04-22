package database

import (
	"context"
	"fmt"
	"time"

	"github.com/ctolnik/Office-Monitor/zapctx"
	"go.uber.org/zap"
)

// The schema defined in `clickhouse/01-schema.sql` uses (key, value,
// updated_at, updated_by) as columns. Earlier revisions of this file queried
// columns that never existed in the table (setting_key, setting_value,
// is_active), which caused `SELECT ... FROM monitoring.system_settings` to
// fail with `Unknown expression identifier` on every /api/settings call.
// All queries below have been realigned to the actual schema and rely on
// ReplacingMergeTree semantics for the key column to make INSERTs idempotent.

// GetSystemSettings returns all system settings as a map.
func (db *Database) GetSystemSettings(ctx context.Context) (map[string]string, error) {
	query := `
		SELECT key, value
		FROM monitoring.system_settings FINAL`

	rows, err := db.conn.Query(ctx, query)
	if err != nil {
		zapctx.Error(ctx, "Failed to query system settings", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			zapctx.Error(ctx, "Failed to scan setting row", zap.Error(err))
			continue
		}
		settings[key] = value
	}

	if err := rows.Err(); err != nil {
		zapctx.Error(ctx, "Error iterating settings rows", zap.Error(err))
		return nil, err
	}

	zapctx.Debug(ctx, "System settings retrieved", zap.Int("count", len(settings)))
	return settings, nil
}

// GetSystemSetting returns a single system setting by key.
func (db *Database) GetSystemSetting(ctx context.Context, key string) (string, error) {
	query := `
		SELECT value
		FROM monitoring.system_settings FINAL
		WHERE key = ?
		LIMIT 1`

	var value string
	err := db.conn.QueryRow(ctx, query, key).Scan(&value)
	if err != nil {
		zapctx.Warn(ctx, "Setting not found or error",
			zap.String("key", key),
			zap.Error(err))
		return "", fmt.Errorf("setting not found: %s", key)
	}

	return value, nil
}

// UpdateSystemSetting updates or inserts a system setting. The underlying
// table is ReplacingMergeTree(updated_at) ordered by key, so repeated
// INSERTs collapse to the latest version at merge time.
func (db *Database) UpdateSystemSetting(ctx context.Context, key, value, updatedBy string) error {
	query := `
		INSERT INTO monitoring.system_settings
			(key, value, updated_at, updated_by)
		VALUES (?, ?, ?, ?)`

	err := db.conn.Exec(ctx, query, key, value, time.Now(), updatedBy)
	if err != nil {
		zapctx.Error(ctx, "Failed to update system setting",
			zap.Error(err),
			zap.String("key", key))
		return err
	}

	zapctx.Info(ctx, "System setting updated",
		zap.String("key", key),
		zap.String("updated_by", updatedBy))

	return nil
}

// UpdateMultipleSettings updates multiple settings at once (batch operation).
func (db *Database) UpdateMultipleSettings(ctx context.Context, settings map[string]string, updatedBy string) error {
	batch, err := db.conn.PrepareBatch(ctx, `
		INSERT INTO monitoring.system_settings
			(key, value, updated_at, updated_by)`)
	if err != nil {
		zapctx.Error(ctx, "Failed to prepare batch for settings update", zap.Error(err))
		return err
	}

	now := time.Now()
	for key, value := range settings {
		if err := batch.Append(key, value, now, updatedBy); err != nil {
			zapctx.Error(ctx, "Failed to append setting to batch",
				zap.Error(err),
				zap.String("key", key))
			return err
		}
	}

	if err := batch.Send(); err != nil {
		zapctx.Error(ctx, "Failed to send settings batch", zap.Error(err))
		return err
	}

	zapctx.Info(ctx, "Multiple settings updated",
		zap.Int("count", len(settings)),
		zap.String("updated_by", updatedBy))

	return nil
}

// DeleteSystemSetting removes a system setting. The table has no soft-delete
// column, so this issues an async ClickHouse mutation to drop the row.
func (db *Database) DeleteSystemSetting(ctx context.Context, key string) error {
	query := `ALTER TABLE monitoring.system_settings DELETE WHERE key = ?`

	err := db.conn.Exec(ctx, query, key)
	if err != nil {
		zapctx.Error(ctx, "Failed to delete system setting",
			zap.Error(err),
			zap.String("key", key))
		return err
	}

	zapctx.Info(ctx, "System setting deleted", zap.String("key", key))
	return nil
}
