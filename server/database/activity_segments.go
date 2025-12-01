package database

import (
        "context"
        "fmt"
        "strings"
        "time"

        "github.com/ctolnik/Office-Monitor/zapctx"
        "go.uber.org/zap"
)

// GetActivitySegmentsByUsername returns activity segments for user in time range
func (db *Database) GetActivitySegmentsByUsername(ctx context.Context, username string, start, end time.Time) ([]ActivitySegment, error) {
        startStr := start.Format("2006-01-02 15:04:05")
        endStr := end.Format("2006-01-02 15:04:05")

        query := fmt.Sprintf(`
                SELECT 
                        timestamp_start,
                        timestamp_end,
                        duration_sec,
                        state,
                        computer_name,
                        username,
                        process_name,
                        window_title,
                        session_id
                FROM monitoring.activity_segments
                WHERE username = ? 
                  AND timestamp_start >= toDateTime64('%s', 3)
                  AND timestamp_start < toDateTime64('%s', 3)
                ORDER BY timestamp_start ASC
                LIMIT 10000`, startStr, endStr)

        zapctx.Info(ctx, "GetActivitySegmentsByUsername",
                zap.String("username", username),
                zap.String("start", startStr),
                zap.String("end", endStr),
                zap.String("query", query))

        rows, err := db.conn.Query(ctx, query, username)
        if err != nil {
                zapctx.Error(ctx, "Query failed", zap.Error(err), zap.String("query", query))
                return nil, err
        }
        defer rows.Close()

        // Load process catalog for category matching
        processCatalog, _ := db.GetProcessCatalog(ctx)

        segments := make([]ActivitySegment, 0)
        for rows.Next() {
                var s ActivitySegment
                if err := rows.Scan(
                        &s.TimestampStart,
                        &s.TimestampEnd,
                        &s.DurationSec,
                        &s.State,
                        &s.ComputerName,
                        &s.Username,
                        &s.ProcessName,
                        &s.WindowTitle,
                        &s.SessionID,
                ); err != nil {
                        zapctx.Error(ctx, "Failed to scan activity segment row", zap.Error(err))
                        continue
                }
                // Set category based on state and process catalog
                if s.State == "idle" || s.State == "offline" {
                        s.Category = s.State
                } else {
                        s.Category = matchProcessToCatalogInternal(s.ProcessName, processCatalog)
                }
                segments = append(segments, s)
        }

        if err := rows.Err(); err != nil {
                zapctx.Error(ctx, "Error iterating activity segment rows", zap.Error(err))
                return nil, err
        }

        // Count segments by state for debugging
        stateCount := make(map[string]int)
        for _, seg := range segments {
                stateCount[seg.State]++
        }

        zapctx.Info(ctx, "GetActivitySegmentsByUsername result",
                zap.String("username", username),
                zap.Int("segments_count", len(segments)),
                zap.Any("states", stateCount))

        return segments, nil
}

