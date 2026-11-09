// Package shell is the contract with the shell page: a snapshot of state when it
// starts, events when things change, commands from the user. A shell owns nothing else.
package shell

import (
	"encoding/json"
	"log"

	"projectb/engine"
	"projectb/prefs"
	"projectb/suggest"
	"projectb/tabs"
	"projectb/update"
	"projectb/window"
)

// command is what the shell sends us.
type command struct {
	T     string        `json:"t"`
	Text  string        `json:"text"`
	ID    int64         `json:"id"`
	Rect  engine.Rect   `json:"rect"`
	Rects []engine.Rect `json:"rects"`
	ViewW float64       `json:"viewW"`
	ViewH float64       `json:"viewH"`
}

// Manager connects the shell page to the managers.
type Manager struct {
	eng     engine.Engine
	tabs    *tabs.Manager
	suggest *suggest.Manager
	window  *window.Manager
	prefs   *prefs.Manager
	update  *update.Manager
}

// New returns a manager.
func New(eng engine.Engine, t *tabs.Manager, s *suggest.Manager, w *window.Manager, p *prefs.Manager, u *update.Manager) *Manager {
	return &Manager{eng: eng, tabs: t, suggest: s, window: w, prefs: p, update: u}
}

// fromShell is the sender of a message from the shell. Any other sender is a tab.
const fromShell engine.TabID = 0

// allowed is the one place that says who may give which command. The engine tags every
// message with its sender, and the page can't change that. The shell may give any
// command; a page in a tab may give none.
func allowed(from engine.TabID, t string) bool {
	return from == fromShell
}

// Handle takes one command from one of our pages: the shell, or a page in tab from.
func (m *Manager) Handle(from engine.TabID, msg []byte) {
	var c command
	if err := json.Unmarshal(msg, &c); err != nil {
		log.Printf("shell: bad message from %d: %v", from, err)
		return
	}
	if !allowed(from, c.T) {
		log.Printf("shell: dropped %q from tab %d, which may not give it", c.T, from)
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
			"update":       m.update.State(),
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
	case "relaunch":
		m.update.Relaunch()
	case "suggest":
		m.send(map[string]any{"t": "suggestions", "id": c.ID, "items": m.suggest.Query(c.Text)})
	case "regions":
		m.eng.SetShellRegions(c.Rects)
	case "dragRegions":
		m.eng.SetDragRegions(c.Rects)
	case "pageBounds":
		m.eng.SetPageBounds(c.Rect, c.ViewW, c.ViewH)
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

// UpdateChanged tells the shell what the updater is doing: nothing, downloading, ready to
// relaunch, or failed.
func (m *Manager) UpdateChanged(s update.State) {
	m.send(map[string]any{"t": "update", "update": s})
}

// WindowControlsChanged tells the shell where the native window controls now are and
// how round the window's corners are.
func (m *Manager) WindowControlsChanged(r engine.Rect, cornerRadius float64) {
	m.send(map[string]any{"t": "controls", "controls": r, "cornerRadius": cornerRadius})
}

// FocusAddressBar gives the shell focus and asks it to put that focus in the address bar.
func (m *Manager) FocusAddressBar() {
	m.eng.FocusShell()
	m.send(map[string]any{"t": "focusAddressBar"})
}

func (m *Manager) send(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("shell: %v", err)
		return
	}
	m.eng.SendToShell(b)
}
