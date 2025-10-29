//go:build !windows
// +build !windows

package monitoring

import (
	"context"
	"fmt"

	"github.com/ctolnik/Office-Monitor/agent/buffer"
	"github.com/ctolnik/Office-Monitor/agent/httpclient"
)

// ActivityTracker monitors active window and process (stub for non-Windows)
type ActivityTracker struct {
	stopChan chan struct{}
}

// NewActivityTracker creates a new activity tracker (stub)
func NewActivityTracker(client *httpclient.Client, eventBuffer *buffer.EventBuffer, computerName string, intervalSec int) (*ActivityTracker, error) {
	return &ActivityTracker{
		stopChan: make(chan struct{}),
	}, nil
}

// Start begins monitoring activity (stub)
func (t *ActivityTracker) Start(ctx context.Context) error {
	<-ctx.Done()
	return fmt.Errorf("activity tracking not supported on this platform")
}

// Stop stops the activity tracker (stub)
func (t *ActivityTracker) Stop() {
	close(t.stopChan)
}
