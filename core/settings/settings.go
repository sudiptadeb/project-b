// Package settings is the contract with the settings page: a page of its own, put over
// the whole of a window while it is open, so the shell never carries it. Each window has
// its own. Like the shell's contract, it sends a snapshot when the page starts and events
// when things change, and turns the user's changes into calls on the managers. It owns
// nothing else.
package settings

import (
	"encoding/json"

	"projectb/commands"
	"projectb/engine"
	"projectb/flags"
	"projectb/prefs"
	"projectb/timeline"
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
	flags    *flags.Manager
	update   *update.Manager
	commands *commands.Manager
	// open holds each window whose settings page is open, with what it was asked to show
	// and whether it has said it's ready.
	open  map[engine.WindowID]*page
	parts []Part
	// relaunch starts the app again, for the flags to take effect.
	relaunch func()
}

// Part is what another manager adds to the settings page's contract: its state in the
// snapshot, and commands of its own, from window w's page. Handle says whether the
// command was one of its.
type Part struct {
	Snapshot func(into map[string]any)
	Handle   func(w engine.WindowID, t string, msg []byte) bool
}

// AddPart adds a part.
func (m *Manager) AddPart(p Part) { m.parts = append(m.parts, p) }

// page is one window's open settings page.
type page struct {
	// section is the section a caller asked the page to show, or "" for its first.
	section string
	// Whether the page has said it's ready. Until then nothing reaches it: its script
	// isn't there to take a message, and its snapshot has it all.
	heard bool
}

// OpenSection opens window w's settings page at one of its sections, by id.
func (m *Manager) OpenSection(w engine.WindowID, section string) {
	if p := m.open[w]; p != nil {
		p.section = section
		m.send(w, map[string]any{"t": "section", "section": section})
		return
	}
	m.Open(w)
	m.open[w].section = section
}

// New returns a manager.
func New(eng engine.Engine, w *window.Manager, p *prefs.Manager, f *flags.Manager, u *update.Manager, c *commands.Manager) *Manager {
	return &Manager{eng: eng, window: w, prefs: p, flags: f, update: u, commands: c, open: map[engine.WindowID]*page{}}
}

// OnRelaunch names what the Flags section's Relaunch does.
func (m *Manager) OnRelaunch(f func()) { m.relaunch = f }

// Open puts the settings page over window w. Each window has one: opening it again does
// nothing.
func (m *Manager) Open(w engine.WindowID) {
	if _, ok := m.open[w]; ok {
		return
	}
	m.open[w] = &page{}
	timeline.Begin("settings.open")
	timeline.Mark("settings.open.go")
	m.eng.OpenSettings(w)
}

// Close takes window w's settings page away.
func (m *Manager) Close(w engine.WindowID) {
	if _, ok := m.open[w]; !ok {
		return
	}
	delete(m.open, w)
	m.commands.SetRecording(false)
	m.eng.CloseSettings(w)
}

// WindowClosed forgets the settings page of a window that closed: it went with the window.
func (m *Manager) WindowClosed(w engine.WindowID) {
	if _, ok := m.open[w]; ok {
		delete(m.open, w)
		m.commands.SetRecording(false)
	}
}

// IsOpen says whether window w's settings page is showing.
func (m *Manager) IsOpen(w engine.WindowID) bool {
	_, ok := m.open[w]
	return ok
}

