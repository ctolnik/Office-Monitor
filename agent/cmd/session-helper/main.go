//go:build windows
// +build windows

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/ctolnik/Office-Monitor/agent/pkg/ipc"
	"github.com/ctolnik/Office-Monitor/agent/pkg/logger"
	"go.uber.org/zap"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procGetDC                      = user32.NewProc("GetDC")
	procReleaseDC                  = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC         = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap     = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject               = gdi32.NewProc("SelectObject")
	procBitBlt                     = gdi32.NewProc("BitBlt")
	procDeleteDC                   = gdi32.NewProc("DeleteDC")
	procDeleteObject               = gdi32.NewProc("DeleteObject")
	procGetDIBits                  = gdi32.NewProc("GetDIBits")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW             = user32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procGetLastInputInfo           = user32.NewProc("GetLastInputInfo")
	procGetTickCount               = kernel32.NewProc("GetTickCount")
)

const (
	SM_CXSCREEN                       = 0
	SM_CYSCREEN                       = 1
	SRCCOPY                           = 0x00CC0020
	BI_RGB                            = 0
	DIB_RGB_COLORS                    = 0
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
)

type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]uint32
}

type LASTINPUTINFO struct {
	CbSize uint32
	DwTime uint32
}

type SessionHelper struct {
	serverURL    string
	computerName string
	username     string
	sessionID    string

	activityInterval   time.Duration
	screenshotInterval time.Duration
	idleThresholdMin   int
	screenshotQuality  int
	screenshotMaxKB    int

	pipe *ipc.PipeClient
	log  *zap.Logger

	currentSegment *ipc.ActivitySegment
	segmentMu      sync.Mutex
	lastSendTime   time.Time

	stopChan chan struct{}
	wg       sync.WaitGroup

	stats struct {
		activitySent     int
		activityFailed   int
		screenshotSent   int
		screenshotFailed int
	}
}

func main() {
	serverURL := flag.String("server", "", "Server URL")
	computerName := flag.String("computer", "", "Computer name")
	username := flag.String("user", "", "Username")
	sessionID := flag.String("session", "0", "Session ID")
	activitySec := flag.Int("activity-interval", 30, "Activity poll interval in seconds")
	screenshotMin := flag.Int("screenshot-interval", 15, "Screenshot interval in minutes")
	idleMin := flag.Int("idle-threshold", 5, "Idle threshold in minutes")
	quality := flag.Int("quality", 75, "Screenshot JPEG quality")
	maxSizeKB := flag.Int("maxsize", 500, "Screenshot max size in KB")
	logFile := flag.String("log", "", "Log file path")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	if *serverURL == "" || *computerName == "" {
		fmt.Println("Usage: session-helper -server=URL -computer=NAME [-user=USER] [-session=ID]")
		os.Exit(1)
	}

	logCfg := logger.DefaultConfig()
	logCfg.Level = *logLevel
	logCfg.FilePath = *logFile
	logCfg.Console = *logFile == ""
	if err := logger.Init(logCfg); err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	if *username == "" {
		*username = os.Getenv("USERNAME")
	}

	helper := &SessionHelper{
		serverURL:          *serverURL,
		computerName:       *computerName,
		username:           *username,
		sessionID:          *sessionID,
		activityInterval:   time.Duration(*activitySec) * time.Second,
		screenshotInterval: time.Duration(*screenshotMin) * time.Minute,
		idleThresholdMin:   *idleMin,
		screenshotQuality:  *quality,
		screenshotMaxKB:    *maxSizeKB,
		pipe:               ipc.NewPipeClient(ipc.PipeName),
		log:                logger.WithComponent("session_helper"),
		stopChan:           make(chan struct{}),
		lastSendTime:       time.Now(),
	}

	helper.log.Info("Session helper starting",
		zap.String("server", *serverURL),
		zap.String("computer", *computerName),
		zap.String("user", *username),
		zap.String("session_id", *sessionID),
		zap.Int("activity_interval_sec", *activitySec),
		zap.Int("screenshot_interval_min", *screenshotMin),
	)

	helper.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	helper.Stop()
	helper.log.Info("Session helper stopped",
		zap.Int("activity_sent", helper.stats.activitySent),
		zap.Int("activity_failed", helper.stats.activityFailed),
		zap.Int("screenshot_sent", helper.stats.screenshotSent),
		zap.Int("screenshot_failed", helper.stats.screenshotFailed),
	)
}

