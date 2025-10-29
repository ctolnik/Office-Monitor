//go:build windows
// +build windows

package monitoring

import (
	"testing"
	"time"
)

func TestUSBMonitorCreation(t *testing.T) {
	monitor := NewUSBMonitor(
		"http://localhost:5000",
		"TEST-PC",
		"testuser",
		false,
		"",
		nil,
		nil,
	)

	if monitor == nil {
		t.Fatal("Failed to create USB monitor")
	}
}

func TestUSBMonitorWithShadowCopy(t *testing.T) {
	monitor := NewUSBMonitor(
		"http://localhost:5000",
		"TEST-PC",
		"testuser",
		true,
		"\\\\server\\shadow",
		[]string{".pdf", ".docx", ".xlsx"},
		[]string{"System Volume Information"},
	)

	if monitor == nil {
		t.Fatal("Failed to create USB monitor with shadow copy")
	}
}

func TestUSBDeviceStructure(t *testing.T) {
	device := &USBDevice{
		DeviceID:     "USB_12345678",
		DeviceName:   "Kingston USB",
		DeviceType:   "removable_disk",
		VolumeSerial: "12345678",
		DriveLetter:  "E:\\",
		ConnectedAt:  time.Now(),
	}

	if device.DeviceID != "USB_12345678" {
		t.Errorf("Expected DeviceID USB_12345678, got %s", device.DeviceID)
	}

	if device.DeviceName != "Kingston USB" {
		t.Errorf("Expected DeviceName Kingston USB, got %s", device.DeviceName)
	}

	if device.DeviceType != "removable_disk" {
		t.Errorf("Expected DeviceType removable_disk, got %s", device.DeviceType)
	}
}

func TestUSBEventStructure(t *testing.T) {
	event := USBEvent{
		Timestamp:    time.Now(),
		ComputerName: "TEST-PC",
		Username:     "testuser",
		DeviceID:     "USB_12345678",
		DeviceName:   "Kingston USB",
		DeviceType:   "removable_disk",
		EventType:    "connected",
		VolumeSerial: "12345678",
	}

	if event.EventType != "connected" {
		t.Errorf("Expected EventType connected, got %s", event.EventType)
	}

	if event.ComputerName != "TEST-PC" {
		t.Errorf("Expected ComputerName TEST-PC, got %s", event.ComputerName)
	}
}
