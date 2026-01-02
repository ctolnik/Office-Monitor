//go:build windows
// +build windows

package monitoring

import (
        "os"
        "syscall"
        "unsafe"

        "golang.org/x/sys/windows"
)

var (
        modUser32   = windows.NewLazySystemDLL("user32.dll")
        modGdi32    = windows.NewLazySystemDLL("gdi32.dll")
        modKernel32 = windows.NewLazySystemDLL("kernel32.dll")
        modWtsapi32 = windows.NewLazySystemDLL("wtsapi32.dll")

        procGetDC                     = modUser32.NewProc("GetDC")
        procReleaseDC                 = modUser32.NewProc("ReleaseDC")
        procGetSystemMetrics          = modUser32.NewProc("GetSystemMetrics")
        procGetForegroundWindow       = modUser32.NewProc("GetForegroundWindow")
        procGetWindowTextW            = modUser32.NewProc("GetWindowTextW")
        procGetWindowThreadProcessId  = modUser32.NewProc("GetWindowThreadProcessId")
        procPostThreadMessage         = modUser32.NewProc("PostThreadMessageW")
        procPeekMessage               = modUser32.NewProc("PeekMessageW")
        procCreateCompatibleDC        = modGdi32.NewProc("CreateCompatibleDC")
        procCreateCompatibleBitmap    = modGdi32.NewProc("CreateCompatibleBitmap")
        procSelectObject              = modGdi32.NewProc("SelectObject")
        procBitBlt                    = modGdi32.NewProc("BitBlt")
        procDeleteDC                  = modGdi32.NewProc("DeleteDC")
        procDeleteObject              = modGdi32.NewProc("DeleteObject")
        procGetDIBits                 = modGdi32.NewProc("GetDIBits")
        procOpenProcess               = modKernel32.NewProc("OpenProcess")
        procCloseHandle               = modKernel32.NewProc("CloseHandle")
        procQueryFullProcessImageName = modKernel32.NewProc("QueryFullProcessImageNameW")
        procGetCurrentThreadId        = modKernel32.NewProc("GetCurrentThreadId")

        procWTSEnumerateSessions      = modWtsapi32.NewProc("WTSEnumerateSessionsW")
        procWTSQuerySessionInformation = modWtsapi32.NewProc("WTSQuerySessionInformationW")
        procWTSFreeMemory             = modWtsapi32.NewProc("WTSFreeMemory")
)

const (
        WTS_CURRENT_SERVER_HANDLE = 0
        WTSActive                 = 0
        WTSUserName               = 5
)

type WTS_SESSION_INFO struct {
        SessionId      uint32
        WinStationName *uint16
        State          uint32
}

func GetActiveSessionUsername() string {
        username := os.Getenv("USERNAME")
        if username != "" && username != "SYSTEM" {
                return username
        }

        wtsUsername := getWTSActiveUsername()
        if wtsUsername != "" {
                return wtsUsername
        }

        if username != "" {
                return username
        }
        return "SYSTEM"
}

func getWTSActiveUsername() string {
        var sessionInfoPtr uintptr
        var count uint32

        ret, _, _ := procWTSEnumerateSessions.Call(
                WTS_CURRENT_SERVER_HANDLE,
                0,
                1,
                uintptr(unsafe.Pointer(&sessionInfoPtr)),
                uintptr(unsafe.Pointer(&count)),
        )

        if ret == 0 || sessionInfoPtr == 0 {
                return ""
        }
        defer procWTSFreeMemory.Call(sessionInfoPtr)

        sessionSize := unsafe.Sizeof(WTS_SESSION_INFO{})
        for i := uint32(0); i < count; i++ {
                session := (*WTS_SESSION_INFO)(unsafe.Pointer(sessionInfoPtr + uintptr(i)*sessionSize))

                if session.State == WTSActive && session.SessionId != 0 {
                        username := querySessionUsername(session.SessionId)
                        if username != "" {
                                return username
                        }
                }
        }

        return ""
}

func querySessionUsername(sessionId uint32) string {
        var buffer *uint16
        var bytesReturned uint32

        ret, _, _ := procWTSQuerySessionInformation.Call(
                WTS_CURRENT_SERVER_HANDLE,
                uintptr(sessionId),
                WTSUserName,
                uintptr(unsafe.Pointer(&buffer)),
                uintptr(unsafe.Pointer(&bytesReturned)),
        )

        if ret == 0 || buffer == nil {
                return ""
        }
        defer procWTSFreeMemory.Call(uintptr(unsafe.Pointer(buffer)))

        return syscall.UTF16ToString((*[256]uint16)(unsafe.Pointer(buffer))[:])
}
