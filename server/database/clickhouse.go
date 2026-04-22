package database

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ctolnik/Office-Monitor/zapctx"
	"go.uber.org/zap"
)

type Database struct {
	conn driver.Conn
}

func New(ctx context.Context, host string, port int, database, username, password string) (*Database, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", host, port)},
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	db := &Database{conn: conn}

	// Auto-sync schema on startup (creates tables if missing)
	if err := db.AutoSyncCategoriesTable(ctx); err != nil {
		zapctx.Warn(ctx, "Failed to auto-sync categories table", zap.Error(err))
	}

	// Auto-sync process catalog rules table (used by /api/process-catalog)
	if err := db.AutoSyncProcessCatalogTable(ctx); err != nil {
		zapctx.Warn(ctx, "Failed to auto-sync process catalog table", zap.Error(err))
	}

	// Seed default category types if table is empty
	if err := db.AutoSeedDefaultCategoryTypes(ctx); err != nil {
		zapctx.Warn(ctx, "Failed to seed default category types", zap.Error(err))
	}

	return db, nil
}

func (db *Database) InsertActivityEvent(ctx context.Context, event ActivityEvent) error {
	query := `INSERT INTO monitoring.activity_events 
                (timestamp, computer_name, username, window_title, process_name, duration)
                VALUES (?, ?, ?, ?, ?, ?)`
	return db.conn.Exec(ctx, query,
		event.Timestamp, event.ComputerName, event.Username,
		event.WindowTitle, event.ProcessName, event.Duration)
}

