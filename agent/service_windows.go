//go:build windows
// +build windows

package main

import (
        "context"
        "log"
        "net/http"
        "time"
        "unsafe"

        "github.com/ctolnik/Office-Monitor/agent/buffer"
        "github.com/ctolnik/Office-Monitor/agent/config"
        "github.com/ctolnik/Office-Monitor/agent/httpclient"
        "github.com/ctolnik/Office-Monitor/agent/logger"
        "github.com/ctolnik/Office-Monitor/agent/monitoring"
        "golang.org/x/sys/windows/svc"
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

        log.Println("Service starting...")

        cfg, err := config.Load(s.configPath)
        if err != nil {
                log.Printf("ERROR: Failed to load config: %v", err)
                changes <- svc.Status{State: svc.StopPending}
                return false, 0
        }
        log.Printf("Config loaded: %s", cfg.Agent.ComputerName)

        if cfg.Logging.File != "" {
                if err := logger.Init(cfg.Logging.File); err != nil {
                        log.Printf("WARNING: File logging failed: %v", err)
                } else {
                        log.Printf("File logging: %s", cfg.Logging.File)
                }
        }

        if err := checkServerAvailability(cfg.Agent.Server.URL); err != nil {
                log.Printf("WARNING: Server unavailable: %v", err)
        } else {
                log.Println("Server available")
        }

        const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptSessionChange
        changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

        var m *monitors
        var cancel context.CancelFunc
        var currentSessionID uint32
        var currentUsername string

        stopCurrentMonitoring := func() {
                if m != nil {
                        stopMonitors(m)
                        m = nil
                }
                if cancel != nil {
                        cancel()
                        cancel = nil
                }
                currentSessionID = 0
                currentUsername = ""
        }

        startMonitoringForSession := func(sessionID uint32) bool {
                if sessionID == 0 {
                        return false
                }
                var username string
                for attempt := 0; attempt < 5; attempt++ {
                        username = monitoring.GetSessionUsername(sessionID)
                        if username != "" && username != "SYSTEM" {
                                break
                        }
                        log.Printf("Session %d: waiting for username (attempt %d/5)", sessionID, attempt+1)
                        time.Sleep(500 * time.Millisecond)
                }
                if username == "" || username == "SYSTEM" {
                        log.Printf("Session %d: no valid user after retries", sessionID)
                        return false
                }
                log.Printf("Starting monitoring for user: %s (session %d)", username, sessionID)
                _, cancel, m = s.startMonitoring(cfg, username, sessionID)
                currentSessionID = sessionID
                currentUsername = username
                log.Println("Monitoring started")
                return true
        }

        username, sessionID := monitoring.GetActiveSessionInfo()
        if username != "" && username != "SYSTEM" && sessionID > 0 {
                log.Printf("User: %s (session %d)", username, sessionID)
                startMonitoringForSession(sessionID)
        } else {
                log.Println("Waiting for user logon...")
        }

loop:
        for {
                select {
                case c := <-r:
                        switch c.Cmd {
                        case svc.Interrogate:
                                changes <- c.CurrentStatus

                        case svc.Stop, svc.Shutdown:
                                log.Println("Shutdown requested")
                                break loop

                        case svc.SessionChange:
                                eventSessionID := getSessionIDFromEvent(c.EventData)

                                switch c.EventType {
                                case WTS_SESSION_LOGON:
                                        log.Printf("Logon session %d (current: %d)", eventSessionID, currentSessionID)
                                        if eventSessionID > 0 {
                                                if currentSessionID != 0 && eventSessionID != currentSessionID {
                                                        log.Printf("New user logging in, stopping monitoring for session %d", currentSessionID)
                                                        stopCurrentMonitoring()
                                                }
                                                if currentSessionID == 0 {
                                                        startMonitoringForSession(eventSessionID)
                                                }
                                        }

                                case WTS_SESSION_LOGOFF:
                                        log.Printf("Logoff session %d (current: %d, user: %s)", eventSessionID, currentSessionID, currentUsername)
                                        if eventSessionID == currentSessionID {
                                                log.Printf("User %s logged off, stopping monitoring", currentUsername)
                                                stopCurrentMonitoring()
                                                log.Println("Monitoring stopped, waiting for next user logon")
                                        }

                                case WTS_SESSION_LOCK:
                                        log.Printf("Lock session %d", eventSessionID)

                                case WTS_SESSION_UNLOCK:
                                        log.Printf("Unlock session %d", eventSessionID)

                                case WTS_CONSOLE_DISCONNECT, WTS_REMOTE_DISCONNECT:
                                        log.Printf("Disconnect session %d (current: %d)", eventSessionID, currentSessionID)
                                        if eventSessionID == currentSessionID {
                                                log.Printf("User %s disconnected, stopping monitoring", currentUsername)
                                                stopCurrentMonitoring()
                                                log.Println("Monitoring stopped, waiting for next user")
                                        }

                                case WTS_CONSOLE_CONNECT, WTS_REMOTE_CONNECT:
                                        log.Printf("Connect session %d (current: %d)", eventSessionID, currentSessionID)
                                        if eventSessionID > 0 {
                                                if currentSessionID != 0 && eventSessionID != currentSessionID {
                                                        log.Printf("New session %d connecting, stopping old session %d", eventSessionID, currentSessionID)
                                                        stopCurrentMonitoring()
                                                }
                                                if currentSessionID == 0 {
                                                        startMonitoringForSession(eventSessionID)
                                                }
                                        }
                                }
                        }
                }
        }

        changes <- svc.Status{State: svc.StopPending}
        stopCurrentMonitoring()

        log.Println("Service stopped")
        return false, 0
}

