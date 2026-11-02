//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	gdiplus  = syscall.NewLazyDLL("gdiplus.dll")

	pRegisterClassExW              = user32.NewProc("RegisterClassExW")
	pCreateWindowExW               = user32.NewProc("CreateWindowExW")
	pDefWindowProcW                = user32.NewProc("DefWindowProcW")
	pDestroyWindow                 = user32.NewProc("DestroyWindow")
	pShowWindow                    = user32.NewProc("ShowWindow")
	pUpdateWindow                  = user32.NewProc("UpdateWindow")
	pGetMessageW                   = user32.NewProc("GetMessageW")
	pTranslateMessage              = user32.NewProc("TranslateMessage")
	pDispatchMessageW              = user32.NewProc("DispatchMessageW")
	pPostQuitMessage               = user32.NewProc("PostQuitMessage")
	pPostMessageW                  = user32.NewProc("PostMessageW")
	pBeginPaint                    = user32.NewProc("BeginPaint")
	pEndPaint                      = user32.NewProc("EndPaint")
	pInvalidateRect                = user32.NewProc("InvalidateRect")
	pGetClientRect                 = user32.NewProc("GetClientRect")
	pGetDC                         = user32.NewProc("GetDC")
	pReleaseDC                     = user32.NewProc("ReleaseDC")
	pSetTimer                      = user32.NewProc("SetTimer")
	pKillTimer                     = user32.NewProc("KillTimer")
	pLoadCursorW                   = user32.NewProc("LoadCursorW")
	pSetCursor                     = user32.NewProc("SetCursor")
	pSetCapture                    = user32.NewProc("SetCapture")
	pReleaseCapture                = user32.NewProc("ReleaseCapture")
	pGetKeyState                   = user32.NewProc("GetKeyState")
	pRegisterHotKey                = user32.NewProc("RegisterHotKey")
	pUnregisterHotKey              = user32.NewProc("UnregisterHotKey")
	pFillRect                      = user32.NewProc("FillRect")
	pDrawTextW                     = user32.NewProc("DrawTextW")
	pSetWindowPos                  = user32.NewProc("SetWindowPos")
	pSetForegroundWindow           = user32.NewProc("SetForegroundWindow")
	pFindWindowW                   = user32.NewProc("FindWindowW")
	pCreatePopupMenu               = user32.NewProc("CreatePopupMenu")
	pAppendMenuW                   = user32.NewProc("AppendMenuW")
	pTrackPopupMenu                = user32.NewProc("TrackPopupMenu")
	pDestroyMenu                   = user32.NewProc("DestroyMenu")
	pGetCursorPos                  = user32.NewProc("GetCursorPos")
	pRegisterWindowMessageW        = user32.NewProc("RegisterWindowMessageW")
	pSetLayeredWindowAttributes    = user32.NewProc("SetLayeredWindowAttributes")
	pEnumDisplayDevicesW           = user32.NewProc("EnumDisplayDevicesW")
	pEnumDisplaySettingsW          = user32.NewProc("EnumDisplaySettingsW")
	pGetDisplayConfigBufferSizes   = user32.NewProc("GetDisplayConfigBufferSizes")
	pQueryDisplayConfig            = user32.NewProc("QueryDisplayConfig")
	pDisplayConfigGetDeviceInfo    = user32.NewProc("DisplayConfigGetDeviceInfo")
	pSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	pSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	pGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	pGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	pTrackMouseEvent               = user32.NewProc("TrackMouseEvent")
	pIsWindowVisible               = user32.NewProc("IsWindowVisible")
	pIsIconic                      = user32.NewProc("IsIconic")
	pLoadImageW                    = user32.NewProc("LoadImageW")
	pSendMessageW                  = user32.NewProc("SendMessageW")
	pMonitorFromPoint              = user32.NewProc("MonitorFromPoint")
	pGetMonitorInfoW               = user32.NewProc("GetMonitorInfoW")

	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pDeleteDC               = gdi32.NewProc("DeleteDC")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pSetDCBrushColor        = gdi32.NewProc("SetDCBrushColor")
	pGetStockObject         = gdi32.NewProc("GetStockObject")
	pCreateFontW            = gdi32.NewProc("CreateFontW")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pGetTextExtentPoint32W  = gdi32.NewProc("GetTextExtentPoint32W")
	pSaveDC                 = gdi32.NewProc("SaveDC")
	pRestoreDC              = gdi32.NewProc("RestoreDC")
	pIntersectClipRect      = gdi32.NewProc("IntersectClipRect")
	pCreatePen              = gdi32.NewProc("CreatePen")
	pPolyline               = gdi32.NewProc("Polyline")
	pAddFontMemResourceEx   = gdi32.NewProc("AddFontMemResourceEx")
	pCreateDCW              = gdi32.NewProc("CreateDCW")
	pSetDeviceGammaRamp     = gdi32.NewProc("SetDeviceGammaRamp")
	pGetDeviceGammaRamp     = gdi32.NewProc("GetDeviceGammaRamp")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pCreateMutexW     = kernel32.NewProc("CreateMutexW")

	pShell_NotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	pShellExecuteW     = shell32.NewProc("ShellExecuteW")

	pRegCreateKeyExW = advapi32.NewProc("RegCreateKeyExW")
	pRegSetValueExW  = advapi32.NewProc("RegSetValueExW")

	pGdiplusStartup         = gdiplus.NewProc("GdiplusStartup")
	pGdipCreateFromHDC      = gdiplus.NewProc("GdipCreateFromHDC")
	pGdipDeleteGraphics     = gdiplus.NewProc("GdipDeleteGraphics")
	pGdipSetSmoothingMode   = gdiplus.NewProc("GdipSetSmoothingMode")
	pGdipSetPixelOffsetMode = gdiplus.NewProc("GdipSetPixelOffsetMode")
	pGdipCreatePen1         = gdiplus.NewProc("GdipCreatePen1")
	pGdipDeletePen          = gdiplus.NewProc("GdipDeletePen")
	pGdipDrawLines          = gdiplus.NewProc("GdipDrawLines")
	pGdipSetPenLineJoin     = gdiplus.NewProc("GdipSetPenLineJoin")
)

