//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
	"unsafe"

	"github.com/ctolnik/Office-Monitor/agent/buffer"
	"github.com/ctolnik/Office-Monitor/agent/config"
	"github.com/ctolnik/Office-Monitor/agent/httpclient"
	"github.com/ctolnik/Office-Monitor/agent/logger"
	"github.com/ctolnik/Office-Monitor/agent/monitoring"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

const serviceName = "OfficeMonitorAgent"

const (
	WTS_CONSOLE_CONNECT    = 0x1
	WTS_CONSOLE_DISCONNECT = 0x2
	WTS_REMOTE_CONNECT     = 0x3
	WTS_REMOTE_DISCONNECT  = 0x4
	WTS_SESSION_LOGON      = 0x5
	WTS_SESSION_LOGOFF     = 0x6
	WTS_SESSION_LOCK       = 0x7
	WTS_SESSION_UNLOCK     = 0x8
)

type WTSSESSION_NOTIFICATION struct {
	Size      uint32
	SessionID uint32
}

func getSessionIDFromEvent(eventData uintptr) uint32 {
	if eventData == 0 {
		return 0
	}
	notification := (*WTSSESSION_NOTIFICATION)(unsafe.Pointer(eventData))
	return notification.SessionID
}

type agentService struct {
	configPath string
}

type serviceLogger struct {
	elog     *eventlog.Log
	mu       sync.Mutex
	inLog    bool
}

func (l *serviceLogger) Info(msg string) {
	log.Println(msg)
	l.writeEvent(1001, msg, "info")
}

func (l *serviceLogger) Error(msg string) {
	log.Println("ERROR: " + msg)
	l.writeEvent(1002, msg, "error")
}

func (l *serviceLogger) Warning(msg string) {
	log.Println("WARNING: " + msg)
	l.writeEvent(1003, msg, "warning")
}

func (l *serviceLogger) writeEvent(id uint32, msg, level string) {
	l.mu.Lock()
	if l.inLog || l.elog == nil {
		l.mu.Unlock()
		return
	}
	l.inLog = true
	l.mu.Unlock()

	defer func() {
		l.mu.Lock()
		l.inLog = false
		l.mu.Unlock()
	}()

	switch level {
	case "info":
		l.elog.Info(id, msg)
	case "error":
		l.elog.Error(id, msg)
	case "warning":
		l.elog.Warning(id, msg)
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
	changes <- svc.Status{State: svc.StartPending}

	elog, _ := eventlog.Open(serviceName)
	defer func() {
		if elog != nil {
			elog.Close()
		}
	}()

	slog := &serviceLogger{elog: elog}

	slog.Info("Service starting...")

	cfg, err := config.Load(s.configPath)
	if err != nil {
		slog.Error("Failed to load config: " + err.Error())
		changes <- svc.Status{State: svc.StopPending}
		return false, 0
	}
	slog.Info("Config loaded: " + cfg.Agent.ComputerName)

	if cfg.Logging.File != "" {
		if err := logger.Init(cfg.Logging.File); err != nil {
			slog.Warning("File logging failed: " + err.Error())
		}
	}

	if err := checkServerAvailability(cfg.Agent.Server.URL); err != nil {
		slog.Warning("Server unavailable: " + err.Error())
	} else {
		slog.Info("Server available")
	}

	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptSessionChange
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	var m *monitors
	var cancel context.CancelFunc
	monitoringStarted := false

	username := monitoring.GetActiveSessionUsername()
	if username != "" && username != "SYSTEM" {
		slog.Info("User: " + username)
		_, cancel, m = s.startMonitoring(cfg, username, slog)
		monitoringStarted = true
		slog.Info("Monitoring started")
	} else {
		slog.Info("Waiting for user logon...")
	}

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus

			case svc.Stop, svc.Shutdown:
				slog.Info("Shutdown")
				break loop

			case svc.SessionChange:
				sessionID := getSessionIDFromEvent(c.EventData)
				
				switch c.EventType {
				case WTS_SESSION_LOGON:
					slog.Info(fmt.Sprintf("Logon session %d", sessionID))
					if !monitoringStarted {
						username = monitoring.GetActiveSessionUsername()
						if username != "" && username != "SYSTEM" {
							slog.Info("User: " + username)
							_, cancel, m = s.startMonitoring(cfg, username, slog)
							monitoringStarted = true
							slog.Info("Monitoring started")
						}
					}

				case WTS_SESSION_LOGOFF:
					slog.Info(fmt.Sprintf("Logoff session %d", sessionID))

				case WTS_SESSION_LOCK:
					slog.Info(fmt.Sprintf("Lock session %d", sessionID))

				case WTS_SESSION_UNLOCK:
					slog.Info(fmt.Sprintf("Unlock session %d", sessionID))

				case WTS_CONSOLE_CONNECT, WTS_REMOTE_CONNECT:
					if !monitoringStarted {
						username = monitoring.GetActiveSessionUsername()
						if username != "" && username != "SYSTEM" {
							slog.Info("User: " + username)
							_, cancel, m = s.startMonitoring(cfg, username, slog)
							monitoringStarted = true
							slog.Info("Monitoring started")
						}
					}
				}
			}
		}
	}

	changes <- svc.Status{State: svc.StopPending}

	if monitoringStarted && m != nil {
		stopMonitors(m)
		if cancel != nil {
			cancel()
		}
	}

	slog.Info("Stopped")
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
		slog.Error("Buffer error: " + err.Error())
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
		m.activityTracker.Start()
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
		m.usbMonitor.Start()
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
		m.screenshotMonitor.Start()
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
		m.fileMonitor.Start()
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
		m.keylogger.Start()
	}

	return ctx, cancel, m
}

func stopMonitors(m *monitors) {
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
}

func getSessionUsername() string {
	return monitoring.GetActiveSessionUsername()
}

func runService(configPath string) error {
	return svc.Run(serviceName, &agentService{configPath: configPath})
}
