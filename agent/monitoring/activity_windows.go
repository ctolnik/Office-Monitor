//go:build windows
// +build windows

package monitoring

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/ctolnik/Office-Monitor/agent/buffer"
	"github.com/ctolnik/Office-Monitor/agent/httpclient"
	"golang.org/x/sys/windows"
)

// ActivityTracker monitors active window and process
type ActivityTracker struct {
	client         *httpclient.Client
	eventBuffer    *buffer.EventBuffer
	computerName   string
	username       string
	intervalSec    int
	currentWindow  string
	currentProcess string
	startTime      time.Time
	stopChan       chan struct{}
}

// ActivityEvent represents an activity event to send to server
type ActivityEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	ComputerName string    `json:"computer_name"`
	Username     string    `json:"username"`
	WindowTitle  string    `json:"window_title"`
	ProcessName  string    `json:"process_name"`
	ProcessPath  string    `json:"process_path"`
	DurationSec  int       `json:"duration_seconds"`
	IdleTimeSec  int       `json:"idle_time_seconds"`
}

// NewActivityTracker creates a new activity tracker
func NewActivityTracker(client *httpclient.Client, eventBuffer *buffer.EventBuffer, computerName string, intervalSec int) (*ActivityTracker, error) {
	username := os.Getenv("USERNAME")
	if username == "" {
		username = "unknown"
	}

	return &ActivityTracker{
		client:       client,
		eventBuffer:  eventBuffer,
		computerName: computerName,
		username:     username,
		intervalSec:  intervalSec,
		stopChan:     make(chan struct{}),
	}, nil
}

// Start begins monitoring activity
func (t *ActivityTracker) Start(ctx context.Context) error {
	log.Println("Activity tracker started")

	ticker := time.NewTicker(time.Duration(t.intervalSec) * time.Second)
	defer ticker.Stop()

	t.startTime = time.Now()

	for {
		select {
		case <-ctx.Done():
			log.Println("Activity tracker context cancelled")
			return ctx.Err()
		case <-t.stopChan:
			log.Println("Activity tracker stop signal received")
			return nil
		case <-ticker.C:
			if err := t.captureActivity(ctx); err != nil {
				log.Printf("Error capturing activity: %v", err)
			}
		}
	}
}

// Stop stops the activity tracker
func (t *ActivityTracker) Stop() {
	close(t.stopChan)
}

// captureActivity captures current window and process info
func (t *ActivityTracker) captureActivity(ctx context.Context) error {
	windowTitle, processName, processPath, err := t.getForegroundWindowInfo()
	if err != nil {
		return fmt.Errorf("failed to get window info: %w", err)
	}

	// Only send if window/process changed
	if windowTitle == t.currentWindow && processName == t.currentProcess {
		return nil
	}

	// Calculate duration of previous activity
	duration := 0
	if !t.startTime.IsZero() {
		duration = int(time.Since(t.startTime).Seconds())
	}

	// Get idle time
	idleTime := t.getIdleTime()

	event := ActivityEvent{
		Timestamp:    time.Now(),
		ComputerName: t.computerName,
		Username:     t.username,
		WindowTitle:  windowTitle,
		ProcessName:  processName,
		ProcessPath:  processPath,
		DurationSec:  duration,
		IdleTimeSec:  int(idleTime.Seconds()),
	}

	// Add to buffer (will be sent when server is available)
	if t.eventBuffer != nil {
		if err := t.eventBuffer.Add("activity", event); err != nil {
			return fmt.Errorf("failed to buffer activity: %w", err)
		}
	} else {
		// Fallback to direct send if no buffer
		if err := t.client.PostJSON(ctx, "/api/activity", event); err != nil {
			return fmt.Errorf("failed to send activity: %w", err)
		}
	}

	// Update current state
	t.currentWindow = windowTitle
	t.currentProcess = processName
	t.startTime = time.Now()

	log.Printf("Activity captured: %s - %s", processName, windowTitle)
	return nil
}

// getForegroundWindowInfo gets the title and process of the foreground window
func (t *ActivityTracker) getForegroundWindowInfo() (windowTitle, processName, processPath string, err error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return "", "", "", fmt.Errorf("no foreground window")
	}

	// Get window title
	titleBuffer := make([]uint16, 256)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&titleBuffer[0])), 256)
	windowTitle = syscall.UTF16ToString(titleBuffer)

	// Get process ID
	var processID uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&processID)))

	// Open process
	hProcess, _, _ := procOpenProcess.Call(
		windows.PROCESS_QUERY_LIMITED_INFORMATION,
		0,
		uintptr(processID),
	)
	if hProcess == 0 {
		return windowTitle, "", "", fmt.Errorf("failed to open process")
	}
	defer procCloseHandle.Call(hProcess)

	// Get process image name
	var size uint32 = 260
	exePath := make([]uint16, size)
	ret, _, _ := procQueryFullProcessImageName.Call(
		hProcess,
		0,
		uintptr(unsafe.Pointer(&exePath[0])),
		uintptr(unsafe.Pointer(&size)),
	)

	if ret != 0 {
		processPath = syscall.UTF16ToString(exePath)
		processName = filepath.Base(processPath)
	}

	return windowTitle, processName, processPath, nil
}

// getIdleTime returns the time since last user input
func (t *ActivityTracker) getIdleTime() time.Duration {
	modUser32 := windows.NewLazySystemDLL("user32.dll")
	modKernel32 := windows.NewLazySystemDLL("kernel32.dll")
	procGetLastInputInfo := modUser32.NewProc("GetLastInputInfo")
	procGetTickCount := modKernel32.NewProc("GetTickCount")

	type LASTINPUTINFO struct {
		cbSize uint32
		dwTime uint32
	}

	var lastInputInfo LASTINPUTINFO
	lastInputInfo.cbSize = uint32(unsafe.Sizeof(lastInputInfo))

	ret, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lastInputInfo)))
	if ret == 0 {
		return 0
	}

	// Get current tick count
	currentTick, _, _ := procGetTickCount.Call()

	// Calculate idle time
	idleMillis := uint32(currentTick) - lastInputInfo.dwTime
	return time.Duration(idleMillis) * time.Millisecond
}
