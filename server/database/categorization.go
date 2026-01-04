package database

import (
	"context"
	"strings"
	"time"
)

// MatchProcessToCategory matches a process name to its category key using process_catalog_v2.
// If no rule matches, returns "neutral".
func (db *Database) MatchProcessToCategory(ctx context.Context, processName, windowTitle string) (string, error) {
	processCatalog, err := db.GetProcessCatalog(ctx)
	if err != nil {
		return "neutral", err
	}
	return matchProcessToCatalogInternal(processName, processCatalog), nil
}

// matchProcessToCatalogInternal matches process to category key using process_catalog rules.
// Matching behavior is intentionally simple for now ("as-is"): by process_names.
func matchProcessToCatalogInternal(processName string, catalog []ProcessCatalogEntry) string {
	if processName == "" {
		return "neutral"
	}

	processLower := strings.ToLower(processName)
	processNorm := strings.TrimSuffix(processLower, ".exe")

	for _, entry := range catalog {
		if !entry.IsActive {
			continue
		}

		categoryKey := "neutral"
		if entry.Category != nil && entry.Category.Key != "" {
			categoryKey = entry.Category.Key
		}

		for _, procName := range entry.ProcessNames {
			catalogNorm := strings.TrimSuffix(strings.ToLower(procName), ".exe")
			if strings.EqualFold(procName, processName) ||
				catalogNorm == processNorm ||
				strings.Contains(processNorm, catalogNorm) ||
				strings.Contains(catalogNorm, processNorm) {
				return categoryKey
			}
		}
	}

	return "neutral"
}

// CalculateProductivity calculates productivity score for a user in time range.
// Returns percentage (0-100) where higher is more productive.
func (db *Database) CalculateProductivity(ctx context.Context, username string, start, end time.Time) (float64, error) {
	activities, err := db.GetActivityEventsByUsername(ctx, username, start, end)
	if err != nil {
		return 0.0, err
	}
	if len(activities) == 0 {
		return 0.0, nil
	}

	processCatalog, err := db.GetProcessCatalog(ctx)
	if err != nil {
		// Treat as empty catalog.
		processCatalog = []ProcessCatalogEntry{}
	}

	var totalTime, productiveTime float64
	for _, activity := range activities {
		duration := float64(activity.Duration)
		totalTime += duration

		category := matchProcessToCatalogInternal(activity.ProcessName, processCatalog)
		if category == "productive" {
			productiveTime += duration
		}
	}

	if totalTime == 0 {
		return 0.0, nil
	}
	return (productiveTime / totalTime) * 100.0, nil
}
