package main

// SupportURL is the donation page opened by the title-bar button.
// Leave it empty to hide the button.
var SupportURL = "https://github.com/g-hostproxy/TarkovNV"

const SupportW = 132.0 // title-bar support button width (kept out of the drag area)

func supportShown() bool { return SupportURL != "" }

func (a *App) drawSupport(u *UI) {
	if !supportShown() {
		return
	}
	p := u.P
	sr := R(u.W-96-SupportW-12, 9, SupportW, 30)
	sh := u.hit("support", sr)
	sbg, sbr, sfg := uint32(cAmberDk), mix(cAmberDk, cAmber, 0.45), uint32(cAmber)
	if sh {
		sbg, sbr = mix(cAmberDk, cAmber, 0.25), cAmber
		u.Cursor = CurHand
		u.Tip(sr, "Enjoying TarkovNV? Opens the TarkovNV GitHub page - use Sponsor / Buy Me a Coffee there")
		if u.In.Released {
			a.plat.OpenURL(SupportURL)
		}
	}
	p.Fill(sr, sbg)
	Stroke(p, sr, sbr)
	cx0, cy0 := sr.X+14, sr.Y+10
	p.Fill(R(cx0, cy0, 11, 10), sfg)
	p.Line(cx0+11, cy0+2, cx0+15, cy0+2, 1.5, sfg)
	p.Line(cx0+15, cy0+2, cx0+15, cy0+6, 1.5, sfg)
	p.Line(cx0+15, cy0+6, cx0+11, cy0+6, 1.5, sfg)
	p.Fill(R(cx0-2, cy0+11, 15, 1.5), sfg)
	p.Line(cx0+3, cy0-6, cx0+4, cy0-2, 1.2, sfg)
	p.Line(cx0+7, cy0-6, cx0+8, cy0-2, 1.2, sfg)
	p.Text(R(sr.X+34, sr.Y, sr.W-40, sr.H), "SUPPORT", FTiny, sfg, AlignL)
}
