package main

import (
	"fmt"
	"math"
	"strings"
)

const (
	AppW    = 1120
	AppH    = 744
	TitleH  = 48
	StatusH = 30
	SideW   = 300
	Version = "1.7.1"

	hkIDNext   = 1
	hkIDPrev   = 2
	hkIDToggle = 3
	hkIDPreset = 100
)

type App struct {
	plat Platform
	cfg  *Config

	displays []DisplayInfo
	disp     int
	vib      VibInfo
	vibOrig  int
	vibDev   string
	satOK    bool // Windows colour-filter boost available
	satErr   bool // last boost attempt failed

	tab       int // 0 editor, 1 settings
	editCh    int // 0 linked, 1 R, 2 G, 3 B
	failedHK  map[int]bool
	hkDirty   bool
	saveDirty bool
	saveAt    float64

	applied    *Ramp
	applyOK    bool
	applyErr   string
	unlocked   bool
	tarkov     bool
	autoPrev   int
	autoActive bool
	lastOther  int // last non-default preset for the toggle hotkey
	listScroll float64
	suspended  bool
	now        float64
	confirmDel float64
	lastSlow   float64

	scenes      []*Scene
	userScenes  []*Scene
	scene       *Scene
	sceneView   SceneView
	holdDefault bool
	scnDropWas  bool
	undo        undoState
	profKey     string // monitor whose preset list is loaded
}

func NewApp(p Platform) *App {
	a := &App{plat: p, failedHK: map[int]bool{}, lastOther: 1, autoPrev: -1}
	a.scenes = BuiltinScenes()
	a.scene = a.scenes[0]
	a.cfg = LoadConfig(p.ConfigDir())
	a.cfg.StartMin = false
	a.satOK = p.SaturationSupported()
	a.RefreshDisplays()
	a.unlocked = p.GammaUnlocked()
	a.hkDirty = true
	if !a.cfg.Presets[a.cfg.Active].IsNeutral() {
		a.lastOther = a.cfg.Active
	}
	a.editCh = 0
	if !a.cur().Linked {
		a.editCh = 1
	}
	a.Apply()
	return a
}

func (a *App) cur() *Preset { return &a.cfg.Presets[a.cfg.Active] }

func (a *App) display() *DisplayInfo {
	if a.disp >= 0 && a.disp < len(a.displays) {
		return &a.displays[a.disp]
	}
	return nil
}

func (a *App) RefreshDisplays() {
	a.displays = a.plat.Displays()
	a.disp = -1
	for i, d := range a.displays {
		if d.Device == a.cfg.Display {
			a.disp = i
		}
	}
	if a.disp < 0 {
		for i, d := range a.displays {
			if d.Primary {
				a.disp = i
			}
		}
	}
	if a.disp < 0 && len(a.displays) > 0 {
		a.disp = 0
	}
	if d := a.display(); d != nil {
		a.cfg.Display = d.Device
		a.loadProfileFor(d)
		if a.vibDev != d.Device {
			a.restoreVibrance()
			a.vib = a.plat.Vibrance(d.Device)
			a.vibDev = d.Device
			a.vibOrig = a.vib.Cur
		}
	}
}

func (a *App) SelectDisplay(i int) {
	if i == a.disp || i < 0 || i >= len(a.displays) {
		return
	}
	if d := a.display(); d != nil {
		a.plat.SetGamma(d.Device, IdentityRamp())
	}
	a.cfg.Display = a.displays[i].Device
	a.RefreshDisplays()
	a.markSave()
	a.Apply()
}

func (a *App) vibLevel(pct float64) int {
	v := a.vib
	if pct >= 50 {
		return v.Def + int(math.Round(float64(v.Max-v.Def)*(pct-50)/50))
	}
	return v.Min + int(math.Round(float64(v.Def-v.Min)*pct/50))
}

func (a *App) vibMinPct() float64 {
	if a.vib.OK && a.vib.Min >= a.vib.Def {
		return 50 // driver only supports boosting
	}
	return 0
}

const VibBoostMax = 250.0

// vibAvailable: can this PC do vibrance at all?
func (a *App) vibAvailable() bool { return a.vib.OK || a.satOK }

// vibRange is the slider range for vibrance.
func (a *App) vibRange() (float64, float64) {
	if a.satOK {
		return 0, VibBoostMax
	}
	return a.vibMinPct(), 100
}

// vibPlan splits a vibrance % into the NVIDIA driver level and an extra
// full-screen saturation factor (1 = none).
//
//	NVIDIA:  0..100 % -> driver (as NVIDIA Control Panel), above 100 % -> driver max + boost
//	others:  everything through the Windows colour filter, 50 % = neutral
func (a *App) vibPlan(v float64) (drv int, sat float64) {
	sat = 1
	if a.vib.OK {
		mn := a.vibMinPct()
		drv = a.vibLevel(clamp(v, mn, 100))
		if v > 100 {
			sat = 1 + (v-100)/100
		} else if v < mn && mn > 0 {
			sat = v / mn
		}
	} else {
		sat = v / 50
	}
	if !a.satOK {
		sat = 1
	}
	return
}

func (a *App) setSat(s float64) {
	if !a.satOK {
		return
	}
	a.satErr = a.plat.SetSaturation(s) != nil && s != 1
}

// Apply pushes the active preset to the selected display.
func (a *App) Apply() {
	d := a.display()
	if d == nil {
		a.applyOK, a.applyErr = false, "NO DISPLAY FOUND"
		return
	}
	p := a.cur()
	ramp := p.Ramp()
	if a.holdDefault {
		a.plat.SetGamma(d.Device, IdentityRamp())
		if a.vib.OK {
			a.plat.SetVibrance(d.Device, a.vib.Def)
		}
		a.setSat(1)
		return
	}
	if err := a.plat.SetGamma(d.Device, ramp); err != nil {
		a.applyOK = false
		a.applyErr = err.Error()
	} else {
		a.applyOK, a.applyErr = true, ""
	}
	a.applied = ramp
	drv, sat := a.vibPlan(p.Vibrance)
	if a.vib.OK {
		a.plat.SetVibrance(d.Device, drv)
	}
	a.setSat(sat)
}

