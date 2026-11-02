//go:build windows

package main

import (
	"math"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

type winPlat struct {
	hwnd, osd    uintptr
	hinst        uintptr
	hkRegistered map[int]bool
	vibEx        bool
	cfgDir       string
	now          float64

	app   *App
	ui    *UI
	gp    *gdiPainter
	scale float64

	memDC, memBmp   uintptr
	bw, bh          int32
	tracking        bool
	trayAdded       bool
	hIconBig        uintptr
	hIconSm         uintptr
	taskbarMsg      uint32
	quitting        bool
	magInit         bool
	satCur          float64
	trayNoticeShown bool

	osdTitle, osdSub string
	osdW, osdH       int32
}

var W *winPlat
var start = time.Now()

func since() float64 { return time.Since(start).Seconds() }

// ======================= GDI painter =======================

type gdiPainter struct {
	hdc   uintptr
	s     float64
	fonts map[FontID]uintptr
	fontS float64
	clipN int
}

func (p *gdiPainter) setScale(s float64) {
	if p.fonts != nil && p.fontS == s {
		return
	}
	for _, f := range p.fonts {
		pDeleteObject.Call(f)
	}
	p.fonts = map[FontID]uintptr{}
	p.fontS = s
	p.s = s
	for id, spec := range Fonts {
		h, _, _ := pCreateFontW.Call(uintptr(int32(-math.Round(spec.Px*s))), 0, 0, 0, uintptr(spec.Weight), 0, 0, 0,
			1 /*DEFAULT_CHARSET*/, 4 /*OUT_TT_PRECIS*/, 0, 5 /*CLEARTYPE_QUALITY*/, 0, uintptr(unsafe.Pointer(u16(spec.Face))))
		p.fonts[FontID(id)] = h
	}
}

func (p *gdiPainter) rc(r Rect) RECT {
	return RECT{int32(math.Round(r.X * p.s)), int32(math.Round(r.Y * p.s)), int32(math.Round((r.X + r.W) * p.s)), int32(math.Round((r.Y + r.H) * p.s))}
}

func (p *gdiPainter) Fill(r Rect, c uint32) {
	rc := p.rc(r)
	if rc.Right <= rc.Left {
		rc.Right = rc.Left + 1
	}
	if rc.Bottom <= rc.Top {
		rc.Bottom = rc.Top + 1
	}
	pSetDCBrushColor.Call(p.hdc, colorref(c))
	br, _, _ := pGetStockObject.Call(DC_BRUSH)
	pFillRect.Call(p.hdc, ptr(&rc), br)
}

func (p *gdiPainter) Line(x1, y1, x2, y2, w float64, c uint32) {
	p.Poly([]Pt{{x1, y1}, {x2, y2}}, w, c)
}

var gdipOK bool

func (p *gdiPainter) Poly(pts []Pt, w float64, c uint32) {
	if len(pts) < 2 {
		return
	}
	if gdipOK {
		var g uintptr
		if r, _, _ := pGdipCreateFromHDC.Call(p.hdc, ptr(&g)); r == 0 {
			pGdipSetSmoothingMode.Call(g, 4)   // AntiAlias
			pGdipSetPixelOffsetMode.Call(g, 4) // Half
			var pen uintptr
			argb := uintptr(0xFF000000 | c)
			pGdipCreatePen1.Call(argb, uintptr(math.Float32bits(float32(w*p.s))), 2 /*UnitPixel*/, ptr(&pen))
			if pen != 0 {
				pGdipSetPenLineJoin.Call(pen, 2) // round
				fp := make([]float32, 0, len(pts)*2)
				for _, q := range pts {
					fp = append(fp, float32(q.X*p.s), float32(q.Y*p.s))
				}
				pGdipDrawLines.Call(g, pen, uintptr(unsafe.Pointer(&fp[0])), uintptr(len(pts)))
				pGdipDeletePen.Call(pen)
			}
			pGdipDeleteGraphics.Call(g)
			return
		}
	}
	pen, _, _ := pCreatePen.Call(0, uintptr(math.Max(1, math.Round(w*p.s))), colorref(c))
	old, _, _ := pSelectObject.Call(p.hdc, pen)
	ip := make([]POINT, len(pts))
	for i, q := range pts {
		ip[i] = POINT{int32(math.Round(q.X * p.s)), int32(math.Round(q.Y * p.s))}
	}
	pPolyline.Call(p.hdc, uintptr(unsafe.Pointer(&ip[0])), uintptr(len(ip)))
	pSelectObject.Call(p.hdc, old)
	pDeleteObject.Call(pen)
}

func (p *gdiPainter) Text(r Rect, s string, f FontID, c uint32, align int) {
	if s == "" {
		return
	}
	rc := p.rc(r)
	old, _, _ := pSelectObject.Call(p.hdc, p.fonts[f])
	pSetTextColor.Call(p.hdc, colorref(c))
	pSetBkMode.Call(p.hdc, 1)
	flags := uintptr(DT_SINGLELINE | DT_VCENTER | DT_NOPREFIX | DT_END_ELLIPSIS)
	switch align {
	case AlignC:
		flags |= DT_CENTER
	case AlignR:
		flags |= DT_RIGHT
	}
	str, _ := syscall.UTF16FromString(s)
	pDrawTextW.Call(p.hdc, uintptr(unsafe.Pointer(&str[0])), uintptr(len(str)-1), ptr(&rc), flags)
	pSelectObject.Call(p.hdc, old)
}

func (p *gdiPainter) Measure(s string, f FontID) float64 {
	if s == "" {
		return 0
	}
	old, _, _ := pSelectObject.Call(p.hdc, p.fonts[f])
	str, _ := syscall.UTF16FromString(s)
	var sz SIZE
	pGetTextExtentPoint32W.Call(p.hdc, uintptr(unsafe.Pointer(&str[0])), uintptr(len(str)-1), ptr(&sz))
	pSelectObject.Call(p.hdc, old)
	return float64(sz.CX) / p.s
}

func (p *gdiPainter) Clip(r Rect) {
	pSaveDC.Call(p.hdc)
	p.clipN++
	rc := p.rc(r)
	pIntersectClipRect.Call(p.hdc, uintptr(rc.Left), uintptr(rc.Top), uintptr(rc.Right), uintptr(rc.Bottom))
}

func (p *gdiPainter) Unclip() {
	if p.clipN > 0 {
		pRestoreDC.Call(p.hdc, ^uintptr(0)) // -1
		p.clipN--
	}
}

// ======================= Window =======================

func (w *winPlat) dpiScale() float64 {
	if pGetDpiForWindow.Find() == nil && w.hwnd != 0 {
		if d, _, _ := pGetDpiForWindow.Call(w.hwnd); d > 0 {
			return float64(d) / 96
		}
	}
	return 1
}

func (w *winPlat) ensureBuffer() {
	var rc RECT
	pGetClientRect.Call(w.hwnd, ptr(&rc))
	cw, ch := rc.Right-rc.Left, rc.Bottom-rc.Top
	if cw < 1 || ch < 1 {
		return
	}
	if w.memDC != 0 && cw == w.bw && ch == w.bh {
		return
	}
	hdc, _, _ := pGetDC.Call(w.hwnd)
	if w.memDC == 0 {
		w.memDC, _, _ = pCreateCompatibleDC.Call(hdc)
	}
	if w.memBmp != 0 {
		pDeleteObject.Call(w.memBmp)
	}
	w.memBmp, _, _ = pCreateCompatibleBitmap.Call(hdc, uintptr(cw), uintptr(ch))
	pSelectObject.Call(w.memDC, w.memBmp)
	pReleaseDC.Call(w.hwnd, hdc)
	w.bw, w.bh = cw, ch
}

// render runs UI frames into the back buffer and schedules a repaint.
func (w *winPlat) render() {
	if w.hwnd == 0 || w.app == nil {
		return
	}
	w.ensureBuffer()
	if w.memDC == 0 {
		return
	}
	w.gp.hdc = w.memDC
	w.gp.setScale(w.scale)
	u := w.ui
	u.W, u.H = float64(w.bw)/w.scale, float64(w.bh)/w.scale
	u.Time = since()
	for i := 0; i < 3; i++ {
		u.Dirty = false
		w.app.Frame(u)
		for w.gp.clipN > 0 {
			w.gp.Unclip()
		}
		u.In.Pressed, u.In.Released, u.In.Dbl, u.In.Wheel = false, false, false, 0
		u.In.Chars, u.In.Keys = u.In.Chars[:0], u.In.Keys[:0]
		if !u.Dirty {
			break
		}
	}
	pInvalidateRect.Call(w.hwnd, 0, 0)
}

func keyMods() uint32 {
	var m uint32
	down := func(vk uintptr) bool { r, _, _ := pGetKeyState.Call(vk); return int16(r) < 0 }
	if down(0x11) {
		m |= ModCtrl
	}
	if down(0x12) {
		m |= ModAlt
	}
	if down(0x10) {
		m |= ModShift
	}
	if down(0x5B) || down(0x5C) {
		m |= ModWin
	}
	return m
}

func (w *winPlat) mouse(lp uintptr) {
	w.ui.In.MX = float64(lo16(lp)) / w.scale
	w.ui.In.MY = float64(hi16(lp)) / w.scale
	w.ui.In.Shift = keyMods()&ModShift != 0
}

var cursors = map[int]uintptr{}

func cursorFor(c int) uintptr {
	if h, ok := cursors[c]; ok {
		return h
	}
	id := map[int]uintptr{CurArrow: 32512, CurHand: 32649, CurBeam: 32513, CurSize: 32644}[c]
	h, _, _ := pLoadCursorW.Call(0, id)
	cursors[c] = h
	return h
}

func wndProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	w := W
	if w == nil || w.ui == nil || w.app == nil {
		r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wp, lp)
		return r
	}
	if w.taskbarMsg != 0 && msg == w.taskbarMsg {
		w.trayAdded = false
		w.addTray()
		return 0
	}
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := pBeginPaint.Call(hwnd, ptr(&ps))
		if w.memDC != 0 {
			pBitBlt.Call(hdc, 0, 0, uintptr(w.bw), uintptr(w.bh), w.memDC, 0, 0, SRCCOPY)
		}
		pEndPaint.Call(hwnd, ptr(&ps))
		return 0
	case WM_SIZE:
		if wp != 1 { // not minimized
			w.render()
		}
		return 0
	case WM_NCHITTEST:
		x := float64(lo16(lp))
		y := float64(hi16(lp))
		var rc RECT
		getWindowRect(hwnd, &rc)
		cx, cy := (x-float64(rc.Left))/w.scale, (y-float64(rc.Top))/w.scale
		if cy >= 0 && cy < TitleH && cx < float64(w.bw)/w.scale-96-map[bool]float64{true: SupportW + 16}[supportShown()] && w.ui.Drop == "" {
			return HTCAPTION
		}
		return HTCLIENT
	case WM_SETCURSOR:
		if lo16(lp) == HTCLIENT {
			pSetCursor.Call(cursorFor(w.ui.Cursor))
			return 1
		}
	case WM_MOUSEMOVE:
		w.mouse(lp)
		w.ui.In.Down = wp&0x1 != 0
		if !w.tracking {
			tme := TRACKMOUSEEVENT{cbSize: uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})), dwFlags: 0x2, hwndTrack: hwnd}
			pTrackMouseEvent.Call(ptr(&tme))
			w.tracking = true
		}
		w.render()
		pSetCursor.Call(cursorFor(w.ui.Cursor))
		return 0
	case WM_MOUSELEAVE:
		w.tracking = false
		if !w.ui.In.Down {
			w.ui.In.MX, w.ui.In.MY = -1000, -1000
			w.render()
		}
		return 0
	case WM_LBUTTONDOWN, WM_LBUTTONDBLCLK:
		w.mouse(lp)
		w.ui.In.Down, w.ui.In.Pressed = true, true
		w.ui.In.Dbl = msg == WM_LBUTTONDBLCLK
		pSetCapture.Call(hwnd)
		w.render()
		return 0
	case WM_LBUTTONUP:
		w.mouse(lp)
		w.ui.In.Down, w.ui.In.Released = false, true
		pReleaseCapture.Call()
		w.render()
		w.render() // let Active clear
		return 0
	case WM_MOUSEWHEEL:
		w.ui.In.Wheel = float64(hi16(wp)) / 120
		w.render()
		return 0
	case WM_KEYDOWN, WM_SYSKEYDOWN:
		capturing := w.ui.Cap != ""
		w.ui.In.Keys = append(w.ui.In.Keys, KeyEvent{VK: uint32(wp), Mods: keyMods()})
		w.render()
		if capturing || (msg == WM_SYSKEYDOWN && wp == 0x79) {
			return 0 // swallow Alt combos / F10 while binding
		}
		if msg == WM_KEYDOWN {
			return 0
		}
	case WM_SYSCHAR:
		if w.ui.Cap != "" {
			return 0
		}
	case WM_CHAR:
		w.ui.In.Chars = append(w.ui.In.Chars, rune(wp))
		w.render()
		return 0
	case WM_TIMER:
		if wp == 1 {
			w.now = since()
			if w.app.Tick(w.now) || w.ui.Focus != "" || w.app.confirmDel > 0 {
				if visible(hwnd) {
					w.render()
				}
			}
		} else if wp == 2 {
			pKillTimer.Call(hwnd, 2)
			pShowWindow.Call(w.osd, SW_HIDE)
		}
		return 0
	case WM_HOTKEY:
		w.app.OnHotkey(int(wp))
		w.updateTrayTip()
		if visible(hwnd) {
			w.render()
		}
		return 0
	case WM_DISPLAYCHANGE:
		w.app.OnDisplayChange()
		w.render()
	case WM_DPICHANGED:
		w.scale = float64(lo16(wp)) / 96
		rc := (*RECT)(unsafe.Pointer(lp))
		pSetWindowPos.Call(hwnd, 0, uintptr(rc.Left), uintptr(rc.Top), uintptr(rc.Right-rc.Left), uintptr(rc.Bottom-rc.Top), SWP_NOZORDER|SWP_NOACTIVATE)
		w.render()
		return 0
	case WM_TRAY:
		switch lo16(lp) {
		case WM_LBUTTONUP, WM_LBUTTONDBLCLK:
			w.showMain()
		case WM_RBUTTONUP:
			w.trayMenu()
		}
		return 0
	case WM_SHOWME:
		w.showMain()
		return 0
	case WM_CLOSE:
		if w.app.cfg.CloseToTray && !w.quitting {
			w.HideToTray()
		} else {
			w.Quit()
		}
		return 0
	case WM_QUERYENDSESSION:
		return 1
	case WM_ENDSESSION:
		if wp != 0 {
			w.app.Shutdown()
		}
		return 0
	case WM_DESTROY:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wp, lp)
	return r
}

