package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ctolnik/Office-Monitor/zapctx"
	"go.uber.org/zap"
)

// GetAgents returns all agents with their status
func (db *Database) GetAgents(ctx context.Context) ([]Agent, error) {
	query := `
                SELECT 
                        computer_name,
                        username,
                        MAX(timestamp) as last_seen,
                        '' as ip_address,
                        '' as os_version,
                        '' as agent_version
                FROM monitoring.activity_events
                WHERE timestamp > now() - INTERVAL 1 DAY
                GROUP BY computer_name, username
                ORDER BY last_seen DESC`

	rows, err := db.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agents := make([]Agent, 0)
	for rows.Next() {
		var a Agent
		var lastSeen time.Time
		if err := rows.Scan(&a.ComputerName, &a.Username, &lastSeen, &a.IPAddress, &a.OSVersion, &a.AgentVersion); err != nil {
			continue
		}

		a.LastSeen = lastSeen.Format(time.RFC3339)

		// Determine status based on last seen
		minutesSince := int(time.Since(lastSeen).Minutes())
		if minutesSince < 5 {
			a.Status = "online"
		} else if minutesSince < 30 {
			a.Status = "idle"
		} else {
			a.Status = "offline"
		}

		// Default config
		a.Config = ConfigUpdate{
			ScreenshotInterval: 60,
			ActivityTracking:   true,
			KeyloggerEnabled:   false,
			USBMonitoring:      true,
			FileMonitoring:     true,
			DLPEnabled:         true,
		}

		agents = append(agents, a)
	}

	return agents, rows.Err()
}

// GetAgentConfig returns configuration for specific agent
func (db *Database) GetAgentConfig(ctx context.Context, computerName string) (*ConfigUpdate, error) {
	query := `
                SELECT 
                        screenshot_interval_minutes,
                        screenshot_enabled,
                        keylogger_enabled,
                        usb_monitoring_enabled,
                        file_copy_monitoring_enabled
                FROM monitoring.agent_configs
                WHERE computer_name = ?
                LIMIT 1`

	var config ConfigUpdate
	var screenshotMin int
	var activityEnabled, keyloggerEnabled, usbEnabled, fileEnabled bool

	err := db.conn.QueryRow(ctx, query, computerName).Scan(
		&screenshotMin, &activityEnabled, &keyloggerEnabled, &usbEnabled, &fileEnabled,
	)

	if err != nil {
		// Return defaults if not found
		return &ConfigUpdate{
			ScreenshotInterval: 60,
			ActivityTracking:   true,
			KeyloggerEnabled:   false,
			USBMonitoring:      true,
			FileMonitoring:     true,
			DLPEnabled:         true,
		}, nil
	}

	config.ScreenshotInterval = screenshotMin * 60
	config.ActivityTracking = activityEnabled
	config.KeyloggerEnabled = keyloggerEnabled
	config.USBMonitoring = usbEnabled
	config.FileMonitoring = fileEnabled
	config.DLPEnabled = fileEnabled

	return &config, nil
}

// UpdateAgentConfig updates agent configuration
func (db *Database) UpdateAgentConfig(ctx context.Context, computerName string, config ConfigUpdate) error {
	query := `
                INSERT INTO monitoring.agent_configs 
                        (computer_name, screenshot_enabled, screenshot_interval_minutes, keylogger_enabled, 
                         usb_monitoring_enabled, file_copy_monitoring_enabled, api_key, last_seen, agent_version)
                VALUES (?, ?, ?, ?, ?, ?, ?, now(), '')
                `

	screenshotMin := config.ScreenshotInterval / 60
	if screenshotMin < 1 {
		screenshotMin = 1
	}

	return db.conn.Exec(ctx, query,
		computerName,
		config.ActivityTracking,
		screenshotMin,
		config.KeyloggerEnabled,
		config.USBMonitoring,
		config.FileMonitoring,
		"default-key",
	)
}

// DeleteAgent removes agent configuration
func (db *Database) DeleteAgent(ctx context.Context, computerName string) error {
	query := `ALTER TABLE monitoring.agent_configs DELETE WHERE computer_name = ?`
	return db.conn.Exec(ctx, query, computerName)
}

