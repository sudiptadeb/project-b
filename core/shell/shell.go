// Package shell is the contract with our own pages, the shell and the settings page: a
// snapshot of state when one starts, events when things change, commands from the user.
// Neither owns anything else.
package shell

import (
	"encoding/json"
	"log"
	"net/url"

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
	On    bool          `json:"on"`
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

// SettingsURL is the settings page. It loads in a tab.
const SettingsURL = "chrome://settings/"

// theShell is the shell as a sender and a receiver. Any other is a tab.
const theShell engine.TabID = 0

// settingsCommands are what the settings page may give: read and set the prefs, read the
// update state, and ask for a check. Nothing else.
var settingsCommands = map[string]bool{
	"prefs":         true,
	"setTabLayout":  true,
	"setAutoUpdate": true,
	"updateState":   true,
	"checkUpdate":   true,
}

// allowed is the one place that says who may give which command. The engine tags every
// message with its sender, and the page can't change that. The shell may give any
// command. The only page of ours the engine binds in a tab is the settings page, so any
// other sender is one.
func allowed(from engine.TabID, t string) bool {
	return from == theShell || settingsCommands[t]
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
		m.sendTo(theShell, map[string]any{
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
	case "openSettings":
		m.tabs.Open(SettingsURL)
	case "closeTab":
		m.tabs.Close(engine.TabID(c.ID))
	case "activateTab":
		m.tabs.Activate(engine.TabID(c.ID))
	case "prefs":
		m.sendTo(from, m.prefsMessage())
	case "setTabLayout":
		m.prefs.SetTabLayout(c.Text)
	case "setAutoUpdate":
		m.prefs.SetAutoUpdate(c.On)
	case "updateState":
		m.sendTo(from, map[string]any{"t": "update", "update": m.update.State(), "running": m.update.Running()})
	case "checkUpdate":
		m.update.CheckNow()
	case "relaunch":
		m.update.Relaunch()
	case "suggest":
		m.sendTo(from, map[string]any{"t": "suggestions", "id": c.ID, "items": m.suggest.Query(c.Text)})
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
	m.sendTo(theShell, map[string]any{"t": "tab", "tab": t})
}

// TabsChanged tells the shell which tabs exist now and which is active.
func (m *Manager) TabsChanged() {
	m.sendTo(theShell, map[string]any{"t": "tabs", "tabs": m.tabs.All(), "active": m.tabs.ActiveID()})
}

// TabLayoutChanged tells the shell where the tabs go now, and every settings page.
func (m *Manager) TabLayoutChanged(layout string) {
	m.sendTo(theShell, map[string]any{"t": "tabLayout", "tabLayout": layout})
	m.toSettings(m.prefsMessage())
}

// AutoUpdateChanged tells every settings page that automatic updates were turned on or off.
func (m *Manager) AutoUpdateChanged(bool) {
	m.toSettings(m.prefsMessage())
}

// UpdateChanged tells the shell and every settings page what the updater is doing:
// nothing, checking, downloading, ready to relaunch, or failed.
func (m *Manager) UpdateChanged(s update.State) {
	msg := map[string]any{"t": "update", "update": s}
	m.sendTo(theShell, msg)
	m.toSettings(msg)
}

// WindowControlsChanged tells the shell where the native window controls now are and
// how round the window's corners are.
func (m *Manager) WindowControlsChanged(r engine.Rect, cornerRadius float64) {
	m.sendTo(theShell, map[string]any{"t": "controls", "controls": r, "cornerRadius": cornerRadius})
}

// FocusAddressBar gives the shell focus and asks it to put that focus in the address bar.
func (m *Manager) FocusAddressBar() {
	m.eng.FocusShell()
	m.sendTo(theShell, map[string]any{"t": "focusAddressBar"})
}

func (m *Manager) prefsMessage() map[string]any {
	return map[string]any{"t": "prefs", "prefs": m.prefs.All()}
}

// toSettings sends to every tab whose address is the settings page. The engine delivers
// only to our own page with its bridge, so a tab still on its way there gets nothing, and
// asks once it has loaded.
func (m *Manager) toSettings(v any) {
	for _, t := range m.tabs.All() {
		if u, err := url.Parse(t.URL); err == nil && u.Scheme == "chrome" && u.Host == "settings" {
			m.sendTo(t.ID, v)
		}
	}
}

// sendTo sends to the shell, or to our page in a tab.
func (m *Manager) sendTo(to engine.TabID, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("shell: %v", err)
		return
	}
	if to == theShell {
		m.eng.SendToShell(b)
	} else {
		m.eng.SendToTab(to, b)
	}
}
