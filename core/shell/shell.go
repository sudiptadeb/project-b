// Package shell is the contract with the shell page: a snapshot of state when it
// starts, events when things change, commands from the user. A shell owns nothing else.
package shell

import (
	"encoding/json"
	"log"

	"projectb/engine"
	"projectb/suggest"
	"projectb/tabs"
)

// command is what the shell sends us.
type command struct {
	T     string        `json:"t"`
	Text  string        `json:"text"`
	ID    int           `json:"id"`
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
}

// New returns a manager.
func New(eng engine.Engine, t *tabs.Manager, s *suggest.Manager) *Manager {
	return &Manager{eng: eng, tabs: t, suggest: s}
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
		m.send(map[string]any{"t": "snapshot", "engine": m.eng.Name(), "tab": m.tabs.Active()})
	case "navigate":
		m.tabs.Navigate(c.Text)
	case "suggest":
		m.send(map[string]any{"t": "suggestions", "id": c.ID, "items": m.suggest.Query(c.Text)})
	case "regions":
		m.eng.SetShellRegions(c.Rects)
	case "pageBounds":
		m.eng.SetPageBounds(c.Rect, c.ViewW, c.ViewH)
	case "focusPage":
		m.eng.FocusPage()
	default:
		log.Printf("shell: unknown command %q", c.T)
	}
}

// TabChanged tells the shell a tab's state changed.
func (m *Manager) TabChanged(t tabs.Tab) {
	m.send(map[string]any{"t": "tab", "tab": t})
}

// FocusAddressBar asks the shell to focus the address bar.
func (m *Manager) FocusAddressBar() {
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
