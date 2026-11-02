package main

// DisplayInfo describes one active monitor.
type DisplayInfo struct {
	Device  string // GDI device name, e.g. \\.\DISPLAY1
	ID      string // stable per-monitor key (survives re-plugging / DISPLAY renumbering)
	Monitor string // friendly monitor name
	Adapter string // GPU name
	Vendor  string // NVIDIA / AMD / INTEL / OTHER
	Primary bool
	W, H    int
	Hz      int
	X, Y    int
}

func (d DisplayInfo) ShortDev() string {
	s := d.Device
	if len(s) > 4 && s[:4] == `\\.\` {
		s = s[4:]
	}
	return s
}

// VibInfo reports NVIDIA Digital Vibrance support for a display.
type VibInfo struct {
	OK                 bool
	Min, Max, Def, Cur int
	Reason             string
}

type HotkeyBinding struct {
	ID int
	HK Hotkey
}

// Platform abstracts everything OS-specific.
type Platform interface {
	Displays() []DisplayInfo
	SetGamma(dev string, r *Ramp) error
	GetGamma(dev string) (*Ramp, error)
	Vibrance(dev string) VibInfo
	SetVibrance(dev string, level int) error
	// RegisterHotkeys replaces all registered hotkeys and returns the IDs that failed.
	RegisterHotkeys(list []HotkeyBinding) map[int]bool
	SuspendHotkeys(on bool)
	GammaUnlocked() bool
	RequestGammaUnlock()
	TarkovRunning() bool
	ShowOSD(title, sub string, dev string)
	ConfigDir() string
	// Full-screen saturation boost (Windows colour filter); 1 = off.
	SaturationSupported() bool
	SetSaturation(s float64) error
	SceneDirs() []string // folders scanned for the user's own screenshots
	OpenFolder(path string)
	OpenURL(url string)
	Minimize()
	HideToTray()
	Quit()
}
