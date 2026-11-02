package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// ---------- Colour curve model (RivaTuner-style per-channel controls) ----------

// Channel holds the three classic RivaTuner colour controls for one channel.
type Channel struct {
	Brightness float64 `json:"brightness"` // -250 .. +250
	Contrast   float64 `json:"contrast"`   // -100 .. +250
	Gamma      float64 `json:"gamma"`      // 0.10 .. 5.00 (1.00 = neutral)
}

func NeutralChannel() Channel { return Channel{0, 0, 1.0} }

const (
	BrightMin, BrightMax = -250.0, 250.0
	ContrMin, ContrMax   = -100.0, 250.0
	GammaMin, GammaMax   = 0.10, 5.00
)

// Eval maps an input level 0..1 to an output level 0..1.
func (c Channel) Eval(x float64) float64 {
	g := c.Gamma
	if g < 0.05 {
		g = 0.05
	}
	y := math.Pow(x, 1.0/g)
	k := 1.0 + c.Contrast/100.0
	y = (y-0.5)*k + 0.5
	y += c.Brightness / 100.0 * 0.5
	if y < 0 {
		y = 0
	}
	if y > 1 {
		y = 1
	}
	return y
}

// Ramp is a Windows gamma ramp: 3 x 256 entries (R, G, B), 16-bit.
type Ramp [3][256]uint16

func IdentityRamp() *Ramp {
	var r Ramp
	for ch := 0; ch < 3; ch++ {
		for i := 0; i < 256; i++ {
			r[ch][i] = uint16(i * 257)
		}
	}
	return &r
}

func (r *Ramp) Near(o *Ramp, tol int) bool {
	for ch := 0; ch < 3; ch++ {
		for i := 0; i < 256; i++ {
			d := int(r[ch][i]) - int(o[ch][i])
			if d < -tol || d > tol {
				return false
			}
		}
	}
	return true
}

// ---------- Hotkeys ----------

const (
	ModAlt   = 0x1
	ModCtrl  = 0x2
	ModShift = 0x4
	ModWin   = 0x8
)

type Hotkey struct {
	Mods uint32 `json:"mods"`
	VK   uint32 `json:"vk"`
}

func (h Hotkey) Empty() bool { return h.VK == 0 }

var keyNames = map[uint32]string{
	0x08: "BKSP", 0x09: "TAB", 0x0D: "ENTER", 0x13: "PAUSE", 0x14: "CAPS", 0x20: "SPACE",
	0x21: "PGUP", 0x22: "PGDN", 0x23: "END", 0x24: "HOME", 0x25: "LEFT", 0x26: "UP",
	0x27: "RIGHT", 0x28: "DOWN", 0x2D: "INS", 0x2E: "DEL", 0x6A: "NUM *", 0x6B: "NUM +",
	0x6C: "NUM SEP", 0x6D: "NUM -", 0x6E: "NUM .", 0x6F: "NUM /", 0x90: "NUMLOCK", 0x91: "SCRLK",
	0xBA: ";", 0xBB: "=", 0xBC: ",", 0xBD: "-", 0xBE: ".", 0xBF: "/", 0xC0: "`",
	0xDB: "[", 0xDC: "\\", 0xDD: "]", 0xDE: "'",
}

func KeyName(vk uint32) string {
	switch {
	case vk >= 0x30 && vk <= 0x39, vk >= 0x41 && vk <= 0x5A:
		return string(rune(vk))
	case vk >= 0x60 && vk <= 0x69:
		return "NUM " + string(rune('0'+vk-0x60))
	case vk >= 0x70 && vk <= 0x87:
		return "F" + itoa(int(vk-0x6F))
	}
	if n, ok := keyNames[vk]; ok {
		return n
	}
	return "KEY " + itoa(int(vk))
}

func (h Hotkey) String() string {
	if h.Empty() {
		return "UNBOUND"
	}
	var parts []string
	if h.Mods&ModCtrl != 0 {
		parts = append(parts, "CTRL")
	}
	if h.Mods&ModAlt != 0 {
		parts = append(parts, "ALT")
	}
	if h.Mods&ModShift != 0 {
		parts = append(parts, "SHIFT")
	}
	if h.Mods&ModWin != 0 {
		parts = append(parts, "WIN")
	}
	parts = append(parts, KeyName(h.VK))
	return strings.Join(parts, " + ")
}

