//go:build windows
// +build windows

package main

import (
        "context"
        "log"
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

        logInfo := func(msg string) {
                log.Println(msg)
                if elog != nil {
                        elog.Info(1, msg)
                }
        }

        logError := func(msg string) {
                log.Println("ERROR: " + msg)
                if elog != nil {
                        elog.Error(1, msg)
                }
        }

        logInfo("Loading configuration...")

        cfg, err := config.Load(s.configPath)
        if err != nil {
                logError("Failed to load config: " + err.Error())
                changes <- svc.Status{State: svc.StopPending}
                return false, 1
        }

        if cfg.Logging.File != "" {
                if err := logger.Init(cfg.Logging.File); err != nil {
                        logError("Failed to initialize file logging: " + err.Error())
                } else {
                        logInfo("Logging to file: " + cfg.Logging.File)
                }
        }

        username := getSessionUsername()
        logInfo("Computer: " + cfg.Agent.ComputerName + ", User: " + username)
        logInfo("Server: " + cfg.Agent.Server.URL)

        httpClient := httpclient.NewClient(httpclient.Config{
                ServerURL:      cfg.Agent.Server.URL,
                APIKey:         cfg.Agent.APIKey,
                TimeoutSeconds: 30,
                RetryAttempts:  3,
        })

        eventBuffer, err := buffer.NewEventBuffer(buffer.Config{
                Client:    httpClient,
                Endpoint:  "/api/events/batch",
                BufferDir: "buffer",
        })
        if err != nil {
                logError("Failed to create event buffer: " + err.Error())
                changes <- svc.Status{State: svc.StopPending}
                return false, 1
        }

        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()
        go eventBuffer.Start(ctx)

        var activityTracker *monitoring.ActivityTracker
        if cfg.ActivityMonitoring.Enabled {
                idleThresholdMin := cfg.ActivityMonitoring.IdleThresholdSeconds / 60
                if idleThresholdMin == 0 {
                        idleThresholdMin = 5
                }
                activityTracker = monitoring.NewActivityTracker(
                        cfg.Agent.Server.URL,
                        cfg.Agent.ComputerName,
                        username,
                        idleThresholdMin,
                        cfg.ActivityMonitoring.IntervalSeconds,
                )
                if err := activityTracker.Start(); err != nil {
                        logError("Activity tracking failed to start: " + err.Error())
                } else {
                        logInfo("Activity tracking: ENABLED")
                }
        }

        var usbMonitor *monitoring.USBMonitor
        if cfg.USBMonitoring.Enabled {
                usbMonitor = monitoring.NewUSBMonitor(
                        cfg.Agent.Server.URL,
                        cfg.Agent.ComputerName,
                        username,
                        cfg.USBMonitoring.ShadowCopyEnabled,
                        cfg.USBMonitoring.ShadowCopyDest,
                        cfg.USBMonitoring.CopyFileExtensions,
                        cfg.USBMonitoring.ExcludePatterns,
                        eventBuffer,
                )
                if err := usbMonitor.Start(); err != nil {
                        logError("USB monitoring failed to start: " + err.Error())
                } else {
                        logInfo("USB monitoring: ENABLED")
                }
        }

        var screenshotMonitor *monitoring.ScreenshotMonitor
        if cfg.Screenshots.Enabled {
                screenshotMonitor = monitoring.NewScreenshotMonitor(
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
                if err := screenshotMonitor.Start(); err != nil {
                        logError("Screenshot capture failed to start: " + err.Error())
                } else {
                        logInfo("Screenshot capture: ENABLED")
                }
        }

        var fileMonitor *monitoring.FileMonitor
        if cfg.FileMonitoring.Enabled {
                fileMonitor = monitoring.NewFileMonitor(
                        cfg.Agent.Server.URL,
                        cfg.Agent.ComputerName,
                        username,
                        cfg.FileMonitoring.MonitoredLocations,
                        cfg.FileMonitoring.LargeCopyThresholdMB,
                        cfg.FileMonitoring.LargeCopyFileCount,
                        cfg.FileMonitoring.DetectExternalCopy,
                        eventBuffer,
                )
                if err := fileMonitor.Start(); err != nil {
                        logError("File monitoring failed to start: " + err.Error())
                } else {
                        logInfo("File monitoring: ENABLED")
                }
        }

        var keylogger *monitoring.Keylogger
        if cfg.Keylogger.Enabled {
                keylogger = monitoring.NewKeylogger(
                        cfg.Agent.Server.URL,
                        cfg.Agent.ComputerName,
                        username,
                        cfg.Keylogger.MonitoredProcesses,
                        cfg.Keylogger.BufferSizeChars,
                        cfg.Keylogger.SendIntervalMin,
                        eventBuffer,
                )
                if err := keylogger.Start(); err != nil {
                        logError("Keylogger failed to start: " + err.Error())
                } else {
                        logInfo("Keylogger: ENABLED")
                }
        }

        changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
        logInfo("Service is running")

loop:
        for {
                select {
                case c := <-r:
                        switch c.Cmd {
                        case svc.Interrogate:
                                changes <- c.CurrentStatus
                                time.Sleep(100 * time.Millisecond)
                                changes <- c.CurrentStatus
                        case svc.Stop, svc.Shutdown:
                                logInfo("Shutdown requested")
                                break loop
                        default:
                                logError("unexpected control request")
                        }
                }
        }

        changes <- svc.Status{State: svc.StopPending}
        logInfo("Stopping service...")

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

        eventBuffer.Stop()
        cancel()

        logInfo("Service stopped")
        return false, 0
}

func getSessionUsername() string {
        return monitoring.GetActiveSessionUsername()
}

func runService(configPath string) error {
        return svc.Run(serviceName, &agentService{configPath: configPath})
}
