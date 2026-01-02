//go:build windows
// +build windows

package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/ctolnik/Office-Monitor/agent/buffer"
	"github.com/ctolnik/Office-Monitor/agent/config"
	"github.com/ctolnik/Office-Monitor/agent/httpclient"
	"github.com/ctolnik/Office-Monitor/agent/logger"
	"github.com/ctolnik/Office-Monitor/agent/monitoring"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

const serviceName = "OfficeMonitorAgent"

type agentService struct {
	configPath string
}

type serviceLogger struct {
	elog *eventlog.Log
}

func (l *serviceLogger) Info(msg string) {
	log.Println(msg)
	if l.elog != nil {
		l.elog.Info(1001, msg)
	}
}

func (l *serviceLogger) Error(msg string) {
	log.Println("ERROR: " + msg)
	if l.elog != nil {
		l.elog.Error(1002, msg)
	}
}

func (l *serviceLogger) Warning(msg string) {
	log.Println("WARNING: " + msg)
	if l.elog != nil {
		l.elog.Warning(1003, msg)
	}
}

func checkServerAvailability(serverURL string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(serverURL + "/api/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (s *agentService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	elog, err := eventlog.Open(serviceName)
	if err != nil {
		log.Printf("Failed to open event log: %v", err)
	}
	defer func() {
		if elog != nil {
			elog.Close()
		}
	}()

	slog := &serviceLogger{elog: elog}

	slog.Info("Service starting...")
	slog.Info("Loading configuration from: " + s.configPath)

	cfg, err := config.Load(s.configPath)
	if err != nil {
		slog.Error("Failed to load config: " + err.Error())
		changes <- svc.Status{State: svc.StopPending}
		return false, 1
	}
	slog.Info("Configuration loaded successfully")
	slog.Info("Computer name: " + cfg.Agent.ComputerName)
	slog.Info("Server URL: " + cfg.Agent.Server.URL)

	if cfg.Logging.File != "" {
		if err := logger.Init(cfg.Logging.File); err != nil {
			slog.Error("Failed to initialize file logging: " + err.Error())
		} else {
			slog.Info("File logging enabled: " + cfg.Logging.File)
		}
	}

	if err := checkServerAvailability(cfg.Agent.Server.URL); err != nil {
		slog.Warning("Server check: " + err.Error())
	} else {
		slog.Info("Server is available")
	}

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	var m *monitors
	var cancel context.CancelFunc
	monitoringStarted := false

	username := monitoring.GetActiveSessionUsername()
	if username != "" && username != "SYSTEM" {
		slog.Info("Active user: " + username)
		_, cancel, m = s.startMonitoring(cfg, username, slog)
		monitoringStarted = true
		slog.Info("Service is fully operational")
	} else {
		slog.Info("No active user, waiting for logon (polling every 10s)...")
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				slog.Info("Shutdown requested")
				break loop
			}

		case <-ticker.C:
			if !monitoringStarted {
				username = monitoring.GetActiveSessionUsername()
				if username != "" && username != "SYSTEM" {
					slog.Info("User logon detected: " + username)
					_, cancel, m = s.startMonitoring(cfg, username, slog)
					monitoringStarted = true
					slog.Info("Service is fully operational")
				}
			}
		}
	}

	changes <- svc.Status{State: svc.StopPending}
	slog.Info("Stopping service...")

	if monitoringStarted && m != nil {
		stopMonitors(m, slog)
		if cancel != nil {
			cancel()
		}
	}

	slog.Info("Service stopped")
	return false, 0
}

type monitors struct {
	activityTracker   *monitoring.ActivityTracker
	usbMonitor        *monitoring.USBMonitor
	screenshotMonitor *monitoring.ScreenshotMonitor
	fileMonitor       *monitoring.FileMonitor
	keylogger         *monitoring.Keylogger
	eventBuffer       *buffer.EventBuffer
}

