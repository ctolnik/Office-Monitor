package database

import (
        "context"
        "fmt"
        "time"

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

        rows, err := db.conn.Query(ctx, query, username)
        if err != nil {
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
                        zap.L().Error("Failed to scan activity segment row", zap.Error(err))
                        continue
                }
                segments = append(segments, s)
        }

        if err := rows.Err(); err != nil {
                zap.L().Error("Error iterating activity segment rows", zap.Error(err))
                return nil, err
        }

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
                  AND state = 'active'
                GROUP BY process_name, window_title
                ORDER BY total_duration DESC
                LIMIT 50`, startStr, endStr)

        rows, err := db.conn.Query(ctx, query, username)
        if err != nil {
                return nil, err
        }
        defer rows.Close()

        apps := make([]ApplicationUsage, 0)
        for rows.Next() {
                var app ApplicationUsage
                var totalDuration uint32
                var count uint32
                
                if err := rows.Scan(&app.ProcessName, &app.WindowTitle, &totalDuration, &count); err != nil {
                        zap.L().Error("Failed to scan application usage row", zap.Error(err))
                        continue
                }
                
                app.Duration = uint64(totalDuration)
                app.TotalDuration = uint64(totalDuration)
                app.Count = int(count)
                app.Category = "neutral" // TODO: get from categories table
                
                apps = append(apps, app)
        }

        if err := rows.Err(); err != nil {
                zap.L().Error("Error iterating application usage rows", zap.Error(err))
                return nil, err
        }

        return apps, nil
}