var pGetWindowRect = user32.NewProc("GetWindowRect")

func getWindowRect(h uintptr, rc *RECT) { pGetWindowRect.Call(h, ptr(rc)) }

func visible(h uintptr) bool {
	v, _, _ := pIsWindowVisible.Call(h)
	ic, _, _ := pIsIconic.Call(h)
	return v != 0 && ic == 0
}

func (w *winPlat) showMain() {
	pShowWindow.Call(w.hwnd, SW_RESTORE)
	pShowWindow.Call(w.hwnd, SW_SHOWNORMAL)
	pSetForegroundWindow.Call(w.hwnd)
	w.render()
}

func (w *winPlat) Minimize() { pShowWindow.Call(w.hwnd, SW_MINIMIZE) }
func (w *winPlat) HideToTray() {
	pShowWindow.Call(w.hwnd, SW_HIDE)
	if !w.trayNoticeShown && w.trayAdded {
		w.trayNoticeShown = true
		n := w.nid()
		n.uFlags |= 0x10 // NIF_INFO
		u16copy(n.szInfoTitle[:], "TarkovNV is still running")
		u16copy(n.szInfo[:], "Your hotkeys stay active. Right-click the tray icon to switch presets or exit.")
		n.dwInfoFlags = 1 // NIIF_INFO
		pShell_NotifyIconW.Call(NIM_MODIFY, ptr(&n))
	}
}
func (w *winPlat) Quit() {
	if w.quitting {
		return
	}
	w.quitting = true
	w.app.Shutdown()
	w.removeTray()
	pDestroyWindow.Call(w.hwnd)
}

