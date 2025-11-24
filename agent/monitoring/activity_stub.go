//go:build !windows
// +build !windows

package monitoring

import (
        "fmt"
)

// ActivityTracker monitors active window and process (stub for non-Windows)
type ActivityTracker struct {
        stopChan chan struct{}
}

// NewActivityTracker creates a new activity tracker (stub)
// Signature matches Windows implementation for cross-platform compatibility
func NewActivityTracker(serverURL, computerName, username string, idleThresholdMin, pollIntervalSec int) *ActivityTracker {
        return &ActivityTracker{
                stopChan: make(chan struct{}),
        }
}

// Start begins monitoring activity (stub)
func (t *ActivityTracker) Start() error {
        return fmt.Errorf("activity tracking not supported on non-Windows platforms")
}

// Stop stops the activity tracker (stub)
func (t *ActivityTracker) Stop() {
        if t.stopChan != nil {
                close(t.stopChan)
        }
}
