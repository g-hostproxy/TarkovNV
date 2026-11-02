package main

import (
	"fmt"
	"math"
)

var chNames = []string{"RED", "GREEN", "BLUE"}
var chCols = []uint32{cChR, cChG, cChB}

// Sample colours from typical Tarkov scenes, used by the colour check strip.
var paletteSwatches = []struct {
	c    uint32
	name string
}{
	{0x050505, "Near-black"},
	{0x0E0E0E, "Deep shadow"},
	{0x1A1A1A, "Dark grey"},
	{0x141C24, "Night sky"},
	{0x1C2416, "Dark foliage"},
	{0x26201A, "Shadowed wood"},
	{0x2A2E26, "Gear / uniform"},
	{0x3A2C26, "Skin in shadow"},
	{0x4E4A44, "Concrete"},
	{0x6A7A52, "Grass"},
	{0xB08566, "Skin"},
	{0x8FA9C8, "Daylight sky"},
}

func fmtTemp(v float64) string { return fmtSigned(v) }

func (a *App) drawTuning(u *UI, body Rect) {
	p := u.P
	pr := a.cur()
	x, y := body.X, body.Y

	// ---------- 01 per-channel grid ----------
	SectionHeader(p, x, y, body.W-300, "01", "PER-CHANNEL  ·  RED / GREEN / BLUE")
	if u.Button("link-all", R(body.Right()-290, y-6, 150, 30), "LINK ALL TO GREEN", BtnGhost, !pr.Linked) {
		pr.Ch[0], pr.Ch[2] = pr.Ch[1], pr.Ch[1]
		pr.Linked = true
		a.editCh = 0
		a.Apply()
		a.markSave()
	}
	u.Tip(R(body.Right()-290, y-6, 150, 30), "Copy the green values to red and blue and edit them together again")
	if u.Button("reset-rgb", R(body.Right()-132, y-6, 132, 30), "RESET RGB", BtnGhost, true) {
		for i := range pr.Ch {
			pr.Ch[i] = NeutralChannel()
		}
		pr.Linked = true
		a.editCh = 0
		a.Apply()
		a.markSave()
	}
	y += 36
	gap := 28.0
	cw := (body.W - 2*gap) / 3
	changed := -1
	for c := 0; c < 3; c++ {
		cx := x + float64(c)*(cw+gap)
		head := R(cx, y, cw, 28)
		col := chCols[c]
		p.Fill(head, mix(cBg, col, 0.12))
		p.Fill(R(head.X, head.Y, 4, head.H), col)
		p.Text(R(head.X+14, head.Y, cw, head.H), chNames[c], FBodyB, col, AlignL)
		if pr.Linked {
			p.Text(R(head.X, head.Y, cw-10, head.H), "LINKED", FTiny, cTextMute, AlignR)
		}
		ch := pr.Ch[c]
		sy := y + 40
		o := SliderOpt{Accent: col, Enabled: true}
		ob := o
		ob.Min, ob.Max, ob.Step, ob.Def, ob.Format = BrightMin, BrightMax, 1, 0, fmtSigned
		if u.Slider(fmt.Sprintf("g-b%d", c), R(cx, sy, cw, 64), "BRIGHTNESS", &ch.Brightness, ob) {
			changed = c
		}
		oc := o
		oc.Min, oc.Max, oc.Step, oc.Def, oc.Format = ContrMin, ContrMax, 1, 0, fmtSigned
		if u.Slider(fmt.Sprintf("g-c%d", c), R(cx, sy+70, cw, 64), "CONTRAST", &ch.Contrast, oc) {
			changed = c
		}
		og := o
		og.Min, og.Max, og.Step, og.Def, og.Center, og.Format = GammaMin, GammaMax, 0.01, 1, 1, fmtGamma
		if u.Slider(fmt.Sprintf("g-g%d", c), R(cx, sy+140, cw, 64), "GAMMA", &ch.Gamma, og) {
			changed = c
		}
		if changed == c {
			// editing one column here always means per-channel tuning
			pr.Linked = false
			pr.Ch[c] = ch
			a.editCh = c + 1
		}
	}
	if changed >= 0 {
		a.Apply()
		a.markSave()
	}
	y += 40 + 210 + 8

	// ---------- 02 tone & balance ----------
	SectionHeader(p, x, y, body.W, "02", "TONE & COLOUR BALANCE  ·  ALL CHANNELS")
	y += 30
	tg := 22.0
	tw := (body.W - 3*tg) / 4
	tone := false
	if u.Slider("t-sh", R(x, y, tw, 64), "SHADOWS", &pr.Shadows, SliderOpt{Min: ShadowMin, Max: ShadowMax, Step: 1, Def: 0, Format: fmtSigned, Accent: cNV, Enabled: true}) {
		tone = true
	}
	u.Tip(R(x, y, tw, 24), "Lifts dark areas only - true black stays black, bright areas untouched")
	if u.Slider("t-hi", R(x+tw+tg, y, tw, 64), "HIGHLIGHTS", &pr.Highlights, SliderOpt{Min: HighMin, Max: HighMax, Step: 1, Def: 0, Format: fmtSigned, Accent: cAmber, Enabled: true}) {
		tone = true
	}
	u.Tip(R(x+tw+tg, y, tw, 24), "Negative = tame glare from sun, snow, flashlights and bright windows")
	if u.Slider("t-tp", R(x+2*(tw+tg), y, tw, 64), "TEMPERATURE", &pr.Temp, SliderOpt{Min: TempMin, Max: TempMax, Step: 1, Def: 0, Format: fmtTemp, Accent: cAmber, Enabled: true}) {
		tone = true
	}
	u.Tip(R(x+2*(tw+tg), y, tw, 24), "Negative = cooler / bluer, positive = warmer. Night maps often look better slightly warm.")
	if u.Slider("t-tn", R(x+3*(tw+tg), y, tw, 64), "TINT", &pr.Tint, SliderOpt{Min: TintMin, Max: TintMax, Step: 1, Def: 0, Format: fmtSigned, Accent: 0xC860C8, Enabled: true}) {
		tone = true
	}
	u.Tip(R(x+3*(tw+tg), y, tw, 24), "Negative = greener, positive = more magenta")
	if tone {
		a.Apply()
		a.markSave()
	}
	y += 80

	// ---------- 03 colour check + curves ----------
	curveW := 250.0
	lw := body.W - curveW - 28
	SectionHeader(p, x, y, lw, "03", "COLOUR CHECK  ·  DEFAULT / PRESET")
	y += 28
	sw := lw / float64(len(paletteSwatches))
	inv := invLut8(a.screenRamp())
	fwd := lut8(pr.Ramp())
	for i, s := range paletteSwatches {
		r8, g8, b8 := byte(s.c>>16), byte(s.c>>8), byte(s.c)
		before := uint32(inv[0][r8])<<16 | uint32(inv[1][g8])<<8 | uint32(inv[2][b8])
		after := uint32(inv[0][fwd[0][r8]])<<16 | uint32(inv[1][fwd[1][g8]])<<8 | uint32(inv[2][fwd[2][b8]])
		bx := x + float64(i)*sw
		p.Fill(R(bx, y, sw+0.5, 34), before)
		p.Fill(R(bx, y+36, sw+0.5, 34), after)
		u.Tip(R(bx, y, sw, 70), s.name)
	}
	Stroke(p, R(x, y, lw, 70), cLine2)
	p.Text(R(x, y+74, lw/2, 16), "TOP: DEFAULT", FMonoS, cTextMute, AlignL)
	p.Text(R(x+lw/2, y+74, lw/2, 16), "BOTTOM: "+pr.Name, FMonoS, cTextMute, AlignR)

	// mini RGB curves
	gx := body.Right() - curveW
	g := R(gx, y-28, curveW, body.Bottom()-(y-28))
	g.H = math.Min(g.H, 128)
	p.Fill(g, cPanel)
	Stroke(p, g, cLine2)
	for i := 1; i < 4; i++ {
		t := float64(i) / 4
		p.Fill(R(g.X+t*g.W, g.Y+1, 1, g.H-2), cPanel3)
		p.Fill(R(g.X+1, g.Y+t*g.H, g.W-2, 1), cPanel3)
	}
	for _, c := range []int{0, 2, 1} {
		pts := make([]Pt, 0, 65)
		for k := 0; k <= 64; k++ {
			t := float64(k) / 64
			pts = append(pts, Pt{g.X + t*g.W, g.Bottom() - pr.Eval(c, t)*(g.H-1)})
		}
		w := 1.4
		if a.editCh == c+1 {
			w = 2.4
		}
		p.Poly(pts, w, chCols[c])
	}
	Brackets(p, g, 8, cTan)
	p.Text(R(g.X+6, g.Y+3, 120, 16), "RGB CURVES", FMonoS, cTextMute, AlignL)
}

