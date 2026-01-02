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
        monitoringStarted := false

        username := monitoring.GetActiveSessionUsername()
        if username != "" && username != "SYSTEM" {
                log.Printf("User: %s", username)
                _, cancel, m = s.startMonitoring(cfg, username)
                monitoringStarted = true
                log.Println("Monitoring started")
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
                                sessionID := getSessionIDFromEvent(c.EventData)

                                switch c.EventType {
                                case WTS_SESSION_LOGON:
                                        log.Printf("Logon session %d", sessionID)
                                        if !monitoringStarted {
                                                username = monitoring.GetActiveSessionUsername()
                                                if username != "" && username != "SYSTEM" {
                                                        log.Printf("User: %s", username)
                                                        _, cancel, m = s.startMonitoring(cfg, username)
                                                        monitoringStarted = true
                                                        log.Println("Monitoring started")
                                                }
                                        }

                                case WTS_SESSION_LOGOFF:
                                        log.Printf("Logoff session %d", sessionID)

                                case WTS_SESSION_LOCK:
                                        log.Printf("Lock session %d", sessionID)

                                case WTS_SESSION_UNLOCK:
                                        log.Printf("Unlock session %d", sessionID)

                                case WTS_CONSOLE_CONNECT, WTS_REMOTE_CONNECT:
                                        log.Printf("Connect session %d", sessionID)
                                        if !monitoringStarted {
                                                username = monitoring.GetActiveSessionUsername()
                                                if username != "" && username != "SYSTEM" {
                                                        log.Printf("User: %s", username)
                                                        _, cancel, m = s.startMonitoring(cfg, username)
                                                        monitoringStarted = true
                                                        log.Println("Monitoring started")
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

        log.Println("Service stopped")
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

func (s *agentService) startMonitoring(cfg *config.Config, username string) (context.Context, context.CancelFunc, *monitors) {
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
                        log.Printf("ERROR: Screenshots: %v", err)
                } else {
                        log.Println("Screenshots: ON")
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
        if m.screenshotMonitor != nil {
                m.screenshotMonitor.Stop()
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
