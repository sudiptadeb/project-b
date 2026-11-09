// Package shell is the contract with the shell page: a snapshot of state when it
// starts, events when things change, commands from the user. A shell owns nothing else.
package shell

import (
	"encoding/json"
	"log"

	"projectb/engine"
	"projectb/suggest"
	"projectb/tabs"
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
}

// New returns a manager.
func New(eng engine.Engine, t *tabs.Manager, s *suggest.Manager, w *window.Manager) *Manager {
	return &Manager{eng: eng, tabs: t, suggest: s, window: w}
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
			"t":        "snapshot",
			"engine":   m.eng.Name(),
			"tabs":     m.tabs.All(),
			"active":   m.tabs.ActiveID(),
			"controls": m.window.Controls(),
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

// WindowControlsChanged tells the shell where the native window controls now are.
func (m *Manager) WindowControlsChanged(r engine.Rect) {
	m.send(map[string]any{"t": "controls", "controls": r})
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
