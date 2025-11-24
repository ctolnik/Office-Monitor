// +build windows

package monitoring

import (
        "bytes"
        "encoding/json"
        "fmt"
        "log"
        "net/http"
        "strings"
        "sync"
        "syscall"
        "time"
        "unsafe"
)

var (
        procGetLastInputInfo = modUser32.NewProc("GetLastInputInfo")
        procGetTickCount     = modKernel32.NewProc("GetTickCount")
)

type LASTINPUTINFO struct {
        CbSize uint32
        DwTime uint32
}

type ActivityState string

const (
        StateActive  ActivityState = "active"
        StateIdle    ActivityState = "idle"
        StateOffline ActivityState = "offline"
)

type ActivitySegment struct {
        TimestampStart time.Time     `json:"timestamp_start"`
        TimestampEnd   time.Time     `json:"timestamp_end"`
        DurationSec    uint32        `json:"duration_sec"`
        State          ActivityState `json:"state"`
        ComputerName   string        `json:"computer_name"`
        Username       string        `json:"username"`
        ProcessName    string        `json:"process_name"`
        WindowTitle    string        `json:"window_title"`
        SessionID      string        `json:"session_id"`
}

type ActivityTracker struct {
        serverURL        string
        computerName     string
        username         string
        enabled          bool
        idleThresholdMin int
        pollIntervalSec  int
        stopChan         chan struct{}
        wg               sync.WaitGroup
        mu               sync.RWMutex
        currentSegment   *ActivitySegment
        sessionID        string
        client           *http.Client
}

func NewActivityTracker(serverURL, computerName, username string, idleThresholdMin, pollIntervalSec int) *ActivityTracker {
        sessionID := fmt.Sprintf("%s-%d", computerName, time.Now().Unix())

        return &ActivityTracker{
                serverURL:        serverURL,
                computerName:     computerName,
                username:         username,
                enabled:          true,
                idleThresholdMin: idleThresholdMin,
                pollIntervalSec:  pollIntervalSec,
                stopChan:         make(chan struct{}),
                sessionID:        sessionID,
                client:           &http.Client{Timeout: 30 * time.Second},
        }
}

func (at *ActivityTracker) Start() error {
        log.Printf("ActivityTracker started (idle threshold: %d min, poll interval: %d sec)",
                at.idleThresholdMin, at.pollIntervalSec)

        at.wg.Add(1)
        go at.trackActivity()

        return nil
}

func (at *ActivityTracker) Stop() {
        log.Println("Stopping ActivityTracker...")
        close(at.stopChan)
        at.wg.Wait()

        at.flushCurrentSegment()
        log.Println("ActivityTracker stopped")
}

func (at *ActivityTracker) trackActivity() {
        defer at.wg.Done()

        ticker := time.NewTicker(time.Duration(at.pollIntervalSec) * time.Second)
        defer ticker.Stop()

        for {
                select {
                case <-at.stopChan:
                        return
                case <-ticker.C:
                        at.checkAndUpdateState()
                }
        }
}

func (at *ActivityTracker) checkAndUpdateState() {
        idleTime := at.getIdleTimeSec()
        currentState := at.determineState(idleTime)

        hwnd := at.getForegroundWindow()
        processName, windowTitle := at.getWindowInfo(hwnd)

        at.mu.Lock()
        defer at.mu.Unlock()

        if at.currentSegment == nil {
                at.startNewSegment(currentState, processName, windowTitle)
                return
        }

        if at.shouldSwitchSegment(currentState, processName, windowTitle) {
                at.finalizeCurrentSegment()
                at.startNewSegment(currentState, processName, windowTitle)
        } else {
                at.currentSegment.TimestampEnd = time.Now()
                at.currentSegment.DurationSec = uint32(at.currentSegment.TimestampEnd.Sub(at.currentSegment.TimestampStart).Seconds())
        }
}

func (at *ActivityTracker) determineState(idleTimeSec int) ActivityState {
        idleThresholdSec := at.idleThresholdMin * 60
        offlineThresholdSec := 30 * 60

        if idleTimeSec < idleThresholdSec {
                return StateActive
        } else if idleTimeSec < offlineThresholdSec {
                return StateIdle
        }
        return StateOffline
}

func (at *ActivityTracker) shouldSwitchSegment(newState ActivityState, newProcess, newTitle string) bool {
        if at.currentSegment.State != newState {
                return true
        }

        if newState == StateActive {
                if at.currentSegment.ProcessName != newProcess {
                        return true
                }
        }

        return false
}

func (at *ActivityTracker) startNewSegment(state ActivityState, processName, windowTitle string) {
        now := time.Now()

        at.currentSegment = &ActivitySegment{
                TimestampStart: now,
                TimestampEnd:   now,
                DurationSec:    0,
                State:          state,
                ComputerName:   at.computerName,
                Username:       at.username,
                ProcessName:    processName,
                WindowTitle:    windowTitle,
                SessionID:      at.sessionID,
        }
}

