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
	if err := db.AutoSyncApplicationCategoriesTable(ctx); err != nil {
		zapctx.Warn(ctx, "Failed to auto-sync application_categories table", zap.Error(err))
		// Don't fail startup - table might be created by migrations
	}

	// Auto-sync process_catalog table (used by /api/process-catalog)
	if err := db.AutoSyncProcessCatalogTable(ctx); err != nil {
		zapctx.Warn(ctx, "Failed to auto-sync process_catalog table", zap.Error(err))
		// Don't fail startup - table might be created by migrations
	}

	// Auto-load default categories if table is empty
	if err := db.AutoLoadDefaultCategories(ctx); err != nil {
		zapctx.Warn(ctx, "Failed to auto-load default categories", zap.Error(err))
		// Don't fail startup - user can add categories manually
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
		event.FileCount, event.OperationType, event.IsUSBTarget)
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
	query := `SELECT computer_name, username, MAX(timestamp) as last_seen
                FROM monitoring.activity_events
                WHERE timestamp > now() - INTERVAL 1 HOUR
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
		if err := rows.Scan(&e.Timestamp, &e.ComputerName, &e.Username, &e.SourcePath, &e.DestinationPath, &e.FileSize, &e.FileCount, &e.OperationType, &e.IsUSBTarget); err != nil {
			continue
		}
		events = append(events, e)
	}

	return events, nil
}

func (db *Database) InsertActivitySegment(ctx context.Context, segment ActivitySegment) error {
	query := `INSERT INTO monitoring.activity_segments
                (timestamp_start, timestamp_end, duration_sec, state, computer_name, username, process_name, window_title, session_id)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	return db.conn.Exec(ctx, query,
		segment.TimestampStart, segment.TimestampEnd, segment.DurationSec,
		segment.State, segment.ComputerName, segment.Username,
		segment.ProcessName, segment.WindowTitle, segment.SessionID)
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

func (db *Database) CreateProcessCatalogEntry(ctx context.Context, entry ProcessCatalogEntry) error {
	query := `INSERT INTO monitoring.process_catalog
                (id, friendly_name, process_names, window_title_patterns, category, is_active, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	// Convert bool to UInt8 for ClickHouse
	var isActive uint8 = 0
	if entry.IsActive {
		isActive = 1
	}

	err := db.conn.Exec(ctx, query,
		entry.ID, entry.FriendlyName, entry.ProcessNames, entry.WindowTitlePatterns,
		entry.Category, isActive, entry.CreatedAt, entry.UpdatedAt)

	if err != nil {
		zapctx.Error(ctx, "Failed to create process catalog entry", zap.Error(err), zap.String("friendly_name", entry.FriendlyName))
		return err
	}

	zapctx.Info(ctx, "Process catalog entry created", zap.String("id", entry.ID), zap.String("friendly_name", entry.FriendlyName))
	return nil
}

func (db *Database) UpdateProcessCatalogEntry(ctx context.Context, entry ProcessCatalogEntry) error {
	// ReplacingMergeTree replaces rows with same ORDER BY key on merge
	// We INSERT new row with updated_at, and it will replace the old one
	query := `INSERT INTO monitoring.process_catalog
                (id, friendly_name, process_names, window_title_patterns, category, is_active, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	// Convert bool to UInt8 for ClickHouse
	var isActive uint8 = 0
	if entry.IsActive {
		isActive = 1
	}

	err := db.conn.Exec(ctx, query,
		entry.ID, entry.FriendlyName, entry.ProcessNames, entry.WindowTitlePatterns,
		entry.Category, isActive, entry.CreatedAt, time.Now())

	if err != nil {
		zapctx.Error(ctx, "Failed to update process catalog entry", zap.Error(err), zap.String("id", entry.ID))
		return err
	}

	zapctx.Info(ctx, "Process catalog entry updated", zap.String("id", entry.ID))
	return nil
}

func (db *Database) GetProcessCatalog(ctx context.Context) ([]ProcessCatalogEntry, error) {
	query := `SELECT id, friendly_name, process_names, window_title_patterns, category, is_active, created_at, updated_at
                FROM monitoring.process_catalog FINAL
                WHERE is_active = 1
                ORDER BY friendly_name`

	rows, err := db.conn.Query(ctx, query)
	if err != nil {
		zapctx.Error(ctx, "Failed to query process catalog", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	entries := make([]ProcessCatalogEntry, 0)
	for rows.Next() {
		var entry ProcessCatalogEntry
		var isActive uint8
		if err := rows.Scan(&entry.ID, &entry.FriendlyName, &entry.ProcessNames,
			&entry.WindowTitlePatterns, &entry.Category, &isActive,
			&entry.CreatedAt, &entry.UpdatedAt); err != nil {
			zapctx.Warn(ctx, "Failed to scan process catalog row", zap.Error(err))
			continue
		}
		entry.IsActive = isActive == 1
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
	query := `ALTER TABLE monitoring.process_catalog UPDATE is_active = 0, updated_at = now() WHERE id = ?`
	return db.conn.Exec(ctx, query, id)
}

func (db *Database) Close() error {
	return db.conn.Close()
}

// GetUniqueUsernames returns list of unique usernames from activity segments
func (db *Database) GetUniqueUsernames(ctx context.Context) ([]string, error) {
	// Use activity_segments as primary source (more up-to-date)
	// Fall back to activity_events if needed
	query := `SELECT DISTINCT username 
                  FROM monitoring.activity_segments 
                  WHERE timestamp_start > now() - INTERVAL 7 DAY
                  ORDER BY username`

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
		users = append(users, username)
	}

	// If no users found in segments, try activity_events as fallback
	if len(users) == 0 {
		fallbackQuery := `SELECT DISTINCT username 
                          FROM monitoring.activity_events 
                          WHERE timestamp > now() - INTERVAL 7 DAY
                          ORDER BY username`

		fallbackRows, err := db.conn.Query(ctx, fallbackQuery)
		if err != nil {
			return users, nil // Return empty list, don't fail
		}
		defer fallbackRows.Close()

		for fallbackRows.Next() {
			var username string
			if err := fallbackRows.Scan(&username); err != nil {
				continue
			}
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
		segments = append(segments, seg)
	}

	return segments, rows.Err()
}
