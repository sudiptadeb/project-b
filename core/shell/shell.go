// Package shell is the contract with the shell, our one page: a snapshot of state when it
// starts, events when things change, commands from the user. It owns nothing else.
package shell

import (
	"encoding/json"

	"projectb/crash"
	"projectb/engine"
	"projectb/prefs"
	"projectb/suggest"
	"projectb/tabs"
	"projectb/timeline"
	"projectb/update"
	"projectb/window"
)

// command is what the shell sends us.
type command struct {
	T    string `json:"t"`
	Text string `json:"text"`
	ID   int64  `json:"id"`
}

// requests are the names the shell uses for engine.WindowRequest.
var requests = map[string]engine.WindowRequest{
	"close":      engine.WindowClose,
	"minimise":   engine.WindowMinimise,
	"zoom":       engine.WindowZoom,
	"fullScreen": engine.WindowFullScreen,
}

// Manager connects the shell page to the managers.
type Manager struct {
	eng     engine.Engine
	tabs    *tabs.Manager
	suggest *suggest.Manager
	window  *window.Manager
	prefs   *prefs.Manager
	update  *update.Manager
	crash   *crash.Manager
	run     func(action string)
	keys    func() map[string]string
	os      string
	parts   []Part
	// moving is the span waiting for the shell's next layout report, or "".
	moving string
}

// Part is what another manager adds to the shell's contract: its state in the snapshot,
// and commands of its own. Handle says whether the command was one of its.
type Part struct {
	Snapshot func(into map[string]any)
	Handle   func(t string, msg []byte) bool
}

// AddPart adds a part.
func (m *Manager) AddPart(p Part) { m.parts = append(m.parts, p) }

// New returns a manager. run runs an action by id, as a menu item would; keys says the key
// each action shows, on platform os, for the shell's hints.
func New(eng engine.Engine, t *tabs.Manager, s *suggest.Manager, w *window.Manager, p *prefs.Manager, u *update.Manager, c *crash.Manager, run func(action string), keys func() map[string]string, os string) *Manager {
	return &Manager{eng: eng, tabs: t, suggest: s, window: w, prefs: p, update: u, crash: c, run: run, keys: keys, os: os}
}

// Interaction runs what one thing the user asked for does, as a span on the timeline: it
// begins here unless the shell began it at the click, and ends once the shell has drawn
// the result.
func (m *Manager) Interaction(name string, do func()) {
	timeline.Begin(name)
	timeline.Mark(name + ".go")
	do()
	timeline.Mark(name + ".done")
	m.send(map[string]any{"t": "paint", "span": name})
}

// SwitchTabLayout is the interaction that moves the tabs. The page moves too, a step
// later: the shell measures where it now goes and Go places it. So this span ends once
// the shell has drawn after that.
func (m *Manager) SwitchTabLayout(layout string) {
	if layout == m.prefs.TabLayout() {
		return
	}
	timeline.Begin("tabs.layout")
	timeline.Mark("tabs.layout.go")
	m.moving = "tabs.layout"
	m.prefs.SetTabLayout(layout)
	timeline.Mark("tabs.layout.done")
}

