// Package prefs owns the user's settings: where the tabs go, and whether the app checks for
// updates on its own. They live in one small JSON file in the profile, read when the app
// starts and written when one changes.
package prefs

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// Tab layouts.
const (
	TabsVertical   = "vertical"
	TabsHorizontal = "horizontal"
)

// Prefs is every setting, as the file holds them and the settings page reads them.
type Prefs struct {
	TabLayout  string `json:"tabLayout"`
	AutoUpdate bool   `json:"autoUpdate"`
}

func defaults() Prefs { return Prefs{TabLayout: TabsVertical, AutoUpdate: true} }

// Manager owns the settings.
type Manager struct {
	path         string
	prefs        Prefs
	onLayout     []func(string)
	onAutoUpdate []func(bool)
}

// New returns a manager with the settings saved under dir, or the defaults. A setting the
// file doesn't hold, or holds a value for that isn't one, keeps its default. The file is a
// few bytes, so it is read right here.
func New(dir string) *Manager {
	m := &Manager{path: filepath.Join(dir, "prefs.json"), prefs: defaults()}
	if b, err := os.ReadFile(m.path); err == nil {
		s := defaults()
		if json.Unmarshal(b, &s) == nil {
			if s.TabLayout != TabsVertical && s.TabLayout != TabsHorizontal {
				s.TabLayout = TabsVertical
			}
			m.prefs = s
		}
	}
	return m
}

// All returns a copy of every setting.
func (m *Manager) All() Prefs { return m.prefs }

// TabLayout says where the tabs go: TabsVertical or TabsHorizontal.
func (m *Manager) TabLayout() string { return m.prefs.TabLayout }

// OnTabLayoutChanged subscribes to the tab layout changing.
func (m *Manager) OnTabLayoutChanged(f func(string)) { m.onLayout = append(m.onLayout, f) }

// SetTabLayout changes where the tabs go. Anything but the two layouts is ignored.
func (m *Manager) SetTabLayout(layout string) {
	if layout != TabsVertical && layout != TabsHorizontal || layout == m.prefs.TabLayout {
		return
	}
	m.prefs.TabLayout = layout
	m.save()
	for _, f := range m.onLayout {
		f(layout)
	}
}

// AutoUpdate says whether the app checks for updates on its own. A check the user asks
// for runs either way.
func (m *Manager) AutoUpdate() bool { return m.prefs.AutoUpdate }

// OnAutoUpdateChanged subscribes to automatic updates being turned on or off.
func (m *Manager) OnAutoUpdateChanged(f func(bool)) {
	m.onAutoUpdate = append(m.onAutoUpdate, f)
}

// SetAutoUpdate turns automatic update checks on or off.
func (m *Manager) SetAutoUpdate(on bool) {
	if on == m.prefs.AutoUpdate {
		return
	}
	m.prefs.AutoUpdate = on
	m.save()
	for _, f := range m.onAutoUpdate {
		f(on)
	}
}

func (m *Manager) save() {
	b, _ := json.Marshal(m.prefs)
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		log.Printf("prefs: %v", err)
		return
	}
	if err := os.WriteFile(m.path, append(b, '\n'), 0o600); err != nil {
		log.Printf("prefs: %v", err)
	}
}
