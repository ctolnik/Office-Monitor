//go:build windows
// +build windows

package monitoring

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ctolnik/Office-Monitor/agent/buffer"
	"golang.org/x/sys/windows"
)

const (
	DBT_DEVICEARRIVAL        = 0x8000
	DBT_DEVICEREMOVECOMPLETE = 0x8004
	DBT_DEVTYP_VOLUME        = 0x00000002
)

type USBMonitor struct {
	serverURL         string
	computerName      string
	username          string
	enabled           bool
	shadowCopyEnabled bool
	shadowCopyDest    string
	copyExtensions    []string
	excludePatterns   []string
	connectedDevices  map[string]*USBDevice
	mu                sync.RWMutex
	client            *http.Client
	eventBuffer       *buffer.EventBuffer
}

type USBDevice struct {
	DeviceID     string    `json:"device_id"`
	DeviceName   string    `json:"device_name"`
	DeviceType   string    `json:"device_type"`
	VolumeSerial string    `json:"volume_serial"`
	DriveLetter  string    `json:"drive_letter"`
	ConnectedAt  time.Time `json:"connected_at"`
}

type USBEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	ComputerName string    `json:"computer_name"`
	Username     string    `json:"username"`
	DeviceID     string    `json:"device_id"`
	DeviceName   string    `json:"device_name"`
	DeviceType   string    `json:"device_type"`
	EventType    string    `json:"event_type"` // connected, disconnected
	VolumeSerial string    `json:"volume_serial"`
}

func NewUSBMonitor(serverURL, computerName, username string, shadowCopyEnabled bool, shadowCopyDest string, copyExtensions, excludePatterns []string, eventBuffer *buffer.EventBuffer) *USBMonitor {
	return &USBMonitor{
		serverURL:         serverURL,
		computerName:      computerName,
		username:          username,
		enabled:           true,
		shadowCopyEnabled: shadowCopyEnabled,
		shadowCopyDest:    shadowCopyDest,
		copyExtensions:    copyExtensions,
		excludePatterns:   excludePatterns,
		connectedDevices:  make(map[string]*USBDevice),
		client:            &http.Client{Timeout: 30 * time.Second},
		eventBuffer:       eventBuffer,
	}
}

func (m *USBMonitor) Start() error {
	log.Println("USB Monitor started")

	// Initial scan for already connected USB drives
	m.scanExistingDrives()

	// Start monitoring for new connections
	go m.monitorDriveChanges()

	return nil
}

func (m *USBMonitor) scanExistingDrives() {
	drives := m.getRemovableDrives()
	for _, drive := range drives {
		device := m.getDriveInfo(drive)
		if device != nil {
			m.mu.Lock()
			m.connectedDevices[device.DriveLetter] = device
			m.mu.Unlock()

			log.Printf("Found existing USB drive: %s (%s)", device.DriveLetter, device.DeviceName)
		}
	}
}

func (m *USBMonitor) getRemovableDrives() []string {
	var drives []string

	for letter := 'A'; letter <= 'Z'; letter++ {
		drive := fmt.Sprintf("%c:\\", letter)
		driveType := windows.GetDriveType(windows.StringToUTF16Ptr(drive))

		if driveType == windows.DRIVE_REMOVABLE {
			drives = append(drives, drive)
		}
	}

	return drives
}

func (m *USBMonitor) getDriveInfo(drivePath string) *USBDevice {
	volumeName := make([]uint16, 256)
	volumeSerial := uint32(0)
	maxComponentLength := uint32(0)
	fileSystemFlags := uint32(0)
	fileSystemName := make([]uint16, 256)

	drivePtr := windows.StringToUTF16Ptr(drivePath)

	err := windows.GetVolumeInformation(
		drivePtr,
		&volumeName[0],
		uint32(len(volumeName)),
		&volumeSerial,
		&maxComponentLength,
		&fileSystemFlags,
		&fileSystemName[0],
		uint32(len(fileSystemName)),
	)

	if err != nil {
		return nil
	}

	device := &USBDevice{
		DeviceID:     fmt.Sprintf("USB_%X", volumeSerial),
		DeviceName:   windows.UTF16ToString(volumeName),
		DeviceType:   "removable_disk",
		VolumeSerial: fmt.Sprintf("%X", volumeSerial),
		DriveLetter:  drivePath,
		ConnectedAt:  time.Now(),
	}

	if device.DeviceName == "" {
		device.DeviceName = "USB Drive"
	}

	return device
}

