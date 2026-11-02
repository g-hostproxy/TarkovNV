package main

import (
	"math"
	"strconv"
	"strings"
)

// ======================= Geometry, fonts, theme =======================

type Rect struct{ X, Y, W, H float64 }
type Pt struct{ X, Y float64 }

func R(x, y, w, h float64) Rect                { return Rect{x, y, w, h} }
func (r Rect) Has(x, y float64) bool           { return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H }
func (r Rect) Inset(d float64) Rect            { return Rect{r.X + d, r.Y + d, r.W - 2*d, r.H - 2*d} }
func (r Rect) Right() float64                  { return r.X + r.W }
func (r Rect) Bottom() float64                 { return r.Y + r.H }
func (r Rect) Cut(dx, dy, dw, dh float64) Rect { return Rect{r.X + dx, r.Y + dy, r.W + dw, r.H + dh} }

type FontID int

const (
	FStencilL FontID = iota
	FStencil
	FHead
	FBody
	FBodyB
	FSmall
	FTiny
	FMono
	FMonoS
	FMonoL
	fontCount
)

type FontSpec struct {
	Face   string
	Px     float64
	Weight int
}

var Fonts = [fontCount]FontSpec{
	FStencilL: {"Saira Stencil One", 27, 400},
	FStencil:  {"Saira Stencil One", 17, 400},
	FHead:     {"Barlow Condensed", 19, 600},
	FBody:     {"Barlow Condensed", 16, 500},
	FBodyB:    {"Barlow Condensed", 16, 700},
	FSmall:    {"Barlow Condensed", 14, 500},
	FTiny:     {"Barlow Condensed", 12.5, 600},
	FMono:     {"Share Tech Mono", 15, 400},
	FMonoS:    {"Share Tech Mono", 12.5, 400},
	FMonoL:    {"Share Tech Mono", 19, 400},
}

const (
	AlignL = 0
	AlignC = 1
	AlignR = 2
)

// Theme: Tarkov-style worn military UI with a night-vision accent.
const (
	cBg       = 0x0A0B09
	cPanel    = 0x12130F
	cPanel2   = 0x191A15
	cPanel3   = 0x22231C
	cLine     = 0x2E2F27
	cLine2    = 0x45463A
	cText     = 0xD4CDB4
	cTextDim  = 0x8F8A77
	cTextMute = 0x57554A
	cTan      = 0xA8946A
	cNV       = 0x8BFF45
	cNVDim    = 0x2F5A1A
	cNVDeep   = 0x16260E
	cAmber    = 0xE0A63A
	cAmberDk  = 0x2A200C
	cRed      = 0xD8492F
	cRedDk    = 0x2C110B
	cChR      = 0xE4593F
	cChG      = 0x7FDB55
	cChB      = 0x4F92E6
)

func mix(a, b uint32, t float64) uint32 {
	ar, ag, ab := float64(a>>16&255), float64(a>>8&255), float64(a&255)
	br, bg, bb := float64(b>>16&255), float64(b>>8&255), float64(b&255)
	r := uint32(ar + (br-ar)*t)
	g := uint32(ag + (bg-ag)*t)
	bl := uint32(ab + (bb-ab)*t)
	return r<<16 | g<<8 | bl
}

// Painter is implemented by the Windows GDI backend and the PNG preview backend.
// All coordinates are logical (96-DPI) pixels; backends scale.
type Painter interface {
	Fill(r Rect, c uint32)
	Line(x1, y1, x2, y2, w float64, c uint32)
	Poly(pts []Pt, w float64, c uint32)
	Text(r Rect, s string, f FontID, c uint32, align int)
	Measure(s string, f FontID) float64
	Clip(r Rect)
	Unclip()
	Image(r Rect, b *PixBuf)
}

func Stroke(p Painter, r Rect, c uint32) {
	p.Fill(R(r.X, r.Y, r.W, 1), c)
	p.Fill(R(r.X, r.Bottom()-1, r.W, 1), c)
	p.Fill(R(r.X, r.Y, 1, r.H), c)
	p.Fill(R(r.Right()-1, r.Y, 1, r.H), c)
}

