//go:build windows
// +build windows

package monitoring

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/ctolnik/Office-Monitor/agent/httpclient"
)

const (
	SM_CXSCREEN    = 0
	SM_CYSCREEN    = 1
	SRCCOPY        = 0x00CC0020
	BI_RGB         = 0
	DIB_RGB_COLORS = 0
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

type ScreenshotMonitor struct {
	serverURL         string
	computerName      string
	username          string
	enabled           bool
	intervalMinutes   int
	quality           int
	maxSizeKB         int
	captureOnlyActive bool
	uploadImmediately bool
	stopChan          chan struct{}
	wg                sync.WaitGroup
	httpClient        *httpclient.Client
	mu                sync.RWMutex
	screenshotQueue   chan *ScreenshotData
	maxQueueSize      int
}

type ScreenshotData struct {
	Timestamp    time.Time `json:"timestamp"`
	ComputerName string    `json:"computer_name"`
	Username     string    `json:"username"`
	ScreenshotID string    `json:"screenshot_id"`
	WindowTitle  string    `json:"window_title"`
	ProcessName  string    `json:"process_name"`
	FileSize     int64     `json:"file_size"`
	ImageData    []byte    `json:"image_data"`
}

func NewScreenshotMonitor(serverURL, computerName, username string, intervalMinutes, quality, maxSizeKB int, captureOnlyActive, uploadImmediately bool, httpClient *httpclient.Client) *ScreenshotMonitor {
	maxQueue := 100
	return &ScreenshotMonitor{
		serverURL:         serverURL,
		computerName:      computerName,
		username:          username,
		enabled:           true,
		intervalMinutes:   intervalMinutes,
		quality:           quality,
		maxSizeKB:         maxSizeKB,
		captureOnlyActive: captureOnlyActive,
		uploadImmediately: uploadImmediately,
		stopChan:          make(chan struct{}),
		httpClient:        httpClient,
		screenshotQueue:   make(chan *ScreenshotData, maxQueue),
		maxQueueSize:      maxQueue,
	}
}

func (m *ScreenshotMonitor) Start() error {
	log.Printf("Screenshot Monitor started (interval: %d minutes, quality: %d, immediate: %v)",
		m.intervalMinutes, m.quality, m.uploadImmediately)

	m.wg.Add(1)
	go m.captureLoop()

	if !m.uploadImmediately {
		m.wg.Add(1)
		go m.uploadWorker()
	}

	return nil
}

func (m *ScreenshotMonitor) Stop() {
	log.Println("Stopping Screenshot Monitor...")
	close(m.stopChan)
	m.wg.Wait()
	log.Println("Screenshot Monitor stopped")
}

func (m *ScreenshotMonitor) captureLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Duration(m.intervalMinutes) * time.Minute)
	defer ticker.Stop()

	m.captureAndSend()

	for {
		select {
		case <-ticker.C:
			m.captureAndSend()
		case <-m.stopChan:
			return
		}
	}
}

func (m *ScreenshotMonitor) captureAndSend() {
	screenshot, err := m.captureScreenshot()
	if err != nil {
		log.Printf("Failed to capture screenshot: %v", err)
		return
	}

	if screenshot == nil {
		return
	}

	if m.uploadImmediately {
		if err := m.sendScreenshot(screenshot); err != nil {
			log.Printf("Failed to send screenshot: %v", err)
		}
	} else {
		select {
		case m.screenshotQueue <- screenshot:
		default:
			log.Printf("Screenshot queue full, dropping oldest screenshot")
		}
	}
}

func (m *ScreenshotMonitor) uploadWorker() {
	defer m.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	batch := make([]*ScreenshotData, 0, 10)

	for {
		select {
		case screenshot := <-m.screenshotQueue:
			batch = append(batch, screenshot)

			if len(batch) >= 10 {
				m.uploadBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				m.uploadBatch(batch)
				batch = batch[:0]
			}

		case <-m.stopChan:
			if len(batch) > 0 {
				m.uploadBatch(batch)
				batch = batch[:0]
			}

			for {
				select {
				case screenshot := <-m.screenshotQueue:
					batch = append(batch, screenshot)
					if len(batch) >= 10 {
						m.uploadBatch(batch)
						batch = batch[:0]
					}
				default:
					if len(batch) > 0 {
						m.uploadBatch(batch)
					}
					log.Println("Screenshot upload worker finished (queue drained)")
					return
				}
			}
		}
	}
}

func (m *ScreenshotMonitor) uploadBatch(screenshots []*ScreenshotData) {
	log.Printf("Uploading batch of %d screenshots", len(screenshots))

	for _, screenshot := range screenshots {
		if err := m.sendScreenshot(screenshot); err != nil {
			log.Printf("Failed to send screenshot %s: %v", screenshot.ScreenshotID, err)
		}
	}
}

func (m *ScreenshotMonitor) captureScreenshot() (*ScreenshotData, error) {
	windowTitle := m.getForegroundWindowTitle()

	if m.captureOnlyActive && windowTitle == "" {
		return nil, nil
	}

	img, err := m.takeScreenshot()
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: m.quality}
	if err := jpeg.Encode(&buf, img, opts); err != nil {
		return nil, fmt.Errorf("failed to encode screenshot: %w", err)
	}

	imageData := buf.Bytes()
	sizeKB := len(imageData) / 1024

	if m.maxSizeKB > 0 && sizeKB > m.maxSizeKB {
		log.Printf("Screenshot too large: %d KB (max: %d KB), skipping", sizeKB, m.maxSizeKB)
		return nil, nil
	}

	screenshotID := fmt.Sprintf("%s_%s_%d", m.computerName, m.username, time.Now().Unix())

	screenshot := &ScreenshotData{
		Timestamp:    time.Now(),
		ComputerName: m.computerName,
		Username:     m.username,
		ScreenshotID: screenshotID,
		WindowTitle:  windowTitle,
		ProcessName:  "",
		FileSize:     int64(len(imageData)),
		ImageData:    imageData,
	}

	log.Printf("Screenshot captured: %s (size: %d KB, window: %s)", screenshotID, sizeKB, windowTitle)

	return screenshot, nil
}

func (m *ScreenshotMonitor) takeScreenshot() (image.Image, error) {
	width, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	height, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)

	hDC, _, _ := procGetDC.Call(0)
	if hDC == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer procReleaseDC.Call(0, hDC)

	hMemDC, _, _ := procCreateCompatibleDC.Call(hDC)
	if hMemDC == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(hMemDC)

	hBitmap, _, _ := procCreateCompatibleBitmap.Call(hDC, width, height)
	if hBitmap == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(hBitmap)

	hOld, _, _ := procSelectObject.Call(hMemDC, hBitmap)
	if hOld == 0 {
		return nil, fmt.Errorf("SelectObject failed")
	}
	defer procSelectObject.Call(hMemDC, hOld)

	ret, _, _ := procBitBlt.Call(hMemDC, 0, 0, width, height, hDC, 0, 0, SRCCOPY)
	if ret == 0 {
		return nil, fmt.Errorf("BitBlt failed")
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

func (m *ScreenshotMonitor) getForegroundWindowTitle() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}

	textLen := 256
	buf := make([]uint16, textLen)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(textLen))

	return syscall.UTF16ToString(buf)
}

func (m *ScreenshotMonitor) sendScreenshot(screenshot *ScreenshotData) error {
	ctx := context.Background()
	return m.httpClient.PostJSON(ctx, "/api/screenshot", screenshot)
}
