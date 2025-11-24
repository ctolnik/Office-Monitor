// +build !windows

package monitoring

type ActivityTracker struct{}

func NewActivityTracker(serverURL, computerName, username string, idleThresholdMin, pollIntervalSec int) *ActivityTracker {
	return &ActivityTracker{}
}

func (at *ActivityTracker) Start() error {
	return nil
}

func (at *ActivityTracker) Stop() {}
