// Package prefs owns the user's settings: where the tabs go, whether the app checks for
// updates on its own, and the frame colour. They live in one small JSON file in the profile, read when the app
// starts and written when one changes.
package prefs

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

// Tab layouts.
const (
	TabsVertical   = "vertical"
	TabsHorizontal = "horizontal"
)

// Themes: the shell and the window follow the system's light or dark, or stay one.
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

// Prefs is every setting, as the file holds them.
type Prefs struct {
	TabLayout  string `json:"tabLayout"`
	AutoUpdate bool   `json:"autoUpdate"`
	FrameColor string `json:"frameColor"`
	Theme      string `json:"theme"`
}

func defaults() Prefs {
	return Prefs{TabLayout: TabsVertical, AutoUpdate: true, FrameColor: "#4f9e9e", Theme: ThemeSystem}
}

func isTheme(t string) bool { return t == ThemeSystem || t == ThemeLight || t == ThemeDark }

// A frame colour is "#rrggbb", as a colour input gives it.
var frameColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Manager owns the settings.
type Manager struct {
	path         string
	prefs        Prefs
	onLayout     []func(string)
	onAutoUpdate []func(bool)
	onFrameColor []func(string)
	onTheme      []func(string)
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
			if !frameColor.MatchString(s.FrameColor) {
				s.FrameColor = defaults().FrameColor
			}
			if !isTheme(s.Theme) {
				s.Theme = ThemeSystem
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

// FrameColor is the colour the window's frame is painted, "#rrggbb". The shell's other
// tones derive from it.
func (m *Manager) FrameColor() string { return m.prefs.FrameColor }

// OnFrameColorChanged subscribes to the frame colour changing.
func (m *Manager) OnFrameColorChanged(f func(string)) {
	m.onFrameColor = append(m.onFrameColor, f)
}

// SetFrameColor changes the frame colour. Anything but "#rrggbb" is ignored.
func (m *Manager) SetFrameColor(color string) {
	if !frameColor.MatchString(color) || color == m.prefs.FrameColor {
		return
	}
	m.prefs.FrameColor = color
	m.save()
	for _, f := range m.onFrameColor {
		f(color)
	}
}

// Theme is ThemeSystem, ThemeLight or ThemeDark.
func (m *Manager) Theme() string { return m.prefs.Theme }

// OnThemeChanged subscribes to the theme changing.
func (m *Manager) OnThemeChanged(f func(string)) { m.onTheme = append(m.onTheme, f) }

// SetTheme changes the theme. Anything but the three themes is ignored.
func (m *Manager) SetTheme(theme string) {
	if !isTheme(theme) || theme == m.prefs.Theme {
		return
	}
	m.prefs.Theme = theme
	m.save()
	for _, f := range m.onTheme {
		f(theme)
	}
}

func (m *Manager) save() {
	b, _ := json.Marshal(m.prefs)
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		log.Printf("prefs: %v", err)
		return
	}
	// A new file renamed over the old, so no reader and no crash sees half of one.
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		log.Printf("prefs: %v", err)
		return
	}
	if err := os.Rename(tmp, m.path); err != nil {
		log.Printf("prefs: %v", err)
	}
}
