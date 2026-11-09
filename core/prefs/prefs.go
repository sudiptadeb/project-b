// Package prefs owns the user's settings: today, where the tabs go. They live in one
// small JSON file in the profile, read when the app starts and written when one changes.
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

type saved struct {
	TabLayout string `json:"tabLayout"`
}

// Manager owns the settings.
type Manager struct {
	path     string
	prefs    saved
	onLayout []func(string)
}

// New returns a manager with the settings saved under dir, or the defaults. The file
// is a few bytes, so it is read right here.
func New(dir string) *Manager {
	m := &Manager{path: filepath.Join(dir, "prefs.json"), prefs: saved{TabLayout: TabsVertical}}
	if b, err := os.ReadFile(m.path); err == nil {
		var s saved
		if json.Unmarshal(b, &s) == nil && (s.TabLayout == TabsVertical || s.TabLayout == TabsHorizontal) {
			m.prefs = s
		}
	}
	return m
}

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
