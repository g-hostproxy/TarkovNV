package main

import (
	"bytes"
	"encoding/json"
)

// Undo/redo works on whole-config snapshots. A snapshot is recorded once a user
// gesture has finished (mouse released, text field / hotkey capture closed), so a
// slider drag or a typed value is a single undo step.

const undoMax = 100

type undoState struct {
	past   [][]byte // snapshots before each change (newest last)
	future [][]byte // undone snapshots for redo
	base   []byte   // state after the last recorded change
}

func (a *App) snapshot() []byte {
	b, _ := json.Marshal(a.cfg)
	return b
}

// comparable form: ignore which preset/display is selected
func undoKey(b []byte) []byte {
	var c Config
	if json.Unmarshal(b, &c) != nil {
		return b
	}
	c.Active, c.Display, c.Monitors, c.PerMonitor = 0, "", nil, false
	k, _ := json.Marshal(c)
	return k
}

// trackUndo is called at the end of every frame.
func (a *App) trackUndo(u *UI) {
	if a.undo.base == nil {
		a.undo.base = a.snapshot()
		return
	}
	if u.Active != "" || u.Focus != "" || u.Cap != "" || u.In.Down {
		return // gesture still in progress
	}
	cur := a.snapshot()
	if bytes.Equal(undoKey(cur), undoKey(a.undo.base)) {
		return
	}
	a.undo.past = append(a.undo.past, a.undo.base)
	if len(a.undo.past) > undoMax {
		a.undo.past = a.undo.past[1:]
	}
	a.undo.future = nil
	a.undo.base = cur
}

func (a *App) restore(b []byte) {
	c := DefaultConfig()
	if json.Unmarshal(b, c) != nil || len(c.Presets) == 0 {
		return
	}
	c.Display = a.cfg.Display // never jump displays on undo
	c.PerMonitor, c.Monitors = a.cfg.PerMonitor, a.cfg.Monitors
	if c.Active < 0 || c.Active >= len(c.Presets) {
		c.Active = 0
	}
	a.cfg = c
	if a.cur().Linked {
		a.editCh = 0
	} else if a.editCh == 0 {
		a.editCh = 1
	}
	a.hkDirty = true
	a.Apply()
	a.markSave()
	a.undo.base = a.snapshot()
}

func (a *App) CanUndo() bool { return len(a.undo.past) > 0 }
func (a *App) CanRedo() bool { return len(a.undo.future) > 0 }

func (a *App) Undo() {
	if !a.CanUndo() {
		return
	}
	prev := a.undo.past[len(a.undo.past)-1]
	a.undo.past = a.undo.past[:len(a.undo.past)-1]
	a.undo.future = append(a.undo.future, a.snapshot())
	a.restore(prev)
}

func (a *App) Redo() {
	if !a.CanRedo() {
		return
	}
	next := a.undo.future[len(a.undo.future)-1]
	a.undo.future = a.undo.future[:len(a.undo.future)-1]
	a.undo.past = append(a.undo.past, a.snapshot())
	a.restore(next)
}

// undoKeys handles Ctrl+Z / Ctrl+Y / Ctrl+Shift+Z inside the app window.
func (a *App) undoKeys(u *UI) {
	if u.Focus != "" || u.Cap != "" {
		return
	}
	for _, k := range u.In.Keys {
		switch {
		case k.VK == 0x5A && k.Mods == ModCtrl:
			a.Undo()
			u.Dirty = true
		case (k.VK == 0x59 && k.Mods == ModCtrl) || (k.VK == 0x5A && k.Mods == ModCtrl|ModShift):
			a.Redo()
			u.Dirty = true
		}
	}
}

func (a *App) drawUndo(u *UI, x, y float64) {
	ut := "UNDO"
	if n := len(a.undo.past); n > 0 {
		ut = "UNDO · " + itoa(n)
	}
	ur := R(x, y+3, 84, 30)
	rr := R(x+90, y+3, 68, 30)
	if u.Button("undo", ur, ut, BtnGhost, a.CanUndo()) {
		a.Undo()
	}
	u.Tip(ur, "Undo last change (Ctrl+Z)")
	if u.Button("redo", rr, "REDO", BtnGhost, a.CanRedo()) {
		a.Redo()
	}
	u.Tip(rr, "Redo (Ctrl+Y)")
}