func (s *agentService) startMonitoring(cfg *config.Config, username string, slog *serviceLogger) (context.Context, context.CancelFunc, *monitors) {
	m := &monitors{}

	httpClient := httpclient.NewClient(httpclient.Config{
		ServerURL:      cfg.Agent.Server.URL,
		APIKey:         cfg.Agent.APIKey,
		TimeoutSeconds: 30,
		RetryAttempts:  3,
	})

	var err error
	m.eventBuffer, err = buffer.NewEventBuffer(buffer.Config{
		Client:    httpClient,
		Endpoint:  "/api/events/batch",
		BufferDir: "buffer",
	})
	if err != nil {
		slog.Error("Failed to create event buffer: " + err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	if m.eventBuffer != nil {
		go m.eventBuffer.Start(ctx)
	}

	if cfg.ActivityMonitoring.Enabled {
		idleThresholdMin := cfg.ActivityMonitoring.IdleThresholdSeconds / 60
		if idleThresholdMin == 0 {
			idleThresholdMin = 5
		}
		m.activityTracker = monitoring.NewActivityTracker(
			cfg.Agent.Server.URL,
			cfg.Agent.ComputerName,
			username,
			idleThresholdMin,
			cfg.ActivityMonitoring.IntervalSeconds,
		)
		if err := m.activityTracker.Start(); err != nil {
			slog.Error("Activity tracking failed: " + err.Error())
		} else {
			slog.Info("Activity tracking: ENABLED")
		}
	}

	if cfg.USBMonitoring.Enabled {
		m.usbMonitor = monitoring.NewUSBMonitor(
			cfg.Agent.Server.URL,
			cfg.Agent.ComputerName,
			username,
			cfg.USBMonitoring.ShadowCopyEnabled,
			cfg.USBMonitoring.ShadowCopyDest,
			cfg.USBMonitoring.CopyFileExtensions,
			cfg.USBMonitoring.ExcludePatterns,
			m.eventBuffer,
		)
		if err := m.usbMonitor.Start(); err != nil {
			slog.Error("USB monitoring failed: " + err.Error())
		} else {
			slog.Info("USB monitoring: ENABLED")
		}
	}

	if cfg.Screenshots.Enabled {
		m.screenshotMonitor = monitoring.NewScreenshotMonitor(
			cfg.Agent.Server.URL,
			cfg.Agent.ComputerName,
			username,
			cfg.Screenshots.IntervalMinutes,
			cfg.Screenshots.Quality,
			cfg.Screenshots.MaxSizeKB,
			cfg.Screenshots.CaptureOnlyActive,
			cfg.Screenshots.UploadImmediately,
			httpClient,
		)
		if err := m.screenshotMonitor.Start(); err != nil {
			slog.Error("Screenshot capture failed: " + err.Error())
		} else {
			slog.Info("Screenshot capture: ENABLED")
		}
	}

	if cfg.FileMonitoring.Enabled {
		m.fileMonitor = monitoring.NewFileMonitor(
			cfg.Agent.Server.URL,
			cfg.Agent.ComputerName,
			username,
			cfg.FileMonitoring.MonitoredLocations,
			cfg.FileMonitoring.LargeCopyThresholdMB,
			cfg.FileMonitoring.LargeCopyFileCount,
			cfg.FileMonitoring.DetectExternalCopy,
			m.eventBuffer,
		)
		if err := m.fileMonitor.Start(); err != nil {
			slog.Error("File monitoring failed: " + err.Error())
		} else {
			slog.Info("File monitoring: ENABLED")
		}
	}

	if cfg.Keylogger.Enabled {
		m.keylogger = monitoring.NewKeylogger(
			cfg.Agent.Server.URL,
			cfg.Agent.ComputerName,
			username,
			cfg.Keylogger.MonitoredProcesses,
			cfg.Keylogger.BufferSizeChars,
			cfg.Keylogger.SendIntervalMin,
			m.eventBuffer,
		)
		if err := m.keylogger.Start(); err != nil {
			slog.Error("Keylogger failed: " + err.Error())
		} else {
			slog.Info("Keylogger: ENABLED")
		}
	}

	return ctx, cancel, m
}

func stopMonitors(m *monitors, slog *serviceLogger) {
	if m.activityTracker != nil {
		m.activityTracker.Stop()
	}
	if m.usbMonitor != nil {
		m.usbMonitor.Stop()
	}
	if m.fileMonitor != nil {
		m.fileMonitor.Stop()
	}
	if m.screenshotMonitor != nil {
		m.screenshotMonitor.Stop()
	}
	if m.keylogger != nil {
		m.keylogger.Stop()
	}
	if m.eventBuffer != nil {
		m.eventBuffer.Stop()
	}
	slog.Info("All monitors stopped")
}

func getSessionUsername() string {
	return monitoring.GetActiveSessionUsername()
}

func runService(configPath string) error {
	return svc.Run(serviceName, &agentService{configPath: configPath})
}