// Corner brackets – the thin tactical frame look.
func Brackets(p Painter, r Rect, l float64, c uint32) {
	p.Fill(R(r.X, r.Y, l, 2), c)
	p.Fill(R(r.X, r.Y, 2, l), c)
	p.Fill(R(r.Right()-l, r.Y, l, 2), c)
	p.Fill(R(r.Right()-2, r.Y, 2, l), c)
	p.Fill(R(r.X, r.Bottom()-2, l, 2), c)
	p.Fill(R(r.X, r.Bottom()-l, 2, l), c)
	p.Fill(R(r.Right()-l, r.Bottom()-2, l, 2), c)
	p.Fill(R(r.Right()-2, r.Bottom()-l, 2, l), c)
}

// Diagonal hazard-style hatching inside r.
func Hatch(p Painter, r Rect, gap float64, c uint32) {
	p.Clip(r)
	for x := r.X - r.H; x < r.Right(); x += gap {
		p.Line(x, r.Bottom(), x+r.H, r.Y, 1, c)
	}
	p.Unclip()
}

// ======================= Immediate-mode UI =======================

type KeyEvent struct {
	VK   uint32
	Mods uint32
}

type Input struct {
	MX, MY   float64
	Down     bool
	Pressed  bool
	Released bool
	Dbl      bool
	Wheel    float64
	Shift    bool
	Chars    []rune
	Keys     []KeyEvent
}

const (
	CurArrow = 0
	CurHand  = 1
	CurBeam  = 2
	CurSize  = 3
)

type UI struct {
	P      Painter
	In     Input
	W, H   float64
	Active string  // widget being dragged / pressed
	Focus  string  // text field with keyboard focus
	Cap    string  // hotkey box currently capturing
	Drop   string  // open dropdown id
	DropW  float64 // optional wider list for the next Dropdown call
	Cursor int
	Dirty  bool
	Time   float64 // seconds, for caret blink

	dropRect   Rect
	dropHeader Rect
	blocked    bool
	overlays   []func()
	tip        string
	tipAt      Pt
	dragStartX float64
	dragStartV float64
	numID      string // slider whose value is being typed
	numBuf     string
	numFresh   bool // first keystroke replaces the pre-filled value
}

// BeginFrame must be called before drawing widgets.
func (u *UI) BeginFrame() {
	u.Cursor = CurArrow
	u.overlays = u.overlays[:0]
	u.tip = ""
	u.blocked = false
	if u.Drop != "" {
		inDrop := u.dropRect.Has(u.In.MX, u.In.MY) || u.dropHeader.Has(u.In.MX, u.In.MY)
		if inDrop {
			u.blocked = true
		} else if u.In.Pressed {
			u.Drop = ""
			u.blocked = true
			u.In.Pressed = false
			u.Dirty = true
		}
	}
	if u.In.Pressed && u.Focus != "" {
		// clicking anywhere commits the text field; the field itself re-grabs focus
		u.Focus = ""
		u.Dirty = true
	}
	if u.In.Pressed && u.Cap != "" {
		u.Cap = ""
		u.Dirty = true
	}
}

func (u *UI) EndFrame() {
	for _, f := range u.overlays {
		f()
	}
	if u.tip != "" {
		p := u.P
		w := p.Measure(u.tip, FSmall) + 20
		x := math.Min(u.tipAt.X, u.W-w-8)
		y := u.tipAt.Y + 22
		if y+28 > u.H {
			y = u.tipAt.Y - 34
		}
		r := R(x, y, w, 26)
		p.Fill(r, cPanel3)
		Stroke(p, r, cLine2)
		p.Text(r, u.tip, FSmall, cText, AlignC)
	}
	if !u.In.Down {
		u.Active = ""
	}
}

// hit reports whether the mouse is over r and the widget may react.
func (u *UI) hit(id string, r Rect) bool {
	if u.blocked {
		return false
	}
	if u.Active != "" && u.Active != id {
		return false
	}
	return r.Has(u.In.MX, u.In.MY)
}

func (u *UI) Tip(r Rect, s string) {
	if !u.blocked && u.Active == "" && r.Has(u.In.MX, u.In.MY) {
		u.tip = s
		u.tipAt = Pt{u.In.MX, u.In.MY}
	}
}

