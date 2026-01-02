//go:build windows
// +build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	gdi32                    = syscall.NewLazyDLL("gdi32.dll")
	procGetSystemMetrics     = user32.NewProc("GetSystemMetrics")
	procGetDC                = user32.NewProc("GetDC")
	procReleaseDC            = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC   = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject         = gdi32.NewProc("SelectObject")
	procBitBlt               = gdi32.NewProc("BitBlt")
	procDeleteDC             = gdi32.NewProc("DeleteDC")
	procDeleteObject         = gdi32.NewProc("DeleteObject")
	procGetDIBits            = gdi32.NewProc("GetDIBits")
	procGetForegroundWindow  = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
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

func main() {
	serverURL := flag.String("server", "", "Server URL")
	computerName := flag.String("computer", "", "Computer name")
	username := flag.String("user", "", "Username")
	interval := flag.Int("interval", 15, "Interval in minutes")
	quality := flag.Int("quality", 75, "JPEG quality (1-100)")
	maxSizeKB := flag.Int("maxsize", 500, "Max size in KB")
	logFile := flag.String("log", "", "Log file path")
	flag.Parse()

	if *serverURL == "" || *computerName == "" {
		fmt.Println("Usage: screenshot-helper -server=URL -computer=NAME [-user=USER] [-interval=15] [-quality=75]")
		os.Exit(1)
	}

	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			log.SetOutput(f)
			defer f.Close()
		}
	}

	if *username == "" {
		*username = os.Getenv("USERNAME")
	}

	log.Printf("Screenshot Helper started (server: %s, computer: %s, user: %s, interval: %dm)",
		*serverURL, *computerName, *username, *interval)

	client := &http.Client{Timeout: 60 * time.Second}
	ticker := time.NewTicker(time.Duration(*interval) * time.Minute)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	captureAndSend(client, *serverURL, *computerName, *username, *quality, *maxSizeKB)

	for {
		select {
		case <-ticker.C:
			captureAndSend(client, *serverURL, *computerName, *username, *quality, *maxSizeKB)
		case <-sigChan:
			log.Println("Screenshot Helper stopped")
			return
		}
	}
}

func captureAndSend(client *http.Client, serverURL, computerName, username string, quality, maxSizeKB int) {
	windowTitle := getForegroundWindowTitle()

	img, err := takeScreenshot()
	if err != nil {
		log.Printf("Screenshot failed: %v", err)
		return
	}

	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, img, opts); err != nil {
		log.Printf("Encode failed: %v", err)
		return
	}

	imageData := buf.Bytes()
	sizeKB := len(imageData) / 1024

	if maxSizeKB > 0 && sizeKB > maxSizeKB {
		log.Printf("Screenshot too large: %d KB (max: %d KB)", sizeKB, maxSizeKB)
		return
	}

	screenshotID := fmt.Sprintf("%s_%s_%d", computerName, username, time.Now().Unix())

	screenshot := &ScreenshotData{
		Timestamp:    time.Now(),
		ComputerName: computerName,
		Username:     username,
		ScreenshotID: screenshotID,
		WindowTitle:  windowTitle,
		ProcessName:  "",
		FileSize:     int64(len(imageData)),
		ImageData:    imageData,
	}

	if err := sendScreenshot(client, serverURL, screenshot); err != nil {
		log.Printf("Send failed: %v", err)
		return
	}

	log.Printf("Screenshot sent: %s (%d KB, window: %s)", screenshotID, sizeKB, windowTitle)
}

func takeScreenshot() (image.Image, error) {
	width, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	height, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)

	if width == 0 || height == 0 {
		return nil, fmt.Errorf("no display available")
	}

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

func getForegroundWindowTitle() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}

	textLen := 256
	buf := make([]uint16, textLen)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(textLen))

	return syscall.UTF16ToString(buf)
}

func sendScreenshot(client *http.Client, serverURL string, screenshot *ScreenshotData) error {
	data, err := json.Marshal(screenshot)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/api/screenshot", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	return nil
}