func (a *App) restoreVibrance() {
	if a.vib.OK && a.vibDev != "" {
		a.plat.SetVibrance(a.vibDev, a.vibOrig)
	}
}

func (a *App) Activate(i int, fromHotkey bool) {
	if i < 0 || i >= len(a.cfg.Presets) {
		return
	}
	a.cfg.Active = i
	if !a.cur().IsNeutral() {
		a.lastOther = i
	}
	if a.cur().Linked {
		a.editCh = 0
	} else if a.editCh == 0 {
		a.editCh = 1
	}
	a.Apply()
	a.markSave()
	if fromHotkey && a.cfg.ShowOSD {
		d := a.display()
		dev := ""
		if d != nil {
			dev = d.Device
		}
		sub := fmt.Sprintf("PRESET %d/%d", i+1, len(a.cfg.Presets))
		if !a.applyOK {
			sub = "WINDOWS REJECTED CURVE"
		}
		a.plat.ShowOSD(a.cur().Name, sub, dev)
	}
}

func (a *App) defaultIndex() int {
	for i := range a.cfg.Presets {
		if a.cfg.Presets[i].IsNeutral() {
			return i
		}
	}
	return 0
}

func (a *App) OnHotkey(id int) {
	n := len(a.cfg.Presets)
	switch {
	case id == hkIDNext:
		a.Activate((a.cfg.Active+1)%n, true)
	case id == hkIDPrev:
		a.Activate((a.cfg.Active-1+n)%n, true)
	case id == hkIDToggle:
		def := a.defaultIndex()
		if a.cfg.Active == def {
			t := a.lastOther
			if t < 0 || t >= n || t == def {
				t = (def + 1) % n
			}
			a.Activate(t, true)
		} else {
			a.Activate(def, true)
		}
	case id >= hkIDPreset && id < hkIDPreset+n:
		a.Activate(id-hkIDPreset, true)
	}
}

func (a *App) hotkeyList() []HotkeyBinding {
	var l []HotkeyBinding
	add := func(id int, h Hotkey) {
		if !h.Empty() {
			l = append(l, HotkeyBinding{id, h})
		}
	}
	add(hkIDNext, a.cfg.HKNext)
	add(hkIDPrev, a.cfg.HKPrev)
	add(hkIDToggle, a.cfg.HKToggle)
	for i, p := range a.cfg.Presets {
		add(hkIDPreset+i, p.Hotkey)
	}
	return l
}

func (a *App) markSave() { a.saveDirty = true; a.saveAt = a.now + 0.8 }

// Tick is called about every 250 ms.
func (a *App) Tick(now float64) (redraw bool) {
	a.now = now
	if a.saveDirty && now >= a.saveAt {
		a.syncProfile()
		SaveConfig(a.plat.ConfigDir(), a.cfg)
		a.saveDirty = false
	}
	if a.hkDirty && !a.suspended {
		a.failedHK = a.plat.RegisterHotkeys(a.hotkeyList())
		a.hkDirty = false
		redraw = true
	}
	// slow checks every ~2 s
	if now-a.lastSlow >= 2 {
		a.lastSlow = now
		if u := a.plat.GammaUnlocked(); u != a.unlocked {
			a.unlocked = u
			if u {
				a.Apply()
			}
			redraw = true
		}
		if a.cfg.KeepApplied && a.applyOK && a.applied != nil && !a.holdDefault {
			if d := a.display(); d != nil {
				if g, err := a.plat.GetGamma(d.Device); err == nil && !g.Near(a.applied, 600) {
					a.Apply()
				}
			}
		}
		t := a.plat.TarkovRunning()
		if t != a.tarkov {
			a.tarkov = t
			redraw = true
			if a.cfg.AutoTarkov {
				if t {
					if idx := a.presetByName(a.cfg.AutoPreset); idx >= 0 && idx != a.cfg.Active {
						a.autoPrev = a.cfg.Active
						a.autoActive = true
						a.Activate(idx, true)
					}
				} else if a.autoActive {
					a.autoActive = false
					if a.autoPrev >= 0 {
						a.Activate(a.autoPrev, true)
					}
				}
			}
		}
	}
	return redraw
}

func (a *App) presetByName(n string) int {
	for i, p := range a.cfg.Presets {
		if p.Name == n {
			return i
		}
	}
	return -1
}

func (a *App) OnDisplayChange() {
	a.RefreshDisplays()
	a.Apply()
}

// Shutdown restores the desktop.
func (a *App) Shutdown() {
	if a.saveDirty {
		a.syncProfile()
		SaveConfig(a.plat.ConfigDir(), a.cfg)
	}
	if a.cfg.ResetOnExit {
		if d := a.display(); d != nil {
			a.plat.SetGamma(d.Device, IdentityRamp())
		}
		a.restoreVibrance()
	}
	a.setSat(1) // the colour filter never outlives TarkovNV
}

// ================================ Drawing ================================

