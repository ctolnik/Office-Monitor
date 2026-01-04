package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ctolnik/Office-Monitor/zapctx"
	"go.uber.org/zap"
)

func (db *Database) AutoSyncCategoriesTable(ctx context.Context) error {
	zapctx.Info(ctx, "🔄 Auto-syncing categories table schema...")

	createTableSQL := `
CREATE TABLE IF NOT EXISTS monitoring.categories (
    id UUID DEFAULT generateUUIDv4(),
    key String,
    name String,
    color String DEFAULT '',
    sort_order Int32 DEFAULT 0,
    is_active UInt8 DEFAULT 1,
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY id
SETTINGS index_granularity = 8192`

	if err := db.conn.Exec(ctx, createTableSQL); err != nil {
		zapctx.Error(ctx, "Failed to create categories table", zap.Error(err))
		return err
	}

	// Unique-like index on key (ClickHouse doesn't enforce uniqueness; this is for query speed).
	indexSQL := []string{
		`ALTER TABLE monitoring.categories ADD INDEX IF NOT EXISTS idx_categories_key key TYPE set(0) GRANULARITY 4`,
		`ALTER TABLE monitoring.categories ADD INDEX IF NOT EXISTS idx_categories_is_active is_active TYPE set(0) GRANULARITY 4`,
	}
	for _, sql := range indexSQL {
		_ = db.conn.Exec(ctx, sql)
	}

	zapctx.Info(ctx, "✅ categories table schema is up to date")
	return nil
}

func (db *Database) AutoSeedDefaultCategoryTypes(ctx context.Context) error {
	// If empty, seed a minimal set. User can change/delete later.
	var count uint64
	if err := db.conn.QueryRow(ctx, "SELECT count() FROM monitoring.categories").Scan(&count); err != nil {
		zapctx.Error(ctx, "Failed to check categories count", zap.Error(err))
		return err
	}
	if count > 0 {
		zapctx.Info(ctx, "✅ Categories already seeded", zap.Uint64("count", count))
		return nil
	}

	zapctx.Info(ctx, "📥 Seeding default categories...")
	seedSQL := `
INSERT INTO monitoring.categories (key, name, color, sort_order, is_active)
VALUES
  ('productive', 'Продуктивно', '#10b981', 1, 1),
  ('unproductive', 'Непродуктивно', '#ef4444', 2, 1),
  ('neutral', 'Нейтрально', '#6b7280', 3, 1),
  ('communication', 'Коммуникации', '#3b82f6', 4, 1),
  ('entertainment', 'Развлечения', '#a855f7', 5, 1),
  ('system', 'Системная', '#64748b', 6, 1)
`
	if err := db.conn.Exec(ctx, seedSQL); err != nil {
		zapctx.Error(ctx, "Failed to seed default categories", zap.Error(err))
		return err
	}

	zapctx.Info(ctx, "✅ Default categories seeded")
	return nil
}

func (db *Database) GetCategoryTypes(ctx context.Context, activeOnly bool) ([]CategoryType, error) {
	query := `
SELECT
  toString(id) as id,
  key,
  name,
  color,
  sort_order,
  is_active,
  created_at,
  updated_at
FROM monitoring.categories FINAL
WHERE 1=1`

	args := make([]interface{}, 0)
	if activeOnly {
		query += " AND is_active = 1"
	}
	query += " ORDER BY sort_order, name"

	rows, err := db.conn.Query(ctx, query, args...)
	if err != nil {
		zapctx.Error(ctx, "Failed to query categories", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	out := make([]CategoryType, 0)
	for rows.Next() {
		var ct CategoryType
		var isActive uint8
		if err := rows.Scan(&ct.ID, &ct.Key, &ct.Name, &ct.Color, &ct.SortOrder, &isActive, &ct.CreatedAt, &ct.UpdatedAt); err != nil {
			zapctx.Warn(ctx, "Failed to scan category row", zap.Error(err))
			continue
		}
		ct.IsActive = isActive == 1
		out = append(out, ct)
	}
	return out, rows.Err()
}

func (db *Database) getCategoryTypeByID(ctx context.Context, id string) (CategoryType, error) {
	query := `
SELECT
  toString(id) as id,
  key,
  name,
  color,
  sort_order,
  is_active,
  created_at,
  updated_at
FROM monitoring.categories FINAL
WHERE toString(id) = ?
LIMIT 1`

	var ct CategoryType
	var isActive uint8
	if err := db.conn.QueryRow(ctx, query, id).Scan(&ct.ID, &ct.Key, &ct.Name, &ct.Color, &ct.SortOrder, &isActive, &ct.CreatedAt, &ct.UpdatedAt); err != nil {
		return CategoryType{}, err
	}
	ct.IsActive = isActive == 1
	return ct, nil
}

func (db *Database) getCategoryTypeByKey(ctx context.Context, key string) (CategoryType, bool, error) {
	query := `
SELECT
  toString(id) as id,
  key,
  name,
  color,
  sort_order,
  is_active,
  created_at,
  updated_at
FROM monitoring.categories FINAL
WHERE key = ?
ORDER BY updated_at DESC
LIMIT 1`

	var ct CategoryType
	var isActive uint8
	err := db.conn.QueryRow(ctx, query, key).Scan(&ct.ID, &ct.Key, &ct.Name, &ct.Color, &ct.SortOrder, &isActive, &ct.CreatedAt, &ct.UpdatedAt)
	if err != nil {
		// clickhouse-go returns error on no rows; treat as not found.
		return CategoryType{}, false, nil
	}
	ct.IsActive = isActive == 1
	return ct, true, nil
}

func validateCategoryKey(key string) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	key = strings.TrimSpace(key)
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("invalid key: %s", key)
	}
	if strings.ToLower(key) != key {
		return fmt.Errorf("key must be lowercase")
	}
	return nil
}

