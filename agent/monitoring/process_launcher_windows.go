//go:build windows
// +build windows

package monitoring

import (
        "fmt"
        "log"
        "os"
        "path/filepath"
        "sync"
        "syscall"
        "unsafe"

        "golang.org/x/sys/windows"
)

var (
        advapi32                    = windows.NewLazySystemDLL("advapi32.dll")
        userenv                     = windows.NewLazySystemDLL("userenv.dll")
        wtsapi32Launcher            = windows.NewLazySystemDLL("wtsapi32.dll")
        procWTSQueryUserToken       = wtsapi32Launcher.NewProc("WTSQueryUserToken")
        procDuplicateTokenEx        = advapi32.NewProc("DuplicateTokenEx")
        procCreateEnvironmentBlock  = userenv.NewProc("CreateEnvironmentBlock")
        procDestroyEnvironmentBlock = userenv.NewProc("DestroyEnvironmentBlock")
        procCreateProcessAsUserW    = advapi32.NewProc("CreateProcessAsUserW")
)

const (
        SecurityImpersonation = 2
        TokenPrimary          = 1
        CREATE_UNICODE_ENVIRONMENT = 0x00000400
        CREATE_NO_WINDOW           = 0x08000000
        NORMAL_PRIORITY_CLASS      = 0x00000020
)

type HelperProcess struct {
        mu          sync.Mutex
        processInfo *windows.ProcessInformation
        helperPath  string
        serverURL   string
        computerName string
        interval    int
        quality     int
        maxSizeKB   int
        logPath     string
        running     bool
}

func NewHelperProcess(helperPath, serverURL, computerName string, interval, quality, maxSizeKB int, logPath string) *HelperProcess {
        return &HelperProcess{
                helperPath:   helperPath,
                serverURL:    serverURL,
                computerName: computerName,
                interval:     interval,
                quality:      quality,
                maxSizeKB:    maxSizeKB,
                logPath:      logPath,
        }
}

func (h *HelperProcess) StartInUserSession(sessionID uint32, username string) error {
        h.mu.Lock()
        defer h.mu.Unlock()

        if h.running {
                return nil
        }

        var userToken windows.Token
        ret, _, err := procWTSQueryUserToken.Call(
                uintptr(sessionID),
                uintptr(unsafe.Pointer(&userToken)),
        )
        if ret == 0 {
                return fmt.Errorf("WTSQueryUserToken failed: %v", err)
        }
        defer userToken.Close()

        var duplicatedToken windows.Token
        ret, _, err = procDuplicateTokenEx.Call(
                uintptr(userToken),
                windows.MAXIMUM_ALLOWED,
                0,
                SecurityImpersonation,
                TokenPrimary,
                uintptr(unsafe.Pointer(&duplicatedToken)),
        )
        if ret == 0 {
                return fmt.Errorf("DuplicateTokenEx failed: %v", err)
        }
        defer duplicatedToken.Close()

        var envBlock uintptr
        ret, _, err = procCreateEnvironmentBlock.Call(
                uintptr(unsafe.Pointer(&envBlock)),
                uintptr(duplicatedToken),
                0,
        )
        if ret == 0 {
                return fmt.Errorf("CreateEnvironmentBlock failed: %v", err)
        }
        defer procDestroyEnvironmentBlock.Call(envBlock)

        helperPath := h.helperPath
        if !filepath.IsAbs(helperPath) {
                exePath, _ := os.Executable()
                helperPath = filepath.Join(filepath.Dir(exePath), helperPath)
        }

        cmdLine := fmt.Sprintf(`"%s" -server=%s -computer=%s -user=%s -interval=%d -quality=%d -maxsize=%d`,
                helperPath, h.serverURL, h.computerName, username, h.interval, h.quality, h.maxSizeKB)
        
        if h.logPath != "" {
                logFile := filepath.Join(filepath.Dir(h.logPath), "screenshot-helper.log")
                cmdLine += fmt.Sprintf(` -log="%s"`, logFile)
        }

        cmdLinePtr, _ := syscall.UTF16PtrFromString(cmdLine)

        var startupInfo windows.StartupInfo
        startupInfo.Cb = uint32(unsafe.Sizeof(startupInfo))
        startupInfo.Desktop, _ = syscall.UTF16PtrFromString("winsta0\\default")

        var processInfo windows.ProcessInformation

        ret, _, err = procCreateProcessAsUserW.Call(
                uintptr(duplicatedToken),
                0,
                uintptr(unsafe.Pointer(cmdLinePtr)),
                0,
                0,
                0,
                CREATE_UNICODE_ENVIRONMENT|CREATE_NO_WINDOW|NORMAL_PRIORITY_CLASS,
                envBlock,
                0,
                uintptr(unsafe.Pointer(&startupInfo)),
                uintptr(unsafe.Pointer(&processInfo)),
        )

        if ret == 0 {
                return fmt.Errorf("CreateProcessAsUserW failed: %v", err)
        }

        // Close thread handle immediately - we don't need it
        if processInfo.Thread != 0 {
                windows.CloseHandle(processInfo.Thread)
                processInfo.Thread = 0
        }

        h.processInfo = &processInfo
        h.running = true

        log.Printf("Screenshot helper started in session %d (PID: %d)", sessionID, processInfo.ProcessId)

        return nil
}

func (h *HelperProcess) Stop() error {
        h.mu.Lock()
        defer h.mu.Unlock()

        if !h.running || h.processInfo == nil {
                return nil
        }

        handle := h.processInfo.Process
        if handle != 0 {
                windows.TerminateProcess(handle, 0)
                windows.WaitForSingleObject(handle, 5000) // Wait up to 5 seconds
                windows.CloseHandle(handle)
        }

        h.processInfo = nil
        h.running = false

        log.Println("Screenshot helper stopped")
        return nil
}

func (h *HelperProcess) IsRunning() bool {
        h.mu.Lock()
        defer h.mu.Unlock()

        if !h.running || h.processInfo == nil {
                return false
        }

        handle := h.processInfo.Process
        if handle == 0 {
                h.running = false
                return false
        }

        var exitCode uint32
        err := windows.GetExitCodeProcess(handle, &exitCode)
        if err != nil {
                return false
        }

        // STILL_ACTIVE = 259
        if exitCode != 259 {
                windows.CloseHandle(handle)
                h.processInfo = nil
                h.running = false
                return false
        }

        return true
}

func FindHelperExecutable() string {
        exePath, err := os.Executable()
        if err != nil {
                return "agent-sh.exe"
        }
        
        dir := filepath.Dir(exePath)
        helperPath := filepath.Join(dir, "agent-sh.exe")
        
        if _, err := os.Stat(helperPath); err == nil {
                return helperPath
        }
        
        return "agent-sh.exe"
}
