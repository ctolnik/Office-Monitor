//go:build !windows
// +build !windows

package monitoring

import (
	"fmt"
)

type ScreenshotMonitor struct {
	enabled bool
}

func NewScreenshotMonitor(serverURL, computerName, username string, intervalMinutes, quality, maxSizeKB int, captureOnlyActive, uploadImmediately bool) *ScreenshotMonitor {
	return &ScreenshotMonitor{
		enabled: false,
	}
}

func (m *ScreenshotMonitor) Start() error {
	return fmt.Errorf("screenshot monitoring not supported on non-Windows platforms")
}

func (m *ScreenshotMonitor) Stop() {
}