// templateButton draws a "TEMPLATES" dropdown that loads a look into the active preset.
func (a *App) templateButton(u *UI, r Rect) {
	p := u.P
	hot := u.hit("tpl", r) || u.Drop == "tpl"
	bg, br := uint32(cPanel2), uint32(cLine2)
	if hot {
		bg, br = cPanel3, cTan
	}
	p.Fill(r, bg)
	Stroke(p, r, br)
	p.Text(R(r.X, r.Y, r.W-18, r.H), "TEMPLATES", FBodyB, cText, AlignC)
	cx, cy := r.Right()-16, r.Y+r.H/2-1
	p.Line(cx-4, cy-2, cx, cy+2, 1.5, cTextDim)
	p.Line(cx, cy+2, cx+4, cy-2, 1.5, cTextDim)
	u.Tip(r, "Load a ready-made look into this preset (keeps its name and hotkey). Undo with Ctrl+Z.")
	ts := Templates()
	items := make([]DropItem, len(ts))
	for i, t := range ts {
		items[i] = DropItem{Title: t.P.Name, Sub: t.Desc}
	}
	u.DropW = 360
	if c := u.Dropdown("tpl", r, items, -1, 44); c >= 0 {
		pr := a.cur()
		pr.CopyLook(&ts[c].P)
		if pr.Linked {
			a.editCh = 0
		} else {
			a.editCh = 2
		}
		a.Apply()
		a.markSave()
	}
}