func (db *Database) InsertActivityEventsBatch(ctx context.Context, events []ActivityEvent) error {
	if len(events) == 0 {
		return nil
	}

	batch, err := db.conn.PrepareBatch(ctx, "INSERT INTO monitoring.activity_events (timestamp, computer_name, username, window_title, process_name, duration)")
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, event := range events {
		if err := batch.Append(
			event.Timestamp,
			event.ComputerName,
			event.Username,
			event.WindowTitle,
			event.ProcessName,
			event.Duration,
		); err != nil {
			return fmt.Errorf("failed to append event: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	return nil
}

func (db *Database) InsertUSBEvent(ctx context.Context, event USBEvent) error {
	query := `INSERT INTO monitoring.usb_events 
                (timestamp, computer_name, username, device_id, device_name, device_type, event_type, volume_serial)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	return db.conn.Exec(ctx, query,
		event.Timestamp, event.ComputerName, event.Username,
		event.DeviceID, event.DeviceName, event.DeviceType,
		event.EventType, event.VolumeSerial)
}

func (db *Database) InsertFileCopyEvent(ctx context.Context, event FileCopyEvent) error {
	query := `INSERT INTO monitoring.file_copy_events 
                (timestamp, computer_name, username, source_path, destination_path, file_size, file_count, operation_type, is_usb_target)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	return db.conn.Exec(ctx, query,
		event.Timestamp, event.ComputerName, event.Username,
		event.SourcePath, event.DestinationPath, event.FileSize,
		event.FileCount, event.OperationType, uint8(event.IsUSBTarget))
}

func (db *Database) InsertScreenshotMetadata(ctx context.Context, meta ScreenshotMetadata) error {
	query := `INSERT INTO monitoring.screenshot_metadata 
                (timestamp, computer_name, username, screenshot_id, minio_path, file_size, window_title, process_name)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	return db.conn.Exec(ctx, query,
		meta.Timestamp, meta.ComputerName, meta.Username,
		meta.ScreenshotID, meta.MinIOPath, meta.FileSize,
		meta.WindowTitle, meta.ProcessName)
}

func (db *Database) InsertKeyboardEvent(ctx context.Context, event KeyboardEvent) error {
	query := `INSERT INTO monitoring.keyboard_events 
                (timestamp, computer_name, username, window_title, process_name, text_content)
                VALUES (?, ?, ?, ?, ?, ?)`
	return db.conn.Exec(ctx, query,
		event.Timestamp, event.ComputerName, event.Username,
		event.WindowTitle, event.ProcessName, event.TextContent)
}

func (db *Database) GetKeyboardEvents(ctx context.Context, computerName string, from, to time.Time) ([]KeyboardEvent, error) {
	query := `SELECT timestamp, computer_name, username, window_title, process_name, text_content
                FROM monitoring.keyboard_events
                WHERE computer_name = ? AND timestamp >= ? AND timestamp <= ?
                ORDER BY timestamp DESC`

	rows, err := db.conn.Query(ctx, query, computerName, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]KeyboardEvent, 0)
	for rows.Next() {
		var e KeyboardEvent
		if err := rows.Scan(&e.Timestamp, &e.ComputerName, &e.Username, &e.WindowTitle, &e.ProcessName, &e.TextContent); err != nil {
			continue
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

func (db *Database) GetActiveEmployees(ctx context.Context) ([]Employee, error) {
	query := `SELECT computer_name, username, MAX(last_seen) as last_seen
                FROM (
                        SELECT computer_name, username, MAX(timestamp) as last_seen
                        FROM monitoring.activity_events
                        WHERE timestamp > now() - INTERVAL 30 DAY
                        GROUP BY computer_name, username
                        UNION ALL
                        SELECT computer_name, username, MAX(timestamp_start) as last_seen
                        FROM monitoring.activity_segments
                        WHERE timestamp_start > now() - INTERVAL 30 DAY
                        GROUP BY computer_name, username
                )
                GROUP BY computer_name, username
                ORDER BY last_seen DESC`

	rows, err := db.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	employees := make([]Employee, 0)
	for rows.Next() {
		var e Employee
		if err := rows.Scan(&e.ComputerName, &e.Username, &e.LastSeen); err != nil {
			continue
		}

		minutesSince := int(time.Since(e.LastSeen).Minutes())
		if minutesSince < 5 {
			e.Status = "active"
		} else if minutesSince < 30 {
			e.Status = "idle"
		} else {
			e.Status = "offline"
		}

		employees = append(employees, e)
	}

	return employees, rows.Err()
}

func (db *Database) GetRecentActivity(ctx context.Context, limit int) ([]ActivityEvent, error) {
	query := `SELECT timestamp, computer_name, username, window_title, process_name, duration
                FROM monitoring.activity_events
                ORDER BY timestamp DESC
                LIMIT ?`

	rows, err := db.conn.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]ActivityEvent, 0)
	for rows.Next() {
		var e ActivityEvent
		if err := rows.Scan(&e.Timestamp, &e.ComputerName, &e.Username,
			&e.WindowTitle, &e.ProcessName, &e.Duration); err != nil {
			continue
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

func (db *Database) GetUSBEvents(ctx context.Context, computerName string, from, to time.Time) ([]USBEvent, error) {
	query := `SELECT timestamp, computer_name, username, device_id, device_name, device_type, event_type, volume_serial
                FROM monitoring.usb_events
                WHERE computer_name = ? AND timestamp BETWEEN ? AND ?
                ORDER BY timestamp DESC`

	rows, err := db.conn.Query(ctx, query, computerName, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]USBEvent, 0)
	for rows.Next() {
		var e USBEvent
		if err := rows.Scan(&e.Timestamp, &e.ComputerName, &e.Username,
			&e.DeviceID, &e.DeviceName, &e.DeviceType,
			&e.EventType, &e.VolumeSerial); err != nil {
			continue
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

func (db *Database) GetFileEvents(ctx context.Context, computerName string, from, to time.Time) ([]FileCopyEvent, error) {
	query := `SELECT timestamp, computer_name, username, source_path, destination_path, file_size, file_count, operation_type, is_usb_target
                FROM monitoring.file_copy_events
                WHERE computer_name = ? AND timestamp BETWEEN ? AND ?
                ORDER BY timestamp DESC
                LIMIT 1000`

	rows, err := db.conn.Query(ctx, query, computerName, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]FileCopyEvent, 0)
	for rows.Next() {
		var e FileCopyEvent
		var isUSBTarget uint8
		if err := rows.Scan(&e.Timestamp, &e.ComputerName, &e.Username, &e.SourcePath, &e.DestinationPath, &e.FileSize, &e.FileCount, &e.OperationType, &isUSBTarget); err != nil {
			continue
		}
		e.IsUSBTarget = USBTargetFlag(isUSBTarget)
		events = append(events, e)
	}

	return events, nil
}

func (db *Database) InsertActivitySegment(ctx context.Context, segment ActivitySegment) error {
	query := `INSERT INTO monitoring.activity_segments
                (timestamp_start, timestamp_end, duration_sec, state, computer_name, username, process_name, window_title, session_id, category)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	return db.conn.Exec(ctx, query,
		segment.TimestampStart, segment.TimestampEnd, segment.DurationSec,
		segment.State, segment.ComputerName, segment.Username,
		segment.ProcessName, segment.WindowTitle, segment.SessionID, segment.Category)
}

func (db *Database) GetDailyActivitySummary(ctx context.Context, computerName string, date time.Time) (*DailyActivitySummary, error) {
	dateStr := date.Format("2006-01-02")

	query := `SELECT 
                state,
                sum(total_seconds) as seconds
        FROM monitoring.daily_activity_summary
        WHERE computer_name = ? AND event_date = ?
        GROUP BY state`

	rows, err := db.conn.Query(ctx, query, computerName, dateStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := &DailyActivitySummary{
		Date:         dateStr,
		ComputerName: computerName,
	}

	for rows.Next() {
		var state string
		var seconds uint64
		if err := rows.Scan(&state, &seconds); err != nil {
			continue
		}

		switch state {
		case "active":
			summary.ActiveSeconds = seconds
		case "idle":
			summary.IdleSeconds = seconds
		case "offline":
			summary.OfflineSeconds = seconds
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	usernameQuery := `SELECT DISTINCT username FROM monitoring.activity_segments 
                WHERE computer_name = ? AND toDate(timestamp_start) = ? LIMIT 1`
	row := db.conn.QueryRow(ctx, usernameQuery, computerName, dateStr)
	_ = row.Scan(&summary.Username)

	topProgramsQuery := `SELECT 
                process_name,
                sum(total_seconds) as duration,
                groupArray(DISTINCT window_title) as titles
        FROM monitoring.program_usage_daily
        WHERE computer_name = ? AND event_date = ? AND state = 'active'
        GROUP BY process_name
        ORDER BY duration DESC
        LIMIT 10`

	programRows, err := db.conn.Query(ctx, topProgramsQuery, computerName, dateStr)
	if err != nil {
		return summary, nil
	}
	defer programRows.Close()

	summary.TopPrograms = make([]ProgramUsage, 0)
	for programRows.Next() {
		var prog ProgramUsage
		var titles []string
		if err := programRows.Scan(&prog.ProcessName, &prog.DurationSec, &titles); err != nil {
			continue
		}
		prog.FriendlyName = prog.ProcessName
		if len(titles) > 0 && titles[0] != "" {
			prog.WindowTitles = titles
		}
		summary.TopPrograms = append(summary.TopPrograms, prog)
	}

	return summary, nil
}

func (db *Database) getProcessCatalogEntryByID(ctx context.Context, id string) (ProcessCatalogEntry, error) {
	query := `
                SELECT
                        toString(pc.id) as id,
                        pc.friendly_name,
                        pc.process_names,
                        pc.window_title_patterns,
                        toString(pc.category_id) as category_id,
                        toString(c.id) as cat_id,
                        c.key,
                        c.name,
                        c.color,
                        c.sort_order,
                        c.is_active,
                        pc.is_active,
                        pc.created_at,
                        pc.updated_at
                FROM monitoring.process_catalog_v2 pc
                LEFT JOIN monitoring.categories c ON c.id = pc.category_id
                WHERE toString(pc.id) = ?
                LIMIT 1`

	var entry ProcessCatalogEntry
	var cat CategoryType
	var catIsActive uint8
	var ruleIsActive uint8

	err := db.conn.QueryRow(ctx, query, id).Scan(
		&entry.ID,
		&entry.FriendlyName,
		&entry.ProcessNames,
		&entry.WindowTitlePatterns,
		&entry.CategoryID,
		&cat.ID,
		&cat.Key,
		&cat.Name,
		&cat.Color,
		&cat.SortOrder,
		&catIsActive,
		&ruleIsActive,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)
	if err != nil {
		return ProcessCatalogEntry{}, err
	}

	entry.IsActive = ruleIsActive == 1
	if cat.Key != "" {
		cat.IsActive = catIsActive == 1
		entry.Category = &cat
	}

	return entry, nil
}

func (db *Database) CreateProcessCatalogEntry(ctx context.Context, entry ProcessCatalogEntry) error {
	query := `INSERT INTO monitoring.process_catalog_v2
                (id, friendly_name, process_names, window_title_patterns, category_id, is_active, created_at, updated_at)
                VALUES (?, ?, ?, ?, toUUID(?), ?, ?, ?)`

	var isActive uint8 = 0
	if entry.IsActive {
		isActive = 1
	}

	err := db.conn.Exec(ctx, query,
		entry.ID,
		entry.FriendlyName,
		entry.ProcessNames,
		entry.WindowTitlePatterns,
		entry.CategoryID,
		isActive,
		entry.CreatedAt,
		entry.UpdatedAt,
	)

	if err != nil {
		zapctx.Error(ctx, "Failed to create process catalog entry", zap.Error(err), zap.String("friendly_name", entry.FriendlyName))
		return err
	}

	zapctx.Info(ctx, "Process catalog entry created", zap.String("id", entry.ID), zap.String("friendly_name", entry.FriendlyName))
	return nil
}

func (db *Database) UpdateProcessCatalogEntry(ctx context.Context, entry ProcessCatalogEntry) error {
	// ReplacingMergeTree replaces rows with same ORDER BY key on merge.
	// We INSERT a new version.
	query := `INSERT INTO monitoring.process_catalog_v2
                (id, friendly_name, process_names, window_title_patterns, category_id, is_active, created_at, updated_at)
                VALUES (?, ?, ?, ?, toUUID(?), ?, ?, ?)`

	var isActive uint8 = 0
	if entry.IsActive {
		isActive = 1
	}

	err := db.conn.Exec(ctx, query,
		entry.ID,
		entry.FriendlyName,
		entry.ProcessNames,
		entry.WindowTitlePatterns,
		entry.CategoryID,
		isActive,
		entry.CreatedAt,
		time.Now(),
	)

	if err != nil {
		zapctx.Error(ctx, "Failed to update process catalog entry", zap.Error(err), zap.String("id", entry.ID))
		return err
	}

	zapctx.Info(ctx, "Process catalog entry updated", zap.String("id", entry.ID))
	return nil
}

func (db *Database) GetProcessCatalog(ctx context.Context) ([]ProcessCatalogEntry, error) {
	query := `
                SELECT
                        toString(pc.id) as id,
                        pc.friendly_name,
                        pc.process_names,
                        pc.window_title_patterns,
                        toString(pc.category_id) as category_id,
                        toString(c.id) as cat_id,
                        c.key,
                        c.name,
                        c.color,
                        c.sort_order,
                        c.is_active,
                        pc.is_active,
                        pc.created_at,
                        pc.updated_at
                FROM monitoring.process_catalog_v2 pc
                LEFT JOIN monitoring.categories c ON c.id = pc.category_id
                WHERE pc.is_active = 1
                ORDER BY pc.friendly_name`

	rows, err := db.conn.Query(ctx, query)
	if err != nil {
		zapctx.Error(ctx, "Failed to query process catalog", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	entries := make([]ProcessCatalogEntry, 0)
	for rows.Next() {
		var entry ProcessCatalogEntry
		var cat CategoryType
		var catIsActive uint8
		var ruleIsActive uint8

		if err := rows.Scan(
			&entry.ID,
			&entry.FriendlyName,
			&entry.ProcessNames,
			&entry.WindowTitlePatterns,
			&entry.CategoryID,
			&cat.ID,
			&cat.Key,
			&cat.Name,
			&cat.Color,
			&cat.SortOrder,
			&catIsActive,
			&ruleIsActive,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		); err != nil {
			zapctx.Warn(ctx, "Failed to scan process catalog row", zap.Error(err))
			continue
		}

		entry.IsActive = ruleIsActive == 1
		if cat.Key != "" {
			cat.IsActive = catIsActive == 1
			entry.Category = &cat
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		zapctx.Error(ctx, "Error iterating process catalog rows", zap.Error(err))
		return nil, err
	}

	zapctx.Debug(ctx, "Process catalog fetched", zap.Int("count", len(entries)))
	return entries, nil
}

func (db *Database) DeleteProcessCatalogEntry(ctx context.Context, id string) error {
	existing, err := db.getProcessCatalogEntryByID(ctx, id)
	if err != nil {
		return err
	}

	existing.IsActive = false
	return db.UpdateProcessCatalogEntry(ctx, existing)
}

func (db *Database) Close() error {
	return db.conn.Close()
}

// Ping verifies that the ClickHouse connection is still alive. It is used by
// the backend /health handler to surface database degradation to upstream
// probes. Returns an error if the underlying connection is not initialized.
func (db *Database) Ping(ctx context.Context) error {
	if db == nil || db.conn == nil {
		return fmt.Errorf("database not initialized")
	}
	return db.conn.Ping(ctx)
}

type TableStats struct {
	ActivityEvents   uint64            `json:"activity_events"`
	ActivitySegments uint64            `json:"activity_segments"`
	USBEvents        uint64            `json:"usb_events"`
	FileEvents       uint64            `json:"file_events"`
	Screenshots      uint64            `json:"screenshots"`
	KeyboardEvents   uint64            `json:"keyboard_events"`
	UniqueUsers      []string          `json:"unique_users"`
	Errors           map[string]string `json:"errors,omitempty"`
	DatabaseExists   bool              `json:"database_exists"`
}

func (db *Database) GetTableStats(ctx context.Context) (*TableStats, error) {
	stats := &TableStats{
		Errors: make(map[string]string),
	}

	// Check if database exists
	var dbExists uint8
	dbCheckRow := db.conn.QueryRow(ctx, "SELECT 1 FROM system.databases WHERE name = 'monitoring'")
	if err := dbCheckRow.Scan(&dbExists); err != nil {
		stats.Errors["database"] = fmt.Sprintf("Database 'monitoring' not found: %v", err)
		stats.DatabaseExists = false
		return stats, nil
	}
	stats.DatabaseExists = true

	tables := []struct {
		name  string
		count *uint64
	}{
		{"activity_events", &stats.ActivityEvents},
		{"activity_segments", &stats.ActivitySegments},
		{"usb_events", &stats.USBEvents},
		{"file_copy_events", &stats.FileEvents},
		{"screenshot_metadata", &stats.Screenshots},
		{"keyboard_events", &stats.KeyboardEvents},
	}

	for _, t := range tables {
		query := fmt.Sprintf("SELECT count() FROM monitoring.%s", t.name)
		row := db.conn.QueryRow(ctx, query)
		if err := row.Scan(t.count); err != nil {
			*t.count = 0
			stats.Errors[t.name] = err.Error()
		}
	}

	users, err := db.GetUniqueUsernames(ctx)
	if err != nil {
		stats.Errors["unique_users"] = err.Error()
	}
	stats.UniqueUsers = users

	return stats, nil
}

// GetUniqueUsernames returns list of unique usernames from all activity tables
func (db *Database) GetUniqueUsernames(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT username FROM (
                SELECT DISTINCT username FROM monitoring.activity_segments 
                WHERE timestamp_start > now() - INTERVAL 90 DAY AND username != ''
                UNION ALL
                SELECT DISTINCT username FROM monitoring.activity_events 
                WHERE timestamp > now() - INTERVAL 90 DAY AND username != ''
        ) ORDER BY username`

	rows, err := db.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]string, 0)
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			continue
		}
		if username != "" {
			users = append(users, username)
		}
	}

	return users, rows.Err()
}

// GetActivitySegments retrieves activity segments for a computer on a specific date
func (db *Database) GetActivitySegments(ctx context.Context, computerName string, date time.Time) ([]ActivitySegment, error) {
	dateStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dateEnd := dateStart.Add(24 * time.Hour)

	query := `SELECT 
                timestamp_start, 
                timestamp_end, 
                duration_sec, 
                state, 
                computer_name, 
                username, 
                process_name, 
                window_title
        FROM monitoring.activity_segments
        WHERE computer_name = ? 
          AND timestamp_start >= ? 
          AND timestamp_start < ?
        ORDER BY timestamp_start ASC`

	rows, err := db.conn.Query(ctx, query, computerName, dateStart, dateEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Load process catalog for category matching
	processCatalog, _ := db.GetProcessCatalog(ctx)

	segments := make([]ActivitySegment, 0)
	for rows.Next() {
		var seg ActivitySegment
		if err := rows.Scan(
			&seg.TimestampStart,
			&seg.TimestampEnd,
			&seg.DurationSec,
			&seg.State,
			&seg.ComputerName,
			&seg.Username,
			&seg.ProcessName,
			&seg.WindowTitle,
		); err != nil {
			continue
		}
		// Set category based on state and process catalog
		if seg.State == "idle" || seg.State == "offline" {
			seg.Category = seg.State // Use state as category for non-active
		} else {
			seg.Category = matchProcessToCatalogInternal(seg.ProcessName, processCatalog)
		}
		segments = append(segments, seg)
	}

	return segments, rows.Err()
}