// ======================= Tray =======================

func (w *winPlat) nid() NOTIFYICONDATAW {
	n := NOTIFYICONDATAW{hWnd: w.hwnd, uID: 1, uFlags: NIF_MESSAGE | NIF_ICON | NIF_TIP, uCallbackMessage: WM_TRAY, hIcon: w.hIconSm}
	n.cbSize = uint32(unsafe.Sizeof(n))
	tip := "TarkovNV"
	if w.app != nil {
		tip = "TarkovNV — " + w.app.cur().Name
	}
	u16copy(n.szTip[:], tip)
	return n
}

func (w *winPlat) addTray() {
	if w.trayAdded {
		return
	}
	n := w.nid()
	r, _, _ := pShell_NotifyIconW.Call(NIM_ADD, ptr(&n))
	w.trayAdded = r != 0
}

func (w *winPlat) updateTrayTip() {
	if w.trayAdded {
		n := w.nid()
		pShell_NotifyIconW.Call(NIM_MODIFY, ptr(&n))
	}
}

func (w *winPlat) removeTray() {
	if w.trayAdded {
		n := w.nid()
		pShell_NotifyIconW.Call(NIM_DELETE, ptr(&n))
		w.trayAdded = false
	}
}

func (w *winPlat) trayMenu() {
	m, _, _ := pCreatePopupMenu.Call()
	for i, p := range w.app.cfg.Presets {
		fl := uintptr(MF_STRING)
		if i == w.app.cfg.Active {
			fl |= MF_CHECKED
		}
		label := p.Name
		if !p.Hotkey.Empty() {
			label += "\t" + p.Hotkey.String()
		}
		pAppendMenuW.Call(m, fl, uintptr(1000+i), uintptr(unsafe.Pointer(u16(label))))
	}
	pAppendMenuW.Call(m, MF_SEPARATOR, 0, 0)
	pAppendMenuW.Call(m, MF_STRING, 2, uintptr(unsafe.Pointer(u16("Open TarkovNV"))))
	if supportShown() {
		pAppendMenuW.Call(m, MF_STRING, 4, uintptr(unsafe.Pointer(u16("Support TarkovNV (GitHub page)"))))
	}
	pAppendMenuW.Call(m, MF_STRING, 3, uintptr(unsafe.Pointer(u16("Exit (restore colours)"))))
	var pt POINT
	pGetCursorPos.Call(ptr(&pt))
	pSetForegroundWindow.Call(w.hwnd)
	cmd, _, _ := pTrackPopupMenu.Call(m, TPM_RETURNCMD|TPM_RIGHTBUTTON|TPM_BOTTOMALIGN, uintptr(pt.X), uintptr(pt.Y), 0, w.hwnd, 0)
	pDestroyMenu.Call(m)
	switch {
	case cmd == 4:
		w.OpenURL(SupportURL)
	case cmd == 2:
		w.showMain()
	case cmd == 3:
		w.Quit()
	case cmd >= 1000:
		w.app.Activate(int(cmd-1000), true)
		w.updateTrayTip()
		w.render()
	}
}

