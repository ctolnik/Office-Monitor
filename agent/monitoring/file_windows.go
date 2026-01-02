//go:build windows
// +build windows

package monitoring

import (
        "bytes"
        "encoding/json"
        "fmt"
        "log"
        "net/http"
        "os"
        "path/filepath"
        "sync"
        "syscall"
        "time"
        "unsafe"

        "github.com/ctolnik/Office-Monitor/agent/buffer"
        "golang.org/x/sys/windows"
)

const (
        FILE_NOTIFY_CHANGE_FILE_NAME  = 0x00000001
        FILE_NOTIFY_CHANGE_DIR_NAME   = 0x00000002
        FILE_NOTIFY_CHANGE_SIZE       = 0x00000008
        FILE_NOTIFY_CHANGE_LAST_WRITE = 0x00000010

        FILE_ACTION_ADDED    = 0x00000001
        FILE_ACTION_REMOVED  = 0x00000002
        FILE_ACTION_MODIFIED = 0x00000003
)

type FileMonitor struct {
        serverURL            string
        computerName         string
        username             string
        enabled              bool
        monitoredLocations   []string
        largeCopyThresholdMB int
        largeCopyFileCount   int
        detectExternalCopy   bool
        client               *http.Client
        stopChan             chan bool
        mu                   sync.RWMutex
        activityBuffer       map[string]*FileActivity
        lastAlertTime        time.Time
        alertCooldownSec     int
        eventBuffer          *buffer.EventBuffer
}

type FileActivity struct {
        Location       string
        FileCount      int
        TotalSizeBytes int64
        StartTime      time.Time
        Files          []string
}

type FileEvent struct {
        Timestamp       time.Time `json:"timestamp"`
        ComputerName    string    `json:"computer_name"`
        Username        string    `json:"username"`
        SourcePath      string    `json:"source_path"`
        DestinationPath string    `json:"destination_path"`
        FileSize        int64     `json:"file_size"`
        FileCount       int       `json:"file_count"`
        OperationType   string    `json:"operation_type"` // large_copy, external_copy
        IsUSBTarget     bool      `json:"is_usb_target"`
}

func NewFileMonitor(serverURL, computerName, username string, monitoredLocations []string, largeCopyThresholdMB, largeCopyFileCount int, detectExternalCopy bool, eventBuffer *buffer.EventBuffer) *FileMonitor {
        return &FileMonitor{
                serverURL:            serverURL,
                computerName:         computerName,
                username:             username,
                enabled:              true,
                monitoredLocations:   monitoredLocations,
                largeCopyThresholdMB: largeCopyThresholdMB,
                largeCopyFileCount:   largeCopyFileCount,
                detectExternalCopy:   detectExternalCopy,
                client:               &http.Client{Timeout: 30 * time.Second},
                stopChan:             make(chan bool),
                activityBuffer:       make(map[string]*FileActivity),
                alertCooldownSec:     60,
                eventBuffer:          eventBuffer,
        }
}

func (m *FileMonitor) Start() error {
        log.Println("File Monitor started")

        // Expand wildcard paths and start monitoring
        for _, location := range m.monitoredLocations {
                expandedPaths := expandWildcardPath(location)
                for _, path := range expandedPaths {
                        go m.monitorLocation(path)
                }
        }

        // Start activity analyzer
        go m.analyzeActivity()

        return nil
}