// GetAllUsers returns all unique users from activity_events
func (db *Database) GetAllUsers(ctx context.Context) ([]string, error) {
	query := `
                SELECT DISTINCT username
                FROM monitoring.activity_events
                WHERE timestamp > now() - INTERVAL 30 DAY
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

	return users, rows.Err()
}

// GetAllEmployees returns all employees from employees table
func (db *Database) GetAllEmployees(ctx context.Context) ([]EmployeeFull, error) {
	query := `
                SELECT 
                        username,
                        full_name,
                        department,
                        position,
                        email,
                        consent_given,
                        consent_date,
                        created_at,
                        is_active
                FROM monitoring.employees
                ORDER BY created_at DESC`

	rows, err := db.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	employees := make([]EmployeeFull, 0)
	for rows.Next() {
		var e EmployeeFull
		var consentDate *time.Time
		var createdAt time.Time

		if err := rows.Scan(&e.Username, &e.FullName, &e.Department, &e.Position,
			&e.Email, &e.ConsentGiven, &consentDate, &createdAt, &e.IsActive); err != nil {
			continue
		}

		e.ID = e.Username // Use username as ID
		e.CreatedAt = createdAt.Format(time.RFC3339)
		if consentDate != nil {
			dateStr := consentDate.Format(time.RFC3339)
			e.ConsentDate = &dateStr
		}

		employees = append(employees, e)
	}

	return employees, rows.Err()
}

// CreateEmployee creates a new employee
func (db *Database) CreateEmployee(ctx context.Context, emp EmployeeFull) error {
	query := `
                INSERT INTO monitoring.employees 
                        (username, full_name, department, position, email, consent_given, consent_date, created_at, is_active)
                VALUES (?, ?, ?, ?, ?, ?, ?, now(), ?)`

	var consentDate *time.Time
	if emp.ConsentDate != nil {
		t, _ := time.Parse(time.RFC3339, *emp.ConsentDate)
		consentDate = &t
	}

	return db.conn.Exec(ctx, query,
		emp.Username, emp.FullName, emp.Department, emp.Position,
		emp.Email, emp.ConsentGiven, consentDate, emp.IsActive,
	)
}

// UpdateEmployee updates existing employee
func (db *Database) UpdateEmployee(ctx context.Context, username string, emp EmployeeFull) error {
	query := `
                ALTER TABLE monitoring.employees 
                UPDATE 
                        full_name = ?,
                        department = ?,
                        position = ?,
                        email = ?,
                        consent_given = ?,
                        consent_date = ?,
                        is_active = ?
                WHERE username = ?`

	var consentDate *time.Time
	if emp.ConsentDate != nil {
		t, _ := time.Parse(time.RFC3339, *emp.ConsentDate)
		consentDate = &t
	}

	return db.conn.Exec(ctx, query,
		emp.FullName, emp.Department, emp.Position, emp.Email,
		emp.ConsentGiven, consentDate, emp.IsActive, username,
	)
}

// DeleteEmployee removes employee
func (db *Database) DeleteEmployee(ctx context.Context, username string) error {
	query := `ALTER TABLE monitoring.employees DELETE WHERE username = ?`
	return db.conn.Exec(ctx, query, username)
}

// GetDashboardStats returns dashboard statistics
func (db *Database) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats := &DashboardStats{}

	// Calculate time thresholds in Go
	now := time.Now()
	weekAgo := now.Add(-7 * 24 * time.Hour)
	fiveMinAgo := now.Add(-5 * time.Minute)

	// Total employees (unique usernames in last 7 days)
	err := db.conn.QueryRow(ctx, `
                SELECT count(DISTINCT username) 
                FROM monitoring.activity_events 
                WHERE timestamp > ?`, weekAgo).Scan(&stats.TotalEmployees)
	if err != nil {
		zapctx.Warn(ctx, "Failed to get total employees", zap.Error(err))
	}

	// Active now (last 5 minutes)
	err = db.conn.QueryRow(ctx, `
                SELECT count(DISTINCT username) 
                FROM monitoring.activity_events 
                WHERE timestamp > ?`, fiveMinAgo).Scan(&stats.ActiveNow)
	if err != nil {
		zapctx.Warn(ctx, "Failed to get active now", zap.Error(err))
	}

	// Offline (total - active)
	stats.Offline = stats.TotalEmployees - stats.ActiveNow

	// Start of today
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Total alerts today
	err = db.conn.QueryRow(ctx, `
                SELECT count(*) 
                FROM monitoring.alerts 
                WHERE timestamp >= ?`, todayStart).Scan(&stats.TotalAlerts)
	if err != nil {
		stats.TotalAlerts = 0
	}

	// Unresolved alerts
	err = db.conn.QueryRow(ctx, `
                SELECT count(*) 
                FROM monitoring.alerts 
                WHERE is_acknowledged = 0`).Scan(&stats.UnresolvedAlerts)
	if err != nil {
		stats.UnresolvedAlerts = 0
	}

	// Screenshots today
	err = db.conn.QueryRow(ctx, `
                SELECT count(*) 
                FROM monitoring.screenshot_metadata 
                WHERE timestamp >= ?`, todayStart).Scan(&stats.TodayScreenshots)
	if err != nil {
		stats.TodayScreenshots = 0
	}

	// USB events today
	err = db.conn.QueryRow(ctx, `
                SELECT count(*) 
                FROM monitoring.usb_events 
                WHERE timestamp >= ?`, todayStart).Scan(&stats.TodayUSBEvents)
	if err != nil {
		stats.TodayUSBEvents = 0
	}

	// File events today
	err = db.conn.QueryRow(ctx, `
                SELECT count(*) 
                FROM monitoring.file_copy_events 
                WHERE timestamp >= ?`, todayStart).Scan(&stats.TodayFileEvents)
	if err != nil {
		stats.TodayFileEvents = 0
	}

	// Average productivity calculation
	// Get all active employees from last 7 days
	usernames := make([]string, 0)
	userQuery := `SELECT DISTINCT username FROM monitoring.activity_events WHERE timestamp > ?`
	userRows, err := db.conn.Query(ctx, userQuery, weekAgo)
	if err == nil {
		defer userRows.Close()
		for userRows.Next() {
			var username string
			if err := userRows.Scan(&username); err == nil {
				usernames = append(usernames, username)
			}
		}
	}

	// Calculate average productivity across all users
	if len(usernames) > 0 {
		totalProductivity := 0.0
		for _, username := range usernames {
			prod, err := db.CalculateProductivity(ctx, username, weekAgo, now)
			if err == nil {
				totalProductivity += prod
			}
		}
		stats.AvgProductivity = totalProductivity / float64(len(usernames))
	} else {
		stats.AvgProductivity = 0.0
	}

	return stats, nil
}

// categorizeApplication determines app category based on process name and window title
func categorizeApplication(processName, windowTitle string) string {
	processLower := strings.ToLower(processName)
	titleLower := strings.ToLower(windowTitle)

	// Productive applications
	productive := []string{
		"code.exe", "idea", "pycharm", "goland", "webstorm", "rider", "clion",
		"visualstudio.exe", "devenv.exe",
		"notepad++.exe", "sublime", "atom.exe", "vim", "nano",
		"datagrip", "dbeaver", "mysql", "postgres", "ssms.exe",
		"docker", "postman", "insomnia",
		"git", "fork.exe", "gitkraken",
		"terminal", "cmd.exe", "powershell.exe", "bash", "mobaxterm.exe", "putty.exe",
		"1cv8c.exe", "1cv8.exe", // 1C Development
		"excel.exe", "winword.exe", "powerpnt.exe", // Office work
		"notion", "obsidian", "onenote",
	}

	// Communication apps
	communication := []string{
		"slack.exe", "teams.exe", "zoom.exe", "skype.exe",
		"discord.exe", "telegram.exe", "whatsapp.exe",
		"outlook.exe", "thunderbird.exe",
	}

	// Unproductive / Entertainment
	unproductive := []string{
		"steam.exe", "epicgameslauncher.exe", "origin.exe",
		"spotify.exe", "vlc.exe",
		"netflix", "twitch",
	}

	// Check productive
	for _, app := range productive {
		if strings.Contains(processLower, app) {
			return "productive"
		}
	}

	// Check communication
	for _, app := range communication {
		if strings.Contains(processLower, app) {
			return "communication"
		}
	}

	// Check unproductive
	for _, app := range unproductive {
		if strings.Contains(processLower, app) {
			return "unproductive"
		}
	}

	// Chrome/Firefox special handling - categorize by site
	if strings.Contains(processLower, "chrome.exe") || strings.Contains(processLower, "firefox.exe") || strings.Contains(processLower, "msedge.exe") {
		// Productive sites
		if strings.Contains(titleLower, "github") || strings.Contains(titleLower, "stackoverflow") ||
			strings.Contains(titleLower, "gitlab") || strings.Contains(titleLower, "documentation") ||
			strings.Contains(titleLower, "aws") || strings.Contains(titleLower, "azure") ||
			strings.Contains(titleLower, "google cloud") || strings.Contains(titleLower, "jenkins") ||
			strings.Contains(titleLower, "confluence") || strings.Contains(titleLower, "jira") {
			return "productive"
		}

		// Social media / entertainment
		if strings.Contains(titleLower, "youtube") || strings.Contains(titleLower, "facebook") ||
			strings.Contains(titleLower, "twitter") || strings.Contains(titleLower, "reddit") ||
			strings.Contains(titleLower, "instagram") || strings.Contains(titleLower, "netflix") {
			return "unproductive"
		}
	}

	// System processes
	if strings.Contains(processLower, "explorer.exe") || strings.Contains(processLower, "taskmgr.exe") ||
		strings.Contains(processLower, "system") {
		return "system"
	}

	return "neutral"
}

// GetApplicationUsage returns application usage statistics
func (db *Database) GetApplicationUsage(ctx context.Context, username string, start, end time.Time) ([]ApplicationUsage, error) {
	// Format timestamps as strings without timezone to match ClickHouse local time
	startStr := start.Format("2006-01-02 15:04:05")
	endStr := end.Format("2006-01-02 15:04:05")

	zapctx.Debug(ctx, "GetApplicationUsage called",
		zap.String("username", username),
		zap.String("start", startStr),
		zap.String("end", endStr))

	// Use direct string interpolation for dates to avoid driver parameter conversion issues
	// This is safe since timestamps are formatted from time.Time, not user input
	query := fmt.Sprintf(`
                SELECT 
                        process_name,
                        window_title,
                        sum(duration) as total_duration,
                        count(*) as count
                FROM monitoring.activity_events
                WHERE username = ? 
                  AND timestamp >= toDateTime64('%s', 3)
                  AND timestamp < toDateTime64('%s', 3)
                GROUP BY process_name, window_title
                ORDER BY total_duration DESC
                LIMIT 50`, startStr, endStr)

	zapctx.Debug(ctx, "Executing query", zap.String("query", query))

	rows, err := db.conn.Query(ctx, query, username)
	if err != nil {
		zapctx.Error(ctx, "Query failed", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	apps := make([]ApplicationUsage, 0)
	var totalDuration uint64

	// First pass: collect data and calculate total
	tempApps := make([]ApplicationUsage, 0)
	for rows.Next() {
		var app ApplicationUsage
		if err := rows.Scan(&app.ProcessName, &app.WindowTitle, &app.Duration, &app.Count); err != nil {
			zapctx.Error(ctx, "Failed to scan row", zap.Error(err))
			continue
		}
		app.Category = categorizeApplication(app.ProcessName, app.WindowTitle)
		totalDuration += app.Duration
		tempApps = append(tempApps, app)
	}

	if err := rows.Err(); err != nil {
		zapctx.Error(ctx, "Error iterating rows", zap.Error(err))
		return nil, err
	}

	// Second pass: calculate percentages
	for _, app := range tempApps {
		if totalDuration > 0 {
			app.Percentage = float64(app.Duration) / float64(totalDuration) * 100
		}
		apps = append(apps, app)
	}

	zapctx.Debug(ctx, "Query completed",
		zap.Int("result_count", len(apps)),
		zap.Uint64("total_duration", totalDuration))

	return apps, nil
}

// GetAlerts returns alerts with optional filtering
func (db *Database) GetAlerts(ctx context.Context, resolved *bool, severity string, limit, offset int) ([]AlertFull, error) {
	query := `
                SELECT 
                        toString(timestamp) as id,
                        timestamp,
                        computer_name,
                        username,
                        alert_type,
                        severity,
                        description,
                        metadata as details,
                        is_acknowledged as is_resolved
                FROM monitoring.alerts
                WHERE 1=1`

	args := make([]interface{}, 0)

	if resolved != nil {
		query += " AND is_acknowledged = ?"
		args = append(args, *resolved)
	}

	if severity != "" {
		query += " AND severity = ?"
		args = append(args, severity)
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	alerts := make([]AlertFull, 0)
	for rows.Next() {
		var a AlertFull
		var ts time.Time
		if err := rows.Scan(&a.ID, &ts, &a.ComputerName, &a.Username,
			&a.AlertType, &a.Severity, &a.Description, &a.Details, &a.IsResolved); err != nil {
			zapctx.Error(ctx, "Failed to scan alert row", zap.Error(err))
			continue
		}
		a.Timestamp = ts.Format(time.RFC3339)
		alerts = append(alerts, a)
	}

	if err := rows.Err(); err != nil {
		zapctx.Error(ctx, "Error iterating alert rows", zap.Error(err))
		return nil, err
	}

	return alerts, nil
}

// ResolveAlert marks alert as resolved
func (db *Database) ResolveAlert(ctx context.Context, alertID string, resolvedBy string) error {
	query := `
                ALTER TABLE monitoring.alerts 
                UPDATE is_acknowledged = true
                WHERE toString(timestamp) = ?`

	return db.conn.Exec(ctx, query, alertID)
}

// GetUSBEventsByUsername returns USB events for user in time range
func (db *Database) GetUSBEventsByUsername(ctx context.Context, username string, start, end time.Time) ([]USBEvent, error) {
	query := `
                SELECT timestamp, computer_name, username, device_id, device_name, device_type, event_type, volume_serial
                FROM monitoring.usb_events
                WHERE username = ? AND timestamp >= ? AND timestamp <= ?
                ORDER BY timestamp DESC`

	rows, err := db.conn.Query(ctx, query, username, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]USBEvent, 0)
	for rows.Next() {
		var e USBEvent
		if err := rows.Scan(&e.Timestamp, &e.ComputerName, &e.Username, &e.DeviceID,
			&e.DeviceName, &e.DeviceType, &e.EventType, &e.VolumeSerial); err != nil {
			zapctx.Error(ctx, "Failed to scan USB event row", zap.Error(err))
			continue
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		zapctx.Error(ctx, "Error iterating USB event rows", zap.Error(err))
		return nil, err
	}

	return events, nil
}

// GetFileEventsByUsername returns file events for user in time range
func (db *Database) GetFileEventsByUsername(ctx context.Context, username string, start, end time.Time) ([]FileCopyEvent, error) {
	query := `
                SELECT timestamp, computer_name, username, source_path, destination_path, 
                           file_size, file_count, operation_type, is_usb_target
                FROM monitoring.file_copy_events
                WHERE username = ? AND timestamp >= ? AND timestamp <= ?
                ORDER BY timestamp DESC`

	rows, err := db.conn.Query(ctx, query, username, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]FileCopyEvent, 0)
	for rows.Next() {
		var e FileCopyEvent
		if err := rows.Scan(&e.Timestamp, &e.ComputerName, &e.Username, &e.SourcePath,
			&e.DestinationPath, &e.FileSize, &e.FileCount, &e.OperationType, &e.IsUSBTarget); err != nil {
			zapctx.Error(ctx, "Failed to scan file event row", zap.Error(err))
			continue
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		zapctx.Error(ctx, "Error iterating file event rows", zap.Error(err))
		return nil, err
	}

	return events, nil
}

// GetScreenshotsByUsername returns screenshots for user in time range
func (db *Database) GetScreenshotsByUsername(ctx context.Context, username string, start, end time.Time) ([]ScreenshotMetadata, error) {
	query := `
                SELECT timestamp, computer_name, username, screenshot_id, minio_path, 
                           file_size, window_title, process_name
                FROM monitoring.screenshot_metadata
                WHERE username = ? AND timestamp >= ? AND timestamp <= ?
                ORDER BY timestamp DESC`

	rows, err := db.conn.Query(ctx, query, username, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	screenshots := make([]ScreenshotMetadata, 0)
	for rows.Next() {
		var s ScreenshotMetadata
		if err := rows.Scan(&s.Timestamp, &s.ComputerName, &s.Username, &s.ScreenshotID,
			&s.MinIOPath, &s.FileSize, &s.WindowTitle, &s.ProcessName); err != nil {
			zapctx.Error(ctx, "Failed to scan screenshot row", zap.Error(err))
			continue
		}
		screenshots = append(screenshots, s)
	}

	if err := rows.Err(); err != nil {
		zapctx.Error(ctx, "Error iterating screenshot rows", zap.Error(err))
		return nil, err
	}

	return screenshots, nil
}

// GetActivityEventsByUsername returns activity events for user in time range
func (db *Database) GetActivityEventsByUsername(ctx context.Context, username string, start, end time.Time) ([]ActivityEvent, error) {
	startStr := start.Format("2006-01-02 15:04:05")
	endStr := end.Format("2006-01-02 15:04:05")

	query := fmt.Sprintf(`
                SELECT timestamp, computer_name, username, window_title, process_name, process_path, duration, idle_time
                FROM monitoring.activity_events
                WHERE username = ? 
                  AND timestamp >= toDateTime64('%s', 3)
                  AND timestamp < toDateTime64('%s', 3)
                ORDER BY timestamp ASC
                LIMIT 10000`, startStr, endStr)

	rows, err := db.conn.Query(ctx, query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]ActivityEvent, 0)
	for rows.Next() {
		var e ActivityEvent
		if err := rows.Scan(&e.Timestamp, &e.ComputerName, &e.Username,
			&e.WindowTitle, &e.ProcessName, &e.ProcessPath,
			&e.Duration, &e.IdleTime); err != nil {
			zapctx.Error(ctx, "Failed to scan activity event row", zap.Error(err))
			continue
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		zapctx.Error(ctx, "Error iterating activity event rows", zap.Error(err))
		return nil, err
	}

	// Enrich events with application categories
	for i := range events {
		category, err := db.MatchProcessToCategory(ctx, events[i].ProcessName, events[i].WindowTitle)
		if err == nil {
			events[i].Category = category
		}
	}

	return events, nil
}

// GetKeyboardEventsByUsername returns keyboard events for user in time range
func (db *Database) GetKeyboardEventsByUsername(ctx context.Context, username string, start, end time.Time) ([]KeyboardEvent, error) {
	query := `
                SELECT timestamp, computer_name, username, window_title, process_name, text_content
                FROM monitoring.keyboard_events
                WHERE username = ? AND timestamp >= ? AND timestamp <= ?
                ORDER BY timestamp DESC`

	rows, err := db.conn.Query(ctx, query, username, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]KeyboardEvent, 0)
	for rows.Next() {
		var e KeyboardEvent
		if err := rows.Scan(&e.Timestamp, &e.ComputerName, &e.Username,
			&e.WindowTitle, &e.ProcessName, &e.TextContent); err != nil {
			zapctx.Error(ctx, "Failed to scan keyboard event row", zap.Error(err))
			continue
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		zapctx.Error(ctx, "Error iterating keyboard event rows", zap.Error(err))
		return nil, err
	}

	return events, nil
}

// GetDailyReport generates a complete daily report for a user
func (db *Database) GetDailyReport(ctx context.Context, username string, date time.Time) (*DailyReport, error) {
	report := &DailyReport{
		Username: username,
		Date:     date.Format("2006-01-02"),
	}

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	// Get activity events (for timeline)
	activityEvents, err := db.GetActivityEventsByUsername(ctx, username, startOfDay, endOfDay)
	if err != nil {
		zapctx.Warn(ctx, "Failed to get activity events", zap.Error(err))
		activityEvents = make([]ActivityEvent, 0)
	}
	report.ActivityEvents = activityEvents

	// Get applications
	apps, err := db.GetApplicationUsage(ctx, username, startOfDay, endOfDay)
	if err != nil {
		zapctx.Warn(ctx, "Failed to get applications", zap.Error(err))
		apps = make([]ApplicationUsage, 0)
	}
	report.Applications = apps

	// Get screenshots
	screenshots, err := db.GetScreenshotsByUsername(ctx, username, startOfDay, endOfDay)
	if err != nil {
		zapctx.Warn(ctx, "Failed to get screenshots", zap.Error(err))
		screenshots = make([]ScreenshotMetadata, 0)
	}
	report.Screenshots = screenshots

	// Get USB events
	usbEvents, err := db.GetUSBEventsByUsername(ctx, username, startOfDay, endOfDay)
	if err != nil {
		zapctx.Warn(ctx, "Failed to get USB events", zap.Error(err))
		usbEvents = make([]USBEvent, 0)
	}
	report.USBEvents = usbEvents

	// Get file events
	fileEvents, err := db.GetFileEventsByUsername(ctx, username, startOfDay, endOfDay)
	if err != nil {
		zapctx.Warn(ctx, "Failed to get file events", zap.Error(err))
		fileEvents = make([]FileCopyEvent, 0)
	}
	report.FileEvents = fileEvents

	// Get keyboard events and format as periods
	keyboardEvents, err := db.GetKeyboardEventsByUsername(ctx, username, startOfDay, endOfDay)
	if err != nil {
		zapctx.Warn(ctx, "Failed to get keyboard events", zap.Error(err))
	} else {
		// Group keyboard events into periods
		periods := make([]KeyboardPeriod, 0)
		for _, ke := range keyboardEvents {
			period := KeyboardPeriod{
				Start:         ke.Timestamp.Format(time.RFC3339),
				End:           ke.Timestamp.Add(5 * time.Minute).Format(time.RFC3339),
				Application:   ke.ProcessName,
				WindowTitle:   ke.WindowTitle,
				FormattedText: ke.TextContent,
				RawKeys:       "[]", // Would need actual key events JSON
			}
			periods = append(periods, period)
		}
		report.KeyboardPeriods = periods
	}

	// Get DLP alerts
	falseVal := false
	dlpAlerts, err := db.GetAlerts(ctx, &falseVal, "critical", 100, 0)
	if err != nil {
		zapctx.Warn(ctx, "Failed to get DLP alerts", zap.Error(err))
	} else {
		// Filter alerts for this user and date
		alerts := make([]AlertFull, 0)
		for _, a := range dlpAlerts {
			if a.Username == username && a.Timestamp >= startOfDay.Format(time.RFC3339) && a.Timestamp < endOfDay.Format(time.RFC3339) {
				alerts = append(alerts, a)
			}
		}
		report.DLPAlerts = alerts
	}

	// Calculate activity summary
	var totalDuration uint64
	var productiveTime, unproductiveTime, neutralTime uint64

	// Calculate time by category
	for _, app := range report.Applications {
		totalDuration += app.Duration
		switch app.Category {
		case "productive":
			productiveTime += app.Duration
		case "unproductive":
			unproductiveTime += app.Duration
		default:
			neutralTime += app.Duration
		}
	}

	// Calculate productivity score: (productive - unproductive) / total * 100
	// Range: 0-100, where 50 is neutral
	var productivityScore float64
	if totalDuration > 0 {
		productivityScore = (float64(productiveTime) / float64(totalDuration)) * 100
	} else {
		productivityScore = 0.0
	}

	// Get actual first and last activity timestamps
	firstActivity := startOfDay.Format(time.RFC3339)
	lastActivity := endOfDay.Format(time.RFC3339)
	if len(screenshots) > 0 {
		firstActivity = screenshots[len(screenshots)-1].Timestamp.Format(time.RFC3339)
		lastActivity = screenshots[0].Timestamp.Format(time.RFC3339)
	}

	report.Summary = ActivitySummary{
		Username:          username,
		StartDate:         startOfDay.Format(time.RFC3339),
		EndDate:           endOfDay.Format(time.RFC3339),
		TotalActiveTime:   totalDuration,
		TotalIdleTime:     0,
		ProductiveTime:    productiveTime,
		UnproductiveTime:  unproductiveTime,
		NeutralTime:       neutralTime,
		FirstActivity:     firstActivity,
		LastActivity:      lastActivity,
		ProductivityScore: productivityScore,
	}

	return report, nil
}