// ---------- Button ----------

const (
	BtnNormal = iota
	BtnPrimary
	BtnGhost
	BtnDanger
	BtnWarn
)

func (u *UI) Button(id string, r Rect, label string, style int, enabled bool) bool {
	p := u.P
	hot := enabled && u.hit(id, r)
	if hot {
		u.Cursor = CurHand
	}
	clicked := false
	if hot && u.In.Pressed {
		u.Active = id
		u.Dirty = true
	}
	if hot && u.In.Released && u.Active == id {
		clicked = true
		u.Dirty = true
	}
	down := u.Active == id && u.In.Down && hot

	bg, border, fg := uint32(cPanel2), uint32(cLine2), uint32(cText)
	switch style {
	case BtnPrimary:
		bg, border, fg = cNVDeep, cNVDim, cNV
		if hot {
			bg, border = mix(cNVDeep, cNVDim, 0.5), cNV
		}
	case BtnGhost:
		bg, border, fg = cPanel, cLine, cTextDim
		if hot {
			bg, border, fg = cPanel2, cLine2, cText
		}
	case BtnDanger:
		bg, border, fg = cPanel2, cLine2, cTextDim
		if hot {
			bg, border, fg = cRedDk, cRed, cRed
		}
	case BtnWarn:
		bg, border, fg = cAmberDk, mix(cAmberDk, cAmber, 0.5), cAmber
		if hot {
			border = cAmber
		}
	default:
		if hot {
			bg, border = cPanel3, cTan
		}
	}
	if down {
		bg = mix(bg, 0, 0.35)
	}
	if !enabled {
		bg, border, fg = cPanel, cLine, cTextMute
	}
	p.Fill(r, bg)
	Stroke(p, r, border)
	p.Text(r, label, FBodyB, fg, AlignC)
	return clicked
}

// ---------- Toggle (checkbox) ----------

func (u *UI) Toggle(id string, r Rect, label string, v *bool, enabled bool) bool {
	p := u.P
	hot := enabled && u.hit(id, r)
	if hot {
		u.Cursor = CurHand
	}
	changed := false
	if hot && u.In.Pressed {
		*v = !*v
		changed = true
		u.Dirty = true
	}
	box := R(r.X, r.Y+(r.H-18)/2, 18, 18)
	border := uint32(cLine2)
	if hot {
		border = cTan
	}
	if !enabled {
		border = cLine
	}
	p.Fill(box, cPanel)
	Stroke(p, box, border)
	if *v {
		c := uint32(cNV)
		if !enabled {
			c = cTextMute
		}
		p.Fill(box.Inset(4), c)
	}
	fg := uint32(cText)
	if !enabled {
		fg = cTextMute
	}
	p.Text(R(r.X+28, r.Y, r.W-28, r.H), label, FBody, fg, AlignL)
	return changed
}

// ---------- Slider ----------

type SliderOpt struct {
	Min, Max, Step, Def float64
	Format              func(float64) string
	Accent              uint32
	Enabled             bool
	Center              float64 // value the fill grows from (e.g. 0 or 1.0)
}

