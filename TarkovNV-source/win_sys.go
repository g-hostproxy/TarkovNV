//go:build windows

package main

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// ======================= Displays =======================

type displayDevice struct {
	cb           uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

const (
	ddAttached = 0x1
	ddPrimary  = 0x4
	ddMirror   = 0x8
)

// friendlyMonitorNames maps GDI device names to {friendly name, stable monitor device path}.
func friendlyMonitorNames() map[string][2]string {
	out := map[string][2]string{}
	if pQueryDisplayConfig.Find() != nil {
		return out
	}
	var np, nm uint32
	const qdcOnlyActive = 2
	if r, _, _ := pGetDisplayConfigBufferSizes.Call(qdcOnlyActive, ptr(&np), ptr(&nm)); r != 0 || np == 0 {
		return out
	}
	paths := make([]byte, 72*int(np))
	modes := make([]byte, 64*int(nm))
	if r, _, _ := pQueryDisplayConfig.Call(qdcOnlyActive, ptr(&np), uintptr(unsafe.Pointer(&paths[0])), ptr(&nm), uintptr(unsafe.Pointer(&modes[0])), 0); r != 0 {
		return out
	}
	le := binary.LittleEndian
	for i := 0; i < int(np); i++ {
		pth := paths[i*72 : i*72+72]
		// source name
		src := make([]byte, 84)
		le.PutUint32(src[0:], 1) // DISPLAYCONFIG_DEVICE_INFO_GET_SOURCE_NAME
		le.PutUint32(src[4:], 84)
		copy(src[8:16], pth[0:8])
		copy(src[16:20], pth[8:12])
		if r, _, _ := pDisplayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&src[0]))); r != 0 {
			continue
		}
		gdi := utf16bytes(src[20:84])
		// target name
		tgt := make([]byte, 420)
		le.PutUint32(tgt[0:], 2) // GET_TARGET_NAME
		le.PutUint32(tgt[4:], 420)
		copy(tgt[8:16], pth[20:28])
		copy(tgt[16:20], pth[28:32])
		if r, _, _ := pDisplayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&tgt[0]))); r != 0 {
			continue
		}
		name := utf16bytes(tgt[36 : 36+128])
		path := utf16bytes(tgt[164:420])
		if _, seen := out[gdi]; !seen {
			out[gdi] = [2]string{name, path}
		}
	}
	return out
}

func utf16bytes(b []byte) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return syscall.UTF16ToString(u)
}

func vendorFromID(id string) string {
	id = strings.ToUpper(id)
	switch {
	case strings.Contains(id, "VEN_10DE"):
		return "NVIDIA"
	case strings.Contains(id, "VEN_1002"), strings.Contains(id, "VEN_1022"):
		return "AMD"
	case strings.Contains(id, "VEN_8086"):
		return "INTEL"
	}
	return "OTHER"
}

