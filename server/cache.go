package main

import (
	"context"
	"sync"
	"time"

	"github.com/ctolnik/Office-Monitor/server/database"
)

// DashboardCache holds cached dashboard statistics with TTL
type DashboardCache struct {
	mu    sync.RWMutex
	stats *database.DashboardStats
	cachedAt time.Time
	ttl   time.Duration
}

// NewDashboardCache creates a new dashboard cache with specified TTL
func NewDashboardCache(ttl time.Duration) *DashboardCache {
	return &DashboardCache{
		ttl: ttl,
	}
}

// Get returns cached stats if available and not expired, otherwise fetches fresh data
func (dc *DashboardCache) Get(ctx context.Context, db *database.Database) (*database.DashboardStats, error) {
	// Try read lock first
	dc.mu.RLock()
	if dc.stats != nil && time.Since(dc.cachedAt) < dc.ttl {
		stats := dc.stats
		dc.mu.RUnlock()
		return stats, nil
	}
	dc.mu.RUnlock()

	// Cache miss or expired - fetch fresh data with write lock
	dc.mu.Lock()
	defer dc.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have refreshed)
	if dc.stats != nil && time.Since(dc.cachedAt) < dc.ttl {
		return dc.stats, nil
	}

	// Fetch fresh data
	stats, err := db.GetDashboardStats(ctx)
	if err != nil {
		return nil, err
	}

	// Update cache
	dc.stats = stats
	dc.cachedAt = time.Now()

	return stats, nil
}

// Invalidate clears the cache
func (dc *DashboardCache) Invalidate() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.stats = nil
}