// Slider draws a labelled slider block. r is the full block (label row + track).
func (u *UI) Slider(id string, r Rect, label string, v *float64, o SliderOpt) bool {
	p := u.P
	track := R(r.X, r.Y+30, r.W, 18)
	hot := o.Enabled && u.hit(id, track.Cut(-6, -6, 12, 12))
	changed := false
	old := *v
	accent := o.Accent
	if accent == 0 {
		accent = cNV
	}

	// label + value
	lc, vc := uint32(cTextDim), uint32(cText)
	if !o.Enabled {
		lc, vc, accent = cTextMute, cTextMute, cTextMute
	}
	p.Text(R(r.X, r.Y, r.W, 22), label, FTiny, lc, AlignL)
	valR := R(r.Right()-90, r.Y-2, 90, 24)
	numKey := id + "#num"
	commit := func() {
		if f, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(u.numBuf, "%")), 64); err == nil {
			*v = f
		}
		u.numID = ""
	}
	if u.Focus == numKey {
		for _, ch := range u.In.Chars {
			if (ch >= '0' && ch <= '9') || ch == '.' || ch == '-' || ch == '+' || ch == ',' {
				if ch == ',' {
					ch = '.'
				}
				if u.numFresh {
					u.numBuf, u.numFresh = "", false
				}
				if len(u.numBuf) < 8 {
					u.numBuf += string(ch)
				}
			}
		}
		for _, k := range u.In.Keys {
			switch k.VK {
			case 0x08:
				u.numFresh = false
				if len(u.numBuf) > 0 {
					u.numBuf = u.numBuf[:len(u.numBuf)-1]
				}
			case 0x0D:
				commit()
				u.Focus = ""
			case 0x1B:
				u.numID = ""
				u.Focus = ""
			}
		}
		if len(u.In.Chars)+len(u.In.Keys) > 0 {
			u.Dirty = true
		}
	}
	if u.numID == id && u.Focus != numKey {
		commit() // clicked elsewhere: keep what was typed
	}
	if o.Enabled && u.hit(numKey, valR) && u.Focus != numKey {
		u.Cursor = CurBeam
		u.Tip(valR, "Click to type an exact value · Enter to set")
		if u.In.Pressed {
			u.Focus = numKey
			u.numID = id
			u.numBuf = strings.TrimPrefix(strings.TrimSuffix(o.Format(*v), "%"), "+")
			u.numFresh = true
			u.Dirty = true
			u.In.Pressed = false
		}
	}
	if u.Focus == numKey {
		box := R(valR.X+6, valR.Y, valR.W-6, valR.H+2)
		p.Fill(box, cNVDeep)
		Stroke(p, box, cNV)
		if u.numFresh {
			tw := p.Measure(u.numBuf, FMonoL)
			p.Fill(R(box.Right()-8-tw-2, box.Y+4, tw+4, box.H-8), cNVDim)
		}
		p.Text(R(box.X+6, box.Y, box.W-12, box.H), u.numBuf, FMonoL, cNV, AlignR)
		if int(u.Time*2)%2 == 0 {
			p.Fill(R(box.Right()-4, box.Y+4, 1.5, box.H-8), cNV)
		}
	} else {
		p.Text(valR, o.Format(*v), FMonoL, vc, AlignR)
		// subtle underline = "editable"
		if o.Enabled {
			tw := p.Measure(o.Format(*v), FMonoL)
			p.Fill(R(valR.Right()-tw, valR.Bottom()-1, tw, 1), cLine2)
		}
	}

	if hot {
		u.Cursor = CurHand
		u.Tip(track, "Drag · Shift = fine · Wheel = step · Double-click = reset")
	}
	if hot && u.In.Dbl {
		*v = o.Def
	} else if hot && u.In.Pressed {
		u.Active = id
		u.dragStartX = u.In.MX
		u.dragStartV = *v
		if !u.In.Shift {
			t := clamp((u.In.MX-track.X)/track.W, 0, 1)
			*v = o.Min + t*(o.Max-o.Min)
		}
	} else if u.Active == id && u.In.Down {
		u.Cursor = CurHand
		if u.In.Shift {
			*v = u.dragStartV + (u.In.MX-u.dragStartX)/track.W*(o.Max-o.Min)*0.2
		} else {
			t := clamp((u.In.MX-track.X)/track.W, 0, 1)
			*v = o.Min + t*(o.Max-o.Min)
		}
	} else if hot && u.In.Wheel != 0 {
		*v += o.Step * u.In.Wheel
	}
	if o.Step > 0 {
		*v = math.Round(*v/o.Step) * o.Step
	}
	*v = clamp(*v, o.Min, o.Max)
	if *v != old {
		changed = true
		u.Dirty = true
	}

	// track
	mid := track.Y + track.H/2
	bar := R(track.X, mid-3, track.W, 6)
	p.Fill(bar, cPanel)
	Stroke(p, bar, cLine)
	// ticks
	for i := 0; i <= 20; i++ {
		x := track.X + track.W*float64(i)/20
		h := 3.0
		if i%5 == 0 {
			h = 6
		}
		p.Fill(R(math.Round(x), track.Bottom()+2, 1, h), cLine2)
	}
	tx := func(val float64) float64 { return track.X + (val-o.Min)/(o.Max-o.Min)*track.W }
	cx, vx := tx(o.Center), tx(*v)
	a, b := math.Min(cx, vx), math.Max(cx, vx)
	fill := accent
	p.Fill(R(a, mid-2, b-a, 4), mix(fill, cBg, 0.25))
	// center marker
	p.Fill(R(math.Round(cx), mid-7, 1, 14), cTextMute)
	// thumb
	th := R(vx-5, track.Y-1, 10, track.H+2)
	tb := uint32(cPanel3)
	if hot || u.Active == id {
		tb = mix(cPanel3, accent, 0.25)
	}
	p.Fill(th, tb)
	Stroke(p, th, accent)
	p.Fill(R(vx-0.5, track.Y+4, 1, track.H-6), accent)
	return changed
}