type monitors struct {
        activityTracker    *monitoring.ActivityTracker
        usbMonitor         *monitoring.USBMonitor
        screenshotMonitor  *monitoring.ScreenshotMonitor
        screenshotHelper   *monitoring.HelperProcess
        fileMonitor        *monitoring.FileMonitor
        keylogger          *monitoring.Keylogger
        eventBuffer        *buffer.EventBuffer
}

func (s *agentService) startMonitoring(cfg *config.Config, username string, sessionID uint32) (context.Context, context.CancelFunc, *monitors) {
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
                log.Printf("ERROR: Buffer: %v", err)
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
                        log.Printf("ERROR: Activity: %v", err)
                } else {
                        log.Println("Activity tracking: ON")
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
                        log.Printf("ERROR: USB: %v", err)
                } else {
                        log.Println("USB monitoring: ON")
                }
        }

        if cfg.Screenshots.Enabled {
                helperPath := monitoring.FindHelperExecutable()
                m.screenshotHelper = monitoring.NewHelperProcess(
                        helperPath,
                        cfg.Agent.Server.URL,
                        cfg.Agent.ComputerName,
                        cfg.Screenshots.IntervalMinutes,
                        cfg.Screenshots.Quality,
                        cfg.Screenshots.MaxSizeKB,
                        cfg.Logging.File,
                )
                if sessionID > 0 {
                        if err := m.screenshotHelper.StartInUserSession(sessionID, username); err != nil {
                                log.Printf("ERROR: Screenshot helper: %v", err)
                        } else {
                                log.Println("Screenshot helper: ON (in user session)")
                        }
                } else {
                        log.Println("Screenshot helper: waiting for user session")
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
                        log.Printf("ERROR: Files: %v", err)
                } else {
                        log.Println("File monitoring: ON")
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
                        log.Printf("ERROR: Keylogger: %v", err)
                } else {
                        log.Println("Keylogger: ON")
                }
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
        if m.screenshotHelper != nil {
                m.screenshotHelper.Stop()
        }
        if m.keylogger != nil {
                m.keylogger.Stop()
        }
        if m.eventBuffer != nil {
                m.eventBuffer.Stop()
        }
        log.Println("All monitors stopped")
}

func getSessionUsername() string {
        return monitoring.GetActiveSessionUsername()
}

func runService(configPath string) error {
        return svc.Run(serviceName, &agentService{configPath: configPath})
}