// Handle takes one command from the shell.
func (m *Manager) Handle(msg []byte) {
	var c command
	if err := json.Unmarshal(msg, &c); err != nil {
		timeline.Warnf("shell: bad message: %v", err)
		return
	}
	switch c.T {
	case "ready":
		snapshot := map[string]any{
			"t":            "snapshot",
			"engine":       m.eng.Name(),
			"tabs":         m.tabs.All(),
			"active":       m.tabs.ActiveID(),
			"windowColor":  m.window.Color(),
			"frameWidth":   m.window.FrameWidth(),
			"controls":     m.window.Controls(),
			"cornerRadius": m.window.CornerRadius(),
			"tabLayout":    m.prefs.TabLayout(),
			"theme":        m.prefs.Theme(),
			"update":       m.update.State(),
			"running":      m.update.Running(),
			"crashLog":     m.crash.Last(),
			"crashReport":  m.crash.Report(),
			"platform":     m.os,
			"keys":         m.keys(),
		}
		for _, p := range m.parts {
			p.Snapshot(snapshot)
		}
		m.send(snapshot)
		timeline.Mark("shell.ready")
	case "timeline":
		timeline.FromPage("shell", msg)
	case "navigate":
		m.tabs.Navigate(c.Text)
	case "back":
		m.tabs.Back()
	case "forward":
		m.tabs.Forward()
	case "reload":
		m.tabs.Reload()
	case "newTab":
		m.Interaction("tab.new", func() { m.tabs.Create() })
	case "closeTab":
		m.Interaction("tab.close", func() { m.tabs.Close(engine.TabID(c.ID)) })
	case "activateTab":
		m.tabs.Activate(engine.TabID(c.ID))
	case "setTabLayout":
		m.SwitchTabLayout(c.Text)
	case "run":
		m.run(c.Text)
	case "relaunch":
		m.update.Relaunch()
	case "dismissCrash":
		m.crash.Dismiss()
		m.send(map[string]any{"t": "crashLog", "crashLog": "", "crashReport": ""})
	case "suggest":
		m.send(map[string]any{"t": "suggestions", "id": c.ID, "items": m.suggest.Query(c.Text)})
	case "layout":
		var l window.ShellLayout
		if err := json.Unmarshal(msg, &l); err != nil {
			timeline.Warnf("shell: bad layout: %v", err)
			return
		}
		m.window.HandleShellLayout(l)
		if m.moving != "" {
			timeline.Mark(m.moving + ".page_moved")
			m.send(map[string]any{"t": "paint", "span": m.moving})
			m.moving = ""
		}
	case "window":
		if r, ok := requests[c.Text]; ok {
			m.window.Request(r)
		} else {
			timeline.Warnf("shell: unknown window request %q", c.Text)
		}
	case "focusPage":
		m.eng.FocusPage()
	case "focusShell":
		m.eng.FocusShell()
	default:
		for _, p := range m.parts {
			if p.Handle(c.T, msg) {
				return
			}
		}
		timeline.Warnf("shell: unknown command %q", c.T)
	}
}

// TabChanged tells the shell a tab's state changed.
func (m *Manager) TabChanged(t tabs.Tab) {
	m.send(map[string]any{"t": "tab", "tab": t})
}

// TabsChanged tells the shell which tabs exist now and which is active.
func (m *Manager) TabsChanged() {
	m.send(map[string]any{"t": "tabs", "tabs": m.tabs.All(), "active": m.tabs.ActiveID()})
}

// ThemeChanged tells the shell to be light, dark or as the system is.
func (m *Manager) ThemeChanged(theme string) {
	m.send(map[string]any{"t": "theme", "theme": theme})
}

// TabLayoutChanged tells the shell where the tabs go now.
func (m *Manager) TabLayoutChanged(layout string) {
	m.send(map[string]any{"t": "tabLayout", "tabLayout": layout})
}

// UpdateChanged tells the shell what the updater is doing: nothing, checking,
// downloading, ready to relaunch, or failed.
func (m *Manager) UpdateChanged(s update.State) {
	m.send(map[string]any{"t": "update", "update": s})
}

// WindowControlsChanged tells the shell where the native window controls now are and
// how round the window's corners are.
func (m *Manager) WindowControlsChanged(r engine.Rect, cornerRadius float64) {
	m.send(map[string]any{"t": "controls", "controls": r, "cornerRadius": cornerRadius})
}

// WindowColorChanged tells the shell the frame's new colour, which the window now paints.
func (m *Manager) WindowColorChanged(color string) {
	m.send(map[string]any{"t": "windowColor", "windowColor": color})
}

// KeysChanged tells the shell the keys its hints show, after the keymap changed.
func (m *Manager) KeysChanged() {
	m.send(map[string]any{"t": "keys", "keys": m.keys()})
}

// FocusAddressBar gives the shell focus and asks it to put that focus in the address bar.
func (m *Manager) FocusAddressBar() {
	m.eng.FocusShell()
	m.send(map[string]any{"t": "focusAddressBar"})
}

// Send sends one message to the shell, for a part.
func (m *Manager) Send(v any) { m.send(v) }

// send sends to the shell.
func (m *Manager) send(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		timeline.Errorf("shell: %v", err)
		return
	}
	m.eng.SendToShell(b)
}