// ---------- Presets & config ----------

type Preset struct {
	Name        string     `json:"name"`
	Linked      bool       `json:"linked"`
	Ch          [3]Channel `json:"channels"` // R, G, B
	UseVibrance bool       `json:"useVibrance"`
	Vibrance    float64    `json:"vibrance"` // 0..250 %, 50 = driver default
	Hotkey      Hotkey     `json:"hotkey"`

	// Tone & colour balance (applied to all channels)
	Shadows    float64 `json:"shadows"`    // 0..100  lifts dark tones only, true black stays black
	Highlights float64 `json:"highlights"` // -100..50 negative tames glare / bright skies
	Temp       float64 `json:"temp"`       // -100 cool .. +100 warm
	Tint       float64 `json:"tint"`       // -100 green .. +100 magenta
}

const (
	ShadowMin, ShadowMax = 0.0, 100.0
	HighMin, HighMax     = -100.0, 50.0
	TempMin, TempMax     = -100.0, 100.0
	TintMin, TintMax     = -100.0, 100.0
)

// channel gains from temperature / tint
func (p *Preset) gain(ch int) float64 {
	t, n := p.Temp/100, p.Tint/100
	switch ch {
	case 0:
		return 1 + 0.25*t + 0.10*n
	case 1:
		return 1 - 0.20*n
	default:
		return 1 - 0.25*t + 0.10*n
	}
}

// Eval is the full curve for one channel: tone (shadows/highlights) ->
// channel brightness/contrast/gamma -> colour balance.
func (p *Preset) Eval(ch int, x float64) float64 {
	t := x
	if s := clamp(p.Shadows, ShadowMin, ShadowMax) / 100; s > 0 {
		t += s * 4 * t * math.Pow(1-t, 4) // peaks around 20 % input, stays monotonic
	}
	if h := clamp(p.Highlights, HighMin, HighMax) / 100; h != 0 {
		t += h * 2 * t * t * (1 - t)
	}
	t = clamp(t, 0, 1)
	y := p.Ch[ch].Eval(t) * p.gain(ch)
	return clamp(y, 0, 1)
}

func (p *Preset) Ramp() *Ramp {
	var r Ramp
	for ch := 0; ch < 3; ch++ {
		prev := uint16(0)
		for i := 0; i < 256; i++ {
			v := uint16(math.Round(p.Eval(ch, float64(i)/255.0) * 65535))
			if v < prev { // ramps must never go backwards
				v = prev
			}
			r[ch][i], prev = v, v
		}
	}
	return &r
}

func (p *Preset) IsNeutral() bool {
	for _, c := range p.Ch {
		if c != NeutralChannel() {
			return false
		}
	}
	return p.Shadows == 0 && p.Highlights == 0 && p.Temp == 0 && p.Tint == 0
}

// CopyLook copies every colour value from t, keeping name and hotkey.
func (p *Preset) CopyLook(t *Preset) {
	name, hk := p.Name, p.Hotkey
	*p = *t
	p.Name, p.Hotkey = name, hk
}

// ---------- Templates ----------

type Template struct {
	P    Preset
	Desc string
}

func tpl(name, desc string, b, c, g, vib, sh, hi, temp, tint float64) Template {
	p := mkPreset(name, b, c, g, vib, Hotkey{})
	p.Shadows, p.Highlights, p.Temp, p.Tint = sh, hi, temp, tint
	return Template{p, desc}
}

func Templates() []Template {
	nvg := tpl("NVG GREEN", "Night-vision look: green-weighted, strong shadow lift", 0, 0, 1, 40, 30, -10, 0, -55)
	nvg.P.Linked = false
	nvg.P.Ch = [3]Channel{{0, 0, 1.4}, {12, 10, 2.3}, {0, 0, 1.3}}
	return []Template{
		tpl("NEUTRAL", "Windows default colours", 0, 0, 1, 50, 0, 0, 0, 0),
		tpl("NIGHT RAID", "Dark outdoor maps at night", 8, 5, 1.7, 65, 25, -10, 0, 0),
		tpl("PITCH BLACK", "Moonless nights - maximum shadow detail", 15, 12, 2.2, 70, 45, -20, -5, 0),
		tpl("DARK INTERIORS", "Basements, bunkers, dorms", 4, 8, 1.4, 60, 20, 0, 0, 0),
		tpl("LABS / BRIGHT LIGHTS", "Dark rooms next to harsh lights", 0, 12, 1.3, 60, 15, -30, 0, 0),
		tpl("DAY CLARITY", "Punchier contrast for daytime", 0, 10, 1.1, 70, 5, 0, 0, 0),
		tpl("FOG / OVERCAST", "Cuts through haze", -4, 15, 0.95, 62, 0, -10, 0, 0),
		tpl("SNOW / SUN GLARE", "Tames bright snow and sky", 0, 8, 0.95, 60, 10, -45, -5, 0),
		tpl("WARM NIGHT", "Night with less blue cast", 6, 6, 1.6, 60, 25, -10, 25, 5),
		nvg,
	}
}