func (at *ActivityTracker) finalizeCurrentSegment() {
        if at.currentSegment == nil {
                return
        }

        at.currentSegment.TimestampEnd = time.Now()
        at.currentSegment.DurationSec = uint32(at.currentSegment.TimestampEnd.Sub(at.currentSegment.TimestampStart).Seconds())

        if at.currentSegment.DurationSec > 0 {
                at.sendSegment(at.currentSegment)
        }
}

func (at *ActivityTracker) flushCurrentSegment() {
        at.mu.Lock()
        defer at.mu.Unlock()

        at.finalizeCurrentSegment()
        at.currentSegment = nil
}

func (at *ActivityTracker) sendSegment(segment *ActivitySegment) {
        segment.WindowTitle = at.parseWindowTitle(segment.ProcessName, segment.WindowTitle)

        data, err := json.Marshal(segment)
        if err != nil {
                log.Printf("Failed to marshal activity segment: %v", err)
                return
        }

        url := fmt.Sprintf("%s/api/activity/segment", at.serverURL)
        resp, err := at.client.Post(url, "application/json", bytes.NewBuffer(data))
        if err != nil {
                log.Printf("Failed to send activity segment: %v", err)
                return
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
                log.Printf("Server returned non-OK status for activity segment: %d", resp.StatusCode)
        }
}

func (at *ActivityTracker) parseWindowTitle(processName, windowTitle string) string {
        processLower := strings.ToLower(processName)

        if strings.Contains(processLower, "chrome") ||
                strings.Contains(processLower, "firefox") ||
                strings.Contains(processLower, "msedge") {
                return at.extractBrowserInfo(windowTitle)
        }

        return windowTitle
}

func (at *ActivityTracker) extractBrowserInfo(title string) string {
        parts := strings.Split(title, " - ")
        if len(parts) < 2 {
                return title
        }

        pageName := parts[0]
        
        for i := len(parts) - 1; i >= 0; i-- {
                part := strings.TrimSpace(parts[i])
                
                if strings.Contains(part, "Chrome") || 
                   strings.Contains(part, "Firefox") || 
                   strings.Contains(part, "Edge") ||
                   strings.Contains(part, "Mozilla") {
                        continue
                }
                
                if strings.Contains(part, ".") && 
                   !strings.Contains(part, " ") && 
                   (strings.HasPrefix(part, "www.") || 
                    strings.Contains(part, "://") ||
                    len(strings.Split(part, ".")) >= 2) {
                        
                        url := part
                        if strings.Contains(url, "://") {
                                urlParts := strings.Split(url, "://")
                                if len(urlParts) == 2 {
                                        url = urlParts[1]
                                }
                        }
                        
                        url = strings.Split(url, "/")[0]
                        url = strings.Split(url, "?")[0]
                        
                        return fmt.Sprintf("%s — %s", pageName, url)
                }
        }

        return title
}

func (at *ActivityTracker) getIdleTimeSec() int {
        var lastInputInfo LASTINPUTINFO
        lastInputInfo.CbSize = uint32(unsafe.Sizeof(lastInputInfo))

        ret, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lastInputInfo)))
        if ret == 0 {
                return 0
        }

        tickCount, _, _ := procGetTickCount.Call()
        idleTimeMs := uint32(tickCount) - lastInputInfo.DwTime

        return int(idleTimeMs / 1000)
}

func (at *ActivityTracker) getForegroundWindow() uintptr {
        hwnd, _, _ := procGetForegroundWindow.Call()
        return hwnd
}

func (at *ActivityTracker) getWindowInfo(hwnd uintptr) (string, string) {
        if hwnd == 0 {
                return "unknown", ""
        }

        var processID uint32
        procGetWindowThreadProcessId.Call(
                hwnd,
                uintptr(unsafe.Pointer(&processID)),
        )

        processName := at.getProcessName(processID)

        titleBuf := make([]uint16, 512)
        procGetWindowTextW.Call(
                hwnd,
                uintptr(unsafe.Pointer(&titleBuf[0])),
                uintptr(len(titleBuf)),
        )

        windowTitle := syscall.UTF16ToString(titleBuf)

        return processName, windowTitle
}

func (at *ActivityTracker) getProcessName(processID uint32) string {
        const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000

        hProcess, _, _ := procOpenProcess.Call(
                PROCESS_QUERY_LIMITED_INFORMATION,
                0,
                uintptr(processID),
        )

        if hProcess == 0 {
                return "unknown"
        }
        defer procCloseHandle.Call(hProcess)

        var size uint32 = 260
        nameBuf := make([]uint16, size)

        ret, _, _ := procQueryFullProcessImageName.Call(
                hProcess,
                0,
                uintptr(unsafe.Pointer(&nameBuf[0])),
                uintptr(unsafe.Pointer(&size)),
        )

        if ret == 0 {
                return "unknown"
        }

        fullPath := syscall.UTF16ToString(nameBuf[:size])

        parts := strings.Split(fullPath, "\\")
        if len(parts) > 0 {
                return parts[len(parts)-1]
        }

        return "unknown"
}