func (h *SessionHelper) Start() {
	// Best-effort connect; senders will retry
	if err := h.pipe.Connect(); err != nil {
		h.log.Warn("Failed to connect to pipe, will retry on send", zap.Error(err))
	}

	h.wg.Add(2)
	go h.activityLoop()
	go h.screenshotLoop()
}

func (h *SessionHelper) Stop() {
	close(h.stopChan)
	h.wg.Wait()
	h.flushCurrentSegment()
}

func (h *SessionHelper) activityLoop() {
	defer h.wg.Done()

	ticker := time.NewTicker(h.activityInterval)
	defer ticker.Stop()

	h.checkAndUpdateActivity()

	for {
		select {
		case <-h.stopChan:
			return
		case <-ticker.C:
			h.checkAndUpdateActivity()
		}
	}
}

func (h *SessionHelper) screenshotLoop() {
        defer h.wg.Done()

        // Give the user desktop a bit of time to become available after logon/RDP connect.
        time.Sleep(5 * time.Second)

        ticker := time.NewTicker(h.screenshotInterval)
        defer ticker.Stop()

        // First capture after initial delay.
        h.captureAndSendScreenshot()

        for {
                select {
                case <-h.stopChan:
                        return
                case <-ticker.C:
                        h.captureAndSendScreenshot()
                }
        }
}

func (h *SessionHelper) checkAndUpdateActivity() {
	idleTime := h.getIdleTimeSec()
	state := h.determineState(idleTime)
	processName, windowTitle := h.getForegroundInfo()

	h.segmentMu.Lock()
	defer h.segmentMu.Unlock()

	if h.currentSegment == nil {
		h.startNewSegment(state, processName, windowTitle)
		return
	}

	shouldSwitch := h.currentSegment.State != state ||
		(state == "active" && h.currentSegment.ProcessName != processName)

	timeSinceLastSend := time.Since(h.lastSendTime).Seconds()
	shouldPeriodicSend := timeSinceLastSend >= 60 && h.currentSegment.DurationSec > 0

	if shouldSwitch {
		h.finalizeAndSendSegment()
		h.startNewSegment(state, processName, windowTitle)
	} else if shouldPeriodicSend {
		h.sendCurrentSegmentSnapshot()
	} else {
		h.currentSegment.TimestampEnd = time.Now()
		h.currentSegment.DurationSec = uint32(h.currentSegment.TimestampEnd.Sub(h.currentSegment.TimestampStart).Seconds())
	}
}

func (h *SessionHelper) startNewSegment(state, processName, windowTitle string) {
	now := time.Now()
	h.currentSegment = &ipc.ActivitySegment{
		TimestampStart: now,
		TimestampEnd:   now,
		DurationSec:    0,
		State:          state,
		ProcessName:    processName,
		WindowTitle:    windowTitle,
	}
}

func (h *SessionHelper) finalizeAndSendSegment() {
	if h.currentSegment == nil {
		return
	}

	h.currentSegment.TimestampEnd = time.Now()
	h.currentSegment.DurationSec = uint32(h.currentSegment.TimestampEnd.Sub(h.currentSegment.TimestampStart).Seconds())

	if h.currentSegment.DurationSec > 0 {
		h.sendActivitySegment(h.currentSegment)
	}
}

func (h *SessionHelper) sendCurrentSegmentSnapshot() {
	if h.currentSegment == nil {
		return
	}

	h.currentSegment.TimestampEnd = time.Now()
	h.currentSegment.DurationSec = uint32(h.currentSegment.TimestampEnd.Sub(h.currentSegment.TimestampStart).Seconds())

	snapshot := *h.currentSegment
	h.sendActivitySegment(&snapshot)
}

func (h *SessionHelper) flushCurrentSegment() {
	h.segmentMu.Lock()
	defer h.segmentMu.Unlock()
	h.finalizeAndSendSegment()
	h.currentSegment = nil
}

func (h *SessionHelper) sendActivitySegment(segment *ipc.ActivitySegment) {
	segment.WindowTitle = h.parseWindowTitle(segment.ProcessName, segment.WindowTitle)

	evt := ipc.Event{
		Type:      ipc.EventTypeActivity,
		Timestamp: time.Now(),
		SessionID: fmt.Sprintf("%s-%s", h.computerName, h.sessionID),
		Username:  h.username,
		Data:      *segment,
	}

	// best-effort reconnect
	if err := h.pipe.Send(evt); err != nil {
		_ = h.pipe.Connect()
		if err2 := h.pipe.Send(evt); err2 != nil {
			h.log.Error("Failed to send activity segment to service", zap.Error(err2))
			h.stats.activityFailed++
			return
		}
	}

	h.stats.activitySent++
	h.lastSendTime = time.Now()
	h.log.Info("Activity segment queued",
		zap.String("state", segment.State),
		zap.String("process", segment.ProcessName),
		zap.Uint32("duration_sec", segment.DurationSec),
		zap.Int("total_sent", h.stats.activitySent),
		zap.Int("total_failed", h.stats.activityFailed),
	)
}

