// Package shell is the contract with the shell, our one page: a snapshot of state when it
// starts, events when things change, commands from the user. It owns nothing else.
package shell

import (
	"encoding/json"
	"log"

	"projectb/crash"
	"projectb/engine"
	"projectb/prefs"
	"projectb/suggest"
	"projectb/tabs"
	"projectb/update"
	"projectb/window"
)

// command is what the shell sends us.
type command struct {
	T    string `json:"t"`
	Text string `json:"text"`
	ID   int64  `json:"id"`
	On   bool   `json:"on"`
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
}

// New returns a manager.
func New(eng engine.Engine, t *tabs.Manager, s *suggest.Manager, w *window.Manager, p *prefs.Manager, u *update.Manager, c *crash.Manager) *Manager {
	return &Manager{eng: eng, tabs: t, suggest: s, window: w, prefs: p, update: u, crash: c}
}

// Handle takes one command from the shell.
func (m *Manager) Handle(msg []byte) {
	var c command
	if err := json.Unmarshal(msg, &c); err != nil {
		log.Printf("shell: bad message: %v", err)
		return
	}
	switch c.T {
	case "ready":
		m.send(map[string]any{
			"t":            "snapshot",
			"engine":       m.eng.Name(),
			"tabs":         m.tabs.All(),
			"active":       m.tabs.ActiveID(),
			"windowColor":  m.window.Color(),
			"frameWidth":   m.window.FrameWidth(),
			"controls":     m.window.Controls(),
			"cornerRadius": m.window.CornerRadius(),
			"tabLayout":    m.prefs.TabLayout(),
			"autoUpdate":   m.prefs.AutoUpdate(),
			"update":       m.update.State(),
			"running":      m.update.Running(),
			"crashLog":     m.crash.Last(),
		})
	case "navigate":
		m.tabs.Navigate(c.Text)
	case "back":
		m.tabs.Back()
	case "forward":
		m.tabs.Forward()
	case "reload":
		m.tabs.Reload()
	case "newTab":
		m.tabs.Create()
	case "closeTab":
		m.tabs.Close(engine.TabID(c.ID))
	case "activateTab":
		m.tabs.Activate(engine.TabID(c.ID))
	case "setTabLayout":
		m.prefs.SetTabLayout(c.Text)
	case "setAutoUpdate":
		m.prefs.SetAutoUpdate(c.On)
	case "setFrameColor":
		m.prefs.SetFrameColor(c.Text)
	case "checkUpdate":
		m.update.CheckNow()
	case "relaunch":
		m.update.Relaunch()
	case "dismissCrash":
		m.crash.Dismiss()
		m.send(map[string]any{"t": "crashLog", "crashLog": ""})
	case "suggest":
		m.send(map[string]any{"t": "suggestions", "id": c.ID, "items": m.suggest.Query(c.Text)})
	case "layout":
		var l window.ShellLayout
		if err := json.Unmarshal(msg, &l); err != nil {
			log.Printf("shell: bad layout: %v", err)
			return
		}
		m.window.HandleShellLayout(l)
	case "window":
		if r, ok := requests[c.Text]; ok {
			m.window.Request(r)
		} else {
			log.Printf("shell: unknown window request %q", c.Text)
		}
	case "focusPage":
		m.eng.FocusPage()
	case "focusShell":
		m.eng.FocusShell()
	default:
		log.Printf("shell: unknown command %q", c.T)
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

// TabLayoutChanged tells the shell where the tabs go now.
func (m *Manager) TabLayoutChanged(layout string) {
	m.send(map[string]any{"t": "tabLayout", "tabLayout": layout})
}

// AutoUpdateChanged tells the shell that automatic updates were turned on or off.
func (m *Manager) AutoUpdateChanged(on bool) {
	m.send(map[string]any{"t": "autoUpdate", "autoUpdate": on})
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

// FocusAddressBar gives the shell focus and asks it to put that focus in the address bar.
func (m *Manager) FocusAddressBar() {
	m.eng.FocusShell()
	m.send(map[string]any{"t": "focusAddressBar"})
}

// send sends to the shell.
func (m *Manager) send(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("shell: %v", err)
		return
	}
	m.eng.SendToShell(b)
}
