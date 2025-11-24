package database

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ctolnik/Office-Monitor/zapctx"
	"go.uber.org/zap"
)

// GetApplicationCategories retrieves application categories with optional filtering
func (db *Database) GetApplicationCategories(ctx context.Context, category, search string, activeOnly bool) ([]ApplicationCategory, error) {
	query := `
		SELECT 
			toString(id) as id,
			process_name,
			process_pattern,
			category,
			created_at,
			updated_at,
			created_by,
			updated_by,
			is_active
		FROM monitoring.application_categories
		WHERE 1=1`

	args := make([]interface{}, 0)

	if activeOnly {
		query += " AND is_active = 1"
	}

	if category != "" {
		query += " AND category = ?"
		args = append(args, category)
	}

	if search != "" {
		query += " AND (process_name ILIKE ? OR process_pattern ILIKE ?)"
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	query += " ORDER BY category, process_name"

	rows, err := db.conn.Query(ctx, query, args...)
	if err != nil {
		zapctx.Error(ctx, "Failed to query application categories", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	categories := make([]ApplicationCategory, 0)
	for rows.Next() {
		var cat ApplicationCategory
		var isActive uint8
		if err := rows.Scan(
			&cat.ID,
			&cat.ProcessName,
			&cat.ProcessPattern,
			&cat.Category,
			&cat.CreatedAt,
			&cat.UpdatedAt,
			&cat.CreatedBy,
			&cat.UpdatedBy,
			&isActive,
		); err != nil {
			zapctx.Error(ctx, "Failed to scan category row", zap.Error(err))
			continue
		}
		cat.IsActive = isActive == 1
		categories = append(categories, cat)
	}

	if err := rows.Err(); err != nil {
		zapctx.Error(ctx, "Error iterating category rows", zap.Error(err))
		return nil, err
	}

	zapctx.Debug(ctx, "Application categories retrieved",
		zap.Int("count", len(categories)),
		zap.String("category_filter", category),
		zap.String("search", search))

	return categories, nil
}

// CreateApplicationCategory creates a new application category
func (db *Database) CreateApplicationCategory(ctx context.Context, cat ApplicationCategory) error {
	// Check for duplicates
	existing, err := db.GetApplicationCategories(ctx, "", cat.ProcessName, false)
	if err != nil {
		return fmt.Errorf("failed to check for duplicates: %w", err)
	}

	for _, e := range existing {
		if strings.EqualFold(e.ProcessName, cat.ProcessName) && e.IsActive {
			return fmt.Errorf("application category for '%s' already exists", cat.ProcessName)
		}
	}

	query := `
		INSERT INTO monitoring.application_categories 
		(process_name, process_pattern, category, created_by, updated_by, is_active)
		VALUES (?, ?, ?, ?, ?, ?)`

	err = db.conn.Exec(ctx, query,
		cat.ProcessName,
		cat.ProcessPattern,
		cat.Category,
		cat.CreatedBy,
		cat.UpdatedBy,
		1,
	)

	if err != nil {
		zapctx.Error(ctx, "Failed to create application category",
			zap.Error(err),
			zap.String("process_name", cat.ProcessName))
		return fmt.Errorf("failed to insert category: %w", err)
	}

	zapctx.Info(ctx, "Application category created",
		zap.String("process_name", cat.ProcessName),
		zap.String("category", cat.Category))

	return nil
}

// UpdateApplicationCategory updates an existing application category
func (db *Database) UpdateApplicationCategory(ctx context.Context, id string, cat ApplicationCategory) error {
	query := `
		ALTER TABLE monitoring.application_categories
		UPDATE 
			process_name = ?,
			process_pattern = ?,
			category = ?,
			updated_by = ?,
			updated_at = now(),
			is_active = ?
		WHERE toString(id) = ?`

	isActive := uint8(0)
	if cat.IsActive {
		isActive = 1
	}

	err := db.conn.Exec(ctx, query,
		cat.ProcessName,
		cat.ProcessPattern,
		cat.Category,
		cat.UpdatedBy,
		isActive,
		id,
	)

	if err != nil {
		zapctx.Error(ctx, "Failed to update application category",
			zap.Error(err),
			zap.String("id", id))
		return fmt.Errorf("failed to update category: %w", err)
	}

	zapctx.Info(ctx, "Application category updated",
		zap.String("id", id),
		zap.String("process_name", cat.ProcessName))

	return nil
}

// DeleteApplicationCategory performs soft delete on application category
func (db *Database) DeleteApplicationCategory(ctx context.Context, id string) error {
	query := `
		ALTER TABLE monitoring.application_categories
		UPDATE 
			is_active = 0,
			updated_at = now()
		WHERE toString(id) = ?`

	err := db.conn.Exec(ctx, query, id)
	if err != nil {
		zapctx.Error(ctx, "Failed to delete application category",
			zap.Error(err),
			zap.String("id", id))
		return fmt.Errorf("failed to delete category: %w", err)
	}

	zapctx.Info(ctx, "Application category deleted (soft delete)", zap.String("id", id))
	return nil
}

// BulkUpdateCategories updates categories for multiple applications
func (db *Database) BulkUpdateCategories(ctx context.Context, ids []string, category, updatedBy string) (int, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("no IDs provided")
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+2)

	for i := range ids {
		placeholders[i] = "?"
		args = append(args, ids[i])
	}


	query := fmt.Sprintf(`
		ALTER TABLE monitoring.application_categories
		UPDATE 
			category = ?,
			updated_by = ?,
			updated_at = now()
		WHERE toString(id) IN (%s)`, strings.Join(placeholders, ","))

	// Reorder args for ClickHouse UPDATE syntax
	reorderedArgs := []interface{}{category, updatedBy}
	for _, id := range ids {
		reorderedArgs = append(reorderedArgs, id)
	}

	err := db.conn.Exec(ctx, query, reorderedArgs...)
	if err != nil {
		zapctx.Error(ctx, "Failed to bulk update categories",
			zap.Error(err),
			zap.Int("count", len(ids)))
		return 0, fmt.Errorf("failed to bulk update: %w", err)
	}

	zapctx.Info(ctx, "Categories bulk updated",
		zap.Int("count", len(ids)),
		zap.String("category", category))

	return len(ids), nil
}

// MatchProcessToCategory matches a process name to its category from database
// This is a Database method that queries and matches the process
func (db *Database) MatchProcessToCategory(ctx context.Context, processName, windowTitle string) (string, error) {
	// Get all categories
	categories, err := db.GetApplicationCategories(ctx, "", "", true)
	if err != nil {
		return "neutral", err
	}
	return matchProcessToCategoryInternal(processName, categories), nil
}

// matchProcessToCategoryInternal is internal helper function
// Priority: 1. Exact match, 2. Pattern match, 3. Default (neutral)
func matchProcessToCategoryInternal(processName string, categories []ApplicationCategory) string {
	if processName == "" {
		return "neutral"
	}

	processLower := strings.ToLower(processName)

	// First pass: exact match
	for _, cat := range categories {
		if !cat.IsActive {
			continue
		}
		if strings.EqualFold(cat.ProcessName, processName) {
			return cat.Category
		}
	}

	// Second pass: pattern match
	for _, cat := range categories {
		if !cat.IsActive || cat.ProcessPattern == "" {
			continue
		}

		// Use filepath.Match for wildcard matching
		matched, err := filepath.Match(strings.ToLower(cat.ProcessPattern), processLower)
		if err == nil && matched {
			return cat.Category
		}
	}

	// Default: neutral
	return "neutral"
}

// CalculateProductivity calculates productivity score for a user in time range
// Returns percentage (0-100) where higher is more productive
func (db *Database) CalculateProductivity(ctx context.Context, username string, start, end time.Time) (float64, error) {
	// Get activities for user
	activities, err := db.GetActivityEventsByUsername(ctx, username, start, end)
	if err != nil {
		return 0.0, err
	}

	if len(activities) == 0 {
		return 0.0, nil
	}

	// Get all categories
	categories, err := db.GetApplicationCategories(ctx, "", "", true)
	if err != nil {
		return 0.0, err
	}

	var totalTime, productiveTime float64

	for _, activity := range activities {
		duration := float64(activity.Duration)
		totalTime += duration

		category := matchProcessToCategoryInternal(activity.ProcessName, categories)
		if category == "productive" {
			productiveTime += duration
		}
	}

	if totalTime == 0 {
		return 0.0, nil
	}

	return (productiveTime / totalTime) * 100.0, nil
}

// DetectContext determines context information based on window title, process, and text content
func DetectContext(processName, windowTitle, textContent string) string {
	if windowTitle == "" {
		return "Work in application"
	}

	lowerTitle := strings.ToLower(windowTitle)
	lowerProcess := strings.ToLower(processName)

	// Email contexts
	if strings.Contains(lowerProcess, "outlook.exe") || strings.Contains(lowerProcess, "thunderbird.exe") {
		if strings.Contains(lowerTitle, "re:") || strings.Contains(lowerTitle, "ответ:") {
			return "Email reply"
		}
		if strings.Contains(lowerTitle, "fwd:") || strings.Contains(lowerTitle, "пересылка:") {
			return "Email forwarding"
		}
		if strings.Contains(lowerTitle, "new message") || strings.Contains(lowerTitle, "новое сообщение") {
			return "Writing new email"
		}
		return "Working with email"
	}

	// Search engines
	if strings.Contains(lowerTitle, "яндекс") || strings.Contains(lowerTitle, "yandex") {
		return "Searching in Yandex"
	}
	if strings.Contains(lowerTitle, "google") && (strings.Contains(lowerTitle, "search") || strings.Contains(lowerTitle, "поиск")) {
		return "Searching in Google"
	}

	// Messengers
	if strings.Contains(lowerProcess, "telegram.exe") || strings.Contains(lowerProcess, "slack.exe") ||
		strings.Contains(lowerProcess, "teams.exe") || strings.Contains(lowerProcess, "discord.exe") {
		return "Messaging"
	}

	// Office documents
	if strings.Contains(lowerProcess, "winword.exe") {
		return "Editing Word document"
	}
	if strings.Contains(lowerProcess, "excel.exe") {
		return "Working with Excel"
	}
	if strings.Contains(lowerProcess, "powerpnt.exe") {
		return "Creating PowerPoint presentation"
	}

	// Development
	if strings.Contains(lowerProcess, "code.exe") || strings.Contains(lowerProcess, "idea64.exe") ||
		strings.Contains(lowerProcess, "pycharm") || strings.Contains(lowerProcess, "goland") {
		if strings.Contains(lowerTitle, ".go") {
			return "Go development"
		}
		if strings.Contains(lowerTitle, ".py") {
			return "Python development"
		}
		if strings.Contains(lowerTitle, ".js") || strings.Contains(lowerTitle, ".ts") {
			return "JavaScript/TypeScript development"
		}
		return "Programming"
	}

	// Terminal
	if strings.Contains(lowerProcess, "cmd.exe") || strings.Contains(lowerProcess, "powershell.exe") ||
		strings.Contains(lowerProcess, "terminal.exe") || strings.Contains(lowerProcess, "wt.exe") {
		return "Working in terminal"
	}

	// Browser - specific sites
	if strings.Contains(lowerProcess, "chrome.exe") || strings.Contains(lowerProcess, "firefox.exe") ||
		strings.Contains(lowerProcess, "msedge.exe") {
		if strings.Contains(lowerTitle, "youtube") {
			return "Watching YouTube"
		}
		if strings.Contains(lowerTitle, "github") {
			return "Working with GitHub"
		}
		if strings.Contains(lowerTitle, "gitlab") {
			return "Working with GitLab"
		}
		if strings.Contains(lowerTitle, "stackoverflow") {
			return "Searching solutions on StackOverflow"
		}
		if strings.Contains(lowerTitle, "documentation") || strings.Contains(lowerTitle, "docs") {
			return "Reading documentation"
		}
		if strings.Contains(lowerTitle, "jira") || strings.Contains(lowerTitle, "confluence") {
			return "Working with Atlassian tools"
		}
		return "Browsing web"
	}

	// Default
	return "Working in application"
}

// GroupKeyboardEvents groups keyboard events into periods
func GroupKeyboardEvents(events []KeyboardEvent) []KeyboardPeriod {
	if len(events) == 0 {
		return []KeyboardPeriod{}
	}

	var periods []KeyboardPeriod
	var currentPeriod *KeyboardPeriod

	for _, event := range events {
		// Start new period if:
		// 1. First event
		// 2. Different application
		// 3. Time gap > 5 minutes
		shouldStartNew := currentPeriod == nil ||
			currentPeriod.Application != event.ProcessName ||
			event.Timestamp.Sub(time.Time{}).Minutes() > 5

		if shouldStartNew {
			if currentPeriod != nil {
				periods = append(periods, *currentPeriod)
			}

			currentPeriod = &KeyboardPeriod{
				Start:         event.Timestamp.Format(time.RFC3339),
				End:           event.Timestamp.Format(time.RFC3339),
				Application:   event.ProcessName,
				WindowTitle:   event.WindowTitle,
				FormattedText: event.TextContent,
				RawKeys:       "[]", // Would need actual key events JSON
			}
		} else {
			// Update current period
			currentPeriod.End = event.Timestamp.Format(time.RFC3339)
			if event.TextContent != "" {
				currentPeriod.FormattedText += " " + event.TextContent
			}
		}
	}

	if currentPeriod != nil {
		periods = append(periods, *currentPeriod)
	}

	return periods
}
