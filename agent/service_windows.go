//go:build windows
// +build windows

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
	"unsafe"

	"github.com/ctolnik/Office-Monitor/agent/buffer"
	"github.com/ctolnik/Office-Monitor/agent/config"
	"github.com/ctolnik/Office-Monitor/agent/httpclient"
	"github.com/ctolnik/Office-Monitor/agent/monitoring"
	"github.com/ctolnik/Office-Monitor/agent/pkg/ipc"
	agentlog "github.com/ctolnik/Office-Monitor/agent/pkg/logger"
	"go.uber.org/zap"
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
	resp, err := client.Get(serverURL + "/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (s *agentService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
        changes <- svc.Status{State: svc.StartPending}

		cfg, err := config.Load(s.configPath)
		if err != nil {
			changes <- svc.Status{State: svc.StopPending}
			return false, 0
		}

		logCfg := agentlog.DefaultConfig()
		logCfg.Level = cfg.Logging.Level
		logCfg.FilePath = cfg.Logging.File
		logCfg.MaxSizeMB = cfg.Logging.MaxSizeMB
		logCfg.MaxBackups = cfg.Logging.MaxBackups
		logCfg.Console = cfg.Logging.File == ""
		_ = agentlog.Init(logCfg)
		defer func() { _ = agentlog.Sync() }()

		log := agentlog.WithComponent("service")
		log.Info("Service starting...", zap.String("computer_name", cfg.Agent.ComputerName))

        if err := checkServerAvailability(cfg.Agent.Server.URL); err != nil {
                log.Warn("Server unavailable", zap.Error(err), zap.String("server_url", cfg.Agent.Server.URL))
        } else {
                log.Info("Server available", zap.String("server_url", cfg.Agent.Server.URL))
        }

        // Single http client + event buffer for whole service lifetime
        httpClient := httpclient.NewClient(httpclient.Config{
                ServerURL:      cfg.Agent.Server.URL,
                APIKey:         cfg.Agent.APIKey,
                TimeoutSeconds: cfg.Agent.Server.TimeoutSeconds,
                RetryAttempts:  cfg.Agent.Server.RetryAttempts,
                RetryDelay:     time.Duration(cfg.Agent.Server.RetryDelay) * time.Second,
        })

        eventBuffer, err := buffer.NewEventBuffer(buffer.Config{
                Client:    httpClient,
                Endpoint:  "/api/events/batch",
                BufferDir: "buffer",
        })
        if err != nil {
                log.Error("Failed to create event buffer", zap.Error(err))
        }

        svcCtx, svcCancel := context.WithCancel(context.Background())
        defer svcCancel()
        if eventBuffer != nil {
                go eventBuffer.Start(svcCtx)
        }

        // IPC server
        pipeServer := ipc.NewPipeServer(ipc.PipeName)
        assembler := NewScreenshotAssembler(cfg.Agent.Server.URL, cfg.Agent.APIKey, cfg.Agent.ComputerName, log)

        pipeServer.RegisterHandler(ipc.EventTypeActivity, func(e ipc.Event) error {
                var seg ipc.ActivitySegment
                b, _ := json.Marshal(e.Data)
                _ = json.Unmarshal(b, &seg)

                payload := map[string]interface{}{
                        "timestamp_start": seg.TimestampStart,
                        "timestamp_end":   seg.TimestampEnd,
                        "duration_sec":    seg.DurationSec,
                        "state":           seg.State,
                        "computer_name":   cfg.Agent.ComputerName,
                        "username":        e.Username,
                        "process_name":    seg.ProcessName,
                        "window_title":    seg.WindowTitle,
                        "session_id":      e.SessionID,
                }
                if eventBuffer != nil {
                        return eventBuffer.Add("activity_segment", payload)
                }
                return nil
        })

        pipeServer.RegisterHandler(ipc.EventTypeShotBegin, assembler.HandleBegin)
        pipeServer.RegisterHandler(ipc.EventTypeShotChunk, assembler.HandleChunk)
        pipeServer.RegisterHandler(ipc.EventTypeShotCommit, assembler.HandleCommit)

        if err := pipeServer.Start(); err != nil {
                log.Error("Failed to start pipe server", zap.Error(err))
        } else {
                log.Info("Pipe server started", zap.String("pipe", ipc.PipeName))
        }
        defer pipeServer.Stop()

        const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptSessionChange
        changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

        var m *monitors
        var cancel context.CancelFunc
        var currentSessionID uint32
        var currentUsername string

        stopCurrentMonitoring := func() {
                if m != nil {
                        stopMonitors(m, log)
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
                        log.Debug("Waiting for username",
                                zap.Uint32("session_id", sessionID),
                                zap.Int("attempt", attempt+1),
                        )
                        time.Sleep(500 * time.Millisecond)
                }
                if username == "" || username == "SYSTEM" {
                        log.Warn("No valid user after retries", zap.Uint32("session_id", sessionID))
                        return false
                }
                log.Info("Starting monitoring for session",
                        zap.String("username", username),
                        zap.Uint32("session_id", sessionID),
                )
                _, cancel, m = s.startMonitoring(cfg, username, sessionID, eventBuffer, log)
                currentSessionID = sessionID
                currentUsername = username
                log.Info("Monitoring started")
                return true
        }

        allSessions := monitoring.EnumerateAllUserSessions()
        log.Info("Found user sessions at startup", zap.Int("count", len(allSessions)))
        for _, sess := range allSessions {
                log.Info("Session",
                        zap.Uint32("session_id", sess.SessionID),
                        zap.String("username", sess.Username),
                        zap.Int("state", int(sess.State)),
                )
        }

        username, sessionID := monitoring.GetActiveSessionInfo()
        if username != "" && username != "SYSTEM" && sessionID > 0 {
                log.Info("Selected session for monitoring",
                        zap.String("username", username),
                        zap.Uint32("session_id", sessionID),
                )
                startMonitoringForSession(sessionID)
        } else {
                log.Info("No active user session found, waiting for user logon...")
        }

loop:
        for {
                select {
                case c := <-r:
                        switch c.Cmd {
                        case svc.Interrogate:
                                changes <- c.CurrentStatus

                        case svc.Stop, svc.Shutdown:
                                log.Info("Shutdown requested")
                                break loop

                        case svc.SessionChange:
                                eventSessionID := getSessionIDFromEvent(c.EventData)

                                switch c.EventType {
                                case WTS_SESSION_LOGON:
								log.Info("Logon session",
									zap.Uint32("session_id", eventSessionID),
									zap.Uint32("current_session_id", currentSessionID),
								)
                                        if eventSessionID > 0 {
                                                if currentSessionID != 0 && eventSessionID != currentSessionID {
												log.Info("New user logon, stopping previous monitoring",
													zap.Uint32("previous_session_id", currentSessionID),
												)
                                                        stopCurrentMonitoring()
                                                }
                                                if currentSessionID == 0 {
                                                        startMonitoringForSession(eventSessionID)
                                                }
                                        }

                                case WTS_SESSION_LOGOFF:
								log.Info("Logoff session",
									zap.Uint32("session_id", eventSessionID),
									zap.Uint32("current_session_id", currentSessionID),
									zap.String("current_username", currentUsername),
								)
                                        if eventSessionID == currentSessionID {
												log.Info("User logged off, stopping monitoring", zap.String("username", currentUsername))
                                                stopCurrentMonitoring()
												log.Info("Monitoring stopped, waiting for next user logon")
                                        }

                                case WTS_SESSION_LOCK:
								log.Info("Lock session", zap.Uint32("session_id", eventSessionID))

                                case WTS_SESSION_UNLOCK:
								log.Info("Unlock session", zap.Uint32("session_id", eventSessionID))

                                case WTS_CONSOLE_DISCONNECT, WTS_REMOTE_DISCONNECT:
								log.Info("Disconnect session",
									zap.Uint32("session_id", eventSessionID),
									zap.Uint32("current_session_id", currentSessionID),
								)
                                        if eventSessionID == currentSessionID {
												log.Info("User disconnected, stopping monitoring", zap.String("username", currentUsername))
                                                stopCurrentMonitoring()
												log.Info("Monitoring stopped, waiting for next user")
                                        }

                                case WTS_CONSOLE_CONNECT, WTS_REMOTE_CONNECT:
								log.Info("Connect session",
									zap.Uint32("session_id", eventSessionID),
									zap.Uint32("current_session_id", currentSessionID),
								)
                                        if eventSessionID > 0 {
                                                if currentSessionID != 0 && eventSessionID != currentSessionID {
												log.Info("New session connected, stopping old monitoring",
													zap.Uint32("new_session_id", eventSessionID),
													zap.Uint32("previous_session_id", currentSessionID),
												)
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

        log.Info("Service stopped")
        return false, 0
}

type monitors struct {
        usbMonitor        *monitoring.USBMonitor
        sessionHelper     *monitoring.HelperProcess
        fileMonitor       *monitoring.FileMonitor
        keylogger         *monitoring.Keylogger
}

func (s *agentService) startMonitoring(cfg *config.Config, username string, sessionID uint32, eventBuffer *buffer.EventBuffer, log *zap.Logger) (context.Context, context.CancelFunc, *monitors) {
        m := &monitors{}

        ctx, cancel := context.WithCancel(context.Background())

        if cfg.USBMonitoring.Enabled {
                m.usbMonitor = monitoring.NewUSBMonitor(
                        cfg.Agent.Server.URL,
                        cfg.Agent.ComputerName,
                        username,
                        cfg.USBMonitoring.ShadowCopyEnabled,
                        cfg.USBMonitoring.ShadowCopyDest,
                        cfg.USBMonitoring.CopyFileExtensions,
                        cfg.USBMonitoring.ExcludePatterns,
                        eventBuffer,
                )
                if err := m.usbMonitor.Start(); err != nil {
                        log.Error("USB monitoring start failed", zap.Error(err))
                } else {
                        log.Info("USB monitoring: ON")
                }
        }

        if cfg.Screenshots.Enabled {
                helperPath := monitoring.FindHelperExecutable()
                m.sessionHelper = monitoring.NewHelperProcess(
                        helperPath,
                        cfg.Agent.Server.URL,
                        cfg.Agent.ComputerName,
                        cfg.Screenshots.IntervalMinutes,
                        cfg.Screenshots.Quality,
                        cfg.Screenshots.MaxSizeKB,
                        cfg.Logging.File,
                )
                if sessionID > 0 {
                        if err := m.sessionHelper.StartInUserSession(sessionID, username); err != nil {
                                log.Error("Session helper start failed", zap.Error(err))
                        } else {
                                log.Info("Session helper: ON")
                        }
                } else {
                        log.Info("Session helper: waiting for user session")
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
                        eventBuffer,
                )
                if err := m.fileMonitor.Start(); err != nil {
                        log.Error("File monitoring start failed", zap.Error(err))
                } else {
                        log.Info("File monitoring: ON")
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
                        eventBuffer,
                )
                if err := m.keylogger.Start(); err != nil {
                        log.Error("Keylogger start failed", zap.Error(err))
                } else {
                        log.Info("Keylogger: ON")
                }
        }

        return ctx, cancel, m
}

func stopMonitors(m *monitors, log *zap.Logger) {
        if m.usbMonitor != nil {
                m.usbMonitor.Stop()
        }
        if m.fileMonitor != nil {
                m.fileMonitor.Stop()
        }
        if m.sessionHelper != nil {
                _ = m.sessionHelper.Stop()
        }
        if m.keylogger != nil {
                m.keylogger.Stop()
        }
        log.Info("All monitors stopped")
}

func getSessionUsername() string {
        return monitoring.GetActiveSessionUsername()
}

func runService(configPath string) error {
        return svc.Run(serviceName, &agentService{configPath: configPath})
}
