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
        WTSConnected              = 1
        WTSConnectQuery           = 2
        WTSShadow                 = 3
        WTSDisconnected           = 4
        WTSIdle                   = 5
        WTSUserName               = 5
)

type WTS_SESSION_INFO struct {
        SessionId      uint32
        WinStationName *uint16
        State          uint32
}

func GetActiveSessionUsername() string {
        username, _ := GetActiveSessionInfo()
        return username
}

func GetActiveSessionInfo() (username string, sessionID uint32) {
        envUsername := os.Getenv("USERNAME")
        if envUsername != "" && envUsername != "SYSTEM" {
                sessionID = getActiveSessionID()
                return envUsername, sessionID
        }

        username, sessionID = getWTSActiveSessionInfo()
        if username != "" {
                return username, sessionID
        }

        username, sessionID = getWTSConnectedSessionInfo()
        if username != "" {
                return username, sessionID
        }

        if envUsername != "" {
                return envUsername, 0
        }
        return "SYSTEM", 0
}

func getActiveSessionID() uint32 {
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
                return 0
        }
        defer procWTSFreeMemory.Call(sessionInfoPtr)

        sessionSize := unsafe.Sizeof(WTS_SESSION_INFO{})
        for i := uint32(0); i < count; i++ {
                session := (*WTS_SESSION_INFO)(unsafe.Pointer(sessionInfoPtr + uintptr(i)*sessionSize))
                if session.State == WTSActive && session.SessionId != 0 {
                        return session.SessionId
                }
        }

        return 0
}

func getWTSActiveSessionInfo() (string, uint32) {
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
                return "", 0
        }
        defer procWTSFreeMemory.Call(sessionInfoPtr)

        sessionSize := unsafe.Sizeof(WTS_SESSION_INFO{})
        for i := uint32(0); i < count; i++ {
                session := (*WTS_SESSION_INFO)(unsafe.Pointer(sessionInfoPtr + uintptr(i)*sessionSize))

                if session.State == WTSActive && session.SessionId != 0 {
                        username := querySessionUsername(session.SessionId)
                        if username != "" {
                                return username, session.SessionId
                        }
                }
        }

        return "", 0
}

func getWTSActiveUsername() string {
        username, _ := getWTSActiveSessionInfo()
        return username
}

func getWTSConnectedSessionInfo() (string, uint32) {
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
                return "", 0
        }
        defer procWTSFreeMemory.Call(sessionInfoPtr)

        sessionSize := unsafe.Sizeof(WTS_SESSION_INFO{})
        for i := uint32(0); i < count; i++ {
                session := (*WTS_SESSION_INFO)(unsafe.Pointer(sessionInfoPtr + uintptr(i)*sessionSize))

                if session.SessionId != 0 && session.State == WTSConnected {
                        username := querySessionUsername(session.SessionId)
                        if username != "" && username != "SYSTEM" {
                                return username, session.SessionId
                        }
                }
        }

        return "", 0
}

func EnumerateAllUserSessions() []struct {
        SessionID uint32
        Username  string
        State     uint32
} {
        var sessionInfoPtr uintptr
        var count uint32
        var result []struct {
                SessionID uint32
                Username  string
                State     uint32
        }

        ret, _, _ := procWTSEnumerateSessions.Call(
                WTS_CURRENT_SERVER_HANDLE,
                0,
                1,
                uintptr(unsafe.Pointer(&sessionInfoPtr)),
                uintptr(unsafe.Pointer(&count)),
        )

        if ret == 0 || sessionInfoPtr == 0 {
                return result
        }
        defer procWTSFreeMemory.Call(sessionInfoPtr)

        sessionSize := unsafe.Sizeof(WTS_SESSION_INFO{})
        for i := uint32(0); i < count; i++ {
                session := (*WTS_SESSION_INFO)(unsafe.Pointer(sessionInfoPtr + uintptr(i)*sessionSize))

                if session.SessionId != 0 {
                        username := querySessionUsername(session.SessionId)
                        if username != "" && username != "SYSTEM" {
                                result = append(result, struct {
                                        SessionID uint32
                                        Username  string
                                        State     uint32
                                }{
                                        SessionID: session.SessionId,
                                        Username:  username,
                                        State:     session.State,
                                })
                        }
                }
        }

        return result
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

func GetSessionUsername(sessionID uint32) string {
        return querySessionUsername(sessionID)
}