func (a *App) Frame(u *UI) {
	p := u.P
	u.BeginFrame()
	// suspend global hotkeys while capturing a new binding
	if (u.Cap != "") != a.suspended {
		a.suspended = u.Cap != ""
		a.plat.SuspendHotkeys(a.suspended)
		if !a.suspended {
			a.hkDirty = true
		}
	}

	a.undoKeys(u)
	p.Fill(R(0, 0, u.W, u.H), cBg)
	a.drawTitle(u)
	a.drawSidebar(u)
	main := R(SideW, TitleH, u.W-SideW, u.H-TitleH-StatusH)
	p.Fill(main, cBg)
	a.drawTabs(u, main)
	body := R(main.X+28, main.Y+64, main.W-56, main.H-80)
	switch a.tab {
	case 0:
		a.drawEditor(u, body)
	case 2:
		a.drawScenes(u, body)
	case 3:
		a.drawTuning(u, body)
	default:
		a.drawSettings(u, body)
	}
	a.drawStatus(u)
	Stroke(p, R(0, 0, u.W, u.H), cLine2)
	u.EndFrame()
	a.trackUndo(u)
}

func (a *App) drawTitle(u *UI) {
	p := u.P
	bar := R(0, 0, u.W, TitleH)
	p.Fill(bar, cPanel)
	p.Fill(R(0, TitleH-1, u.W, 1), cLine)
	Hatch(p, R(0, 0, 10, TitleH-1), 6, cLine2)
	x := 24.0
	w := p.Measure("TARKOV", FStencilL)
	p.Text(R(x, 4, w+4, TitleH-6), "TARKOV", FStencilL, cText, AlignL)
	x += w + 8
	nv := R(x, 11, 42, 26)
	p.Fill(nv, cNV)
	p.Text(nv.Cut(0, 1, 0, 0), "NV", FStencil, cBg, AlignC)
	x += 56
	p.Fill(R(x, 12, 1, 24), cLine2)
	p.Text(R(x+14, 8, 360, 18), "VISUAL PRESET CONTROLLER", FTiny, cTan, AlignL)
	p.Text(R(x+14, 24, 420, 16), "GAMMA · BRIGHTNESS · CONTRAST · DIGITAL VIBRANCE", FMonoS, cTextMute, AlignL)

	a.drawSupport(u)

	// window buttons
	bw := 48.0
	cl := R(u.W-bw, 0, bw, TitleH-1)
	mn := R(u.W-2*bw, 0, bw, TitleH-1)
	if u.hit("win-min", mn) {
		p.Fill(mn, cPanel3)
		u.Cursor = CurHand
		if u.In.Released {
			a.plat.Minimize()
		}
	}
	p.Fill(R(mn.X+18, mn.Y+24, 12, 1.5), cTextDim)
	ch := u.hit("win-close", cl)
	if ch {
		p.Fill(cl, cRed)
		u.Cursor = CurHand
		u.Tip(cl, map[bool]string{true: "Hide to tray (hotkeys keep working)", false: "Exit TarkovNV"}[a.cfg.CloseToTray])
		if u.In.Released {
			if a.cfg.CloseToTray {
				a.plat.HideToTray()
			} else {
				a.plat.Quit()
			}
		}
	}
	xc := uint32(cTextDim)
	if ch {
		xc = cText
	}
	cx, cy := cl.X+cl.W/2, cl.Y+cl.H/2
	p.Line(cx-6, cy-6, cx+6, cy+6, 1.5, xc)
	p.Line(cx-6, cy+6, cx+6, cy-6, 1.5, xc)
}

func vendorColor(v string) uint32 {
	switch v {
	case "NVIDIA":
		return cNV
	case "AMD":
		return cRed
	case "INTEL":
		return cChB
	}
	return cTextDim
}

