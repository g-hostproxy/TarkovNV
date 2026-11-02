package main

import (
	"math"
	"path/filepath"
	"strings"
)

func tagColor(t string) uint32 {
	switch t {
	case "DARK":
		return cChB
	case "BRIGHT":
		return cAmber
	case "YOURS":
		return cTan
	case "CALIB":
		return cNV
	}
	return cTextDim
}

// screenRamp is what the monitor is actually doing to every pixel right now.
func (a *App) screenRamp() *Ramp {
	if a.holdDefault || !a.applyOK || a.applied == nil {
		return IdentityRamp()
	}
	return a.applied
}

// editChannelValues returns the channel currently being edited and its accent.
func (a *App) editChannel() (Channel, int, uint32) {
	pr := a.cur()
	if a.editCh > 0 {
		ci := a.editCh - 1
		return pr.Ch[ci], ci, []uint32{cChR, cChG, cChB}[ci]
	}
	return pr.Ch[1], 1, cNV
}

func (a *App) commitChannel(ch Channel, ci int) {
	pr := a.cur()
	if a.editCh == 0 {
		pr.Ch = [3]Channel{ch, ch, ch}
	} else {
		pr.Ch[ci] = ch
	}
	a.Apply()
	a.markSave()
}

func (a *App) drawScenes(u *UI, body Rect) {
	p := u.P
	pr := a.cur()

	// ---------- scene chips ----------
	x, y := body.X, body.Y
	chipW := 112.0
	for i, s := range a.scenes {
		r := R(x+float64(i)*(chipW+6), y, chipW, 36)
		a.sceneChip(u, "scn"+itoa(i), r, s)
	}
	nx := x + float64(len(a.scenes))*(chipW+6)
	ur := R(nx, y, body.Right()-nx, 36)
	isUser := a.scene != nil && a.scene.Tag == "YOURS"
	hot := u.hit("scnu", ur) || u.Drop == "scnu"
	bg, br := uint32(cPanel), uint32(cLine2)
	if hot {
		bg, br = cPanel2, cTan
	}
	if isUser {
		bg, br = mix(cBg, cTan, 0.12), cTan
	}
	p.Fill(ur, bg)
	Stroke(p, ur, br)
	label := "SCREENSHOTS"
	if isUser {
		label = a.scene.Name
	}
	p.Text(R(ur.X+14, ur.Y, ur.W-44, ur.H), label, FBodyB, map[bool]uint32{true: cTan, false: cText}[isUser], AlignL)
	cx, cy := ur.Right()-18, ur.Y+16
	p.Line(cx-5, cy-2, cx, cy+3, 1.5, cTextDim)
	p.Line(cx, cy+3, cx+5, cy-2, 1.5, cTextDim)
	if u.Drop == "scnu" && !a.scnDropWas {
		a.userScenes = UserScenes(a.plat.SceneDirs(), 12)
	}
	var items []DropItem
	for _, s := range a.userScenes {
		dir := filepath.Base(filepath.Dir(s.Path))
		items = append(items, DropItem{Title: s.Name, Sub: dir})
	}
	nUser := len(items)
	if nUser == 0 {
		items = append(items, DropItem{Title: "NO SCREENSHOTS FOUND", Sub: "Take one in-game (PrtSc key) or add images to the folder below"})
	}
	dirs := a.plat.SceneDirs()
	for _, d := range dirs {
		t := "OPEN CUSTOM SCENES FOLDER (drop any PNG/JPG here)"
		if strings.Contains(strings.ToLower(d), "escape from tarkov") {
			t = "OPEN TARKOV SCREENSHOTS FOLDER"
		}
		items = append(items, DropItem{Title: t, Sub: d})
	}
	u.DropW = 430
	if c := u.Dropdown("scnu", ur, items, -1, 46); c >= 0 {
		switch {
		case c < nUser:
			a.scene = a.userScenes[c]
		case nUser == 0 && c == 0:
		default:
			k := c - nUser
			if nUser == 0 {
				k--
			}
			if k >= 0 && k < len(dirs) {
				a.plat.OpenFolder(dirs[k])
			}
		}
	}
	a.scnDropWas = u.Drop == "scnu"
	u.Tip(ur, "Your own Tarkov screenshots (Documents\\Escape from Tarkov\\Screenshots)")

	// ---------- image ----------
	frame := R(body.X, y+48, body.W, math.Round(body.W*9/16))
	p.Fill(frame, 0)
	s := a.scene
	calib := s != nil && s.Tag == "CALIB"
	ok := calib || (s != nil && s.Load(1600))
	if calib {
		a.drawCalibration(u, frame)
		a.holdButton(u, frame, pr)
	} else if !ok {
		p.Text(frame, "COULD NOT LOAD THIS IMAGE", FHead, cRed, AlignC)
	} else {
		a.sceneView.Update(s, a.screenRamp(), pr.Ramp())
		iw, ih := float64(s.img.W), float64(s.img.H)
		k := math.Min(frame.W/iw, frame.H/ih)
		img := R(frame.X+(frame.W-iw*k)/2, frame.Y+(frame.H-ih*k)/2, iw*k, ih*k)
		if a.holdDefault {
			p.Image(img, s.img) // screen is really at default: draw the raw shot
		} else {
			p.Image(img, a.sceneView.After) // = exactly what the game looks like with this preset
		}

		a.holdButton(u, img, pr)

		l, c := pr.Name, uint32(cNV)
		if a.holdDefault {
			l, c = "DEFAULT COLOURS", cText
		}
		Badge(p, img.X+10, img.Bottom()-30, l, c, AlignL)
	}
	Brackets(p, frame, 12, cTan)

	// ---------- quick sliders ----------
	sy := frame.Bottom() + 20
	ch, ci, accent := a.editChannel()
	gap := 18.0
	sw := (body.W - 4*gap) / 5
	chLabel := ""
	if a.editCh > 0 {
		chLabel = " · " + []string{"R", "G", "B"}[ci]
	}
	changed := false
	if u.Slider("q-b", R(body.X, sy, sw, 64), "BRIGHTNESS"+chLabel, &ch.Brightness, SliderOpt{Min: BrightMin, Max: BrightMax, Step: 1, Def: 0, Format: fmtSigned, Accent: accent, Enabled: true}) {
		changed = true
	}
	if u.Slider("q-c", R(body.X+sw+gap, sy, sw, 64), "CONTRAST"+chLabel, &ch.Contrast, SliderOpt{Min: ContrMin, Max: ContrMax, Step: 1, Def: 0, Format: fmtSigned, Accent: accent, Enabled: true}) {
		changed = true
	}
	if u.Slider("q-g", R(body.X+2*(sw+gap), sy, sw, 64), "GAMMA"+chLabel, &ch.Gamma, SliderOpt{Min: GammaMin, Max: GammaMax, Step: 0.01, Def: 1, Center: 1, Format: fmtGamma, Accent: accent, Enabled: true}) {
		changed = true
	}
	if changed {
		a.commitChannel(ch, ci)
	}
	shr := R(body.X+3*(sw+gap), sy, sw, 64)
	if u.Slider("q-s", shr, "SHADOWS", &pr.Shadows, SliderOpt{Min: ShadowMin, Max: ShadowMax, Step: 1, Def: 0, Format: fmtSigned, Accent: cNV, Enabled: true}) {
		a.Apply()
		a.markSave()
	}
	u.Tip(R(shr.X, shr.Y, shr.W, 24), "Lifts dark areas only - true black stays black")
	vr := R(body.X+4*(sw+gap), sy, sw, 64)
	if a.vibSlider(u, "q-v", vr, "VIBRANCE", pr) {
		a.Apply()
		a.markSave()
	}
}

