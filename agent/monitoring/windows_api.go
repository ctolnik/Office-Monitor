// +build windows

package monitoring

import (
        "golang.org/x/sys/windows"
)

var (
        modUser32   = windows.NewLazySystemDLL("user32.dll")
        modGdi32    = windows.NewLazySystemDLL("gdi32.dll")
        modKernel32 = windows.NewLazySystemDLL("kernel32.dll")

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
)