const (
	WM_DESTROY         = 0x0002
	WM_SIZE            = 0x0005
	WM_PAINT           = 0x000F
	WM_CLOSE           = 0x0010
	WM_QUERYENDSESSION = 0x0011
	WM_ENDSESSION      = 0x0016
	WM_ERASEBKGND      = 0x0014
	WM_SETCURSOR       = 0x0020
	WM_GETMINMAXINFO   = 0x0024
	WM_SETICON         = 0x0080
	WM_NCHITTEST       = 0x0084
	WM_KEYDOWN         = 0x0100
	WM_CHAR            = 0x0102
	WM_SYSKEYDOWN      = 0x0104
	WM_SYSCHAR         = 0x0106
	WM_TIMER           = 0x0113
	WM_MOUSEMOVE       = 0x0200
	WM_LBUTTONDOWN     = 0x0201
	WM_LBUTTONUP       = 0x0202
	WM_LBUTTONDBLCLK   = 0x0203
	WM_RBUTTONUP       = 0x0205
	WM_MOUSEWHEEL      = 0x020A
	WM_MOUSELEAVE      = 0x02A3
	WM_HOTKEY          = 0x0312
	WM_DISPLAYCHANGE   = 0x007E
	WM_DPICHANGED      = 0x02E0
	WM_ACTIVATE        = 0x0006
	WM_APP             = 0x8000
	WM_TRAY            = WM_APP + 1
	WM_SHOWME          = WM_APP + 2

	WS_POPUP        = 0x80000000
	WS_SYSMENU      = 0x00080000
	WS_MINIMIZEBOX  = 0x00020000
	WS_CLIPCHILDREN = 0x02000000

	WS_EX_APPWINDOW   = 0x00040000
	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_LAYERED     = 0x00080000
	WS_EX_TRANSPARENT = 0x00000020
	WS_EX_NOACTIVATE  = 0x08000000

	CS_DBLCLKS    = 0x0008
	CS_DROPSHADOW = 0x00020000

	SW_HIDE           = 0
	SW_SHOWNORMAL     = 1
	SW_MINIMIZE       = 6
	SW_SHOWNOACTIVATE = 4
	SW_RESTORE        = 9

	HTCLIENT  = 1
	HTCAPTION = 2

	DC_BRUSH = 18

	DT_CENTER       = 0x1
	DT_RIGHT        = 0x2
	DT_VCENTER      = 0x4
	DT_SINGLELINE   = 0x20
	DT_NOPREFIX     = 0x800
	DT_END_ELLIPSIS = 0x8000

	SRCCOPY = 0x00CC0020

	MOD_NOREPEAT = 0x4000

	NIM_ADD     = 0
	NIM_MODIFY  = 1
	NIM_DELETE  = 2
	NIF_MESSAGE = 0x1
	NIF_ICON    = 0x2
	NIF_TIP     = 0x4

	MF_STRING       = 0x0
	MF_SEPARATOR    = 0x800
	MF_CHECKED      = 0x8
	MF_GRAYED       = 0x1
	TPM_RETURNCMD   = 0x100
	TPM_RIGHTBUTTON = 0x2
	TPM_BOTTOMALIGN = 0x20

	HKEY_CURRENT_USER  = 0x80000001
	HKEY_LOCAL_MACHINE = 0x80000002
	KEY_SET_VALUE      = 0x0002
	KEY_QUERY_VALUE    = 0x0001
	KEY_WOW64_64KEY    = 0x0100
	REG_SZ             = 1
	REG_DWORD          = 4

	SWP_NOSIZE     = 0x1
	SWP_NOMOVE     = 0x2
	SWP_NOZORDER   = 0x4
	SWP_NOACTIVATE = 0x10
	SWP_SHOWWINDOW = 0x40
)

type WNDCLASSEXW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type MSG struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      POINT
	private uint32
}

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }

type PAINTSTRUCT struct {
	hdc         uintptr
	fErase      int32
	rcPaint     RECT
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type SIZE struct{ CX, CY int32 }

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type NOTIFYICONDATAW struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         GUID
	hBalloonIcon     uintptr
}

type TRACKMOUSEEVENT struct {
	cbSize      uint32
	dwFlags     uint32
	hwndTrack   uintptr
	dwHoverTime uint32
}

type MONITORINFO struct {
	cbSize    uint32
	rcMonitor RECT
	rcWork    RECT
	dwFlags   uint32
}

func u16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func u16copy(dst []uint16, s string) {
	r, _ := syscall.UTF16FromString(s)
	n := copy(dst[:len(dst)-1], r)
	dst[n] = 0
}

func colorref(c uint32) uintptr {
	return uintptr((c>>16)&0xFF | (c & 0xFF00) | (c&0xFF)<<16)
}

func lo16(v uintptr) int { return int(int16(v & 0xFFFF)) }
func hi16(v uintptr) int { return int(int16((v >> 16) & 0xFFFF)) }

func ptr[T any](p *T) uintptr { return uintptr(unsafe.Pointer(p)) }