func (a *App) drawSidebar(u *UI) {
	p := u.P
	side := R(0, TitleH, SideW, u.H-TitleH-StatusH)
	p.Fill(side, cPanel)
	p.Fill(R(SideW-1, TitleH, 1, side.H), cLine)
	x, w := 20.0, SideW-40.0

	// ---- display ----
	y := TitleH + 18.0
	SectionHeader(p, x, y, w, "01", "TARGET DISPLAY")
	card := R(x, y+28, w, 96)
	d := a.display()
	hot := u.hit("dispcard", card) || u.Drop == "disp"
	p.Fill(card, map[bool]uint32{true: cPanel3, false: cPanel2}[hot])
	Stroke(p, card, map[bool]uint32{true: cTan, false: cLine2}[hot])
	Brackets(p, card, 8, map[bool]uint32{true: cNV, false: cTan}[hot])
	if d != nil {
		p.Text(R(card.X+14, card.Y+10, card.W-50, 22), strings.ToUpper(d.Monitor), FHead, cText, AlignL)
		p.Text(R(card.X+14, card.Y+33, card.W-40, 18), fmt.Sprintf("%d × %d  @  %d HZ", d.W, d.H, d.Hz), FMono, cTextDim, AlignL)
		p.Text(R(card.X+14, card.Y+52, card.W-28, 16), strings.ToUpper(d.Adapter), FSmall, cTextDim, AlignL)
		bx := card.X + 14
		if d.Primary {
			bx += Badge(p, bx, card.Y+70, "PRIMARY", cTan, AlignL) + 6
		}
		bx += Badge(p, bx, card.Y+70, d.Vendor, vendorColor(d.Vendor), AlignL) + 6
		Badge(p, bx, card.Y+70, d.ShortDev(), cTextDim, AlignL)
	} else {
		p.Text(card, "NO DISPLAY DETECTED", FHead, cRed, AlignC)
	}
	// chevron
	cx, cy := card.Right()-22, card.Y+22
	p.Line(cx-5, cy-2, cx, cy+3, 1.5, cTextDim)
	p.Line(cx, cy+3, cx+5, cy-2, 1.5, cTextDim)
	if hot {
		if a.cfg.PerMonitor {
			u.Tip(card, "Choose which monitor TarkovNV controls - each monitor keeps its own presets")
		} else {
			u.Tip(card, "Choose which monitor TarkovNV controls (the one Tarkov runs on)")
		}
	}
	var items []DropItem
	for _, dd := range a.displays {
		b, bc := dd.Vendor, vendorColor(dd.Vendor)
		if dd.Primary {
			b, bc = "PRIMARY", cTan
		}
		sub := fmt.Sprintf("%s · %d×%d @ %d Hz · %s", dd.ShortDev(), dd.W, dd.H, dd.Hz, dd.Adapter)
		if a.cfg.PerMonitor {
			if n := a.profileCount(dd.ID); n > 0 {
				sub = fmt.Sprintf("%d presets · %s", n, sub)
			} else {
				sub = "new list (copies current) · " + sub
			}
		}
		items = append(items, DropItem{strings.ToUpper(dd.Monitor), sub, b, bc})
	}
	u.DropW = 420
	if c := u.Dropdown("disp", card, items, a.disp, 52); c >= 0 {
		a.SelectDisplay(c)
	}

	// ---- presets ----
	y = card.Bottom() + 26
	ph := "PRESETS"
	if a.cfg.PerMonitor {
		ph = "PRESETS · THIS MONITOR"
	}
	SectionHeader(p, x, y, w, "02", ph)
	btnY := side.Bottom() - 20 - 36
	list := R(x, y+28, w, btnY-12-(y+28))
	rowH := 50.0
	n := len(a.cfg.Presets)
	maxScroll := math.Max(0, float64(n)*rowH-list.H)
	if list.Has(u.In.MX, u.In.MY) && u.In.Wheel != 0 && u.Drop == "" {
		a.listScroll = clamp(a.listScroll-u.In.Wheel*rowH, 0, maxScroll)
		u.Dirty = true
	}
	a.listScroll = clamp(a.listScroll, 0, maxScroll)
	p.Clip(list)
	for i := 0; i < n; i++ {
		pr := &a.cfg.Presets[i]
		row := R(list.X, list.Y+float64(i)*rowH-a.listScroll, list.W, rowH-4)
		if row.Bottom() < list.Y || row.Y > list.Bottom() {
			continue
		}
		id := "preset" + itoa(i)
		rhot := u.hit(id, row) && list.Has(u.In.MX, u.In.MY)
		active := i == a.cfg.Active
		bg := uint32(cPanel)
		if rhot {
			bg = cPanel2
			u.Cursor = CurHand
			if u.In.Pressed {
				a.Activate(i, false)
				u.Dirty = true
			}
		}
		if active {
			bg = cNVDeep
		}
		p.Fill(row, bg)
		border := uint32(cLine)
		if active {
			border = cNVDim
		}
		Stroke(p, row, border)
		if active {
			p.Fill(R(row.X, row.Y, 4, row.H), cNV)
		}
		nc := uint32(cText)
		if active {
			nc = cNV
		}
		p.Text(R(row.X+16, row.Y+5, row.W-90, 22), pr.Name, FBodyB, nc, AlignL)
		hk := pr.Hotkey.String()
		hc := uint32(cTextDim)
		if a.failedHK[hkIDPreset+i] && !pr.Hotkey.Empty() {
			hk += " · IN USE"
			hc = cRed
		}
		p.Text(R(row.X+16, row.Y+25, row.W-90, 16), hk, FMonoS, hc, AlignL)
		// mini curve thumbnail
		thumb := R(row.Right()-58, row.Y+7, 46, row.H-14)
		p.Fill(thumb, cBg)
		Stroke(p, thumb, cLine)
		pts := make([]Pt, 0, 24)
		for k := 0; k <= 23; k++ {
			t := float64(k) / 23
			v := pr.Eval(1, t)
			pts = append(pts, Pt{thumb.X + 2 + t*(thumb.W-4), thumb.Bottom() - 2 - v*(thumb.H-4)})
		}
		cc := uint32(cTextDim)
		if active {
			cc = cNV
		}
		p.Poly(pts, 1.2, cc)
		if active {
			Badge(p, thumb.X-8, row.Y+(row.H-18)/2, "LIVE", cNV, AlignR)
		}
	}
	p.Unclip()
	if maxScroll > 0 {
		sb := R(list.Right()+6, list.Y, 3, list.H)
		p.Fill(sb, cPanel2)
		th := list.H * list.H / (float64(n) * rowH)
		p.Fill(R(sb.X, list.Y+(list.H-th)*a.listScroll/maxScroll, 3, th), cLine2)
	}

	bw := (w - 16) / 3
	if u.Button("p-new", R(x, btnY, bw, 36), "+ NEW", BtnNormal, n < 30) {
		np := mkPreset(a.uniqueName("NEW PRESET"), 0, 0, 1, 50, Hotkey{})
		a.cfg.Presets = append(a.cfg.Presets, np)
		a.Activate(len(a.cfg.Presets)-1, false)
		a.listScroll = 1e9
		a.hkDirty = true
		a.tab = 0
		u.Focus = "name"
	}
	if u.Button("p-dup", R(x+bw+8, btnY, bw, 36), "DUPLICATE", BtnNormal, n < 30) {
		np := *a.cur()
		np.Name = a.uniqueName(np.Name)
		np.Hotkey = Hotkey{}
		a.cfg.Presets = append(a.cfg.Presets, np)
		a.Activate(len(a.cfg.Presets)-1, false)
		a.listScroll = 1e9
		a.hkDirty = true
	}
	delLabel := "DELETE"
	confirming := a.confirmDel > a.now
	if confirming {
		delLabel = "CONFIRM?"
	}
	if u.Button("p-del", R(x+2*(bw+8), btnY, bw, 36), delLabel, map[bool]int{true: BtnWarn, false: BtnDanger}[confirming], n > 1) {
		if confirming {
			a.cfg.Presets = append(a.cfg.Presets[:a.cfg.Active], a.cfg.Presets[a.cfg.Active+1:]...)
			if a.cfg.Active >= len(a.cfg.Presets) {
				a.cfg.Active = len(a.cfg.Presets) - 1
			}
			a.confirmDel = 0
			a.Activate(a.cfg.Active, false)
			a.hkDirty = true
		} else {
			a.confirmDel = a.now + 3
		}
	}
}