func (h *SessionHelper) determineState(idleTimeSec int) string {
	if idleTimeSec < 0 {
		return "active"
	}

	idleThresholdSec := h.idleThresholdMin * 60
	offlineThresholdSec := 30 * 60

	if idleTimeSec < idleThresholdSec {
		return "active"
	} else if idleTimeSec < offlineThresholdSec {
		return "idle"
	}
	return "offline"
}

func (h *SessionHelper) getIdleTimeSec() int {
	var lastInputInfo LASTINPUTINFO
	lastInputInfo.CbSize = uint32(unsafe.Sizeof(lastInputInfo))

	ret, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lastInputInfo)))
	if ret == 0 {
		return -1
	}

	tickCount, _, _ := procGetTickCount.Call()
	idleTimeMs := uint32(tickCount) - lastInputInfo.DwTime

	return int(idleTimeMs / 1000)
}

func (h *SessionHelper) getForegroundInfo() (string, string) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return "unknown", ""
	}

	var processID uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&processID)))

	processName := h.getProcessName(processID)

	titleBuf := make([]uint16, 512)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&titleBuf[0])), uintptr(len(titleBuf)))
	windowTitle := syscall.UTF16ToString(titleBuf)

	return processName, windowTitle
}

func (h *SessionHelper) getProcessName(processID uint32) string {
	hProcess, _, _ := procOpenProcess.Call(PROCESS_QUERY_LIMITED_INFORMATION, 0, uintptr(processID))
	if hProcess == 0 {
		return "unknown"
	}
	defer procCloseHandle.Call(hProcess)

	var pathBuf [260]uint16
	pathSize := uint32(len(pathBuf))
	ret, _, _ := procQueryFullProcessImageNameW.Call(
		hProcess,
		0,
		uintptr(unsafe.Pointer(&pathBuf[0])),
		uintptr(unsafe.Pointer(&pathSize)),
	)
	if ret == 0 {
		return "unknown"
	}

	fullPath := syscall.UTF16ToString(pathBuf[:pathSize])
	parts := strings.Split(fullPath, "\\")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return fullPath
}

func (h *SessionHelper) parseWindowTitle(processName, windowTitle string) string {
	processLower := strings.ToLower(processName)

	if strings.Contains(processLower, "chrome") ||
		strings.Contains(processLower, "firefox") ||
		strings.Contains(processLower, "msedge") {
		return h.extractBrowserInfo(windowTitle)
	}

	return windowTitle
}

