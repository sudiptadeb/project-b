// Package tabs owns the tabs: which exist, in what order, which one is active, and
// each one's state. Tab state changes only through this manager. The active tab is the
// one the user acts on: the shell shows it and gives it the keyboard, and says where it
// put it in its layout, which shows it.
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
	// Progress is how far the load has got, 0 to 1.
	Progress float64 `json:"progress"`
	Blocked  bool    `json:"blocked"`
	Crashed  bool    `json:"crashed"`
	Error    string  `json:"error,omitempty"`
	// Favicon is a data: URL, or "" when the page has none.
	Favicon string `json:"favicon,omitempty"`

	CanGoBack    bool `json:"canGoBack"`
	CanGoForward bool `json:"canGoForward"`
}

// Gate runs before a navigation. Returning false blocks it, and later gates don't run.
type Gate func(url string) bool

const searchURL = "https://duckduckgo.com/?q="

// BlankURL is what a new tab loads, so its page area holds a real page in content's
// default colours from the start, rather than no page at all. The tab still reads as
// empty: no address, titled New Tab, nothing in history.
const BlankURL = "about:blank"

// Manager owns the tabs.
type Manager struct {
	eng    engine.Engine
	tabs   map[engine.TabID]*Tab
	order  []engine.TabID
	active engine.TabID
	// recent is every tab, the most recently shown first. While a cycle through it runs,
	// cycle is the order it started with and at is where it has got to.
	recent []engine.TabID
	cycle  []engine.TabID
	at     int

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

// Create opens a new tab at the end, on the blank page, and makes it active.
func (m *Manager) Create() engine.TabID {
	id := m.eng.CreateTab()
	m.add(id)
	m.eng.LoadURL(id, BlankURL)
	m.Activate(id)
	return id
}

// address is what a tab shows for a URL: nothing for the blank page.
func address(u string) string {
	if u == BlankURL {
		return ""
	}
	return u
}

func (m *Manager) add(id engine.TabID) *Tab {
	t := &Tab{ID: id}
	m.tabs[id] = t
	m.order = append(m.order, id)
	m.recent = append(m.recent, id)
	return t
}

// Activate makes a tab the active one.
func (m *Manager) Activate(id engine.TabID) {
	if m.tabs[id] == nil || id == m.active {
		return
	}
	m.active = id
	if m.cycle == nil {
		m.touch(id)
	}
	m.listChanged()
}

// touch makes a tab the most recent.
func (m *Manager) touch(id engine.TabID) {
	if i := slices.Index(m.recent, id); i >= 0 {
		m.recent = slices.Delete(m.recent, i, i+1)
	}
	m.recent = slices.Insert(m.recent, 0, id)
}

// CycleRecent shows the next tab back in recent order, as the OS's app switcher does
// while its key is held: each press goes one further, past the oldest it comes round to
// the newest, and the order stays as it was until EndCycle. Reverse goes the other way.
func (m *Manager) CycleRecent(reverse bool) {
	if len(m.recent) < 2 {
		return
	}
	if m.cycle == nil {
		m.cycle = slices.Clone(m.recent)
		m.at = 0
	}
	n := len(m.cycle)
	if reverse {
		m.at = (m.at + n - 1) % n
	} else {
		m.at = (m.at + 1) % n
	}
	m.Activate(m.cycle[m.at])
}

// EndCycle ends a cycle through recent tabs: the tab it stopped on becomes the most recent.
func (m *Manager) EndCycle() {
	if m.cycle == nil {
		return
	}
	m.cycle = nil
	m.touch(m.active)
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
	m.EndCycle()
	i := slices.Index(m.order, id)
	delete(m.tabs, id)
	m.order = slices.Delete(m.order, i, i+1)
	if j := slices.Index(m.recent, id); j >= 0 {
		m.recent = slices.Delete(m.recent, j, j+1)
	}
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

// ReloadBypassingCache reloads the active tab without the cache.
func (m *Manager) ReloadBypassingCache() {
	if t := m.tabs[m.active]; t != nil && t.URL != "" {
		m.eng.ReloadBypassingCache(m.active)
	}
}

// Stop stops the active tab loading.
func (m *Manager) Stop() {
	if t := m.tabs[m.active]; t != nil && t.Loading {
		m.eng.Stop(m.active)
	}
}

// Duplicate opens a copy of the active tab, history and all, as the last tab, and shows it.
func (m *Manager) Duplicate() {
	t := m.tabs[m.active]
	if t == nil {
		return
	}
	m.HandleTabOpened(m.active, m.eng.DuplicateTab(m.active), t.URL, true)
}

// Open opens a URL in a new last tab and shows it.
func (m *Manager) Open(u string) { m.OpenIn(u, true) }

// OpenIn opens a URL in a new last tab, shown or behind the active one.
func (m *Manager) OpenIn(u string, foreground bool) engine.TabID {
	id := m.eng.CreateTab()
	m.HandleTabOpened(m.active, id, u, foreground)
	m.eng.LoadURL(id, u)
	return id
}

// Get returns a copy of one tab.
func (m *Manager) Get(id engine.TabID) (Tab, bool) {
	if t, ok := m.tabs[id]; ok {
		return *t, true
	}
	return Tab{}, false
}

// ReloadTab reloads one tab, with or without the cache.
func (m *Manager) ReloadTab(id engine.TabID, bypassCache bool) {
	if m.tabs[id] == nil {
		return
	}
	if bypassCache {
		m.eng.ReloadBypassingCache(id)
	} else {
		m.eng.Reload(id)
	}
}

// Load loads a URL in one tab.
func (m *Manager) Load(id engine.TabID, u string) {
	if m.tabs[id] != nil {
		m.eng.LoadURL(id, u)
	}
}

// zoomFactors are the steps zooming in and out moves through, as browsers have them.
var zoomFactors = []float64{0.25, 1.0 / 3, 0.5, 2.0 / 3, 0.75, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2, 2.5, 3, 4, 5}

// ZoomIn and ZoomOut move the active tab's page zoom one step; ZoomReset puts it back to
// actual size.
func (m *Manager) ZoomIn()    { m.zoomStep(1) }
func (m *Manager) ZoomOut()   { m.zoomStep(-1) }
func (m *Manager) ZoomReset() { m.zoomTo(1) }

func (m *Manager) zoomStep(dir int) {
	if m.tabs[m.active] == nil {
		return
	}
	now := m.eng.ZoomOf(m.active)
	const near = 0.001
	if dir > 0 {
		for _, f := range zoomFactors {
			if f > now+near {
				m.zoomTo(f)
				return
			}
		}
		return
	}
	for i := len(zoomFactors) - 1; i >= 0; i-- {
		if zoomFactors[i] < now-near {
			m.zoomTo(zoomFactors[i])
			return
		}
	}
}

func (m *Manager) zoomTo(f float64) {
	if m.tabs[m.active] != nil {
		m.eng.Zoom(m.active, f)
	}
}

// HandleTabOpened records a tab a page opened, and shows it if the page asked for the
// foreground.
func (m *Manager) HandleTabOpened(opener, id engine.TabID, u string, foreground bool) {
	t := m.add(id)
	t.URL = address(u)
	if foreground {
		m.Activate(id)
	} else {
		m.listChanged()
	}
}

// HandleTabFocused makes the tab whose page took the keyboard the active one.
func (m *Manager) HandleTabFocused(id engine.TabID) { m.Activate(id) }

// HandleNavigationStarting runs the gates and records the result.
func (m *Manager) HandleNavigationStarting(id engine.TabID, u string) bool {
	t := m.tabs[id]
	if t == nil {
		return false
	}
	t.URL, t.Error, t.Crashed = address(u), "", false
	for _, g := range m.gates {
		if !g(u) {
			t.Blocked, t.Loading = true, false
			m.changed(t)
			return false
		}
	}
	t.Blocked, t.Title, t.Favicon, t.Progress = false, "", "", 0
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
	t.URL, t.Error, t.Blocked, t.Crashed = address(u), "", false, false
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
	t.URL, t.Error, t.Loading = address(u), reason, false
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

// HandleTitleChanged records a new title. Content titles an untitled page with its
// address, so the blank page arrives titled about:blank; the tab shows no title.
func (m *Manager) HandleTitleChanged(id engine.TabID, title string) {
	if title == BlankURL {
		title = ""
	}
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

// HandleLoadProgressChanged records how far the load has got.
func (m *Manager) HandleLoadProgressChanged(id engine.TabID, progress float64) {
	t := m.tabs[id]
	if t == nil || t.Progress == progress {
		return
	}
	t.Progress = progress
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