// GetApplicationUsageFromSegments returns application usage statistics from activity segments
func (db *Database) GetApplicationUsageFromSegments(ctx context.Context, username string, start, end time.Time) ([]ApplicationUsage, error) {
        startStr := start.Format("2006-01-02 15:04:05")
        endStr := end.Format("2006-01-02 15:04:05")

        query := fmt.Sprintf(`
                SELECT 
                        process_name,
                        window_title,
                        sum(duration_sec) as total_duration,
                        count(*) as count
                FROM monitoring.activity_segments
                WHERE username = ? 
                  AND timestamp_start >= toDateTime64('%s', 3)
                  AND timestamp_start < toDateTime64('%s', 3)
                GROUP BY process_name, window_title
                ORDER BY total_duration DESC
                LIMIT 50`, startStr, endStr)

        zapctx.Info(ctx, "GetApplicationUsageFromSegments",
                zap.String("username", username),
                zap.String("start", startStr),
                zap.String("end", endStr),
                zap.String("query", query))

        rows, err := db.conn.Query(ctx, query, username)
        if err != nil {
                zapctx.Error(ctx, "Query failed", zap.Error(err))
                return nil, err
        }
        defer rows.Close()

        // Load process catalog first (user's "Справочник программ")
        processCatalog, err := db.GetProcessCatalog(ctx)
        if err != nil {
                zapctx.Warn(ctx, "Failed to load process catalog", zap.Error(err))
                processCatalog = []ProcessCatalogEntry{}
        }
        zapctx.Debug(ctx, "Process catalog loaded for matching", zap.Int("entries", len(processCatalog)))

        // Load application categories as fallback
        categories, err := db.GetApplicationCategories(ctx, "", "", true)
        if err != nil {
                zapctx.Warn(ctx, "Failed to load application categories, using default 'neutral'", zap.Error(err))
                categories = []ApplicationCategory{}
        }

        apps := make([]ApplicationUsage, 0)
        for rows.Next() {
                var app ApplicationUsage
                var totalDuration uint64
                var count uint64

                if err := rows.Scan(&app.ProcessName, &app.WindowTitle, &totalDuration, &count); err != nil {
                        zapctx.Error(ctx, "Failed to scan application usage row", zap.Error(err))
                        continue
                }

                // Skip "unknown" processes (system/protected processes that agent can't monitor)
                if app.ProcessName == "unknown" || app.ProcessName == "" {
                        continue
                }

                app.Duration = totalDuration
                app.TotalDuration = totalDuration
                app.Count = int(count)

                // Get friendly name from process catalog (e.g., "chrome.exe" -> "Google Chrome")
                app.ApplicationName = getFriendlyNameFromCatalog(app.ProcessName, processCatalog)

                // Match process to category: first try process_catalog, then application_categories
                app.Category = matchProcessToCatalogInternal(app.ProcessName, processCatalog)
                if app.Category == "neutral" {
                        app.Category = matchProcessToCategoryInternal(app.ProcessName, categories)
                }

                apps = append(apps, app)
        }

        if err := rows.Err(); err != nil {
                zapctx.Error(ctx, "Error iterating application usage rows", zap.Error(err))
                return nil, err
        }

        zapctx.Info(ctx, "GetApplicationUsageFromSegments result",
                zap.String("username", username),
                zap.Int("apps_count", len(apps)))

        return apps, nil
}