func (a *App) sceneChip(u *UI, id string, r Rect, s *Scene) {
	p := u.P
	sel := a.scene == s
	hot := u.hit(id, r)
	bg, br := uint32(cPanel), uint32(cLine2)
	if hot {
		bg, br = cPanel2, cTan
		u.Cursor = CurHand
		if u.In.Pressed {
			a.scene = s
			u.Dirty = true
		}
	}
	tc := tagColor(s.Tag)
	if sel {
		bg, br = mix(cBg, tc, 0.14), tc
	}
	p.Fill(r, bg)
	Stroke(p, r, br)
	p.Fill(R(r.X, r.Y, 3, r.H), tc)
	p.Text(R(r.X+12, r.Y, r.W-20, r.H), s.Name, FBodyB, map[bool]uint32{true: cText, false: cTextDim}[sel || hot], AlignL)
	p.Text(R(r.X, r.Y, r.W-10, r.H), strings.ToUpper(s.Tag), FTiny, tc, AlignR)
}

func (a *App) holdButton(u *UI, img Rect, pr *Preset) {
	p := u.P
	hold := R(img.Right()-206, img.Bottom()-42, 196, 32)
	hh := u.hit("hold", hold)
	if hh {
		u.Cursor = CurHand
		u.Tip(hold, "Hold: your whole screen switches to real default colours until you let go")
		if u.In.Pressed && !a.holdDefault {
			u.Active = "hold"
			a.holdDefault = true
			a.Apply()
			u.Dirty = true
		}
	}
	if a.holdDefault && !(u.Active == "hold" && u.In.Down) {
		a.holdDefault = false
		a.Apply()
		u.Dirty = true
	}
	hb, hf := uint32(cBg), uint32(cAmber)
	if hh || a.holdDefault {
		hb = cAmberDk
	}
	p.Fill(hold, hb)
	Stroke(p, hold, map[bool]uint32{true: cAmber, false: mix(cAmberDk, cAmber, 0.5)}[hh || a.holdDefault])
	ht := "HOLD · TRUE DEFAULT"
	if a.holdDefault {
		ht = "SHOWING DEFAULT…"
	}
	p.Text(hold, ht, FTiny, hf, AlignC)
}