// ======================= On-screen indicator =======================

func osdProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	w := W
	switch msg {
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := pBeginPaint.Call(hwnd, ptr(&ps))
		mdc, _, _ := pCreateCompatibleDC.Call(hdc)
		bmp, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(w.osdW), uintptr(w.osdH))
		pSelectObject.Call(mdc, bmp)
		p := &gdiPainter{hdc: mdc}
		p.setScale(w.scale)
		lw, lh := float64(w.osdW)/w.scale, float64(w.osdH)/w.scale
		r := R(0, 0, lw, lh)
		p.Fill(r, cPanel)
		Stroke(p, r, cNVDim)
		p.Fill(R(0, 0, 6, lh), cNV)
		Brackets(p, r.Inset(4), 8, cTan)
		p.Text(R(22, 10, lw-120, 18), "TARKOV NV // PRESET", FTiny, cTan, AlignL)
		p.Text(R(22, 28, lw-40, 30), w.osdTitle, FHead, cNV, AlignL)
		p.Text(R(lw-150, 10, 130, 18), w.osdSub, FMonoS, cTextDim, AlignR)
		pBitBlt.Call(hdc, 0, 0, uintptr(w.osdW), uintptr(w.osdH), mdc, 0, 0, SRCCOPY)
		for _, f := range p.fonts {
			pDeleteObject.Call(f)
		}
		pDeleteObject.Call(bmp)
		pDeleteDC.Call(mdc)
		pEndPaint.Call(hwnd, ptr(&ps))
		return 0
	case WM_NCHITTEST:
		return ^uintptr(0) // HTTRANSPARENT
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wp, lp)
	return r
}