// ---------- Text field ----------

func (u *UI) TextField(id string, r Rect, s *string, max int) bool {
	p := u.P
	hot := u.hit(id, r)
	if hot {
		u.Cursor = CurBeam
		if u.In.Pressed {
			u.Focus = id
			u.Dirty = true
		}
	}
	changed := false
	focused := u.Focus == id
	if focused {
		for _, ch := range u.In.Chars {
			if ch >= 32 && ch != 127 && len([]rune(*s)) < max {
				*s += strings.ToUpper(string(ch))
				changed = true
			}
		}
		for _, k := range u.In.Keys {
			switch k.VK {
			case 0x08: // backspace
				rs := []rune(*s)
				if len(rs) > 0 {
					*s = string(rs[:len(rs)-1])
					changed = true
				}
			case 0x0D, 0x1B: // enter / esc
				u.Focus = ""
				u.Dirty = true
			}
		}
	}
	border := uint32(cLine2)
	if hot {
		border = cTan
	}
	if focused {
		border = cNV
	}
	p.Fill(r, cPanel)
	Stroke(p, r, border)
	tr := R(r.X+12, r.Y, r.W-24, r.H)
	p.Text(tr, *s, FHead, cText, AlignL)
	if focused && int(u.Time*2)%2 == 0 {
		w := p.Measure(*s, FHead)
		p.Fill(R(tr.X+w+2, r.Y+10, 2, r.H-20), cNV)
	}
	if changed {
		u.Dirty = true
	}
	return changed
}

// ---------- Hotkey capture box ----------

func isModifierVK(vk uint32) bool {
	switch vk {
	case 0x10, 0x11, 0x12, 0x5B, 0x5C, 0xA0, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5:
		return true
	}
	return false
}

func (u *UI) HotkeyBox(id string, r Rect, hk *Hotkey, failed bool) bool {
	p := u.P
	hot := u.hit(id, r)
	changed := false
	if hot {
		u.Cursor = CurHand
		u.Tip(r, "Click, then press a key combo · Esc cancels · Backspace clears")
		if u.In.Pressed {
			u.Cap = id
			u.Dirty = true
		}
	}
	capturing := u.Cap == id
	if capturing {
		for _, k := range u.In.Keys {
			if isModifierVK(k.VK) {
				continue
			}
			switch {
			case k.VK == 0x1B && k.Mods == 0:
				u.Cap = ""
			case (k.VK == 0x08 || k.VK == 0x2E) && k.Mods == 0:
				*hk = Hotkey{}
				changed = true
				u.Cap = ""
			default:
				*hk = Hotkey{Mods: k.Mods, VK: k.VK}
				changed = true
				u.Cap = ""
			}
			u.Dirty = true
			break
		}
	}
	bg, border, fg := uint32(cPanel), uint32(cLine2), uint32(cText)
	label := hk.String()
	if hk.Empty() {
		fg = cTextMute
	}
	if hot {
		border = cTan
	}
	inUse := failed && !hk.Empty() && u.Cap != id
	if inUse {
		border, fg = cRed, cRed
	}
	if u.Cap == id {
		bg, border, fg = cNVDeep, cNV, cNV
		label = "PRESS KEYS…"
	}
	p.Fill(r, bg)
	Stroke(p, r, border)
	p.Text(R(r.X+12, r.Y, r.W-24, r.H), label, FMono, fg, AlignL)
	if inUse {
		Badge(p, r.Right()-8, r.Y+(r.H-18)/2, "IN USE", cRed, AlignR)
		u.Tip(r, "Another program already uses this combo - pick a different one")
	}
	return changed
}

