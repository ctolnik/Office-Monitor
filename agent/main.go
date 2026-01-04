//go:build windows
// +build windows

package main

import (
        "context"
        "encoding/json"
        "flag"
        "fmt"
        "log"
        "os"
        "os/exec"
        "os/signal"
        "path/filepath"
        "syscall"
        "time"

        "github.com/ctolnik/Office-Monitor/agent/buffer"
        "github.com/ctolnik/Office-Monitor/agent/config"
        "github.com/ctolnik/Office-Monitor/agent/httpclient"
        "github.com/ctolnik/Office-Monitor/agent/pkg/ipc"
        agentlog "github.com/ctolnik/Office-Monitor/agent/pkg/logger"
        "github.com/ctolnik/Office-Monitor/agent/monitoring"
        "go.uber.org/zap"
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

        // Provide a sane default log path if config doesn't specify one.
        if cfg.Logging.File == "" {
                if programData := os.Getenv("ProgramData"); programData != "" {
                        cfg.Logging.File = filepath.Join(programData, "MonitoringAgent", "agent.log")
                }
        }

        logCfg := agentlog.DefaultConfig()
        logCfg.Level = cfg.Logging.Level
        logCfg.FilePath = cfg.Logging.File
        logCfg.MaxSizeMB = cfg.Logging.MaxSizeMB
        logCfg.MaxBackups = cfg.Logging.MaxBackups
        logCfg.Console = true
        if err := agentlog.Init(logCfg); err != nil {
                log.Printf("WARNING: Failed to initialize zap logger, falling back to stdout: %v", err)
        }
        defer func() { _ = agentlog.Sync() }()

        zlog := agentlog.WithComponent("interactive")

        username, sessionID := monitoring.GetActiveSessionInfo()
        zlog.Info("Interactive mode context",
                zap.String("computer_name", cfg.Agent.ComputerName),
                zap.String("username", username),
                zap.Uint32("session_id", sessionID),
                zap.String("server_url", cfg.Agent.Server.URL),
        )

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
                zlog.Fatal("Failed to create event buffer", zap.Error(err))
        }

        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()
        go eventBuffer.Start(ctx)

        // Start IPC server even in interactive mode (for debugging).
        pipeServer := ipc.NewPipeServer(ipc.PipeName)
        assembler := NewScreenshotAssembler(cfg.Agent.Server.URL, cfg.Agent.APIKey, cfg.Agent.ComputerName, zlog)

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
                return eventBuffer.Add("activity_segment", payload)
        })
        pipeServer.RegisterHandler(ipc.EventTypeShotBegin, assembler.HandleBegin)
        pipeServer.RegisterHandler(ipc.EventTypeShotChunk, assembler.HandleChunk)
        pipeServer.RegisterHandler(ipc.EventTypeShotCommit, assembler.HandleCommit)

        if err := pipeServer.Start(); err != nil {
                zlog.Fatal("Failed to start pipe server", zap.Error(err))
        }
        defer pipeServer.Stop()

        // Spawn helper in the current user session (like service would do) so you can test IPC locally.
        helperPath := monitoring.FindHelperExecutable()
        if !filepath.IsAbs(helperPath) {
                exePath, _ := os.Executable()
                helperPath = filepath.Join(filepath.Dir(exePath), helperPath)
        }

        helperLogPath := ""
        if cfg.Logging.File != "" {
                helperLogPath = filepath.Join(filepath.Dir(cfg.Logging.File), "agent-sh.log")
        }

        args := []string{
                "-server=" + cfg.Agent.Server.URL,
                "-computer=" + cfg.Agent.ComputerName,
                "-user=" + username,
                fmt.Sprintf("-session=%d", sessionID),
                fmt.Sprintf("-activity-interval=%d", cfg.ActivityMonitoring.IntervalSeconds),
                fmt.Sprintf("-screenshot-interval=%d", cfg.Screenshots.IntervalMinutes),
                fmt.Sprintf("-idle-threshold=%d", max(1, cfg.ActivityMonitoring.IdleThresholdSeconds/60)),
                fmt.Sprintf("-quality=%d", cfg.Screenshots.Quality),
                fmt.Sprintf("-maxsize=%d", cfg.Screenshots.MaxSizeKB),
        }
        if helperLogPath != "" {
                args = append(args, "-log="+helperLogPath, "-log-level="+cfg.Logging.Level)
        }

        cmd := exec.Command(helperPath, args...)
        if err := cmd.Start(); err != nil {
                zlog.Error("Failed to start helper process", zap.Error(err), zap.String("helper_path", helperPath))
        } else {
                zlog.Info("Helper process started",
                        zap.String("helper_path", helperPath),
                        zap.Int("pid", cmd.Process.Pid),
                )
        }

        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
        <-sigChan

        zlog.Info("Shutting down...")
        if cmd.Process != nil {
                _ = cmd.Process.Kill()
        }
        eventBuffer.Stop()
        cancel()
        zlog.Info("Agent stopped")
}

func max(a, b int) int {
        if a > b {
                return a
        }
        return b
}