func (w *winPlat) Displays() []DisplayInfo {
	names := friendlyMonitorNames()
	var list []DisplayInfo
	for i := uint32(0); i < 32; i++ {
		var dd displayDevice
		dd.cb = uint32(unsafe.Sizeof(dd))
		if r, _, _ := pEnumDisplayDevicesW.Call(0, uintptr(i), ptr(&dd), 0); r == 0 {
			break
		}
		if dd.StateFlags&ddAttached == 0 || dd.StateFlags&ddMirror != 0 {
			continue
		}
		d := DisplayInfo{
			Device:  syscall.UTF16ToString(dd.DeviceName[:]),
			Adapter: strings.TrimSpace(syscall.UTF16ToString(dd.DeviceString[:])),
			Vendor:  vendorFromID(syscall.UTF16ToString(dd.DeviceID[:])),
			Primary: dd.StateFlags&ddPrimary != 0,
		}
		d.Monitor = names[d.Device][0]
		d.ID = strings.ToUpper(names[d.Device][1])
		if d.Monitor == "" || d.ID == "" {
			var mon displayDevice
			mon.cb = uint32(unsafe.Sizeof(mon))
			// EDD_GET_DEVICE_INTERFACE_NAME (1): DeviceID becomes the monitor's stable interface path
			if r, _, _ := pEnumDisplayDevicesW.Call(uintptr(unsafe.Pointer(&dd.DeviceName[0])), 0, ptr(&mon), 1); r != 0 {
				if d.Monitor == "" {
					d.Monitor = syscall.UTF16ToString(mon.DeviceString[:])
				}
				if d.ID == "" {
					d.ID = strings.ToUpper(syscall.UTF16ToString(mon.DeviceID[:]))
				}
			}
		}
		if d.ID == "" {
			d.ID = strings.ToUpper(d.Monitor + "|" + d.Device)
		}
		if d.Monitor == "" || strings.EqualFold(d.Monitor, "Generic PnP Monitor") {
			d.Monitor = "Monitor " + strings.TrimPrefix(d.Device, `\\.\DISPLAY`)
		}
		dm := make([]byte, 220)
		binary.LittleEndian.PutUint16(dm[68:], 220)
		if r, _, _ := pEnumDisplaySettingsW.Call(uintptr(unsafe.Pointer(&dd.DeviceName[0])), 0xFFFFFFFF, uintptr(unsafe.Pointer(&dm[0]))); r != 0 {
			d.X = int(int32(binary.LittleEndian.Uint32(dm[76:])))
			d.Y = int(int32(binary.LittleEndian.Uint32(dm[80:])))
			d.W = int(binary.LittleEndian.Uint32(dm[172:]))
			d.H = int(binary.LittleEndian.Uint32(dm[176:]))
			d.Hz = int(binary.LittleEndian.Uint32(dm[184:]))
		}
		list = append(list, d)
	}
	return list
}

// ======================= Gamma ramp =======================

func withDisplayDC(dev string, f func(hdc uintptr) error) error {
	hdc, _, _ := pCreateDCW.Call(0, uintptr(unsafe.Pointer(u16(dev))), 0, 0)
	if hdc == 0 {
		return errors.New("cannot open display")
	}
	defer pDeleteDC.Call(hdc)
	return f(hdc)
}

func (w *winPlat) SetGamma(dev string, r *Ramp) error {
	return withDisplayDC(dev, func(hdc uintptr) error {
		if ok, _, _ := pSetDeviceGammaRamp.Call(hdc, uintptr(unsafe.Pointer(r))); ok == 0 {
			return errors.New("windows rejected the gamma ramp")
		}
		return nil
	})
}

func (w *winPlat) GetGamma(dev string) (*Ramp, error) {
	var r Ramp
	err := withDisplayDC(dev, func(hdc uintptr) error {
		if ok, _, _ := pGetDeviceGammaRamp.Call(hdc, uintptr(unsafe.Pointer(&r))); ok == 0 {
			return errors.New("cannot read gamma ramp")
		}
		return nil
	})
	return &r, err
}

// ======================= NVIDIA Digital Vibrance (NVAPI) =======================

type nvAPI struct {
	ok        bool
	tried     bool
	qi        uintptr
	getHandle uintptr
	getDVC    uintptr
	setDVC    uintptr
	getDVCEx  uintptr
	setDVCEx  uintptr
}

var nv nvAPI

func (n *nvAPI) fn(id uint32) uintptr {
	r, _, _ := syscall.SyscallN(n.qi, uintptr(id))
	return r
}

func (n *nvAPI) init() bool {
	if n.tried {
		return n.ok
	}
	n.tried = true
	defer func() {
		if recover() != nil {
			n.ok = false
		}
	}()
	dll := syscall.NewLazyDLL("nvapi64.dll")
	if dll.Load() != nil {
		return false
	}
	qi := dll.NewProc("nvapi_QueryInterface")
	if qi.Find() != nil {
		return false
	}
	n.qi = qi.Addr()
	initFn := n.fn(0x0150E828)
	if initFn == 0 {
		return false
	}
	if r, _, _ := syscall.SyscallN(initFn); r != 0 {
		return false
	}
	n.getHandle = n.fn(0x35C29134) // NvAPI_GetAssociatedNvidiaDisplayHandle
	n.getDVC = n.fn(0x4085DE45)    // NvAPI_GetDVCInfo
	n.setDVC = n.fn(0x172409B4)    // NvAPI_SetDVCLevel
	n.getDVCEx = n.fn(0x0E45002D)  // NvAPI_GetDVCInfoEx
	n.setDVCEx = n.fn(0x4A82C2B1)  // NvAPI_SetDVCLevelEx
	n.ok = n.getHandle != 0 && (n.getDVC != 0 || n.getDVCEx != 0)
	return n.ok
}