// Handle takes one command from window w's settings page.
func (m *Manager) Handle(w engine.WindowID, msg []byte) {
	var c command
	if err := json.Unmarshal(msg, &c); err != nil {
		timeline.Warnf("settings: bad message: %v", err)
		return
	}
	switch c.T {
	case "ready":
		// A page Go didn't open, or one closed a moment ago, still asks: it hears nothing
		// more, as it isn't open.
		section := ""
		if p := m.open[w]; p != nil {
			p.heard = true
			section = p.section
		}
		snapshot := map[string]any{
			"t":                  "snapshot",
			"windowColor":        m.window.Color(),
			"controls":           m.window.Controls(w),
			"cornerRadius":       m.window.CornerRadius(w),
			"tabLayout":          m.prefs.TabLayout(),
			"theme":              m.prefs.Theme(),
			"addressBarPosition": m.prefs.AddressBarPosition(),
			"autoUpdate":         m.update.Auto(),
			"update":             m.update.State(),
			"running":            m.update.Running(),
			"keyboard":           m.commands.View(),
			"flags":              m.flags.View(),
		}
		snapshot["section"] = section
		for _, p := range m.parts {
			p.Snapshot(snapshot)
		}
		m.send(w, snapshot)
		timeline.Mark("settings.ready")
	case "timeline":
		timeline.FromPage("settings", msg)
	case "close":
		m.Close(w)
	case "setTabLayout":
		m.prefs.SetTabLayout(c.Text)
	case "setTheme":
		m.prefs.SetTheme(c.Text)
	case "setAddressBarPosition":
		m.prefs.SetAddressBarPosition(c.Text)
	case "setAutoUpdate":
		m.update.SetAuto(c.On)
	case "setFrameColor":
		m.prefs.SetFrameColor(c.Text)
	case "checkUpdate":
		m.update.CheckNow()
	case "useTemplate":
		m.commands.UseTemplate(c.Text)
	case "addKey":
		if err := m.commands.AddBinding(c.ID, c.Text, c.Reserve); err != nil {
			timeline.Errorf("settings: %v", err)
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
	case "setFlag":
		m.flags.Set(c.ID, c.Text)
	case "relaunch":
		// With a release ready, relaunching is applying it, as the shell's own button
		// does: on a phone that hands it to Android's installer, which a plain relaunch
		// never would.
		if m.update.State().Phase == update.Ready {
			m.update.Relaunch()
		} else if m.relaunch != nil {
			m.relaunch()
		}
	default:
		for _, p := range m.parts {
			if p.Handle(w, c.T, msg) {
				return
			}
		}
		timeline.Warnf("settings: unknown command %q", c.T)
	}
}

// TabLayoutChanged, ThemeChanged, AutoUpdateChanged, UpdateChanged and WindowColorChanged
// tell every open settings page what the managers now say; WindowControlsChanged tells
// window w's.
func (m *Manager) TabLayoutChanged(layout string) {
	m.sendAll(map[string]any{"t": "tabLayout", "tabLayout": layout})
}

func (m *Manager) AddressBarPositionChanged(at string) {
	m.sendAll(map[string]any{"t": "addressBarPosition", "addressBarPosition": at})
}

func (m *Manager) ThemeChanged(theme string) {
	m.sendAll(map[string]any{"t": "theme", "theme": theme})
}

func (m *Manager) AutoUpdateChanged(on bool) {
	m.sendAll(map[string]any{"t": "autoUpdate", "autoUpdate": on})
}

func (m *Manager) UpdateChanged(s update.State) {
	m.sendAll(map[string]any{"t": "update", "update": s})
}

func (m *Manager) WindowColorChanged(color string) {
	m.sendAll(map[string]any{"t": "windowColor", "windowColor": color})
}

func (m *Manager) WindowControlsChanged(w engine.WindowID, r engine.Rect, cornerRadius float64) {
	m.send(w, map[string]any{"t": "controls", "controls": r, "cornerRadius": cornerRadius})
}

// KeyboardChanged sends the keys again after any change to them.
func (m *Manager) KeyboardChanged() {
	m.sendAll(map[string]any{"t": "keyboard", "keyboard": m.commands.View()})
}

// FlagsChanged sends the flags again after one is saved.
func (m *Manager) FlagsChanged() {
	m.sendAll(map[string]any{"t": "flags", "flags": m.flags.View()})
}

// Send sends one message to every open settings page, for a part.
func (m *Manager) Send(v any) { m.sendAll(v) }

func (m *Manager) sendAll(v any) {
	for _, w := range m.window.Windows() {
		m.send(w, v)
	}
}

// send sends to window w's settings page, if it is open and has said it's ready.
func (m *Manager) send(w engine.WindowID, v any) {
	if p := m.open[w]; p == nil || !p.heard {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		timeline.Errorf("settings: %v", err)
		return
	}
	m.eng.SendToSettings(w, b)
}
