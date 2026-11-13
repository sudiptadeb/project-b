// Package shell is the contract with the shell, the page each window has: a snapshot of
// state when it starts, events when things change, commands from the user. Every message
// says which window's shell it is to or from. It owns nothing else.
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
	// The windows whose shell page has said it's ready. Until then nothing reaches it:
	// its script isn't there to take a message, and the snapshot it asks for has it all,
	// a new window's first tab included.
	heard map[engine.WindowID]bool
	// Whether app.ready is marked: a shell first showed Go's state. Once a process.
	ready bool
	// moving is the span waiting for a shell's next layout report, or "".
	moving string
}

// Part is what another manager adds to the shell's contract: its state in the snapshot,
// and commands of its own, from window w's shell. Handle says whether the command was
// one of its.
type Part struct {
	Snapshot func(into map[string]any)
	Handle   func(w engine.WindowID, t string, msg []byte) bool
}

// AddPart adds a part.
func (m *Manager) AddPart(p Part) { m.parts = append(m.parts, p) }

// New returns a manager. run runs an action by id, as a menu item would; keys says the key
// each action shows, on platform os, for the shell's hints.
func New(eng engine.Engine, t *tabs.Manager, s *suggest.Manager, w *window.Manager, p *prefs.Manager, u *update.Manager, c *crash.Manager, run func(action string), keys func() map[string]string, os string) *Manager {
	return &Manager{eng: eng, tabs: t, suggest: s, window: w, prefs: p, update: u, crash: c, run: run, keys: keys, os: os, heard: map[engine.WindowID]bool{}}
}

