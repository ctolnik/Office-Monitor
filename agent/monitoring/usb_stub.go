//go:build !windows
// +build !windows

package monitoring

import (
	"fmt"
	"time"
)

// USB monitoring is only supported on Windows
type USBMonitor struct{}

type USBDevice struct {
	DeviceID     string
	DeviceName   string
	DeviceType   string
	VolumeSerial string
	DriveLetter  string
	ConnectedAt  time.Time
}

func NewUSBMonitor(serverURL, computerName, username string, shadowCopyEnabled bool, shadowCopyDest string, copyExtensions, excludePatterns []string) *USBMonitor {
	return &USBMonitor{}
}

func (m *USBMonitor) Start() error {
	return fmt.Errorf("USB monitoring is only supported on Windows")
}

func (m *USBMonitor) Stop() {}

func (m *USBMonitor) GetConnectedDevices() []*USBDevice {
	return nil
}