// expandWildcardPath expands paths like C:\Users\*\Documents to actual user paths
func expandWildcardPath(pattern string) []string {
        // First expand environment variables
        pattern = os.ExpandEnv(pattern)
        
        // If no wildcard, return as-is
        if !containsWildcard(pattern) {
                return []string{pattern}
        }
        
        // Find the wildcard position
        parts := splitPath(pattern)
        var prefix string
        var wildcardIdx int
        var suffix []string
        
        for i, part := range parts {
                if part == "*" || part == "?" || containsWildcard(part) {
                        wildcardIdx = i
                        suffix = parts[i+1:]
                        break
                }
                if prefix == "" {
                        prefix = part
                } else {
                        prefix = filepath.Join(prefix, part)
                }
        }
        
        // Read directory at prefix
        entries, err := os.ReadDir(prefix)
        if err != nil {
                log.Printf("Cannot read directory %s: %v", prefix, err)
                return nil
        }
        
        var result []string
        wildcardPart := parts[wildcardIdx]
        
        for _, entry := range entries {
                if !entry.IsDir() {
                        continue
                }
                
                name := entry.Name()
                
                // Skip system directories
                if name == "Public" || name == "Default" || name == "Default User" || name == "All Users" {
                        continue
                }
                
                // Match against wildcard pattern
                if wildcardPart == "*" || matchWildcard(wildcardPart, name) {
                        fullPath := filepath.Join(prefix, name)
                        if len(suffix) > 0 {
                                fullPath = filepath.Join(fullPath, filepath.Join(suffix...))
                        }
                        
                        // Check if final path exists
                        if _, err := os.Stat(fullPath); err == nil {
                                result = append(result, fullPath)
                        }
                }
        }
        
        if len(result) == 0 {
                log.Printf("No directories matched pattern: %s", pattern)
        }
        
        return result
}

func containsWildcard(s string) bool {
        for _, c := range s {
                if c == '*' || c == '?' {
                        return true
                }
        }
        return false
}

func splitPath(path string) []string {
        path = filepath.Clean(path)
        var parts []string
        
        for path != "" && path != "." {
                dir, file := filepath.Split(path)
                if file != "" {
                        parts = append([]string{file}, parts...)
                }
                path = filepath.Clean(dir)
                if path == dir {
                        if path != "" && path != "." {
                                parts = append([]string{path}, parts...)
                        }
                        break
                }
        }
        
        return parts
}

func matchWildcard(pattern, name string) bool {
        if pattern == "*" {
                return true
        }
        matched, _ := filepath.Match(pattern, name)
        return matched
}

func (m *FileMonitor) monitorLocation(location string) {
        log.Printf("Monitoring location: %s", location)

        // Convert to UTF16 for Windows API
        pathPtr, err := windows.UTF16PtrFromString(location)
        if err != nil {
                log.Printf("Failed to convert path: %v", err)
                return
        }

        // Open directory for monitoring
        handle, err := windows.CreateFile(
                pathPtr,
                windows.FILE_LIST_DIRECTORY,
                windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
                nil,
                windows.OPEN_EXISTING,
                windows.FILE_FLAG_BACKUP_SEMANTICS,
                0,
        )

        if err != nil {
                log.Printf("Failed to open directory %s: %v", location, err)
                return
        }
        defer windows.CloseHandle(handle)

        // Buffer for change notifications
        buffer := make([]byte, 64*1024)

        for {
                select {
                case <-m.stopChan:
                        return
                default:
                        var bytesReturned uint32

                        err := windows.ReadDirectoryChanges(
                                handle,
                                &buffer[0],
                                uint32(len(buffer)),
                                true, // watch subtree
                                FILE_NOTIFY_CHANGE_FILE_NAME|FILE_NOTIFY_CHANGE_SIZE|FILE_NOTIFY_CHANGE_LAST_WRITE,
                                &bytesReturned,
                                nil,
                                0,
                        )

                        if err != nil {
                                if err == syscall.ERROR_OPERATION_ABORTED {
                                        return
                                }
                                log.Printf("ReadDirectoryChanges error: %v", err)
                                time.Sleep(1 * time.Second)
                                continue
                        }

                        if bytesReturned == 0 {
                                continue
                        }

                        // Parse notifications
                        m.parseNotifications(buffer[:bytesReturned], location)
                }
        }
}

