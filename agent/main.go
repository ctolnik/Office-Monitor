package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ctolnik/Office-Monitor/agent/config"
	"github.com/ctolnik/Office-Monitor/agent/monitoring"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to config file")
	version    = "1.0.0"
)

func main() {
	flag.Parse()

	log.Printf("Employee Monitoring Agent v%s starting...", version)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Computer: %s, User: %s", cfg.Agent.ComputerName, os.Getenv("USERNAME"))
	log.Printf("Server: %s", cfg.Agent.Server.URL)

	// Initialize activity tracker
	var activityTracker *ActivityTracker
	if cfg.ActivityMonitoring.Enabled {
		activityTracker, err = NewActivityTracker(
			cfg.Agent.Server.URL,
			cfg.Agent.ComputerName,
			cfg.ActivityMonitoring.IntervalSeconds,
		)
		if err != nil {
			log.Fatalf("Failed to create activity tracker: %v", err)
		}
		go activityTracker.Start()
		log.Println("Activity monitoring: ENABLED")
	} else {
		log.Println("Activity monitoring: DISABLED")
	}

	// Initialize USB monitor
	var usbMonitor *monitoring.USBMonitor
	if cfg.USBMonitoring.Enabled {
		usbMonitor = monitoring.NewUSBMonitor(
			cfg.Agent.Server.URL,
			cfg.Agent.ComputerName,
			os.Getenv("USERNAME"),
			cfg.USBMonitoring.ShadowCopyEnabled,
			cfg.USBMonitoring.ShadowCopyDest,
			cfg.USBMonitoring.CopyFileExtensions,
			cfg.USBMonitoring.ExcludePatterns,
		)
		if err := usbMonitor.Start(); err != nil {
			log.Printf("WARNING: USB monitoring failed to start: %v", err)
		} else {
			log.Println("USB monitoring: ENABLED")
			if cfg.USBMonitoring.ShadowCopyEnabled {
				log.Printf("Shadow copy: ENABLED -> %s", cfg.USBMonitoring.ShadowCopyDest)
			}
		}
	} else {
		log.Println("USB monitoring: DISABLED")
	}

	// Initialize screenshot capture
	var screenshotMonitor *monitoring.ScreenshotMonitor
	if cfg.Screenshots.Enabled {
		screenshotMonitor = monitoring.NewScreenshotMonitor(
			cfg.Agent.Server.URL,
			cfg.Agent.ComputerName,
			os.Getenv("USERNAME"),
			cfg.Screenshots.IntervalMinutes,
			cfg.Screenshots.Quality,
			cfg.Screenshots.MaxSizeKB,
			cfg.Screenshots.CaptureOnlyActive,
			cfg.Screenshots.UploadImmediately,
		)
		if err := screenshotMonitor.Start(); err != nil {
			log.Printf("WARNING: Screenshot capture failed to start: %v", err)
		} else {
			log.Printf("Screenshot capture: ENABLED (interval: %dm, quality: %d)",
				cfg.Screenshots.IntervalMinutes, cfg.Screenshots.Quality)
		}
	} else {
		log.Println("Screenshot capture: DISABLED")
	}

	// Initialize file monitoring
	var fileMonitor *monitoring.FileMonitor
	if cfg.FileMonitoring.Enabled {
		fileMonitor = monitoring.NewFileMonitor(
			cfg.Agent.Server.URL,
			cfg.Agent.ComputerName,
			os.Getenv("USERNAME"),
			cfg.FileMonitoring.MonitoredLocations,
			cfg.FileMonitoring.LargeCopyThresholdMB,
			cfg.FileMonitoring.LargeCopyFileCount,
			cfg.FileMonitoring.DetectExternalCopy,
		)
		if err := fileMonitor.Start(); err != nil {
			log.Printf("WARNING: File monitoring failed to start: %v", err)
		} else {
			log.Println("File monitoring: ENABLED")
			log.Printf("Monitoring %d locations, thresholds: %dMB / %d files",
				len(cfg.FileMonitoring.MonitoredLocations),
				cfg.FileMonitoring.LargeCopyThresholdMB,
				cfg.FileMonitoring.LargeCopyFileCount)
		}
	} else {
		log.Println("File monitoring: DISABLED")
	}

	// Initialize keylogger
	var keylogger *monitoring.Keylogger
	if cfg.Keylogger.Enabled {
		log.Println("WARNING: Keylogger enabled - ensure legal compliance!")
		keylogger = monitoring.NewKeylogger(
			cfg.Agent.Server.URL,
			cfg.Agent.ComputerName,
			os.Getenv("USERNAME"),
			cfg.Keylogger.MonitoredProcesses,
			cfg.Keylogger.BufferSizeChars,
			cfg.Keylogger.SendIntervalMin,
		)
		if err := keylogger.Start(); err != nil {
			log.Printf("WARNING: Keylogger failed to start: %v", err)
		} else {
			log.Printf("Keylogger: ENABLED (processes: %v)", cfg.Keylogger.MonitoredProcesses)
		}
	} else {
		log.Println("Keylogger: DISABLED")
	}

	log.Println("Agent is running. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Cleanup
	if activityTracker != nil {
		activityTracker.Stop()
	}
	if usbMonitor != nil {
		usbMonitor.Stop()
	}
	if fileMonitor != nil {
		fileMonitor.Stop()
	}
	if screenshotMonitor != nil {
		screenshotMonitor.Stop()
	}
	if keylogger != nil {
		keylogger.Stop()
	}

	log.Println("Agent stopped.")
}

// ActivityTracker handles activity monitoring
type ActivityTracker struct {
	serverURL      string
	computerName   string
	intervalSec    int
	currentWindow  string
	currentProcess string
	startTime      time.Time
	stopChan       chan bool
}

func NewActivityTracker(serverURL, computerName string, intervalSec int) (*ActivityTracker, error) {
	return &ActivityTracker{
		serverURL:    serverURL,
		computerName: computerName,
		intervalSec:  intervalSec,
		stopChan:     make(chan bool),
	}, nil
}

func (t *ActivityTracker) Start() {
	log.Println("Activity tracker started")
	// Implementation will use existing code from main_old_backup.go
	// Placeholder for now
	ticker := time.NewTicker(time.Duration(t.intervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// TODO: Get foreground window and process
			// TODO: Send to server
		case <-t.stopChan:
			return
		}
	}
}

func (t *ActivityTracker) Stop() {
	close(t.stopChan)
	log.Println("Activity tracker stopped")
}