func (a *App) uniqueName(base string) string {
	base = strings.TrimSpace(base)
	if a.presetByName(base) < 0 {
		return base
	}
	for i := 2; ; i++ {
		n := fmt.Sprintf("%s %d", base, i)
		if len(n) > 24 {
			n = n[len(n)-24:]
		}
		if a.presetByName(n) < 0 {
			return n
		}
	}
}

func (a *App) drawTabs(u *UI, main Rect) {
	p := u.P
	tabs := []struct {
		idx int
		t   string
	}{{0, "EDITOR"}, {3, "RGB & TONE"}, {2, "TEST SCENES"}, {1, "SETTINGS"}}
	x := main.X + 28
	y := main.Y + 14
	for _, tb := range tabs {
		i, t := tb.idx, tb.t
		w := p.Measure(t, FHead) + 28
		r := R(x, y, w, 36)
		hot := u.hit("tab"+itoa(i), r)
		if hot {
			u.Cursor = CurHand
			if u.In.Pressed {
				a.tab = i
				u.Dirty = true
			}
		}
		c := uint32(cTextDim)
		if hot {
			c = cText
		}
		if a.tab == i {
			c = cText
			p.Fill(R(r.X, r.Bottom()-2, r.W, 2), cNV)
		}
		p.Text(r, t, FHead, c, AlignC)
		x += w + 6
	}
	a.drawUndo(u, x+14, y)
	p.Fill(R(main.X+28, y+36, main.W-56, 1), cLine)
	// live status on the right
	d := a.display()
	if d != nil {
		s := "LIVE ON " + d.ShortDev()
		c := uint32(cNV)
		if !a.applyOK {
			s, c = "NOT APPLIED", cRed
		}
		w := p.Measure(s, FMono)
		rx := main.Right() - 28
		p.Text(R(rx-w, y, w, 36), s, FMono, c, AlignR)
		p.Fill(R(rx-w-16, y+15, 7, 7), c)
	}
}

func fmtSigned(v float64) string {
	n := int(math.Round(v))
	if n > 0 {
		return "+" + itoa(n)
	}
	return itoa(n)
}
func fmtGamma(v float64) string { return fmt.Sprintf("%.2f", v) }
func fmtPct(v float64) string   { return itoa(int(math.Round(v))) + "%" }

