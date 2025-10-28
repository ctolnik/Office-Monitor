// +build windows

package monitoring

import (
        "testing"
        "time"
)

func TestFileMonitorCreation(t *testing.T) {
        locations := []string{"C:\\Users", "C:\\Documents"}
        
        monitor := NewFileMonitor(
                "http://localhost:5000",
                "TEST-PC",
                "testuser",
                locations,
                100,  // 100 MB threshold
                1000, // 1000 files threshold
                true,
        )
        
        if monitor == nil {
                t.Fatal("Failed to create File monitor")
        }
}

func TestFileActivityTracking(t *testing.T) {
        monitor := NewFileMonitor(
                "http://localhost:5000",
                "TEST-PC",
                "testuser",
                []string{"C:\\Test"},
                50,
                500,
                false,
        )
        
        // Manually create activity to test GetStats
        monitor.mu.Lock()
        monitor.activityBuffer["C:\\Test"] = &FileActivity{
                Location:       "C:\\Test",
                FileCount:      2,
                TotalSizeBytes: 3 * 1024 * 1024,
                StartTime:      time.Now(),
                Files:          []string{"file1.txt", "file2.txt"},
        }
        monitor.mu.Unlock()
        
        stats := monitor.GetStats()
        
        if len(stats) != 1 {
                t.Errorf("Expected 1 activity location, got %d", len(stats))
        }
        
        activity := stats["C:\\Test"]
        if activity == nil {
                t.Fatal("Activity for C:\\Test not found")
        }
        
        if activity.FileCount != 2 {
                t.Errorf("Expected 2 files, got %d", activity.FileCount)
        }
        
        expectedSize := int64(3 * 1024 * 1024)
        if activity.TotalSizeBytes != expectedSize {
                t.Errorf("Expected size %d, got %d", expectedSize, activity.TotalSizeBytes)
        }
}

func TestFileEventStructure(t *testing.T) {
        event := FileEvent{
                Timestamp:       time.Now(),
                ComputerName:    "TEST-PC",
                Username:        "testuser",
                SourcePath:      "C:\\Users\\Documents",
                DestinationPath: "D:\\Backup",
                FileSize:        262144000,
                FileCount:       1500,
                OperationType:   "large_copy",
                IsUSBTarget:     false,
        }
        
        if event.OperationType != "large_copy" {
                t.Errorf("Expected operation type large_copy, got %s", event.OperationType)
        }
        
        if event.IsUSBTarget {
                t.Error("Expected IsUSBTarget to be false")
        }
        
        if event.FileCount != 1500 {
                t.Errorf("Expected 1500 files, got %d", event.FileCount)
        }
}
