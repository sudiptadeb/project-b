// Package tabs owns the tabs: which exist, in what order, which one is active, and
// each one's state. Tab state changes only through this manager.
package tabs

import (
	"net/url"
	"slices"
	"strings"

	"projectb/engine"
)

// Tab is a copy of one tab's state. Subscribers can keep it.
type Tab struct {
	ID      engine.TabID `json:"id"`
	URL     string       `json:"url"`
	Title   string       `json:"title"`
	Loading bool         `json:"loading"`
	Blocked bool         `json:"blocked"`
	Crashed bool         `json:"crashed"`
	Error   string       `json:"error,omitempty"`
	// Favicon is a data: URL, or "" when the page has none.
	Favicon string `json:"favicon,omitempty"`

	CanGoBack    bool `json:"canGoBack"`
	CanGoForward bool `json:"canGoForward"`
}

// Gate runs before a navigation. Returning false blocks it, and later gates don't run.
type Gate func(url string) bool

const searchURL = "https://duckduckgo.com/?q="

// Manager owns the tabs.
type Manager struct {
	eng    engine.Engine
	tabs   map[engine.TabID]*Tab
	order  []engine.TabID
	active engine.TabID

	gates      []Gate
	onChanged  []func(Tab)
	onFinished []func(Tab)
	onTitle    []func(Tab)
	onList     []func()
	onEmpty    []func()
}

// New returns an empty manager.
func New(eng engine.Engine) *Manager {
	return &Manager{eng: eng, tabs: map[engine.TabID]*Tab{}}
}

// AddGate adds a gate. Gates run in the order they were added.
func (m *Manager) AddGate(g Gate) { m.gates = append(m.gates, g) }

// OnChanged subscribes to any change in a tab's state.
func (m *Manager) OnChanged(f func(Tab)) { m.onChanged = append(m.onChanged, f) }

// OnNavigationFinished subscribes to committed navigations.
func (m *Manager) OnNavigationFinished(f func(Tab)) { m.onFinished = append(m.onFinished, f) }

// OnTitleChanged subscribes to title changes.
func (m *Manager) OnTitleChanged(f func(Tab)) { m.onTitle = append(m.onTitle, f) }

// OnListChanged subscribes to a tab opening or closing, or the active tab changing.
func (m *Manager) OnListChanged(f func()) { m.onList = append(m.onList, f) }

// OnEmpty subscribes to the last tab closing.
func (m *Manager) OnEmpty(f func()) { m.onEmpty = append(m.onEmpty, f) }

// Create opens a new tab at the end and makes it active.
func (m *Manager) Create() engine.TabID {
	id := m.eng.CreateTab()
	m.add(id)
	m.Activate(id)
	return id
}

func (m *Manager) add(id engine.TabID) *Tab {
	t := &Tab{ID: id}
	m.tabs[id] = t
	m.order = append(m.order, id)
	return t
}

// Activate shows a tab.
func (m *Manager) Activate(id engine.TabID) {
	if m.tabs[id] == nil || id == m.active {
		return
	}
	m.active = id
	m.eng.ShowTab(id)
	m.listChanged()
}

// ActivateAt shows the tab at a position, counting from zero. Out of range does nothing.
func (m *Manager) ActivateAt(i int) {
	if i >= 0 && i < len(m.order) {
		m.Activate(m.order[i])
	}
}

// ActivateLast shows the last tab.
func (m *Manager) ActivateLast() { m.ActivateAt(len(m.order) - 1) }

// ActivateNext shows the tab after the active one, or before it, wrapping around.
func (m *Manager) ActivateNext(reverse bool) {
	n := len(m.order)
	if n < 2 {
		return
	}
	i := slices.Index(m.order, m.active)
	if reverse {
		m.ActivateAt((i + n - 1) % n)
	} else {
		m.ActivateAt((i + 1) % n)
	}
}

// Close closes a tab. Closing the active one activates its right neighbour, or the
// left one at the end. Closing the last tab leaves no tabs and fires OnEmpty.
func (m *Manager) Close(id engine.TabID) {
	if m.tabs[id] == nil {
		return
	}
	i := slices.Index(m.order, id)
	delete(m.tabs, id)
	m.order = slices.Delete(m.order, i, i+1)
	m.eng.CloseTab(id)
	if len(m.order) == 0 {
		m.active = 0
		m.listChanged()
		for _, f := range m.onEmpty {
			f()
		}
		return
	}
	if id == m.active {
		m.active = 0
		m.ActivateAt(min(i, len(m.order)-1))
		return
	}
	m.listChanged()
}

// CloseActive closes the active tab.
func (m *Manager) CloseActive() { m.Close(m.active) }

// All returns copies of the tabs, in order.
func (m *Manager) All() []Tab {
	out := make([]Tab, len(m.order))
	for i, id := range m.order {
		out[i] = *m.tabs[id]
	}
	return out
}

// ActiveID returns the active tab's id, or zero when there is none.
func (m *Manager) ActiveID() engine.TabID { return m.active }

// Active returns a copy of the active tab.
func (m *Manager) Active() Tab {
	if t, ok := m.tabs[m.active]; ok {
		return *t
	}
	return Tab{}
}