func mkPreset(name string, b, c, g, vib float64, hk Hotkey) Preset {
	ch := Channel{b, c, g}
	return Preset{Name: name, Linked: true, Ch: [3]Channel{ch, ch, ch}, UseVibrance: true, Vibrance: vib, Hotkey: hk}
}

const (
	vkNum0 = 0x60
)

func DefaultPresets() []Preset {
	return []Preset{
		mkPreset("DEFAULT", 0, 0, 1.00, 50, Hotkey{ModCtrl, vkNum0}),
		mkPreset("NIGHT RAID", 10, 5, 1.80, 65, Hotkey{ModCtrl, vkNum0 + 1}),
		mkPreset("DARK INTERIORS", 5, 8, 1.45, 60, Hotkey{ModCtrl, vkNum0 + 2}),
		mkPreset("DAY CLARITY", 0, 10, 1.10, 70, Hotkey{ModCtrl, vkNum0 + 3}),
		mkPreset("FOG / OVERCAST", -4, 15, 0.95, 62, Hotkey{ModCtrl, vkNum0 + 4}),
	}
}

// MonitorProfile is one monitor's own preset list.
type MonitorProfile struct {
	Name    string   `json:"name"`
	Presets []Preset `json:"presets"`
	Active  int      `json:"active"`
}

func copyPresets(p []Preset) []Preset { return append([]Preset(nil), p...) }

type Config struct {
	Version     int      `json:"version"`
	Display     string   `json:"display"` // GDI device name, e.g. \\.\DISPLAY1
	Presets     []Preset `json:"presets"`
	Active      int      `json:"active"`
	HKNext      Hotkey   `json:"hotkeyNext"`
	HKPrev      Hotkey   `json:"hotkeyPrev"`
	HKToggle    Hotkey   `json:"hotkeyToggle"`
	KeepApplied bool     `json:"keepApplied"`
	ShowOSD     bool     `json:"showOsd"`
	AutoTarkov  bool     `json:"autoTarkov"`
	AutoPreset  string   `json:"autoPreset"`
	ResetOnExit bool     `json:"resetOnExit"`
	StartMin    bool     `json:"startMinimized"`
	CloseToTray bool     `json:"closeToTray"`

	PerMonitor bool                       `json:"perMonitor"`
	Monitors   map[string]*MonitorProfile `json:"monitors,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		Version:     1,
		Presets:     DefaultPresets(),
		Active:      0,
		HKNext:      Hotkey{ModCtrl, 0x6B}, // Ctrl + Num +
		HKPrev:      Hotkey{ModCtrl, 0x6D}, // Ctrl + Num -
		HKToggle:    Hotkey{ModCtrl, 0x6A}, // Ctrl + Num *
		KeepApplied: true,
		ShowOSD:     true,
		AutoTarkov:  false,
		AutoPreset:  "NIGHT RAID",
		ResetOnExit: true,
		StartMin:    false,
		CloseToTray: true,
		PerMonitor:  true,
	}
}

func LoadConfig(dir string) *Config {
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return DefaultConfig()
	}
	c := DefaultConfig()
	if json.Unmarshal(b, c) != nil || len(c.Presets) == 0 {
		return DefaultConfig()
	}
	for i := range c.Presets {
		for ch := range c.Presets[i].Ch {
			if c.Presets[i].Ch[ch].Gamma <= 0 {
				c.Presets[i].Ch[ch].Gamma = 1
			}
		}
	}
	if c.Active < 0 || c.Active >= len(c.Presets) {
		c.Active = 0
	}
	return c
}

func SaveConfig(dir string, c *Config) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "config.json.tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "config.json"))
}

// ---------- small helpers ----------

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