// ---------- Dropdown ----------

type DropItem struct {
	Title, Sub string
	Badge      string
	BadgeColor uint32
}

// Dropdown: the caller draws the header; this handles open/close and the list overlay.
// Returns the chosen index or -1.
func (u *UI) Dropdown(id string, header Rect, items []DropItem, sel int, rowH float64) int {
	hot := u.hit(id, header)
	if u.Drop == id && header.Has(u.In.MX, u.In.MY) {
		hot = true
	}
	if hot {
		u.Cursor = CurHand
		if u.In.Pressed {
			if u.Drop == id {
				u.Drop = ""
			} else {
				u.Drop = id
			}
			u.Dirty = true
			u.In.Pressed = false
		}
	}
	if u.Drop != id {
		return -1
	}
	u.dropHeader = header
	lw := header.W
	if u.DropW > lw {
		lw = u.DropW
	}
	u.DropW = 0
	lx := header.X
	if lx+lw > u.W-8 {
		lx = header.Right() - lw // open towards the left
	}
	list := R(lx, header.Bottom()+2, lw, rowH*float64(len(items))+8)
	if list.Bottom() > u.H-8 {
		list.Y = header.Y - list.H - 2
	}
	u.dropRect = list
	choice := -1
	mx, my := u.In.MX, u.In.MY
	pressed := u.In.Pressed
	for i := range items {
		row := R(list.X+4, list.Y+4+float64(i)*rowH, list.W-8, rowH)
		if row.Has(mx, my) {
			u.Cursor = CurHand
			if pressed {
				choice = i
			}
		}
	}
	if choice >= 0 {
		u.Drop = ""
		u.Dirty = true
		u.In.Pressed = false
	}
	u.overlays = append(u.overlays, func() {
		p := u.P
		p.Fill(list.Cut(4, 4, 0, 0), 0x000000)
		p.Fill(list, cPanel2)
		Stroke(p, list, cTan)
		for i, it := range items {
			row := R(list.X+4, list.Y+4+float64(i)*rowH, list.W-8, rowH)
			if row.Has(mx, my) {
				p.Fill(row, cPanel3)
			}
			if i == sel {
				p.Fill(R(row.X, row.Y+4, 3, row.H-8), cNV)
			}
			tw := row.W - 28
			if it.Badge != "" {
				tw = row.W - 100
			}
			if it.Sub != "" {
				p.Text(R(row.X+14, row.Y+4, tw, row.H/2-2), it.Title, FBodyB, cText, AlignL)
				p.Text(R(row.X+14, row.Y+row.H/2, tw, row.H/2-4), it.Sub, FSmall, cTextDim, AlignL)
			} else {
				p.Text(R(row.X+14, row.Y, tw, row.H), it.Title, FBodyB, cText, AlignL)
			}
			if it.Badge != "" {
				Badge(p, row.Right()-8, row.Y+(row.H-18)/2, it.Badge, it.BadgeColor, AlignR)
			}
		}
	})
	return choice
}

// Badge draws a small tag. x is the left edge (AlignL) or right edge (AlignR). Returns its width.
func Badge(p Painter, x, y float64, s string, c uint32, align int) float64 {
	w := p.Measure(s, FTiny) + 12
	if align == AlignR {
		x -= w
	}
	r := R(x, y, w, 18)
	p.Fill(r, mix(cBg, c, 0.14))
	Stroke(p, r, mix(cBg, c, 0.6))
	p.Text(r, s, FTiny, c, AlignC)
	return w
}

// SectionHeader draws "01 // TITLE" with a rule.
func SectionHeader(p Painter, x, y, w float64, num, title string) {
	nw := p.Measure(num, FMonoS)
	p.Text(R(x, y, nw, 18), num, FMonoS, cNV, AlignL)
	tw := p.Measure(title, FTiny)
	p.Text(R(x+nw+8, y, tw+2, 18), title, FTiny, cTan, AlignL)
	lx := x + nw + 8 + tw + 10
	p.Fill(R(lx, y+9, x+w-lx, 1), cLine)
}
