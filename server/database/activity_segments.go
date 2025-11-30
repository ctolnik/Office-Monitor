package database

import (
	"context"
	"fmt"
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
	processCatalog, _ := db.GetProcessCatalog(ctx)

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