func (a *App) drawEditor(u *UI, body Rect) {
	p := u.P
	pr := a.cur()
	leftW := 430.0
	lx := body.X
	y := body.Y

	// name + hotkey
	p.Text(R(lx, y, 200, 16), "PRESET NAME", FTiny, cTextDim, AlignL)
	p.Text(R(lx+leftW-210, y, 210, 16), "HOTKEY", FTiny, cTextDim, AlignL)
	name := pr.Name
	if u.TextField("name", R(lx, y+20, leftW-226, 42), &name, 24) {
		pr.Name = name
		a.markSave()
	}
	if u.Focus != "name" && strings.TrimSpace(pr.Name) == "" {
		pr.Name = a.uniqueName("PRESET")
	}
	if u.HotkeyBox("hk-preset", R(lx+leftW-210, y+20, 210, 42), &pr.Hotkey, a.failedHK[hkIDPreset+a.cfg.Active]) {
		a.clearDuplicateHotkey(pr.Hotkey, &pr.Hotkey)
		a.hkDirty = true
		a.markSave()
	}
	y += 80

	// warnings
	if !a.applyOK && a.display() != nil {
		r := R(lx, y, body.W, 46)
		p.Fill(r, cRedDk)
		Stroke(p, r, mix(cRedDk, cRed, 0.6))
		p.Fill(R(r.X, r.Y, 4, r.H), cRed)
		msg := "WINDOWS REJECTED THIS CURVE — unlock the extended range (or turn off HDR)."
		if a.unlocked {
			msg = "WINDOWS REJECTED THIS CURVE — restart after unlocking, turn off HDR, or soften values."
		}
		p.Text(R(r.X+16, r.Y, r.W-200, r.H), msg, FBody, cText, AlignL)
		if !a.unlocked && u.Button("unlock1", R(r.Right()-178, r.Y+7, 168, 32), "UNLOCK RANGE", BtnWarn, true) {
			a.plat.RequestGammaUnlock()
		}
		y += 58
	} else if !a.unlocked {
		r := R(lx, y, body.W, 46)
		p.Fill(r, cAmberDk)
		Stroke(p, r, mix(cAmberDk, cAmber, 0.4))
		p.Fill(R(r.X, r.Y, 4, r.H), cAmber)
		p.Text(R(r.X+16, r.Y, r.W-200, r.H), "Windows caps how far gamma can be pushed. Strong night presets need the extended range.", FBody, cText, AlignL)
		if u.Button("unlock1", R(r.Right()-178, r.Y+7, 168, 32), "UNLOCK (ADMIN)", BtnWarn, true) {
			a.plat.RequestGammaUnlock()
		}
		u.Tip(R(r.Right()-178, r.Y+7, 168, 32), "One-time Windows setting (GdiIcmGammaRange). May need a restart.")
		y += 58
	}

	// channel selector
	chs := []struct {
		l string
		c uint32
	}{{"LINKED RGB", cNV}, {"RED", cChR}, {"GREEN", cChG}, {"BLUE", cChB}}
	segW := (leftW - 3*4) / 4
	for i, c := range chs {
		r := R(lx+float64(i)*(segW+4), y, segW, 34)
		hot := u.hit("ch"+itoa(i), r)
		sel := a.editCh == i
		bg, fg, br := uint32(cPanel), uint32(cTextDim), uint32(cLine2)
		if hot {
			bg, fg = cPanel2, cText
			u.Cursor = CurHand
			if u.In.Pressed {
				a.editCh = i
				wasLinked := pr.Linked
				pr.Linked = i == 0
				if pr.Linked && !wasLinked {
					// relinking: all channels follow the green channel
					pr.Ch[0], pr.Ch[2] = pr.Ch[1], pr.Ch[1]
					a.Apply()
				}
				a.markSave()
				u.Dirty = true
			}
		}
		if sel {
			bg, fg, br = mix(cBg, c.c, 0.12), c.c, c.c
		}
		p.Fill(r, bg)
		Stroke(p, r, br)
		p.Text(r, c.l, FBodyB, fg, AlignC)
	}
	y += 52

	ci := 1 // channel shown when linked
	accent := uint32(cNV)
	if a.editCh > 0 {
		ci = a.editCh - 1
		accent = []uint32{cChR, cChG, cChB}[ci]
	}
	ch := pr.Ch[ci]
	changed := false
	sl := func(id, label string, v *float64, o SliderOpt) {
		o.Accent, o.Enabled = accent, true
		if u.Slider(id, R(lx, y, leftW, 64), label, v, o) {
			changed = true
		}
		y += 72
	}
	sl("s-b", "BRIGHTNESS", &ch.Brightness, SliderOpt{Min: BrightMin, Max: BrightMax, Step: 1, Def: 0, Center: 0, Format: fmtSigned})
	sl("s-c", "CONTRAST", &ch.Contrast, SliderOpt{Min: ContrMin, Max: ContrMax, Step: 1, Def: 0, Center: 0, Format: fmtSigned})
	sl("s-g", "GAMMA", &ch.Gamma, SliderOpt{Min: GammaMin, Max: GammaMax, Step: 0.01, Def: 1, Center: 1, Format: fmtGamma})
	if changed {
		if a.editCh == 0 {
			pr.Ch = [3]Channel{ch, ch, ch}
		} else {
			pr.Ch[ci] = ch
		}
		a.Apply()
		a.markSave()
	}

	// vibrance
	y += 4
	p.Fill(R(lx, y, leftW, 1), cLine)
	y += 16
	vr := R(lx, y, leftW, 64)
	if a.vibSlider(u, "s-v", vr, "DIGITAL VIBRANCE", pr) {
		a.Apply()
		a.markSave()
	}
	y += 80
	a.templateButton(u, R(lx, y, 150, 34))
	if u.Button("reset-p", R(lx+158, y, 120, 34), "RESET ALL", BtnGhost, true) {
		pr.CopyLook(&Templates()[0].P)
		a.editCh = 0
		a.Apply()
		a.markSave()
	}
	u.Tip(R(lx+158, y, 120, 34), "Back to neutral colours (all channels, tone and vibrance)")
	if u.Button("apply", R(lx+leftW-144, y, 144, 34), "RE-APPLY", BtnPrimary, a.display() != nil) {
		a.Apply()
	}
	u.Tip(R(lx+leftW-144, y, 144, 34), "Changes apply instantly. Use this if a game or driver reset your colours.")

	// ---------------- right column: curve + preview ----------------
	rx := lx + leftW + 44
	rw := body.Right() - rx
	gy := body.Y + 80
	if !a.applyOK || !a.unlocked {
		gy += 58
	}
	SectionHeader(p, rx, gy, rw, "//", "RESPONSE CURVE")
	gs := math.Min(rw, 290)
	g := R(rx, gy+28, gs, gs)
	p.Fill(g, cPanel)
	Stroke(p, g, cLine2)
	for i := 1; i < 8; i++ {
		t := float64(i) / 8
		c := uint32(cPanel3)
		if i == 4 {
			c = cLine
		}
		p.Fill(R(g.X+t*g.W, g.Y+1, 1, g.H-2), c)
		p.Fill(R(g.X+1, g.Y+t*g.H, g.W-2, 1), c)
	}
	// identity
	for t := 0.0; t < 1; t += 0.04 {
		p.Line(g.X+t*g.W, g.Bottom()-t*g.H, g.X+(t+0.02)*g.W, g.Bottom()-(t+0.02)*g.H, 1, cLine2)
	}
	cols := []uint32{cChR, cChG, cChB}
	order := []int{0, 2, 1}
	rmp := pr.Ramp()
	same := rmp[0] == rmp[1] && rmp[1] == rmp[2]
	for _, c := range order {
		if same && c != 1 {
			continue
		}
		col := cols[c]
		if same {
			col = cNV
		}
		w := 1.6
		if a.editCh == c+1 {
			w = 2.6
		}
		pts := make([]Pt, 0, 129)
		for k := 0; k <= 128; k++ {
			t := float64(k) / 128
			v := pr.Eval(c, t)
			pts = append(pts, Pt{g.X + t*g.W, g.Bottom() - v*(g.H-1)})
		}
		p.Poly(pts, w, col)
	}
	Brackets(p, g, 10, cTan)
	p.Text(R(g.X+8, g.Bottom()-22, 100, 18), "IN", FMonoS, cTextMute, AlignL)
	p.Text(R(g.X+8, g.Y+4, 100, 18), "OUT", FMonoS, cTextMute, AlignL)

	// shadow detail preview
	py := g.Bottom() + 24
	SectionHeader(p, rx, py, rw, "//", "SHADOW DETAIL  ·  BEFORE / AFTER")
	steps := 16
	sw := rw / float64(steps)
	ramp := pr.Ramp()
	for i := 0; i < steps; i++ {
		lvl := float64(i) / float64(steps-1) * 0.55 // focus on the dark half
		idx := int(lvl * 255)
		in := uint32(idx)
		before := in<<16 | in<<8 | in
		ar := uint32(ramp[0][idx] >> 8)
		ag := uint32(ramp[1][idx] >> 8)
		ab := uint32(ramp[2][idx] >> 8)
		after := ar<<16 | ag<<8 | ab
		bx := rx + float64(i)*sw
		p.Fill(R(bx, py+28, sw+0.5, 26), before)
		p.Fill(R(bx, py+56, sw+0.5, 26), after)
	}
	Stroke(p, R(rx, py+28, rw, 54), cLine2)
	p.Text(R(rx, py+86, rw/2, 16), "BLACK", FMonoS, cTextMute, AlignL)
	p.Text(R(rx+rw/2, py+86, rw/2, 16), "MID-GREY", FMonoS, cTextMute, AlignR)
}