func (db *Database) CreateCategoryType(ctx context.Context, ct CategoryType) (CategoryType, error) {
	if err := validateCategoryKey(ct.Key); err != nil {
		return CategoryType{}, err
	}
	if strings.TrimSpace(ct.Name) == "" {
		return CategoryType{}, fmt.Errorf("name is required")
	}

	// Uniqueness check (best-effort).
	existing, found, err := db.getCategoryTypeByKey(ctx, ct.Key)
	if err != nil {
		return CategoryType{}, err
	}
	if found && existing.IsActive {
		return CategoryType{}, fmt.Errorf("category key already exists")
	}

	ct.ID = fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		time.Now().UnixNano()&0xFFFFFFFF,
		(time.Now().UnixNano()>>32)&0xFFFF,
		0x4000|((time.Now().UnixNano()>>48)&0x0FFF),
		0x8000|((time.Now().UnixNano()>>60)&0x3FFF),
		time.Now().UnixNano()&0xFFFFFFFFFFFF)
	ct.CreatedAt = time.Now()
	ct.UpdatedAt = time.Now()
	ct.IsActive = true

	query := `
INSERT INTO monitoring.categories (id, key, name, color, sort_order, is_active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	isActive := uint8(0)
	if ct.IsActive {
		isActive = 1
	}

	if err := db.conn.Exec(ctx, query, ct.ID, ct.Key, ct.Name, ct.Color, ct.SortOrder, isActive, ct.CreatedAt, ct.UpdatedAt); err != nil {
		zapctx.Error(ctx, "Failed to create category", zap.Error(err), zap.String("key", ct.Key))
		return CategoryType{}, err
	}

	return ct, nil
}

type CategoryTypeUpdate struct {
	Key       *string `json:"key,omitempty"`
	Name      *string `json:"name,omitempty"`
	Color     *string `json:"color,omitempty"`
	SortOrder *int32  `json:"sort_order,omitempty"`
	IsActive  *bool   `json:"is_active,omitempty"`
}

func (db *Database) UpdateCategoryType(ctx context.Context, id string, patch CategoryTypeUpdate) (CategoryType, error) {
	existing, err := db.getCategoryTypeByID(ctx, id)
	if err != nil {
		return CategoryType{}, err
	}

	updated := existing
	if patch.Key != nil {
		if err := validateCategoryKey(*patch.Key); err != nil {
			return CategoryType{}, err
		}
		updated.Key = *patch.Key
	}
	if patch.Name != nil {
		if strings.TrimSpace(*patch.Name) == "" {
			return CategoryType{}, fmt.Errorf("name is required")
		}
		updated.Name = *patch.Name
	}
	if patch.Color != nil {
		updated.Color = *patch.Color
	}
	if patch.SortOrder != nil {
		updated.SortOrder = *patch.SortOrder
	}
	if patch.IsActive != nil {
		updated.IsActive = *patch.IsActive
	}
	updated.UpdatedAt = time.Now()

	// Best-effort unique key check if key changed.
	if updated.Key != existing.Key {
		ex2, found, err := db.getCategoryTypeByKey(ctx, updated.Key)
		if err != nil {
			return CategoryType{}, err
		}
		if found && ex2.IsActive && ex2.ID != updated.ID {
			return CategoryType{}, fmt.Errorf("category key already exists")
		}
	}

	query := `
INSERT INTO monitoring.categories (id, key, name, color, sort_order, is_active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	isActive := uint8(0)
	if updated.IsActive {
		isActive = 1
	}

	if err := db.conn.Exec(ctx, query, updated.ID, updated.Key, updated.Name, updated.Color, updated.SortOrder, isActive, updated.CreatedAt, updated.UpdatedAt); err != nil {
		zapctx.Error(ctx, "Failed to update category", zap.Error(err), zap.String("id", id))
		return CategoryType{}, err
	}

	return updated, nil
}

func (db *Database) DeleteCategoryType(ctx context.Context, id string) error {
	existing, err := db.getCategoryTypeByID(ctx, id)
	if err != nil {
		return err
	}
	existing.IsActive = false
	existing.UpdatedAt = time.Now()

	query := `
INSERT INTO monitoring.categories (id, key, name, color, sort_order, is_active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	if err := db.conn.Exec(ctx, query, existing.ID, existing.Key, existing.Name, existing.Color, existing.SortOrder, uint8(0), existing.CreatedAt, existing.UpdatedAt); err != nil {
		zapctx.Error(ctx, "Failed to delete category", zap.Error(err), zap.String("id", id))
		return err
	}
	return nil
}
