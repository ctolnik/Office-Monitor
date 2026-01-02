//go:build windows
// +build windows

package main

import (
        "context"
        "flag"
        "log"
        "os"
        "os/signal"
        "syscall"

        "github.com/ctolnik/Office-Monitor/agent/buffer"
        "github.com/ctolnik/Office-Monitor/agent/config"
        "github.com/ctolnik/Office-Monitor/agent/httpclient"
        "github.com/ctolnik/Office-Monitor/agent/logger"
        "github.com/ctolnik/Office-Monitor/agent/monitoring"
        "golang.org/x/sys/windows/svc"
)

var (
        configPath = flag.String("config", "config.yaml", "Path to config file")
        version    = "1.1.0"
)

func main() {
        flag.Parse()

        isService, err := svc.IsWindowsService()
        if err != nil {
                log.Fatalf("Failed to determine if running as service: %v", err)
        }

        if isService {
                if err := runService(*configPath); err != nil {
                        log.Fatalf("Service failed: %v", err)
                }
                return
        }

        runInteractive(*configPath)
}

func runInteractive(configPath string) {
        log.Printf("Employee Monitoring Agent v%s starting (interactive mode)...", version)

        cfg, err := config.Load(configPath)
        if err != nil {
                log.Fatalf("Failed to load config: %v", err)
        }

        if cfg.Logging.File != "" {
                if err := logger.Init(cfg.Logging.File); err != nil {
                        log.Printf("WARNING: Failed to initialize file logging: %v", err)
                        log.Println("Continuing with console logging only")
                } else {
                        log.Printf("Logging to file: %s", cfg.Logging.File)
                }
        }

        username := monitoring.GetActiveSessionUsername()
        log.Printf("Computer: %s, User: %s", cfg.Agent.ComputerName, username)
        log.Printf("Server: %s", cfg.Agent.Server.URL)

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
                log.Fatalf("Failed to create event buffer: %v", err)
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
                        log.Printf("WARNING: Activity tracking failed to start: %v", err)
                } else {
                        log.Printf("Activity tracking: ENABLED (idle threshold: %dm, poll interval: %ds)",
                                idleThresholdMin, cfg.ActivityMonitoring.IntervalSeconds)
                }
        } else {
                log.Println("Activity tracking: DISABLED")
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
                        log.Printf("WARNING: Screenshot capture failed to start: %v", err)
                } else {
                        log.Printf("Screenshot capture: ENABLED (interval: %dm, quality: %d)",
                                cfg.Screenshots.IntervalMinutes, cfg.Screenshots.Quality)
                }
        } else {
                log.Println("Screenshot capture: DISABLED")
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

        var keylogger *monitoring.Keylogger
        if cfg.Keylogger.Enabled {
                log.Println("WARNING: Keylogger enabled - ensure legal compliance!")
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
                        log.Printf("WARNING: Keylogger failed to start: %v", err)
                } else {
                        log.Printf("Keylogger: ENABLED (processes: %v)", cfg.Keylogger.MonitoredProcesses)
                }
        } else {
                log.Println("Keylogger: DISABLED")
        }

        log.Println("Agent is running. Press Ctrl+C to stop.")

        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
        <-sigChan

        log.Println("Shutting down...")

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

        log.Println("Agent stopped.")
}