// drawCalibration renders a black/white level test pattern. It is drawn with plain
// colours, so the monitor's live gamma ramp shows exactly what the preset does.
func (a *App) drawCalibration(u *UI, f Rect) {
	p := u.P
	p.Fill(f, 0)
	n := 16
	sq, gp := 38.0, 6.0
	tw := float64(n)*(sq+gp) - gp
	x0 := f.X + (f.W-tw)/2
	y := f.Y + 14
	p.Text(R(x0, y, tw, 18), "BLACK LEVEL", FTiny, 0x9A9A9A, AlignL)
	p.Text(R(x0, y, tw, 18), "LEVELS 2 - 32 OF 255", FMonoS, 0x6A6A6A, AlignR)
	y += 22
	for i := 0; i < n; i++ {
		v := uint32(2 * (i + 1))
		p.Fill(R(x0+float64(i)*(sq+gp), y, sq, sq), v<<16|v<<8|v)
		p.Text(R(x0+float64(i)*(sq+gp), y+sq+2, sq, 14), itoa(i+1), FMonoS, 0x5A5A5A, AlignC)
	}
	y += sq + 20
	p.Text(R(x0, y, tw, 16), "Aim: square 1 barely visible, every other square clearly separate. Raise SHADOWS or GAMMA if the first squares merge.", FSmall, 0x8A8A8A, AlignL)
	y += 30
	p.Text(R(x0, y, tw, 16), "FULL RANGE  0 - 255", FTiny, 0x9A9A9A, AlignL)
	y += 20
	steps := 64
	bw := tw / float64(steps)
	for i := 0; i < steps; i++ {
		v := uint32(i * 255 / (steps - 1))
		p.Fill(R(x0+float64(i)*bw, y, bw+0.5, 26), v<<16|v<<8|v)
	}
	y += 46
	wp := R(x0-12, y, tw+24, 112)
	p.Fill(wp, 0xFFFFFF)
	p.Text(R(x0, y+6, tw, 18), "HIGHLIGHT LEVEL", FTiny, 0x555555, AlignL)
	p.Text(R(x0, y+6, tw, 18), "LEVELS 223 - 253 ON WHITE", FMonoS, 0x777777, AlignR)
	sy := y + 28
	for i := 0; i < n; i++ {
		v := uint32(223 + 2*i)
		p.Fill(R(x0+float64(i)*(sq+gp), sy, sq, sq), v<<16|v<<8|v)
		p.Text(R(x0+float64(i)*(sq+gp), sy+sq+2, sq, 14), itoa(i+1), FMonoS, 0x888888, AlignC)
	}
	p.Text(R(x0, sy+sq+18, tw, 16), "Aim: the last squares still visible against white. Lower HIGHLIGHTS if bright areas blow out.", FSmall, 0x666666, AlignL)
}