func (n *nvAPI) handle(dev string) (uintptr, bool) {
	name := append([]byte(dev), 0)
	var h uintptr
	r, _, _ := syscall.SyscallN(n.getHandle, uintptr(unsafe.Pointer(&name[0])), ptr(&h))
	return h, r == 0 && h != 0
}

type dvcInfo struct {
	version uint32
	cur     int32
	min     int32
	max     int32
}

type dvcInfoEx struct {
	version uint32
	cur     int32
	min     int32
	max     int32
	def     int32
}

func (w *winPlat) Vibrance(dev string) VibInfo {
	if !nv.init() {
		return VibInfo{Reason: "Requires an NVIDIA GPU"}
	}
	h, ok := nv.handle(dev)
	if !ok {
		return VibInfo{Reason: "Display not on NVIDIA GPU"}
	}
	if nv.getDVCEx != 0 {
		inf := dvcInfoEx{version: uint32(unsafe.Sizeof(dvcInfoEx{})) | 1<<16}
		if r, _, _ := syscall.SyscallN(nv.getDVCEx, h, 0, ptr(&inf)); r == 0 && inf.max > inf.min {
			w.vibEx = true
			def := int(inf.def)
			if def < int(inf.min) || def > int(inf.max) {
				def = int(inf.min)
			}
			return VibInfo{OK: true, Min: int(inf.min), Max: int(inf.max), Def: def, Cur: int(inf.cur)}
		}
	}
	if nv.getDVC != 0 {
		inf := dvcInfo{version: uint32(unsafe.Sizeof(dvcInfo{})) | 1<<16}
		if r, _, _ := syscall.SyscallN(nv.getDVC, h, 0, ptr(&inf)); r == 0 && inf.max > inf.min {
			w.vibEx = false
			return VibInfo{OK: true, Min: int(inf.min), Max: int(inf.max), Def: int(inf.min), Cur: int(inf.cur)}
		}
	}
	return VibInfo{Reason: "Vibrance not supported by driver"}
}

func (w *winPlat) SetVibrance(dev string, level int) error {
	if !nv.init() {
		return errors.New("nvapi unavailable")
	}
	h, ok := nv.handle(dev)
	if !ok {
		return errors.New("no nvidia display")
	}
	var r uintptr
	if w.vibEx && nv.setDVCEx != 0 {
		inf := dvcInfoEx{version: uint32(unsafe.Sizeof(dvcInfoEx{})) | 1<<16, cur: int32(level)}
		r, _, _ = syscall.SyscallN(nv.setDVCEx, h, 0, ptr(&inf))
	} else if nv.setDVC != 0 {
		r, _, _ = syscall.SyscallN(nv.setDVC, h, 0, uintptr(level))
	}
	if r != 0 {
		return errors.New("nvapi set failed")
	}
	return nil
}

// ======================= Hotkeys =======================

func (w *winPlat) RegisterHotkeys(list []HotkeyBinding) map[int]bool {
	for id := range w.hkRegistered {
		pUnregisterHotKey.Call(w.hwnd, uintptr(id))
	}
	w.hkRegistered = map[int]bool{}
	failed := map[int]bool{}
	for _, b := range list {
		r, _, _ := pRegisterHotKey.Call(w.hwnd, uintptr(b.ID), uintptr(b.HK.Mods|MOD_NOREPEAT), uintptr(b.HK.VK))
		if r == 0 {
			failed[b.ID] = true
		} else {
			w.hkRegistered[b.ID] = true
		}
	}
	return failed
}