func (m *USBMonitor) monitorDriveChanges() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !m.enabled {
			continue
		}

		currentDrives := m.getRemovableDrives()
		currentMap := make(map[string]bool)

		for _, drive := range currentDrives {
			currentMap[drive] = true

			m.mu.RLock()
			_, exists := m.connectedDevices[drive]
			m.mu.RUnlock()

			if !exists {
				device := m.getDriveInfo(drive)
				if device != nil {
					m.handleDeviceConnected(device)
				}
			}
		}

		m.mu.Lock()
		for drive, device := range m.connectedDevices {
			if !currentMap[drive] {
				m.handleDeviceDisconnected(device)
				delete(m.connectedDevices, drive)
			}
		}
		m.mu.Unlock()
	}
}

func (m *USBMonitor) handleDeviceConnected(device *USBDevice) {
	m.mu.Lock()
	m.connectedDevices[device.DriveLetter] = device
	m.mu.Unlock()

	log.Printf("USB device connected: %s (%s) - %s", device.DeviceName, device.DriveLetter, device.VolumeSerial)

	// Send event to server
	event := USBEvent{
		Timestamp:    time.Now(),
		ComputerName: m.computerName,
		Username:     m.username,
		DeviceID:     device.DeviceID,
		DeviceName:   device.DeviceName,
		DeviceType:   device.DeviceType,
		EventType:    "connected",
		VolumeSerial: device.VolumeSerial,
	}

	if err := m.sendEvent(event); err != nil {
		log.Printf("Failed to send USB connection event: %v", err)
	}

	// Start shadow copy if enabled
	if m.shadowCopyEnabled {
		go m.shadowCopyDrive(device)
	}
}

func (m *USBMonitor) handleDeviceDisconnected(device *USBDevice) {
	log.Printf("USB device disconnected: %s (%s)", device.DeviceName, device.DriveLetter)

	event := USBEvent{
		Timestamp:    time.Now(),
		ComputerName: m.computerName,
		Username:     m.username,
		DeviceID:     device.DeviceID,
		DeviceName:   device.DeviceName,
		DeviceType:   device.DeviceType,
		EventType:    "disconnected",
		VolumeSerial: device.VolumeSerial,
	}

	if err := m.sendEvent(event); err != nil {
		log.Printf("Failed to send USB disconnection event: %v", err)
	}
}

func (m *USBMonitor) shadowCopyDrive(device *USBDevice) {
	log.Printf("Starting shadow copy for %s to %s", device.DriveLetter, m.shadowCopyDest)

	destPath := filepath.Join(m.shadowCopyDest, m.computerName, device.VolumeSerial, time.Now().Format("2006-01-02_150405"))

	if err := os.MkdirAll(destPath, 0755); err != nil {
		log.Printf("Failed to create shadow copy directory: %v", err)
		return
	}

	fileCount := 0
	totalSize := int64(0)

	err := filepath.Walk(device.DriveLetter, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			// Check exclude patterns
			for _, pattern := range m.excludePatterns {
				if matched, _ := filepath.Match(pattern, info.Name()); matched {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check if file extension is in copy list (if specified)
		if len(m.copyExtensions) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			found := false
			for _, allowedExt := range m.copyExtensions {
				if ext == strings.ToLower(allowedExt) {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}

		// Check exclude patterns for files
		for _, pattern := range m.excludePatterns {
			if matched, _ := filepath.Match(pattern, info.Name()); matched {
				return nil
			}
		}

		// Copy file
		relPath, _ := filepath.Rel(device.DriveLetter, path)
		destFile := filepath.Join(destPath, relPath)

		if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
			return nil
		}

		if err := m.copyFile(path, destFile); err != nil {
			log.Printf("Failed to copy %s: %v", path, err)
			return nil
		}

		fileCount++
		totalSize += info.Size()

		if fileCount%100 == 0 {
			log.Printf("Shadow copy progress: %d files, %.2f MB", fileCount, float64(totalSize)/1024/1024)
		}

		return nil
	})

	if err != nil {
		log.Printf("Shadow copy error: %v", err)
	}

	log.Printf("Shadow copy completed: %d files, %.2f MB copied to %s", fileCount, float64(totalSize)/1024/1024, destPath)
}

func (m *USBMonitor) copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func (m *USBMonitor) sendEvent(event USBEvent) error {
	// Use buffer if available (offline-ready)
	if m.eventBuffer != nil {
		return m.eventBuffer.Add("usb", event)
	}

	// Fallback to direct send
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/usb/event", m.serverURL)
	resp, err := m.client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}

func (m *USBMonitor) Stop() {
	m.enabled = false
	log.Println("USB Monitor stopped")
}

func (m *USBMonitor) GetConnectedDevices() []*USBDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*USBDevice, 0, len(m.connectedDevices))
	for _, device := range m.connectedDevices {
		devices = append(devices, device)
	}

	return devices
}