func (h *SessionHelper) extractBrowserInfo(title string) string {
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

func (h *SessionHelper) captureAndSendScreenshot() {
	processName, windowTitle := h.getForegroundInfo()

	img, err := h.takeScreenshot()
	if err != nil {
		h.log.Error("Screenshot capture failed", zap.Error(err))
		h.stats.screenshotFailed++
		return
	}

	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: h.screenshotQuality}
	if err := jpeg.Encode(&buf, img, opts); err != nil {
		h.log.Error("Screenshot encode failed", zap.Error(err))
		h.stats.screenshotFailed++
		return
	}

	imageData := buf.Bytes()
	sizeKB := len(imageData) / 1024

	if h.screenshotMaxKB > 0 && sizeKB > h.screenshotMaxKB {
		h.log.Warn("Screenshot too large, skipping",
			zap.Int("size_kb", sizeKB),
			zap.Int("max_kb", h.screenshotMaxKB),
		)
		return
	}

	screenshotID := fmt.Sprintf("%s_%s_%d", h.computerName, h.username, time.Now().Unix())
	sum := sha256.Sum256(imageData)

	begin := ipc.ScreenshotBegin{
		ScreenshotID: screenshotID,
		Timestamp:    time.Now(),
		ComputerName: h.computerName,
		Username:     h.username,
		SessionID:    fmt.Sprintf("%s-%s", h.computerName, h.sessionID),
		ProcessName:  processName,
		WindowTitle:  windowTitle,
		Quality:      h.screenshotQuality,
		MimeType:     "image/jpeg",
		TotalSize:    len(imageData),
		SHA256:       hex.EncodeToString(sum[:]),
	}

	if err := h.pipe.Send(ipc.Event{Type: ipc.EventTypeShotBegin, Timestamp: time.Now(), SessionID: begin.SessionID, Username: h.username, Data: begin}); err != nil {
		_ = h.pipe.Connect()
		if err2 := h.pipe.Send(ipc.Event{Type: ipc.EventTypeShotBegin, Timestamp: time.Now(), SessionID: begin.SessionID, Username: h.username, Data: begin}); err2 != nil {
			h.log.Error("Failed to send screenshot begin", zap.Error(err2))
			h.stats.screenshotFailed++
			return
		}
	}

	const chunkSize = 32 * 1024
	for off := 0; off < len(imageData); off += chunkSize {
		end := off + chunkSize
		if end > len(imageData) {
			end = len(imageData)
		}
		chunk := ipc.ScreenshotChunk{ScreenshotID: screenshotID, Offset: off, Data: imageData[off:end]}
		if err := h.pipe.Send(ipc.Event{Type: ipc.EventTypeShotChunk, Timestamp: time.Now(), SessionID: begin.SessionID, Username: h.username, Data: chunk}); err != nil {
			h.log.Error("Failed to send screenshot chunk", zap.Error(err), zap.Int("offset", off))
			h.stats.screenshotFailed++
			return
		}
	}

	if err := h.pipe.Send(ipc.Event{Type: ipc.EventTypeShotCommit, Timestamp: time.Now(), SessionID: begin.SessionID, Username: h.username, Data: ipc.ScreenshotCommit{ScreenshotID: screenshotID}}); err != nil {
		h.log.Error("Failed to send screenshot commit", zap.Error(err))
		h.stats.screenshotFailed++
		return
	}

	h.stats.screenshotSent++
	h.log.Info("Screenshot queued",
		zap.String("id", screenshotID),
		zap.Int("size_kb", sizeKB),
		zap.String("window", windowTitle),
		zap.Int("total_sent", h.stats.screenshotSent),
	)
}

func (h *SessionHelper) takeScreenshot() (image.Image, error) {
	width, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	height, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)

	if width == 0 || height == 0 {
		return nil, fmt.Errorf("no display available")
	}

        hDC, _, callErr := procGetDC.Call(0)
        if hDC == 0 {
                return nil, fmt.Errorf("GetDC failed: %v", callErr)
        }
	defer procReleaseDC.Call(0, hDC)

        hMemDC, _, callErr := procCreateCompatibleDC.Call(hDC)
        if hMemDC == 0 {
                return nil, fmt.Errorf("CreateCompatibleDC failed: %v", callErr)
        }
	defer procDeleteDC.Call(hMemDC)

        hBitmap, _, callErr := procCreateCompatibleBitmap.Call(hDC, width, height)
        if hBitmap == 0 {
                return nil, fmt.Errorf("CreateCompatibleBitmap failed: %v", callErr)
        }
	defer procDeleteObject.Call(hBitmap)

        hOld, _, callErr := procSelectObject.Call(hMemDC, hBitmap)
        if hOld == 0 {
                return nil, fmt.Errorf("SelectObject failed: %v", callErr)
        }
	defer procSelectObject.Call(hMemDC, hOld)

        ret, _, callErr := procBitBlt.Call(hMemDC, 0, 0, width, height, hDC, 0, 0, SRCCOPY)
        if ret == 0 {
                return nil, fmt.Errorf("BitBlt failed: %v", callErr)
        }

	var bi BITMAPINFO
	bi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bi.BmiHeader))
	bi.BmiHeader.BiWidth = int32(width)
	bi.BmiHeader.BiHeight = -int32(height)
	bi.BmiHeader.BiPlanes = 1
	bi.BmiHeader.BiBitCount = 32
	bi.BmiHeader.BiCompression = BI_RGB

	bitmapDataSize := uintptr(width * height * 4)
	bitmapData := make([]byte, bitmapDataSize)

	ret, _, _ = procGetDIBits.Call(
		hMemDC,
		hBitmap,
		0,
		height,
		uintptr(unsafe.Pointer(&bitmapData[0])),
		uintptr(unsafe.Pointer(&bi)),
		DIB_RGB_COLORS,
	)
	if ret == 0 {
		return nil, fmt.Errorf("GetDIBits failed")
	}

	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			i := (y*int(width) + x) * 4
			img.Set(x, y, color.RGBA{
				R: bitmapData[i+2],
				G: bitmapData[i+1],
				B: bitmapData[i+0],
				A: 255,
			})
		}
	}

	return img, nil
}

var _ = winio.ListenPipe