// Navigate turns what the user typed into a URL and loads it in the active tab.
func (m *Manager) Navigate(text string) {
	u := NormalizeInput(text)
	if u == "" || m.tabs[m.active] == nil {
		return
	}
	m.eng.LoadURL(m.active, u)
}

// Back moves the active tab back in its history.
func (m *Manager) Back() {
	if t := m.tabs[m.active]; t != nil && t.CanGoBack {
		m.eng.GoBack(m.active)
	}
}

// Forward moves the active tab forward in its history.
func (m *Manager) Forward() {
	if t := m.tabs[m.active]; t != nil && t.CanGoForward {
		m.eng.GoForward(m.active)
	}
}

// Reload reloads the active tab.
func (m *Manager) Reload() {
	if t := m.tabs[m.active]; t != nil && t.URL != "" {
		m.eng.Reload(m.active)
	}
}

// HandleTabOpened records a tab a page opened, and shows it if the page asked for the
// foreground.
func (m *Manager) HandleTabOpened(opener, id engine.TabID, u string, foreground bool) {
	t := m.add(id)
	t.URL = u
	if foreground {
		m.Activate(id)
	} else {
		m.listChanged()
	}
}

// HandleNavigationStarting runs the gates and records the result.
func (m *Manager) HandleNavigationStarting(id engine.TabID, u string) bool {
	t := m.tabs[id]
	if t == nil {
		return false
	}
	t.URL, t.Error, t.Crashed = u, "", false
	for _, g := range m.gates {
		if !g(u) {
			t.Blocked, t.Loading = true, false
			m.changed(t)
			return false
		}
	}
	t.Blocked, t.Title, t.Favicon = false, "", ""
	m.changed(t)
	return true
}

// HandleNavigationFinished records a committed navigation.
func (m *Manager) HandleNavigationFinished(id engine.TabID, u string) {
	t := m.tabs[id]
	if t == nil {
		return
	}
	// A page restored from the back-forward cache never reports a start, so a commit
	// has to clear what an earlier navigation left behind.
	t.URL, t.Error, t.Blocked, t.Crashed = u, "", false, false
	for _, f := range m.onFinished {
		f(*t)
	}
	m.changed(t)
}

// HandleNavigationFailed records a failed navigation.
func (m *Manager) HandleNavigationFailed(id engine.TabID, u, reason string) {
	t := m.tabs[id]
	if t == nil {
		return
	}
	t.URL, t.Error, t.Loading = u, reason, false
	m.changed(t)
}

// HandleHistoryChanged records whether the tab can go back and forward.
func (m *Manager) HandleHistoryChanged(id engine.TabID, canGoBack, canGoForward bool) {
	t := m.tabs[id]
	if t == nil || (t.CanGoBack == canGoBack && t.CanGoForward == canGoForward) {
		return
	}
	t.CanGoBack, t.CanGoForward = canGoBack, canGoForward
	m.changed(t)
}

// HandleTitleChanged records a new title.
func (m *Manager) HandleTitleChanged(id engine.TabID, title string) {
	t := m.tabs[id]
	if t == nil || t.Title == title {
		return
	}
	t.Title = title
	for _, f := range m.onTitle {
		f(*t)
	}
	m.changed(t)
}

// HandleFaviconChanged records the page's icon.
func (m *Manager) HandleFaviconChanged(id engine.TabID, dataURL string) {
	t := m.tabs[id]
	if t == nil || t.Favicon == dataURL {
		return
	}
	t.Favicon = dataURL
	m.changed(t)
}

// HandleLoadingChanged records the loading state.
func (m *Manager) HandleLoadingChanged(id engine.TabID, loading bool) {
	t := m.tabs[id]
	if t == nil || t.Loading == loading {
		return
	}
	t.Loading = loading
	m.changed(t)
}

// HandlePageCrashed records that the tab's renderer died.
func (m *Manager) HandlePageCrashed(id engine.TabID) {
	t := m.tabs[id]
	if t == nil {
		return
	}
	t.Crashed, t.Loading = true, false
	m.changed(t)
}

func (m *Manager) changed(t *Tab) {
	for _, f := range m.onChanged {
		f(*t)
	}
}

func (m *Manager) listChanged() {
	for _, f := range m.onList {
		f()
	}
}

// NormalizeInput turns address bar text into a URL, or a search for anything else.
func NormalizeInput(text string) string {
	s := strings.TrimSpace(text)
	switch {
	case s == "":
		return ""
	case strings.Contains(s, "://"),
		strings.HasPrefix(s, "about:"),
		strings.HasPrefix(s, "chrome:"),
		strings.HasPrefix(s, "data:"):
		return s
	case strings.ContainsAny(s, " \t"):
		return searchURL + url.QueryEscape(s)
	case s == "localhost", strings.HasPrefix(s, "localhost:"), strings.HasPrefix(s, "127.0.0.1"):
		return "http://" + s
	case strings.Contains(s, "."):
		return "https://" + s
	}
	return searchURL + url.QueryEscape(s)
}
