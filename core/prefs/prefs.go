// Package prefs owns the profile's settings: where the tabs go, the frame colour, the
// theme and the shell. They live in one small JSON file in the profile, read when the app starts and
// written when one changes. Automatic updates are the app's, not a profile's
// (core/update).
package prefs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"projectb/shells"
	"projectb/timeline"
	"regexp"
)

// Tab layouts.
const (
	TabsVertical   = "vertical"
	TabsHorizontal = "horizontal"
)

// Where a phone shell puts its address bar: at the foot, by the thumb, or on top.
const (
	AddressBottom = "bottom"
	AddressTop    = "top"
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
	FrameColor string `json:"frameColor"`
	Theme      string `json:"theme"`
	// Shell is the id of the shell every window loads.
	Shell string `json:"shell"`
	// AddressBarPosition is AddressBottom or AddressTop. Only the Mobile shell reads it.
	AddressBarPosition string `json:"addressBarPosition"`
}

// Defaults are the settings before the user changes any.
func Defaults() Prefs {
	return Prefs{TabLayout: TabsVertical, FrameColor: "#4f9e9e", Theme: ThemeSystem, Shell: shells.Default(), AddressBarPosition: AddressBottom}
}

func isTheme(t string) bool { return t == ThemeSystem || t == ThemeLight || t == ThemeDark }

// A frame colour is "#rrggbb", as a colour input gives it.
var frameColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Manager owns the settings.
type Manager struct {
	path         string
	prefs        Prefs
	onLayout     []func(string)
	onFrameColor []func(string)
	onTheme      []func(string)
	onShell      []func(string)
	onAddress    []func(string)
	onChanged    []func(Prefs)
}

// New returns a manager with the settings saved under dir, or the defaults. A setting the
// file doesn't hold, or holds a value for that isn't one, keeps its default. The file is a
// few bytes, so it is read right here.
func New(dir string) *Manager {
	m := &Manager{path: filepath.Join(dir, "prefs.json"), prefs: Defaults()}
	if b, err := os.ReadFile(m.path); err == nil {
		s := Defaults()
		if json.Unmarshal(b, &s) == nil {
			if s.TabLayout != TabsVertical && s.TabLayout != TabsHorizontal {
				s.TabLayout = TabsVertical
			}
			if !frameColor.MatchString(s.FrameColor) {
				s.FrameColor = Defaults().FrameColor
			}
			if !isTheme(s.Theme) {
				s.Theme = ThemeSystem
			}
			if s.AddressBarPosition != AddressBottom && s.AddressBarPosition != AddressTop {
				s.AddressBarPosition = AddressBottom
			}
			if !shells.Fits(s.Shell) {
				s.Shell = Defaults().Shell
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
	m.changed()
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
	m.changed()
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
	m.changed()
}

// Shell is the id of the shell every window loads.
func (m *Manager) Shell() string { return m.prefs.Shell }

// OnShellChanged subscribes to the shell changing.
func (m *Manager) OnShellChanged(f func(string)) { m.onShell = append(m.onShell, f) }

// SetShell changes the shell. Anything but a shell's id is ignored.
func (m *Manager) SetShell(id string) {
	if !shells.Has(id) || id == m.prefs.Shell {
		return
	}
	m.prefs.Shell = id
	m.save()
	for _, f := range m.onShell {
		f(id)
	}
	m.changed()
}

// AddressBarPosition is where a phone shell puts its address bar: AddressBottom or
// AddressTop.
func (m *Manager) AddressBarPosition() string { return m.prefs.AddressBarPosition }

// OnAddressBarPositionChanged subscribes to the address bar moving.
func (m *Manager) OnAddressBarPositionChanged(f func(string)) {
	m.onAddress = append(m.onAddress, f)
}

// SetAddressBarPosition moves the address bar. Anything but the two positions is ignored.
func (m *Manager) SetAddressBarPosition(at string) {
	if at != AddressBottom && at != AddressTop || at == m.prefs.AddressBarPosition {
		return
	}
	m.prefs.AddressBarPosition = at
	m.save()
	for _, f := range m.onAddress {
		f(at)
	}
	m.changed()
}

// OnChanged subscribes to any setting changing, with them all as they are now.
func (m *Manager) OnChanged(f func(Prefs)) { m.onChanged = append(m.onChanged, f) }

// changed tells OnChanged's subscribers, after a setting's own have heard.
func (m *Manager) changed() {
	for _, f := range m.onChanged {
		f(m.prefs)
	}
}

// Set sets one setting by its name in the file, through its own setter, so a value that
// setter refuses changes nothing. It says whether name is a setting.
func (m *Manager) Set(name, value string) bool {
	switch name {
	case "tabLayout":
		m.SetTabLayout(value)
	case "frameColor":
		m.SetFrameColor(value)
	case "theme":
		m.SetTheme(value)
	case "shell":
		m.SetShell(value)
	case "addressBarPosition":
		m.SetAddressBarPosition(value)
	default:
		return false
	}
	return true
}

// Apply sets each setting values holds, in the form the file has them, through its own
// setter, so every listener hears of it as of a change by hand. A setting values doesn't
// hold keeps its value. A profile reset to a template takes the template's values so.
func (m *Manager) Apply(values json.RawMessage) error {
	p := m.prefs
	if err := json.Unmarshal(values, &p); err != nil {
		return err
	}
	m.SetTabLayout(p.TabLayout)
	m.SetFrameColor(p.FrameColor)
	m.SetTheme(p.Theme)
	m.SetShell(p.Shell)
	m.SetAddressBarPosition(p.AddressBarPosition)
	return nil
}

func (m *Manager) save() {
	b, _ := json.Marshal(m.prefs)
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		timeline.Errorf("prefs: %v", err)
		return
	}
	// A new file renamed over the old, so no reader and no crash sees half of one.
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		timeline.Errorf("prefs: %v", err)
		return
	}
	if err := os.Rename(tmp, m.path); err != nil {
		timeline.Errorf("prefs: %v", err)
	}
}