func (a *App) clearDuplicateHotkey(h Hotkey, keep *Hotkey) {
	if h.Empty() {
		return
	}
	for i := range a.cfg.Presets {
		if &a.cfg.Presets[i].Hotkey != keep && a.cfg.Presets[i].Hotkey == h {
			a.cfg.Presets[i].Hotkey = Hotkey{}
		}
	}
	for _, g := range []*Hotkey{&a.cfg.HKNext, &a.cfg.HKPrev, &a.cfg.HKToggle} {
		if g != keep && *g == h {
			*g = Hotkey{}
		}
	}
}

func (a *App) drawSettings(u *UI, body Rect) {
	p := u.P
	colW := (body.W - 40) / 2
	lx, rx := body.X, body.X+colW+40
	y := body.Y

	// ---- global hotkeys ----
	SectionHeader(p, lx, y, colW, "03", "GLOBAL HOTKEYS")
	y += 30
	hkRow := func(id, label, help string, h *Hotkey, fid int) {
		p.Text(R(lx, y, colW-230, 20), label, FBodyB, cText, AlignL)
		p.Text(R(lx, y+20, colW-230, 18), help, FSmall, cTextDim, AlignL)
		if u.HotkeyBox(id, R(lx+colW-222, y+2, 222, 36), h, a.failedHK[fid]) {
			a.clearDuplicateHotkey(*h, h)
			a.hkDirty = true
			a.markSave()
		}
		y += 52
	}
	hkRow("hk-next", "NEXT PRESET", "Cycle forward through the list", &a.cfg.HKNext, hkIDNext)
	hkRow("hk-prev", "PREVIOUS PRESET", "Cycle backward", &a.cfg.HKPrev, hkIDPrev)
	hkRow("hk-tog", "TOGGLE DEFAULT", "DEFAULT / last preset", &a.cfg.HKToggle, hkIDToggle)
	p.Text(R(lx, y, colW, 36), "Per-preset hotkeys are set in the Preset Editor.", FSmall, cTextMute, AlignL)
	p.Text(R(lx, y+18, colW, 36), "Hotkeys work in-game (borderless windowed recommended).", FSmall, cTextMute, AlignL)
	y += 56

	// ---- tarkov ----
	SectionHeader(p, lx, y, colW, "04", "ESCAPE FROM TARKOV")
	y += 30
	st, sc := "GAME NOT RUNNING", uint32(cTextDim)
	if a.tarkov {
		st, sc = "GAME RUNNING", cNV
	}
	Badge(p, lx, y, st, sc, AlignL)
	y += 30
	if u.Toggle("t-auto", R(lx, y, colW, 26), "Auto-switch preset when the game starts", &a.cfg.AutoTarkov, true) {
		a.markSave()
	}
	y += 32
	p.Text(R(lx+28, y, 80, 36), "PRESET", FTiny, cTextDim, AlignL)
	dr := R(lx+90, y, colW-90, 36)
	en := a.cfg.AutoTarkov
	p.Fill(dr, cPanel)
	Stroke(p, dr, map[bool]uint32{true: cLine2, false: cLine}[en])
	ap := a.cfg.AutoPreset
	if a.presetByName(ap) < 0 {
		ap = "— CHOOSE —"
	}
	p.Text(R(dr.X+12, dr.Y, dr.W-40, dr.H), ap, FBodyB, map[bool]uint32{true: cText, false: cTextMute}[en], AlignL)
	cx, cy := dr.Right()-18, dr.Y+16
	p.Line(cx-5, cy-2, cx, cy+3, 1.5, cTextDim)
	p.Line(cx, cy+3, cx+5, cy-2, 1.5, cTextDim)
	if en {
		var items []DropItem
		for _, pr := range a.cfg.Presets {
			items = append(items, DropItem{Title: pr.Name})
		}
		if c := u.Dropdown("auto", dr, items, a.presetByName(a.cfg.AutoPreset), 34); c >= 0 {
			a.cfg.AutoPreset = a.cfg.Presets[c].Name
			a.markSave()
		}
	}
	y += 44
	p.Text(R(lx+28, y, colW-28, 18), "Switches back to your previous preset when the game closes.", FSmall, cTextMute, AlignL)

	// ---- behaviour ----
	y = body.Y
	SectionHeader(p, rx, y, colW, "05", "BEHAVIOUR")
	y += 30
	tg := func(id, label string, v *bool) bool {
		c := u.Toggle(id, R(rx, y, colW, 26), label, v, true)
		if c {
			a.markSave()
		}
		y += 34
		return c
	}
	if tg("t-keep", "Keep colours applied (re-apply if reset)", &a.cfg.KeepApplied) && a.cfg.KeepApplied {
		a.Apply()
	}
	tg("t-osd", "On-screen indicator on hotkey switch", &a.cfg.ShowOSD)
	tg("t-reset", "Restore normal colours on exit", &a.cfg.ResetOnExit)
	tg("t-tray", "Close button hides to system tray", &a.cfg.CloseToTray)
	pm := a.cfg.PerMonitor
	if u.Toggle("t-permon", R(rx, y, colW, 26), "Separate presets for each monitor", &pm, true) {
		a.setPerMonitor(pm)
	}
	u.Tip(R(rx, y, colW, 26), "Each monitor keeps its own preset list, active preset and hotkeys")
	y += 34
	if a.cfg.PerMonitor && len(a.displays) > 1 {
		cr := R(rx+28, y-4, 250, 30)
		if u.Button("copy-all", cr, "COPY THIS LIST TO ALL MONITORS", BtnGhost, true) {
			a.copyListToAllMonitors()
		}
		u.Tip(cr, "Overwrite the other monitors' lists with this monitor's presets")
		y += 34
	}
	y += 16

	// ---- gamma range ----
	SectionHeader(p, rx, y, colW, "06", "WINDOWS GAMMA RANGE")
	y += 30
	if a.unlocked {
		Badge(p, rx, y, "EXTENDED RANGE UNLOCKED", cNV, AlignL)
		y += 30
		p.Text(R(rx, y, colW, 18), "Strong night presets are allowed.", FSmall, cTextDim, AlignL)
	} else {
		Badge(p, rx, y, "DEFAULT (LIMITED) RANGE", cAmber, AlignL)
		y += 30
		p.Text(R(rx, y, colW, 18), "Windows rejects strong curves unless this is unlocked (one-time,", FSmall, cTextDim, AlignL)
		p.Text(R(rx, y+18, colW, 18), "needs admin, same tweak RivaTuner users apply). Restart may be needed.", FSmall, cTextDim, AlignL)
		y += 44
		if u.Button("unlock2", R(rx, y, 220, 34), "UNLOCK EXTENDED RANGE", BtnWarn, true) {
			a.plat.RequestGammaUnlock()
		}
	}
}