func (w *winPlat) SuspendHotkeys(on bool) {
	if on {
		for id := range w.hkRegistered {
			pUnregisterHotKey.Call(w.hwnd, uintptr(id))
		}
		w.hkRegistered = map[int]bool{}
	}
}

// ======================= Registry helpers =======================

func regReadDWORD(root syscall.Handle, path, name string, flags uint32) (uint32, bool) {
	var k syscall.Handle
	if syscall.RegOpenKeyEx(root, u16(path), 0, KEY_QUERY_VALUE|flags, &k) != nil {
		return 0, false
	}
	defer syscall.RegCloseKey(k)
	var typ, val uint32
	n := uint32(4)
	if syscall.RegQueryValueEx(k, u16(name), nil, &typ, (*byte)(unsafe.Pointer(&val)), &n) != nil || typ != REG_DWORD {
		return 0, false
	}
	return val, true
}

func regOpenWrite(root uintptr, path string, flags uint32) (uintptr, error) {
	var k uintptr
	r, _, _ := pRegCreateKeyExW.Call(root, uintptr(unsafe.Pointer(u16(path))), 0, 0, 0, uintptr(KEY_SET_VALUE|flags), 0, ptr(&k), 0)
	if r != 0 {
		return 0, syscall.Errno(r)
	}
	return k, nil
}

const icmKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\ICM`

func (w *winPlat) GammaUnlocked() bool {
	v, ok := regReadDWORD(syscall.HKEY_LOCAL_MACHINE, icmKey, "GdiIcmGammaRange", KEY_WOW64_64KEY)
	return ok && v >= 256
}

// unlockGammaRange runs in the elevated helper process (--unlock-gamma).
func unlockGammaRange() int {
	k, err := regOpenWrite(HKEY_LOCAL_MACHINE, icmKey, KEY_WOW64_64KEY)
	if err != nil {
		return 1
	}
	defer syscall.RegCloseKey(syscall.Handle(k))
	v := uint32(256)
	r, _, _ := pRegSetValueExW.Call(k, uintptr(unsafe.Pointer(u16("GdiIcmGammaRange"))), 0, REG_DWORD, ptr(&v), 4)
	if r != 0 {
		return 1
	}
	return 0
}

func (w *winPlat) RequestGammaUnlock() {
	exe, _ := os.Executable()
	pShellExecuteW.Call(w.hwnd, uintptr(unsafe.Pointer(u16("runas"))), uintptr(unsafe.Pointer(u16(exe))),
		uintptr(unsafe.Pointer(u16("--unlock-gamma"))), 0, SW_HIDE)
}

// ======================= Misc =======================

func (w *winPlat) TarkovRunning() bool {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(snap)
	var pe syscall.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	for err = syscall.Process32First(snap, &pe); err == nil; err = syscall.Process32Next(snap, &pe) {
		n := syscall.UTF16ToString(pe.ExeFile[:])
		if strings.EqualFold(n, "EscapeFromTarkov.exe") || strings.EqualFold(n, "EscapeFromTarkov_BE.exe") {
			return true
		}
	}
	return false
}

func (w *winPlat) ConfigDir() string {
	if w.cfgDir != "" {
		return w.cfgDir
	}
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	// portable mode: keep settings next to the exe if a portable marker or config exists there
	if _, err := os.Stat(filepath.Join(exeDir, "portable.txt")); err == nil {
		w.cfgDir = exeDir
		return w.cfgDir
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = exeDir
	}
	w.cfgDir = filepath.Join(base, "TarkovNV")
	return w.cfgDir
}

// ======================= Test scene folders =======================

var (
	pSHGetKnownFolderPath = shell32.NewProc("SHGetKnownFolderPath")
	pCoTaskMemFree        = syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree")
)

func documentsDir() string {
	// FOLDERID_Documents {FDD39AD0-238F-46AF-ADB4-6C85480369C7} - follows OneDrive redirection
	id := GUID{0xFDD39AD0, 0x238F, 0x46AF, [8]byte{0xAD, 0xB4, 0x6C, 0x85, 0x48, 0x03, 0x69, 0xC7}}
	var out *uint16
	if pSHGetKnownFolderPath.Find() == nil {
		if r, _, _ := pSHGetKnownFolderPath.Call(ptr(&id), 0, 0, uintptr(unsafe.Pointer(&out))); r == 0 && out != nil {
			defer pCoTaskMemFree.Call(uintptr(unsafe.Pointer(out)))
			n := 0
			for p := unsafe.Pointer(out); *(*uint16)(p) != 0; p = unsafe.Add(p, 2) {
				n++
			}
			return syscall.UTF16ToString(unsafe.Slice(out, n))
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents")
}

func (w *winPlat) SceneDirs() []string {
	var dirs []string
	tk := filepath.Join(documentsDir(), "Escape from Tarkov", "Screenshots")
	if st, err := os.Stat(tk); err == nil && st.IsDir() {
		dirs = append(dirs, tk)
	}
	dirs = append(dirs, filepath.Join(w.ConfigDir(), "scenes"))
	return dirs
}

func (w *winPlat) OpenFolder(path string) {
	os.MkdirAll(path, 0o755)
	pShellExecuteW.Call(w.hwnd, uintptr(unsafe.Pointer(u16("open"))), uintptr(unsafe.Pointer(u16(path))), 0, 0, SW_SHOWNORMAL)
}

// ======================= Saturation boost (Windows colour filter) =======================
// Uses the Magnification API full-screen colour effect (Windows 8+). It is applied by the
// desktop compositor to every monitor and is removed automatically when TarkovNV exits.

var (
	magDLL                       = syscall.NewLazyDLL("Magnification.dll")
	pMagInitialize               = magDLL.NewProc("MagInitialize")
	pMagUninitialize             = magDLL.NewProc("MagUninitialize")
	pMagSetFullscreenColorEffect = magDLL.NewProc("MagSetFullscreenColorEffect")
)

func (w *winPlat) SaturationSupported() bool {
	return magDLL.Load() == nil && pMagSetFullscreenColorEffect.Find() == nil
}

func saturationMatrix(s float64) (m [5][5]float32) {
	lr, lg, lb := 0.2126, 0.7152, 0.0722
	k := 1 - s
	m[0] = [5]float32{float32(lr*k + s), float32(lr * k), float32(lr * k), 0, 0}
	m[1] = [5]float32{float32(lg * k), float32(lg*k + s), float32(lg * k), 0, 0}
	m[2] = [5]float32{float32(lb * k), float32(lb * k), float32(lb*k + s), 0, 0}
	m[3][3] = 1
	m[4][4] = 1
	return
}

func (w *winPlat) SetSaturation(s float64) error {
	if !w.SaturationSupported() {
		return errors.New("colour filter not supported")
	}
	if w.magInit && s == w.satCur {
		return nil
	}
	if s == 1 {
		if w.magInit {
			m := saturationMatrix(1)
			pMagSetFullscreenColorEffect.Call(ptr(&m))
			pMagUninitialize.Call()
			w.magInit = false
		}
		w.satCur = 1
		return nil
	}
	if !w.magInit {
		if r, _, _ := pMagInitialize.Call(); r == 0 {
			return errors.New("MagInitialize failed")
		}
		w.magInit = true
	}
	m := saturationMatrix(s)
	if r, _, _ := pMagSetFullscreenColorEffect.Call(ptr(&m)); r == 0 {
		return errors.New("colour filter rejected")
	}
	w.satCur = s
	return nil
}

// OpenURL opens a web page in the user's default browser (only https links are allowed).
func (w *winPlat) OpenURL(url string) {
	if len(url) < 8 || url[:8] != "https://" {
		return
	}
	pShellExecuteW.Call(w.hwnd, uintptr(unsafe.Pointer(u16("open"))), uintptr(unsafe.Pointer(u16(url))), 0, 0, SW_SHOWNORMAL)
}
