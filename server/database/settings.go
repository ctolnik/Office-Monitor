package database

import (
	"context"
	"fmt"
	
	"github.com/ctolnik/Office-Monitor/zapctx"
	"go.uber.org/zap"
)

// GetSystemSettings returns all system settings as a map
func (db *Database) GetSystemSettings(ctx context.Context) (map[string]string, error) {
	query := `
		SELECT setting_key, setting_value
		FROM monitoring.system_settings
		WHERE is_active = 1`
	
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

// GetSystemSetting returns a single system setting by key
func (db *Database) GetSystemSetting(ctx context.Context, key string) (string, error) {
	query := `
		SELECT setting_value
		FROM monitoring.system_settings
		WHERE setting_key = ? AND is_active = 1
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

// UpdateSystemSetting updates or inserts a system setting
func (db *Database) UpdateSystemSetting(ctx context.Context, key, value, updatedBy string) error {
	query := `
		INSERT INTO monitoring.system_settings 
			(setting_key, setting_value, updated_by, updated_at, is_active)
		VALUES (?, ?, ?, now(), 1)`
	
	err := db.conn.Exec(ctx, query, key, value, updatedBy)
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

// UpdateMultipleSettings updates multiple settings at once (batch operation)
func (db *Database) UpdateMultipleSettings(ctx context.Context, settings map[string]string, updatedBy string) error {
	batch, err := db.conn.PrepareBatch(ctx, `
		INSERT INTO monitoring.system_settings 
			(setting_key, setting_value, updated_by, updated_at, is_active)`)
	if err != nil {
		zapctx.Error(ctx, "Failed to prepare batch for settings update", zap.Error(err))
		return err
	}
	
	for key, value := range settings {
		if err := batch.Append(key, value, updatedBy, "now()", 1); err != nil {
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

// DeleteSystemSetting soft-deletes a system setting
func (db *Database) DeleteSystemSetting(ctx context.Context, key string) error {
	query := `ALTER TABLE monitoring.system_settings UPDATE is_active = 0 WHERE setting_key = ?`
	
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
