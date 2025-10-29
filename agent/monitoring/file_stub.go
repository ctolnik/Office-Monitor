//go:build !windows
// +build !windows

package monitoring

import (
	"fmt"
)

// File monitoring is only supported on Windows
type FileMonitor struct{}

type FileActivity struct {
	Location       string
	FileCount      int
	TotalSizeBytes int64
}

func NewFileMonitor(serverURL, computerName, username string, monitoredLocations []string, largeCopyThresholdMB, largeCopyFileCount int, detectExternalCopy bool) *FileMonitor {
	return &FileMonitor{}
}

func (m *FileMonitor) Start() error {
	return fmt.Errorf("file monitoring is only supported on Windows")
}

func (m *FileMonitor) Stop() {}

func (m *FileMonitor) GetStats() map[string]*FileActivity {
	return nil
}
