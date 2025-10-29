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

func New(host string, port int, database, username, password string) (*Database, error) {
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

	return &Database{conn: conn}, nil
}

func (db *Database) InsertActivityEvent(ctx context.Context, event ActivityEvent) error {
	query := `INSERT INTO monitoring.activity_events 
                (timestamp, computer_name, username, window_title, process_name, duration)
                VALUES (?, ?, ?, ?, ?, ?)`

	start := time.Now()
	err := db.conn.Exec(ctx, query,
		event.Timestamp, event.ComputerName, event.Username,
		event.WindowTitle, event.ProcessName, event.Duration)

	duration := time.Since(start)
	if err != nil {
		zapctx.Error(ctx, "Failed to insert activity event to ClickHouse",
			zap.Error(err),
			zap.Duration("duration", duration),
			zap.String("computer_name", event.ComputerName),
		)
		return err
	}

	if duration > 100*time.Millisecond {
		zapctx.Warn(ctx, "Slow INSERT query detected",
			zap.Duration("duration", duration),
			zap.String("table", "activity_events"),
		)
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

	start := time.Now()
	rows, err := db.conn.Query(ctx, query)
	if err != nil {
		zapctx.Error(ctx, "Failed to query active employees",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)),
		)
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

	duration := time.Since(start)
	if duration > 200*time.Millisecond {
		zapctx.Warn(ctx, "Slow SELECT query detected",
			zap.Duration("duration", duration),
			zap.String("table", "activity_events"),
			zap.Int("result_count", len(employees)),
		)
	}

	return employees, rows.Err()
}

func (db *Database) GetRecentActivity(ctx context.Context, limit int) ([]ActivityEvent, error) {
	query := `SELECT timestamp, computer_name, username, window_title, process_name, duration
                FROM monitoring.activity_events
                ORDER BY timestamp DESC
                LIMIT ?`

	start := time.Now()
	rows, err := db.conn.Query(ctx, query, limit)
	if err != nil {
		zapctx.Error(ctx, "Failed to query recent activity",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)),
			zap.Int("limit", limit),
		)
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

	duration := time.Since(start)
	if duration > 200*time.Millisecond {
		zapctx.Warn(ctx, "Slow SELECT query detected",
			zap.Duration("duration", duration),
			zap.String("table", "activity_events"),
			zap.Int("limit", limit),
			zap.Int("result_count", len(events)),
		)
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

func (db *Database) Close() error {
	return db.conn.Close()
}
