//go:build windows
// +build windows

package monitoring

import (
	"testing"
	"time"
)

func TestScreenshotMonitorCreation(t *testing.T) {
	monitor := NewScreenshotMonitor(
		"http://localhost:5000",
		"TEST-PC",
		"testuser",
		15,
		75,
		500,
		true,
		true,
		nil, // httpClient
	)

	if monitor == nil {
		t.Fatal("Expected monitor to be created, got nil")
	}

	if !monitor.enabled {
		t.Error("Expected monitor to be enabled")
	}

	if monitor.intervalMinutes != 15 {
		t.Errorf("Expected interval 15 minutes, got %d", monitor.intervalMinutes)
	}

	if monitor.quality != 75 {
		t.Errorf("Expected quality 75, got %d", monitor.quality)
	}
}

func TestScreenshotDataStructure(t *testing.T) {
	data := ScreenshotData{
		Timestamp:    time.Now(),
		ComputerName: "TEST-PC",
		Username:     "testuser",
		ScreenshotID: "test_screenshot_123",
		WindowTitle:  "Test Window",
		ProcessName:  "test.exe",
		FileSize:     102400,
		ImageData:    make([]byte, 1024),
	}

	if data.ScreenshotID != "test_screenshot_123" {
		t.Errorf("Expected screenshot ID test_screenshot_123, got %s", data.ScreenshotID)
	}

	if data.FileSize != 102400 {
		t.Errorf("Expected file size 102400, got %d", data.FileSize)
	}

	if len(data.ImageData) != 1024 {
		t.Errorf("Expected image data length 1024, got %d", len(data.ImageData))
	}
}

func TestGetForegroundWindowTitle(t *testing.T) {
	monitor := NewScreenshotMonitor(
		"http://localhost:5000",
		"TEST-PC",
		"testuser",
		15,
		75,
		500,
		false,
		true,
		nil, // httpClient
	)

	title := monitor.getForegroundWindowTitle()

	if title == "" {
		t.Skip("No foreground window detected (expected in test environment)")
	}

	t.Logf("Detected foreground window: %s", title)
}

func TestScreenshotBuffering(t *testing.T) {
	monitor := NewScreenshotMonitor(
		"http://localhost:5000",
		"TEST-PC",
		"testuser",
		15,
		75,
		500,
		false,
		false,
		nil, // httpClient
	)

	if monitor.uploadImmediately {
		t.Error("Expected uploadImmediately to be false")
	}

	if monitor.screenshotQueue == nil {
		t.Fatal("Expected screenshot queue to be initialized")
	}

	if cap(monitor.screenshotQueue) != 100 {
		t.Errorf("Expected queue capacity 100, got %d", cap(monitor.screenshotQueue))
	}

	testScreenshot := &ScreenshotData{
		Timestamp:    time.Now(),
		ComputerName: "TEST-PC",
		Username:     "testuser",
		ScreenshotID: "test_123",
		FileSize:     1024,
		ImageData:    make([]byte, 1024),
	}

	monitor.screenshotQueue <- testScreenshot

	if len(monitor.screenshotQueue) != 1 {
		t.Errorf("Expected 1 screenshot in queue, got %d", len(monitor.screenshotQueue))
	}
}