func (w *winPlat) ShowOSD(title, sub, dev string) {
	if w.osd == 0 {
		return
	}
	w.osdTitle, w.osdSub = title, sub
	w.osdW, w.osdH = int32(380*w.scale), int32(68*w.scale)
	x, y, mw := int32(0), int32(0), int32(1920)
	for _, d := range w.app.displays {
		if d.Device == dev {
			x, y, mw = int32(d.X), int32(d.Y), int32(d.W)
		}
	}
	px := x + (mw-w.osdW)/2
	py := y + int32(48*w.scale)
	pSetWindowPos.Call(w.osd, ^uintptr(0) /*HWND_TOPMOST*/, uintptr(px), uintptr(py), uintptr(w.osdW), uintptr(w.osdH), SWP_NOACTIVATE|SWP_SHOWWINDOW)
	pShowWindow.Call(w.osd, SW_SHOWNOACTIVATE)
	pInvalidateRect.Call(w.osd, 0, 0)
	pUpdateWindow.Call(w.osd)
	pSetTimer.Call(w.hwnd, 2, 1700, 0)
}

// ======================= Startup =======================

func loadFonts() {
	for _, f := range embeddedFonts() {
		var n uint32
		pAddFontMemResourceEx.Call(uintptr(unsafe.Pointer(&f.data[0])), uintptr(len(f.data)), 0, ptr(&n))
	}
}

