package main

// Per-monitor presets: every monitor (by stable ID) keeps its own preset list,
// active preset and hotkeys. cfg.Presets always holds the list of the monitor
// currently being controlled; it is copied back into cfg.Monitors on save and
// whenever the target monitor changes.

// syncProfile stores the working preset list into the current monitor's profile.
func (a *App) syncProfile() {
	if !a.cfg.PerMonitor || a.profKey == "" {
		return
	}
	if a.cfg.Monitors == nil {
		a.cfg.Monitors = map[string]*MonitorProfile{}
	}
	name := ""
	if d := a.display(); d != nil && d.ID == a.profKey {
		name = d.Monitor
	} else if old := a.cfg.Monitors[a.profKey]; old != nil {
		name = old.Name
	}
	a.cfg.Monitors[a.profKey] = &MonitorProfile{Name: name, Presets: copyPresets(a.cfg.Presets), Active: a.cfg.Active}
}

// loadProfileFor switches the working preset list to monitor d.
// A monitor seen for the first time starts with a copy of the current list.
func (a *App) loadProfileFor(d *DisplayInfo) {
	if !a.cfg.PerMonitor || d == nil || d.ID == a.profKey {
		return
	}
	a.syncProfile()
	if prof := a.cfg.Monitors[d.ID]; prof != nil && len(prof.Presets) > 0 {
		a.cfg.Presets = copyPresets(prof.Presets)
		a.cfg.Active = prof.Active
		if a.cfg.Active < 0 || a.cfg.Active >= len(a.cfg.Presets) {
			a.cfg.Active = 0
		}
	}
	a.profKey = d.ID
	a.syncProfile()
	a.afterListSwap()
}

func (a *App) afterListSwap() {
	if a.cur().Linked {
		a.editCh = 0
	} else if a.editCh == 0 {
		a.editCh = 1
	}
	a.listScroll = 0
	a.hkDirty = true
	a.undo = undoState{} // undo history belongs to one monitor's list
	a.markSave()
}

// setPerMonitor turns separate per-monitor lists on or off.
func (a *App) setPerMonitor(on bool) {
	a.cfg.PerMonitor = on
	a.profKey = ""
	if on {
		if d := a.display(); d != nil {
			// adopt the list on screen right now for this monitor
			a.profKey = d.ID
			a.syncProfile()
		}
	}
	a.afterListSwap()
}

// profileCount returns how many presets a monitor has saved (0 = none yet).
func (a *App) profileCount(id string) int {
	if id == a.profKey {
		return len(a.cfg.Presets)
	}
	if p := a.cfg.Monitors[id]; p != nil {
		return len(p.Presets)
	}
	return 0
}

// copyListToAllMonitors gives every known monitor a copy of the current list.
func (a *App) copyListToAllMonitors() {
	if !a.cfg.PerMonitor {
		return
	}
	a.syncProfile()
	for _, d := range a.displays {
		if d.ID == a.profKey {
			continue
		}
		a.cfg.Monitors[d.ID] = &MonitorProfile{Name: d.Monitor, Presets: copyPresets(a.cfg.Presets), Active: a.cfg.Active}
	}
	a.markSave()
}
