//go:build extensions

package extensions

import (
	"strconv"

	"projectb/engine"
)

// Prop is one thing a toolbar button shows, which its extension can set for every tab
// or for one.
type Prop int

const (
	Title Prop = iota
	Badge
	BadgeColor
	BadgeTextColor
	Popup // The page the button opens, a URL, or "" to be told of the click instead.
	Icon  // A data: URL.
	props
)

// values is what an extension set: nil where it set nothing.
type values struct {
	props   [props]*string
	enabled *bool
	// blocked is how many requests the extension's rules blocked in this tab's page.
	blocked int
}

// action is one button's state: what its extension set for all tabs, and for each tab.
type action struct {
	all  values
	tabs map[engine.TabID]*values
}

func (m *Manager) values(id string, tab engine.TabID) *values {
	a := m.actions[id]
	if a == nil {
		a = &action{tabs: map[engine.TabID]*values{}}
		m.actions[id] = a
	}
	if tab == 0 {
		return &a.all
	}
	if a.tabs[tab] == nil {
		a.tabs[tab] = &values{}
	}
	return a.tabs[tab]
}

// SetProp sets what a button shows, in one tab or, with tab zero, in all of them. A nil
// value takes back what was set.
func (m *Manager) SetProp(id string, tab engine.TabID, p Prop, value *string) {
	m.values(id, tab).props[p] = value
	m.changed()
}

// Prop returns what a button shows in a tab: what was set for the tab, else for all
// tabs, else what the manifest says.
func (m *Manager) Prop(id string, tab engine.TabID, p Prop) string {
	if a := m.actions[id]; a != nil {
		if v := a.tabs[tab]; v != nil && v.props[p] != nil {
			return *v.props[p]
		}
		// A count of blocked requests shows where the tab has no badge text of its own.
		if v := a.tabs[tab]; p == Badge && v != nil && v.blocked > 0 {
			return strconv.Itoa(v.blocked)
		}
		if a.all.props[p] != nil {
			return *a.all.props[p]
		}
	}
	e, _ := m.Get(id)
	switch p {
	case Title:
		if e.ActionTitle != "" {
			return e.ActionTitle
		}
		return e.Name
	case Popup:
		return e.ActionPopup
	case Icon:
		if e.ActionIcon != "" {
			return e.ActionIcon
		}
		return e.Icon
	}
	return ""
}

// SetButtonEnabled lets a button be pressed or not, in one tab or in all.
func (m *Manager) SetButtonEnabled(id string, tab engine.TabID, on bool) {
	m.values(id, tab).enabled = &on
	m.changed()
}

// ButtonEnabled says whether a button can be pressed in a tab.
func (m *Manager) ButtonEnabled(id string, tab engine.TabID) bool {
	e, ok := m.Get(id)
	if !ok || !e.Enabled || !e.HasAction {
		return false
	}
	if a := m.actions[id]; a != nil {
		if v := a.tabs[tab]; v != nil && v.enabled != nil {
			return *v.enabled
		}
		if a.all.enabled != nil {
			return *a.all.enabled
		}
	}
	return true
}

// HandleBlockedCount takes the engine's count of requests an extension blocked in a tab.
func (m *Manager) HandleBlockedCount(id string, tab engine.TabID, count int) {
	m.values(id, tab).blocked = count
	m.changed()
}

// ForgetTab drops what extensions set for one tab: it closed, or went to another page.
func (m *Manager) ForgetTab(tab engine.TabID) {
	for _, a := range m.actions {
		delete(a.tabs, tab)
	}
	m.changed()
}

// Buttons returns every enabled extension's button as the active tab shows it, pinned
// ones first in the order they were pinned.
func (m *Manager) Buttons() []Button {
	tab := m.activeTab()
	button := func(e engine.Extension) Button {
		return Button{
			ID: e.ID, Name: e.Name,
			Title: m.Prop(e.ID, tab, Title), Icon: m.Prop(e.ID, tab, Icon),
			Badge: m.Prop(e.ID, tab, Badge), BadgeColor: m.Prop(e.ID, tab, BadgeColor),
			BadgeTextColor: m.Prop(e.ID, tab, BadgeTextColor),
			Enabled:        m.ButtonEnabled(e.ID, tab), Pinned: m.Pinned(e.ID),
		}
	}
	out := []Button{}
	for _, id := range m.saved.Pinned {
		if e, ok := m.Get(id); ok && e.Enabled {
			out = append(out, button(e))
		}
	}
	for _, e := range m.list {
		if e.Enabled && !m.Pinned(e.ID) {
			out = append(out, button(e))
		}
	}
	return out
}

// OpenPopup is which extension's popup is showing, or "".
func (m *Manager) OpenPopup() string { return m.popup }

// Click is a press on an extension's button, which sits at anchor in the window. It
// opens the button's popup under it, or closes that popup if it is the one showing. A
// button with no popup tells its extension it was clicked.
func (m *Manager) Click(id string, anchor engine.Rect) {
	if m.popup == id {
		m.ClosePopup()
		return
	}
	tab := m.activeTab()
	if !m.ButtonEnabled(id, tab) {
		return
	}
	if popup := m.Prop(id, tab, Popup); popup != "" {
		m.popup = id
		m.eng.OpenExtensionPopup(id, popup, anchor)
		m.changed()
	} else if m.onClicked != nil {
		m.onClicked(id)
	}
}

// ClosePopup closes the popup that is open, if one is.
func (m *Manager) ClosePopup() {
	if m.popup != "" {
		m.eng.CloseExtensionPopup()
	}
}

// HandlePopupClosed takes the engine's word that the popup is gone.
func (m *Manager) HandlePopupClosed() {
	m.popup = ""
	m.changed()
}
