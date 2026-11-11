// Package settings is the contract with the settings page: a page of its own, put over
// the whole window while it is open, so the shell never carries it. Like the shell's
// contract, it sends a snapshot when the page starts and events when things change, and
// turns the user's changes into calls on the managers. It owns nothing else.
package settings

import (
	"encoding/json"
	"log"

	"projectb/commands"
	"projectb/engine"
	"projectb/prefs"
	"projectb/update"
	"projectb/window"
)

// command is what the settings page sends us.
type command struct {
	T       string `json:"t"`
	Text    string `json:"text"`
	ID      string `json:"id"`
	Index   int    `json:"index"`
	On      bool   `json:"on"`
	Reserve string `json:"reserve"`
}

// Manager connects the settings page to the managers.
type Manager struct {
	eng      engine.Engine
	window   *window.Manager
	prefs    *prefs.Manager
	update   *update.Manager
	commands *commands.Manager
	open     bool
	// section is the section a caller asked the page to show, or "" for its first.
	section string
	parts   []Part
}

// Part is what another manager adds to the settings page's contract: its state in the
// snapshot, and commands of its own. Handle says whether the command was one of its.
type Part struct {
	Snapshot func(into map[string]any)
	Handle   func(t string, msg []byte) bool
}

// AddPart adds a part.
func (m *Manager) AddPart(p Part) { m.parts = append(m.parts, p) }

// OpenSection opens the settings page at one of its sections, by id.
func (m *Manager) OpenSection(section string) {
	m.section = section
	if m.open {
		m.send(map[string]any{"t": "section", "section": section})
		return
	}
	m.Open()
}

// New returns a manager.
func New(eng engine.Engine, w *window.Manager, p *prefs.Manager, u *update.Manager, c *commands.Manager) *Manager {
	return &Manager{eng: eng, window: w, prefs: p, update: u, commands: c}
}

// Open puts the settings page over the window. It is one page: opening it again does
// nothing.
func (m *Manager) Open() {
	if m.open {
		return
	}
	m.open = true
	m.eng.OpenSettings()
}

// Close takes the settings page away.
func (m *Manager) Close() {
	if !m.open {
		return
	}
	m.open = false
	m.section = ""
	m.commands.SetRecording(false)
	m.eng.CloseSettings()
}

// IsOpen says whether the settings page is showing.
func (m *Manager) IsOpen() bool { return m.open }

// Handle takes one command from the settings page.
func (m *Manager) Handle(msg []byte) {
	var c command
	if err := json.Unmarshal(msg, &c); err != nil {
		log.Printf("settings: bad message: %v", err)
		return
	}
	switch c.T {
	case "ready":
		snapshot := map[string]any{
			"t":            "snapshot",
			"windowColor":  m.window.Color(),
			"frameWidth":   m.window.FrameWidth(),
			"controls":     m.window.Controls(),
			"cornerRadius": m.window.CornerRadius(),
			"tabLayout":    m.prefs.TabLayout(),
			"theme":        m.prefs.Theme(),
			"autoUpdate":   m.prefs.AutoUpdate(),
			"update":       m.update.State(),
			"running":      m.update.Running(),
			"keyboard":     m.commands.View(),
		}
		snapshot["section"] = m.section
		for _, p := range m.parts {
			p.Snapshot(snapshot)
		}
		m.send(snapshot)
	case "close":
		m.Close()
	case "setTabLayout":
		m.prefs.SetTabLayout(c.Text)
	case "setTheme":
		m.prefs.SetTheme(c.Text)
	case "setAutoUpdate":
		m.prefs.SetAutoUpdate(c.On)
	case "setFrameColor":
		m.prefs.SetFrameColor(c.Text)
	case "checkUpdate":
		m.update.CheckNow()
	case "useTemplate":
		m.commands.UseTemplate(c.Text)
	case "addKey":
		if err := m.commands.AddBinding(c.ID, c.Text, c.Reserve); err != nil {
			log.Printf("settings: %v", err)
		}
	case "removeKey":
		m.commands.RemoveBinding(c.ID, c.Index)
	case "setReserve":
		m.commands.SetReserve(c.ID, c.Index, c.Reserve)
	case "setHidden":
		m.commands.SetHidden(c.ID, c.On)
	case "resetAction":
		m.commands.ResetAction(c.ID)
	case "dismissMissing":
		m.commands.DismissMissing()
	case "recording":
		m.commands.SetRecording(c.On)
	default:
		for _, p := range m.parts {
			if p.Handle(c.T, msg) {
				return
			}
		}
		log.Printf("settings: unknown command %q", c.T)
	}
}

// TabLayoutChanged, ThemeChanged, AutoUpdateChanged, UpdateChanged, WindowColorChanged and
// WindowControlsChanged tell an open settings page what the managers now say.
func (m *Manager) TabLayoutChanged(layout string) {
	m.send(map[string]any{"t": "tabLayout", "tabLayout": layout})
}

func (m *Manager) ThemeChanged(theme string) {
	m.send(map[string]any{"t": "theme", "theme": theme})
}

func (m *Manager) AutoUpdateChanged(on bool) {
	m.send(map[string]any{"t": "autoUpdate", "autoUpdate": on})
}

func (m *Manager) UpdateChanged(s update.State) {
	m.send(map[string]any{"t": "update", "update": s})
}

func (m *Manager) WindowColorChanged(color string) {
	m.send(map[string]any{"t": "windowColor", "windowColor": color})
}

func (m *Manager) WindowControlsChanged(r engine.Rect, cornerRadius float64) {
	m.send(map[string]any{"t": "controls", "controls": r, "cornerRadius": cornerRadius})
}

// KeyboardChanged sends the keys again after any change to them.
func (m *Manager) KeyboardChanged() {
	m.send(map[string]any{"t": "keyboard", "keyboard": m.commands.View()})
}

// Send sends one message to the settings page while it is open, for a part.
func (m *Manager) Send(v any) { m.send(v) }

func (m *Manager) send(v any) {
	if !m.open {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("settings: %v", err)
		return
	}
	m.eng.SendToSettings(b)
}