func (a *App) drawStatus(u *UI) {
	p := u.P
	bar := R(0, u.H-StatusH, u.W, StatusH)
	p.Fill(bar, cPanel)
	p.Fill(R(0, bar.Y, u.W, 1), cLine)
	x := 20.0
	d := a.display()
	s, c := "GAMMA OK", uint32(cNV)
	if d == nil {
		s, c = "NO DISPLAY", cRed
	} else if !a.applyOK {
		s, c = "GAMMA REJECTED", cRed
	}
	p.Fill(R(x, bar.Y+12, 6, 6), c)
	x += 14
	p.Text(R(x, bar.Y, 200, bar.H), s, FMonoS, c, AlignL)
	x += p.Measure(s, FMonoS) + 24
	p.Text(R(x, bar.Y, 400, bar.H), "PRESET: "+a.cur().Name, FMonoS, cTextDim, AlignL)
	x += p.Measure("PRESET: "+a.cur().Name, FMonoS) + 24
	if a.vibAvailable() {
		lo, hi := a.vibRange()
		p.Text(R(x, bar.Y, 300, bar.H), "VIBRANCE: "+fmtPct(clamp(a.cur().Vibrance, lo, hi)), FMonoS, cTextDim, AlignL)
	}
	right := "TARKOVNV v" + Version
	t := "EFT: NOT RUNNING"
	tc := uint32(cTextMute)
	if a.tarkov {
		t, tc = "EFT: RUNNING", cNV
	}
	rw := p.Measure(right, FMonoS)
	p.Text(R(u.W-20-rw, bar.Y, rw, bar.H), right, FMonoS, cTextMute, AlignR)
	tw := p.Measure(t, FMonoS)
	p.Text(R(u.W-20-rw-24-tw, bar.Y, tw, bar.H), t, FMonoS, tc, AlignR)
}

// vibSlider draws the vibrance slider with a badge saying which engine is in use.
func (a *App) vibSlider(u *UI, id string, r Rect, label string, pr *Preset) bool {
	p := u.P
	lo, hi := a.vibRange()
	ok := a.vibAvailable()
	v := clamp(pr.Vibrance, lo, hi)
	f := fmtPct
	if !ok {
		v, f = 50, func(float64) string { return "N/A" }
	}
	ch := u.Slider(id, r, label, &v, SliderOpt{Min: lo, Max: hi, Step: 1, Def: 50, Center: 50, Format: f, Accent: cNV, Enabled: ok})
	if ch {
		pr.Vibrance = v
	}
	// engine badge next to the label
	bx := r.X + p.Measure(label, FTiny) + 10
	var tag string
	var tc uint32
	switch {
	case !ok:
		tag, tc = "NOT AVAILABLE ON THIS PC", cTextMute
	case a.satErr && v != 50:
		tag, tc = "BOOST BLOCKED BY WINDOWS", cRed
	case a.vib.OK && v > 100:
		tag, tc = "NVIDIA MAX + BOOST", cAmber
	case a.vib.OK:
		tag, tc = "NVIDIA DRIVER", cNV
	default:
		tag, tc = "WINDOWS FILTER", cTan
	}
	if r.W > 300 {
		Badge(p, bx, r.Y+1, tag, tc, AlignL)
	}
	tip := "Vibrance is not available on this PC"
	switch {
	case a.vib.OK && a.satOK:
		tip = "0-100% = NVIDIA driver · 101-250% = driver max + TarkovNV boost (Windows colour filter, all monitors)"
	case a.vib.OK:
		tip = "100% is the NVIDIA driver maximum (boost needs Windows 8 or newer)"
	case a.satOK:
		tip = "Saturation via Windows full-screen colour filter · 50% = normal"
	}
	u.Tip(R(r.X, r.Y, r.W-100, 24), tip)
	return ch
}