func (m *FileMonitor) parseNotifications(buffer []byte, location string) {
        offset := uint32(0)

        for {
                if offset >= uint32(len(buffer)) {
                        break
                }

                // FILE_NOTIFY_INFORMATION structure
                info := (*windows.FileNotifyInformation)(unsafe.Pointer(&buffer[offset]))

                if info.Action == FILE_ACTION_ADDED || info.Action == FILE_ACTION_MODIFIED {
                        nameLen := info.FileNameLength / 2
                        name := syscall.UTF16ToString((*[1 << 16]uint16)(unsafe.Pointer(&info.FileName))[:nameLen])

                        fullPath := filepath.Join(location, name)

                        // Get file info
                        if fileInfo, err := os.Stat(fullPath); err == nil && !fileInfo.IsDir() {
                                m.recordFileActivity(location, fullPath, fileInfo.Size())
                        }
                }

                if info.NextEntryOffset == 0 {
                        break
                }
                offset += info.NextEntryOffset
        }
}

func (m *FileMonitor) recordFileActivity(location, filePath string, size int64) {
        m.mu.Lock()
        defer m.mu.Unlock()

        activity, exists := m.activityBuffer[location]
        if !exists {
                activity = &FileActivity{
                        Location:  location,
                        StartTime: time.Now(),
                        Files:     make([]string, 0),
                }
                m.activityBuffer[location] = activity
        }

        activity.FileCount++
        activity.TotalSizeBytes += size
        activity.Files = append(activity.Files, filePath)

        // Keep only last 1000 files in memory
        if len(activity.Files) > 1000 {
                activity.Files = activity.Files[len(activity.Files)-1000:]
        }
}

func (m *FileMonitor) analyzeActivity() {
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()

        for {
                select {
                case <-ticker.C:
                        m.checkForLargeCopyActivity()
                case <-m.stopChan:
                        return
                }
        }
}

func (m *FileMonitor) checkForLargeCopyActivity() {
        m.mu.Lock()
        defer m.mu.Unlock()

        // Cooldown between alerts
        if time.Since(m.lastAlertTime).Seconds() < float64(m.alertCooldownSec) {
                return
        }

        for location, activity := range m.activityBuffer {
                duration := time.Since(activity.StartTime).Seconds()
                sizeMB := float64(activity.TotalSizeBytes) / 1024 / 1024

                // Check thresholds
                isLargeCopy := false

                if m.largeCopyThresholdMB > 0 && sizeMB > float64(m.largeCopyThresholdMB) {
                        isLargeCopy = true
                }

                if m.largeCopyFileCount > 0 && activity.FileCount > m.largeCopyFileCount {
                        isLargeCopy = true
                }

                if isLargeCopy {
                        log.Printf("Large copy detected: %s - %d files, %.2f MB in %.0f seconds",
                                location, activity.FileCount, sizeMB, duration)

                        event := FileEvent{
                                Timestamp:       time.Now(),
                                ComputerName:    m.computerName,
                                Username:        m.username,
                                SourcePath:      location,
                                DestinationPath: "unknown",
                                FileSize:        activity.TotalSizeBytes,
                                FileCount:       activity.FileCount,
                                OperationType:   "large_copy",
                                IsUSBTarget:     false,
                        }

                        if err := m.sendEvent(event); err != nil {
                                log.Printf("Failed to send file event: %v", err)
                        }

                        // Reset activity buffer for this location
                        delete(m.activityBuffer, location)
                        m.lastAlertTime = time.Now()
                }
        }

        // Clean up old activities (older than 5 minutes)
        for location, activity := range m.activityBuffer {
                if time.Since(activity.StartTime) > 5*time.Minute {
                        delete(m.activityBuffer, location)
                }
        }
}

func (m *FileMonitor) sendEvent(event FileEvent) error {
        // Use buffer if available (offline-ready)
        if m.eventBuffer != nil {
                return m.eventBuffer.Add("file", event)
        }

        // Fallback to direct send
        data, err := json.Marshal(event)
        if err != nil {
                return err
        }

        url := fmt.Sprintf("%s/api/file/event", m.serverURL)
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

func (m *FileMonitor) Stop() {
        m.enabled = false
        close(m.stopChan)
        log.Println("File Monitor stopped")
}

func (m *FileMonitor) GetStats() map[string]*FileActivity {
        m.mu.RLock()
        defer m.mu.RUnlock()

        stats := make(map[string]*FileActivity)
        for k, v := range m.activityBuffer {
                stats[k] = v
        }

        return stats
}