// GetApplicationTimeline returns time periods when each application was used
func (db *Database) GetApplicationTimeline(ctx context.Context, username string, start, end time.Time) ([]ApplicationTimeline, error) {
        startStr := start.Format("2006-01-02 15:04:05")
        endStr := end.Format("2006-01-02 15:04:05")

        // Get all activity segments ordered by process and time
        query := fmt.Sprintf(`
                SELECT 
                        process_name,
                        window_title,
                        timestamp_start,
                        timestamp_end,
                        duration_sec
                FROM monitoring.activity_segments
                WHERE username = ? 
                  AND timestamp_start >= toDateTime64('%s', 3)
                  AND timestamp_start < toDateTime64('%s', 3)
                  AND state = 'active'
                ORDER BY process_name, timestamp_start ASC`, startStr, endStr)

        rows, err := db.conn.Query(ctx, query, username)
        if err != nil {
                zapctx.Error(ctx, "GetApplicationTimeline query failed", zap.Error(err))
                return nil, err
        }
        defer rows.Close()

        // Load process catalog for category and friendly name matching
        processCatalog, _ := db.GetProcessCatalog(ctx)

        // Map to collect periods by process
        timelineMap := make(map[string]*ApplicationTimeline)

        for rows.Next() {
                var processName, windowTitle string
                var tsStart, tsEnd time.Time
                var durationSec uint32

                if err := rows.Scan(&processName, &windowTitle, &tsStart, &tsEnd, &durationSec); err != nil {
                        zapctx.Warn(ctx, "Failed to scan timeline row", zap.Error(err))
                        continue
                }

                // Skip unknown processes
                if processName == "unknown" || processName == "" {
                        continue
                }

                // Get or create timeline entry for this process
                timeline, exists := timelineMap[processName]
                if !exists {
                        category := matchProcessToCatalogInternal(processName, processCatalog)
                        friendlyName := getFriendlyNameFromCatalog(processName, processCatalog)
                        timeline = &ApplicationTimeline{
                                ProcessName:  processName,
                                FriendlyName: friendlyName,
                                Category:     category,
                                TotalSeconds: 0,
                                Periods:      make([]ApplicationTimePeriod, 0),
                        }
                        timelineMap[processName] = timeline
                }

                // Add this period
                period := ApplicationTimePeriod{
                        Start:       tsStart.Format(time.RFC3339),
                        End:         tsEnd.Format(time.RFC3339),
                        DurationSec: durationSec,
                        WindowTitle: windowTitle,
                }
                timeline.Periods = append(timeline.Periods, period)
                timeline.TotalSeconds += uint64(durationSec)
        }

        if err := rows.Err(); err != nil {
                zapctx.Error(ctx, "Error iterating timeline rows", zap.Error(err))
                return nil, err
        }

        // Convert map to slice and sort by total time descending
        result := make([]ApplicationTimeline, 0, len(timelineMap))
        for _, timeline := range timelineMap {
                result = append(result, *timeline)
        }

        // Sort by TotalSeconds descending
        for i := 0; i < len(result)-1; i++ {
                for j := i + 1; j < len(result); j++ {
                        if result[j].TotalSeconds > result[i].TotalSeconds {
                                result[i], result[j] = result[j], result[i]
                        }
                }
        }

        zapctx.Info(ctx, "GetApplicationTimeline result",
                zap.String("username", username),
                zap.Int("apps_count", len(result)))

        return result, nil
}

// getFriendlyNameFromCatalog returns friendly name from process catalog
func getFriendlyNameFromCatalog(processName string, catalog []ProcessCatalogEntry) string {
        processLower := strings.ToLower(processName)
        processNorm := strings.TrimSuffix(processLower, ".exe")

        for _, entry := range catalog {
                if !entry.IsActive {
                        continue
                }
                for _, procName := range entry.ProcessNames {
                        catalogNorm := strings.TrimSuffix(strings.ToLower(procName), ".exe")
                        if strings.EqualFold(procName, processName) ||
                                catalogNorm == processNorm ||
                                strings.Contains(processNorm, catalogNorm) ||
                                strings.Contains(catalogNorm, processNorm) {
                                return entry.FriendlyName
                        }
                }
        }

        // Return process name without .exe as fallback
        return strings.TrimSuffix(processName, ".exe")
}

