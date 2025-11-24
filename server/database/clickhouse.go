package database

import (
        "context"
        "fmt"
        "time"

        "github.com/ClickHouse/clickhouse-go/v2"
        "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
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
        return db.conn.Exec(ctx, query,
                event.Timestamp, event.ComputerName, event.Username,
                event.WindowTitle, event.ProcessName, event.Duration)
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
        row.Scan(&summary.Username)

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
        
        return db.conn.Exec(ctx, query,
                entry.ID, entry.FriendlyName, entry.ProcessNames, entry.WindowTitlePatterns,
                entry.Category, entry.IsActive, entry.CreatedAt, entry.UpdatedAt)
}

func (db *Database) UpdateProcessCatalogEntry(ctx context.Context, entry ProcessCatalogEntry) error {
        query := `ALTER TABLE monitoring.process_catalog UPDATE
                friendly_name = ?,
                process_names = ?,
                window_title_patterns = ?,
                category = ?,
                is_active = ?,
                updated_at = ?
                WHERE id = ?`
        
        return db.conn.Exec(ctx, query,
                entry.FriendlyName, entry.ProcessNames, entry.WindowTitlePatterns,
                entry.Category, entry.IsActive, entry.UpdatedAt, entry.ID)
}

func (db *Database) GetProcessCatalog(ctx context.Context) ([]ProcessCatalogEntry, error) {
        query := `SELECT id, friendly_name, process_names, window_title_patterns, category, is_active, created_at, updated_at
                FROM monitoring.process_catalog FINAL
                WHERE is_active = 1
                ORDER BY friendly_name`

        rows, err := db.conn.Query(ctx, query)
        if err != nil {
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
                        continue
                }
                entry.IsActive = isActive == 1
                entries = append(entries, entry)
        }

        return entries, rows.Err()
}

func (db *Database) DeleteProcessCatalogEntry(ctx context.Context, id string) error {
        query := `ALTER TABLE monitoring.process_catalog UPDATE is_active = 0, updated_at = now() WHERE id = ?`
        return db.conn.Exec(ctx, query, id)
}

func (db *Database) Close() error {
        return db.conn.Close()
}
