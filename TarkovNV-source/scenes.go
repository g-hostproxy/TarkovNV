package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed assets/scenes/bunker_dark1.jpg
var sceneBunker1 []byte

//go:embed assets/scenes/bunker_dark2.jpg
var sceneBunker2 []byte

//go:embed assets/scenes/alley_normal.jpg
var sceneAlley []byte

//go:embed assets/scenes/outdoor_bright.jpg
var sceneOutdoor []byte

// PixBuf is a top-down 32-bit BGRA pixel buffer (GDI DIB layout).
type PixBuf struct {
	W, H int
	Pix  []byte
}

type Scene struct {
	Name  string
	Tag   string // DARK / NORMAL / BRIGHT / YOURS
	Path  string // empty = built-in
	data  []byte
	img   *PixBuf // decoded + downscaled original
	thumb *PixBuf
	err   bool
}

func BuiltinScenes() []*Scene {
	return []*Scene{
		{Name: "BUNKER", Tag: "DARK", data: sceneBunker1},
		{Name: "BUNKER II", Tag: "DARK", data: sceneBunker2},
		{Name: "ALLEY", Tag: "NORMAL", data: sceneAlley},
		{Name: "OUTDOOR", Tag: "BRIGHT", data: sceneOutdoor},
		{Name: "LEVELS", Tag: "CALIB"},
	}
}

// UserScenes lists screenshots (newest first) from the given folders.
func UserScenes(dirs []string, max int) []*Scene {
	type f struct {
		path string
		t    int64
	}
	var files []f
	seen := map[string]bool{}
	for _, d := range dirs {
		ents, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range ents {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if e.IsDir() || (ext != ".png" && ext != ".jpg" && ext != ".jpeg") {
				continue
			}
			p := filepath.Join(d, e.Name())
			if seen[p] {
				continue
			}
			seen[p] = true
			info, err := e.Info()
			if err != nil {
				continue
			}
			files = append(files, f{p, info.ModTime().Unix()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].t > files[j].t })
	if len(files) > max {
		files = files[:max]
	}
	var out []*Scene
	for _, x := range files {
		n := strings.TrimSuffix(filepath.Base(x.path), filepath.Ext(x.path))
		out = append(out, &Scene{Name: strings.ToUpper(n), Tag: "YOURS", Path: x.path})
	}
	return out
}

// Load decodes the scene (lazily) and downsizes it to at most maxW pixels wide.
func (s *Scene) Load(maxW int) bool {
	if s.img != nil {
		return true
	}
	if s.err {
		return false
	}
	data := s.data
	if s.Path != "" {
		b, err := os.ReadFile(s.Path)
		if err != nil {
			s.err = true
			return false
		}
		data = b
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		s.err = true
		return false
	}
	b := src.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)
	full := &PixBuf{W: b.Dx(), H: b.Dy(), Pix: make([]byte, b.Dx()*b.Dy()*4)}
	for i := 0; i < len(full.Pix); i += 4 {
		full.Pix[i], full.Pix[i+1], full.Pix[i+2], full.Pix[i+3] = rgba.Pix[i+2], rgba.Pix[i+1], rgba.Pix[i], 255
	}
	s.img = downscale(full, maxW)
	s.thumb = downscale(s.img, 192)
	return true
}

// downscale with a box filter (keeps aspect ratio).
func downscale(src *PixBuf, maxW int) *PixBuf {
	if src.W <= maxW {
		return src
	}
	w := maxW
	h := src.H * maxW / src.W
	dst := &PixBuf{W: w, H: h, Pix: make([]byte, w*h*4)}
	for y := 0; y < h; y++ {
		y0, y1 := y*src.H/h, (y+1)*src.H/h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < w; x++ {
			x0, x1 := x*src.W/w, (x+1)*src.W/w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sb, sg, sr, n int
			for yy := y0; yy < y1; yy++ {
				o := (yy*src.W + x0) * 4
				for xx := x0; xx < x1; xx++ {
					sb += int(src.Pix[o])
					sg += int(src.Pix[o+1])
					sr += int(src.Pix[o+2])
					o += 4
					n++
				}
			}
			d := (y*w + x) * 4
			dst.Pix[d], dst.Pix[d+1], dst.Pix[d+2], dst.Pix[d+3] = byte(sb/n), byte(sg/n), byte(sr/n), 255
		}
	}
	return dst
}

// lut8 converts a 16-bit ramp into 8-bit lookup tables.
func lut8(r *Ramp) (l [3][256]byte) {
	for c := 0; c < 3; c++ {
		for i := 0; i < 256; i++ {
			l[c][i] = byte(r[c][i] >> 8)
		}
	}
	return
}

// invLut8 builds the inverse of a (monotonic) screen ramp: for every wanted output
// level, the smallest input level that reaches it.
func invLut8(r *Ramp) (l [3][256]byte) {
	for c := 0; c < 3; c++ {
		i := 0
		for t := 0; t < 256; t++ {
			want := uint16(t * 257)
			for i < 255 && r[c][i] < want {
				i++
			}
			// pick the closer of i-1 and i
			if i > 0 && int(want)-int(r[c][i-1]) < int(r[c][i])-int(want) {
				l[c][t] = byte(i - 1)
			} else {
				l[c][t] = byte(i)
			}
		}
	}
	return
}

// SceneView caches the two processed versions of the current scene.
//
// The whole monitor is already passed through the live gamma ramp (screen ramp S).
// To show a target image T on screen we must draw S⁻¹(T):
//   - BEFORE (default colours) = S⁻¹(original)
//   - AFTER  (preset P)        = S⁻¹(P(original))
//
// When the preset is applied (S == P) the AFTER side is just the raw screenshot,
// which is exactly how the game itself looks.
type SceneView struct {
	scene         *Scene
	screen, pre   Ramp
	Before, After *PixBuf
}

func (v *SceneView) Update(s *Scene, screen, preset *Ramp) {
	if s == nil || s.img == nil {
		return
	}
	if v.scene == s && v.screen == *screen && v.pre == *preset && v.Before != nil {
		return
	}
	v.scene, v.screen, v.pre = s, *screen, *preset
	inv := invLut8(screen)
	fwd := lut8(preset)
	src := s.img
	if v.Before == nil || v.Before.W != src.W || v.Before.H != src.H {
		v.Before = &PixBuf{W: src.W, H: src.H, Pix: make([]byte, len(src.Pix))}
		v.After = &PixBuf{W: src.W, H: src.H, Pix: make([]byte, len(src.Pix))}
	}
	bp, ap, sp := v.Before.Pix, v.After.Pix, src.Pix
	for i := 0; i < len(sp); i += 4 {
		b, g, r := sp[i], sp[i+1], sp[i+2]
		bp[i], bp[i+1], bp[i+2], bp[i+3] = inv[2][b], inv[1][g], inv[0][r], 255
		ap[i], ap[i+1], ap[i+2], ap[i+3] = inv[2][fwd[2][b]], inv[1][fwd[1][g]], inv[0][fwd[0][r]], 255
	}
}