// GetActivityPeriods returns chronological activity periods for user
// Groups consecutive segments of the same type into periods
// Example output: "9:03-9:29 Worked in 1C", "9:29-9:39 Idle", "9:39-10:00 Chrome"
func (db *Database) GetActivityPeriods(ctx context.Context, username string, start, end time.Time) ([]ActivityPeriod, error) {
        // Get all segments sorted by time
        segments, err := db.GetActivitySegmentsByUsername(ctx, username, start, end)
        if err != nil {
                return nil, err
        }

        if len(segments) == 0 {
                return []ActivityPeriod{}, nil
        }

        // Load process catalog for friendly names
        processCatalog, _ := db.GetProcessCatalog(ctx)

        periods := make([]ActivityPeriod, 0)
        var currentPeriod *ActivityPeriod
        windowTitlesSet := make(map[string]bool)

        for _, seg := range segments {
                // Determine the grouping key: for active state use process, for idle/offline use state
                var groupKey string
                if seg.State == "active" {
                        groupKey = seg.ProcessName
                } else {
                        groupKey = seg.State // idle or offline
                }

                // Check if we should start a new period
                startNewPeriod := currentPeriod == nil ||
                        (seg.State == "active" && currentPeriod.ProcessName != seg.ProcessName) ||
                        (seg.State != "active" && currentPeriod.State != seg.State)

                if startNewPeriod {
                        // Save previous period
                        if currentPeriod != nil {
                                // Convert window titles set to slice
                                titles := make([]string, 0, len(windowTitlesSet))
                                for title := range windowTitlesSet {
                                        if title != "" {
                                                titles = append(titles, title)
                                        }
                                }
                                currentPeriod.WindowTitles = titles
                                currentPeriod.Description = buildPeriodDescription(currentPeriod, processCatalog)
                                periods = append(periods, *currentPeriod)
                        }

                        // Start new period
                        friendlyName := ""
                        category := seg.Category
                        if seg.State == "active" {
                                friendlyName = getFriendlyNameFromCatalog(seg.ProcessName, processCatalog)
                                if category == "" {
                                        category = matchProcessToCatalogInternal(seg.ProcessName, processCatalog)
                                }
                        } else {
                                category = seg.State // idle or offline
                        }

                        currentPeriod = &ActivityPeriod{
                                Start:        seg.TimestampStart.Format(time.RFC3339),
                                End:          seg.TimestampEnd.Format(time.RFC3339),
                                DurationSec:  seg.DurationSec,
                                State:        seg.State,
                                ProcessName:  seg.ProcessName,
                                FriendlyName: friendlyName,
                                Category:     category,
                        }
                        windowTitlesSet = make(map[string]bool)
                        if seg.WindowTitle != "" {
                                windowTitlesSet[seg.WindowTitle] = true
                        }
                } else {
                        // Extend current period
                        currentPeriod.End = seg.TimestampEnd.Format(time.RFC3339)
                        currentPeriod.DurationSec += seg.DurationSec
                        if seg.WindowTitle != "" {
                                windowTitlesSet[seg.WindowTitle] = true
                        }
                }

                _ = groupKey // used for documentation
        }

        // Don't forget the last period
        if currentPeriod != nil {
                titles := make([]string, 0, len(windowTitlesSet))
                for title := range windowTitlesSet {
                        if title != "" {
                                titles = append(titles, title)
                        }
                }
                currentPeriod.WindowTitles = titles
                currentPeriod.Description = buildPeriodDescription(currentPeriod, processCatalog)
                periods = append(periods, *currentPeriod)
        }

        zapctx.Info(ctx, "GetActivityPeriods result",
                zap.String("username", username),
                zap.Int("periods_count", len(periods)))

        return periods, nil
}

// buildPeriodDescription creates human-readable description for a period
func buildPeriodDescription(period *ActivityPeriod, catalog []ProcessCatalogEntry) string {
        // Format duration
        durationStr := formatDuration(period.DurationSec)

        switch period.State {
        case "idle":
                return fmt.Sprintf("Простой (%s)", durationStr)
        case "offline":
                return fmt.Sprintf("Отключен (%s)", durationStr)
        default:
                appName := period.FriendlyName
                if appName == "" {
                        appName = strings.TrimSuffix(period.ProcessName, ".exe")
                }
                return fmt.Sprintf("Работал в %s (%s)", appName, durationStr)
        }
}

// formatDuration formats seconds into human-readable string
func formatDuration(seconds uint32) string {
        if seconds < 60 {
                return fmt.Sprintf("%d сек", seconds)
        }
        if seconds < 3600 {
                mins := seconds / 60
                secs := seconds % 60
                if secs > 0 {
                        return fmt.Sprintf("%d мин %d сек", mins, secs)
                }
                return fmt.Sprintf("%d мин", mins)
        }
        hours := seconds / 3600
        mins := (seconds % 3600) / 60
        if mins > 0 {
                return fmt.Sprintf("%d ч %d мин", hours, mins)
        }
        return fmt.Sprintf("%d ч", hours)
}