func initGdiplus() {
	if pGdiplusStartup.Find() != nil {
		return
	}
	in := struct {
		version  uint32
		cb       uintptr
		noThread int32
		noCodecs int32
	}{version: 1}
	var tok uintptr
	r, _, _ := pGdiplusStartup.Call(ptr(&tok), ptr(&in), 0)
	gdipOK = r == 0
}

func hasArg(a string) bool {
	for _, x := range os.Args[1:] {
		if x == a {
			return true
		}
	}
	return false
}

func main() {
	runtime.LockOSThread()

	if hasArg("--unlock-gamma") {
		os.Exit(unlockGammaRange())
	}

	// single instance
	pCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(u16("TarkovNV_SingleInstance_Mutex"))))
	if e := syscall.GetLastError(); e == syscall.Errno(183) { // ERROR_ALREADY_EXISTS
		if h, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(u16("TarkovNV_Main"))), 0); h != 0 {
			pPostMessageW.Call(h, WM_SHOWME, 0, 0)
		}
		return
	}

	if pSetProcessDpiAwarenessContext.Find() == nil {
		pSetProcessDpiAwarenessContext.Call(^uintptr(3)) // -4 = PER_MONITOR_AWARE_V2
	} else if pSetProcessDPIAware.Find() == nil {
		pSetProcessDPIAware.Call()
	}
	loadFonts()
	initGdiplus()

	w := &winPlat{hkRegistered: map[int]bool{}, scale: 1}
	W = w
	w.hinst, _, _ = pGetModuleHandleW.Call(0)
	w.hIconBig, _, _ = pLoadImageW.Call(w.hinst, 1, 1 /*IMAGE_ICON*/, 32, 32, 0)
	w.hIconSm, _, _ = pLoadImageW.Call(w.hinst, 1, 1, 16, 16, 0)
	w.taskbarMsg = uint32(func() uintptr {
		r, _, _ := pRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(u16("TaskbarCreated"))))
		return r
	}())

	cls := WNDCLASSEXW{
		style:         CS_DBLCLKS | CS_DROPSHADOW,
		lpfnWndProc:   syscall.NewCallback(wndProc),
		hInstance:     w.hinst,
		hIcon:         w.hIconBig,
		hIconSm:       w.hIconSm,
		hCursor:       cursorFor(CurArrow),
		lpszClassName: u16("TarkovNV_Main"),
	}
	cls.cbSize = uint32(unsafe.Sizeof(cls))
	pRegisterClassExW.Call(ptr(&cls))

	osdCls := WNDCLASSEXW{lpfnWndProc: syscall.NewCallback(osdProc), hInstance: w.hinst, lpszClassName: u16("TarkovNV_OSD"), hCursor: cursorFor(CurArrow)}
	osdCls.cbSize = uint32(unsafe.Sizeof(osdCls))
	pRegisterClassExW.Call(ptr(&osdCls))

	// create hidden first so we can query its DPI, then size it
	w.hwnd, _, _ = pCreateWindowExW.Call(WS_EX_APPWINDOW, uintptr(unsafe.Pointer(u16("TarkovNV_Main"))), uintptr(unsafe.Pointer(u16("TarkovNV"))),
		WS_POPUP|WS_SYSMENU|WS_MINIMIZEBOX|WS_CLIPCHILDREN, 0, 0, 100, 100, 0, 0, w.hinst, 0)
	if w.hwnd == 0 {
		return
	}
	w.osd, _, _ = pCreateWindowExW.Call(WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_LAYERED|WS_EX_TRANSPARENT|WS_EX_NOACTIVATE,
		uintptr(unsafe.Pointer(u16("TarkovNV_OSD"))), 0, WS_POPUP, 0, 0, 10, 10, 0, 0, w.hinst, 0)
	if w.osd != 0 {
		pSetLayeredWindowAttributes.Call(w.osd, 0, 238, 2 /*LWA_ALPHA*/)
	}

	// centre on the monitor under the cursor
	var pt POINT
	pGetCursorPos.Call(ptr(&pt))
	mon, _, _ := pMonitorFromPoint.Call(uintptr(uint32(pt.X))|uintptr(uint32(pt.Y))<<32, 2 /*NEAREST*/)
	mi := MONITORINFO{}
	mi.cbSize = uint32(unsafe.Sizeof(mi))
	pGetMonitorInfoW.Call(mon, ptr(&mi))
	pSetWindowPos.Call(w.hwnd, 0, uintptr(mi.rcWork.Left+10), uintptr(mi.rcWork.Top+10), 10, 10, SWP_NOZORDER|SWP_NOACTIVATE)
	w.scale = w.dpiScale()
	ww, wh := int32(AppW*w.scale), int32(AppH*w.scale)
	workW, workH := mi.rcWork.Right-mi.rcWork.Left, mi.rcWork.Bottom-mi.rcWork.Top
	if ww > workW || wh > workH { // tiny screens: shrink to fit
		f := math.Min(float64(workW)/float64(ww), float64(workH)/float64(wh)) * 0.98
		w.scale *= f
		ww, wh = int32(AppW*w.scale), int32(AppH*w.scale)
	}
	x := mi.rcWork.Left + (workW-ww)/2
	y := mi.rcWork.Top + (workH-wh)/2
	pSetWindowPos.Call(w.hwnd, 0, uintptr(x), uintptr(y), uintptr(ww), uintptr(wh), SWP_NOZORDER|SWP_NOACTIVATE)

	if w.hIconBig != 0 {
		pSendMessageW.Call(w.hwnd, WM_SETICON, 1, w.hIconBig)
		pSendMessageW.Call(w.hwnd, WM_SETICON, 0, w.hIconSm)
	}

	w.gp = &gdiPainter{}
	w.ui = &UI{P: w.gp}
	w.ui.In.MX, w.ui.In.MY = -1000, -1000
	w.app = NewApp(w)
	w.now = since()
	w.app.Tick(w.now)
	w.addTray()

	w.render()
	pShowWindow.Call(w.hwnd, SW_SHOWNORMAL)
	pUpdateWindow.Call(w.hwnd)
	pSetForegroundWindow.Call(w.hwnd)
	pSetTimer.Call(w.hwnd, 1, 250, 0)

	var m MSG
	for {
		r, _, _ := pGetMessageW.Call(ptr(&m), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(ptr(&m))
		pDispatchMessageW.Call(ptr(&m))
	}
	if !w.quitting {
		w.app.Shutdown()
		w.removeTray()
	}
}

// ======================= Images (test scenes) =======================

var (
	pStretchDIBits     = gdi32.NewProc("StretchDIBits")
	pSetStretchBltMode = gdi32.NewProc("SetStretchBltMode")
	pSetBrushOrgEx     = gdi32.NewProc("SetBrushOrgEx")
)

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

func (p *gdiPainter) Image(r Rect, b *PixBuf) {
	if b == nil || len(b.Pix) == 0 {
		return
	}
	rc := p.rc(r)
	bi := bitmapInfoHeader{biSize: 40, biWidth: int32(b.W), biHeight: -int32(b.H), biPlanes: 1, biBitCount: 32}
	pSetStretchBltMode.Call(p.hdc, 4) // HALFTONE
	pSetBrushOrgEx.Call(p.hdc, 0, 0, 0)
	pStretchDIBits.Call(p.hdc, uintptr(rc.Left), uintptr(rc.Top), uintptr(rc.Right-rc.Left), uintptr(rc.Bottom-rc.Top),
		0, 0, uintptr(b.W), uintptr(b.H), uintptr(unsafe.Pointer(&b.Pix[0])), ptr(&bi), 0, SRCCOPY)
}