// Interaction runs what one thing the user asked for in window w does, as a span on the
// timeline: it begins here unless the shell began it at the click, and ends once w's
// shell has drawn the result.
func (m *Manager) Interaction(w engine.WindowID, name string, do func()) {
	timeline.Begin(name)
	timeline.Mark(name + ".go")
	do()
	timeline.Mark(name + ".done")
	m.send(w, map[string]any{"t": "paint", "span": name})
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

// Handle takes one command from window w's shell.
func (m *Manager) Handle(w engine.WindowID, msg []byte) {
	var c command
	if err := json.Unmarshal(msg, &c); err != nil {
		timeline.Warnf("shell: bad message: %v", err)
		return
	}
	switch c.T {
	case "ready":
		m.heard[w] = true
		snapshot := map[string]any{
			"t":            "snapshot",
			"tabs":         m.tabs.AllIn(w),
			"active":       m.tabs.ActiveIn(w),
			"windowColor":  m.window.Color(),
			"frameWidth":   m.window.FrameWidth(),
			"controls":     m.window.Controls(w),
			"cornerRadius": m.window.CornerRadius(w),
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
		m.send(w, snapshot)
		timeline.Mark("shell.ready")
	case "shown":
		// The shell shows the snapshot, so the app can be used. Tests wait for this line.
		if !m.ready {
			m.ready = true
			timeline.Mark("app.ready")
		}
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
		m.Interaction(w, "tab.new", func() { m.tabs.CreateIn(w) })
	case "closeTab":
		m.Interaction(w, "tab.close", func() { m.tabs.Close(engine.TabID(c.ID)) })
	case "activateTab":
		m.Interaction(w, "tab.activate", func() { m.tabs.Activate(engine.TabID(c.ID)) })
	case "setTabLayout":
		m.SwitchTabLayout(c.Text)
	case "run":
		m.run(c.Text)
	case "relaunch":
		m.update.Relaunch()
	case "dismissCrash":
		m.crash.Dismiss()
		m.sendAll(map[string]any{"t": "crashLog", "crashLog": "", "crashReport": ""})
	case "suggest":
		m.send(w, map[string]any{"t": "suggestions", "id": c.ID, "items": m.suggest.Query(c.Text)})
	case "layout":
		var l window.ShellLayout
		if err := json.Unmarshal(msg, &l); err != nil {
			timeline.Warnf("shell: bad layout: %v", err)
			return
		}
		m.window.HandleShellLayout(w, l)
		if m.moving != "" {
			timeline.Mark(m.moving + ".page_moved")
			m.send(w, map[string]any{"t": "paint", "span": m.moving})
			m.moving = ""
		}
	case "window":
		if r, ok := requests[c.Text]; ok {
			m.window.Request(w, r)
		} else {
			timeline.Warnf("shell: unknown window request %q", c.Text)
		}
	case "focusTab":
		m.eng.FocusTab(engine.TabID(c.ID))
	case "focusShell":
		m.eng.FocusShell(w)
	default:
		for _, p := range m.parts {
			if p.Handle(w, c.T, msg) {
				return
			}
		}
		timeline.Warnf("shell: unknown command %q", c.T)
	}
}

// TabChanged tells the shell of a tab's window that its state changed.
func (m *Manager) TabChanged(t tabs.Tab) {
	m.send(m.tabs.WindowOf(t.ID), map[string]any{"t": "tab", "tab": t})
}

// TabsChanged tells each window's shell which tabs it has now and which is active.
func (m *Manager) TabsChanged() {
	for _, w := range m.window.Windows() {
		m.send(w, map[string]any{"t": "tabs", "tabs": m.tabs.AllIn(w), "active": m.tabs.ActiveIn(w)})
	}
}

// ThemeChanged tells the shells to be light, dark or as the system is.
func (m *Manager) ThemeChanged(theme string) {
	m.sendAll(map[string]any{"t": "theme", "theme": theme})
}

// TabLayoutChanged tells the shells where the tabs go now.
func (m *Manager) TabLayoutChanged(layout string) {
	m.sendAll(map[string]any{"t": "tabLayout", "tabLayout": layout})
}

// UpdateChanged tells the shells what the updater is doing: nothing, checking,
// downloading, ready to relaunch, or failed.
func (m *Manager) UpdateChanged(s update.State) {
	m.sendAll(map[string]any{"t": "update", "update": s})
}

// WindowControlsChanged tells window w's shell where its native window controls now are
// and how round its corners are.
func (m *Manager) WindowControlsChanged(w engine.WindowID, r engine.Rect, cornerRadius float64) {
	m.send(w, map[string]any{"t": "controls", "controls": r, "cornerRadius": cornerRadius})
}

// WindowColorChanged tells the shells the frame's new colour, which the windows now paint.
func (m *Manager) WindowColorChanged(color string) {
	m.sendAll(map[string]any{"t": "windowColor", "windowColor": color})
}

// KeysChanged tells the shells the keys their hints show, after the keymap changed.
func (m *Manager) KeysChanged() {
	m.sendAll(map[string]any{"t": "keys", "keys": m.keys()})
}

// FocusAddressBar gives window w's shell focus and asks it to put that focus in the
// address bar.
func (m *Manager) FocusAddressBar(w engine.WindowID) {
	m.eng.FocusShell(w)
	m.send(w, map[string]any{"t": "focusAddressBar"})
}

// Send sends one message to every shell, for a part.
func (m *Manager) Send(v any) { m.sendAll(v) }

// sendAll sends to every window's shell.
func (m *Manager) sendAll(v any) {
	for _, w := range m.window.Windows() {
		m.send(w, v)
	}
}

// send sends to window w's shell, once it has said it's ready.
func (m *Manager) send(w engine.WindowID, v any) {
	if !m.heard[w] {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		timeline.Errorf("shell: %v", err)
		return
	}
	m.eng.SendToShell(w, b)
}

// HandleShellCrashed waits for window w's reloaded shell to say it's ready, as the first
// did. Every other window's shell goes on hearing.
func (m *Manager) HandleShellCrashed(w engine.WindowID) { delete(m.heard, w) }

// WindowClosed forgets the shell of a window that closed: it went with the window.
func (m *Manager) WindowClosed(w engine.WindowID) { delete(m.heard, w) }
