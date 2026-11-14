// Package tabs owns the windows and the tabs: which windows exist, which is in front,
// each one's tabs in order and which of them is active, and each tab's state, the
// workspace it is in among it. Tab state changes only through this manager. A window's active tab is the one the user acts on
// there: its shell shows it and gives it the keyboard, and says where it put it in its
// layout, which shows it. Commands act on the active tab of the window in front. A tab
// keeps its id for its life.
package tabs

import (
	"maps"
	"net/url"
	"slices"
	"strings"

	"projectb/engine"
)

// Tab is a copy of one tab's state. Subscribers can keep it.
type Tab struct {
	ID engine.TabID `json:"id"`
	// Workspace is the id of the workspace the tab is in: "" is the profile's own storage,
	// Main's. It is given when the tab is made and never changes, whichever window the tab
	// is in.
	Workspace string `json:"workspace"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Loading   bool   `json:"loading"`
	// Progress is how far the load has got, 0 to 1.
	Progress float64 `json:"progress"`
	Blocked  bool    `json:"blocked"`
	Crashed  bool    `json:"crashed"`
	Error    string  `json:"error,omitempty"`
	// Favicon is a data: URL, or "" when the page has none.
	Favicon string `json:"favicon,omitempty"`

	CanGoBack    bool `json:"canGoBack"`
	CanGoForward bool `json:"canGoForward"`
	// Group is the id of the tab group it is in, "" for none (groups.go).
	Group string `json:"group,omitempty"`
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
	eng engine.Engine
	// first is the first workspace in the profile's list, where a tab opens when nothing
	// picks one.
	first    func() string
	tabs     map[engine.TabID]*Tab
	windowOf map[engine.TabID]engine.WindowID
	windows  map[engine.WindowID]*window
	// used is every window, the one in front first, then in the order they were last in
	// front.
	used []engine.WindowID

	gates      []Gate
	onChanged  []func(Tab)
	onFinished []func(Tab)
	onTitle    []func(Tab)
	onList     []func()
	onEmpty    []func(engine.WindowID)
	onFront    []func(engine.WindowID)
	onMoved    []func(tab engine.TabID, from engine.WindowID, fromIndex int, to engine.WindowID)
	// groupSeq numbers the groups, for their ids.
	groupSeq int
}

// window is one window's tabs: their order, the active one, and recent use.
type window struct {
	order  []engine.TabID
	active engine.TabID
	// recent is every tab, the most recently shown first. While a cycle through it runs,
	// cycle is the order it started with and at is where it has got to.
	recent []engine.TabID
	cycle  []engine.TabID
	at     int
	// groups are its tab groups, in the order they were made.
	groups []*Group
}

// New returns a manager with no windows. first says which workspace is first in the
// profile's list.
func New(eng engine.Engine, first func() string) *Manager {
	return &Manager{eng: eng, first: first, tabs: map[engine.TabID]*Tab{}, windowOf: map[engine.TabID]engine.WindowID{}, windows: map[engine.WindowID]*window{}}
}

// AddGate adds a gate. Gates run in the order they were added.
func (m *Manager) AddGate(g Gate) { m.gates = append(m.gates, g) }

// OnChanged subscribes to any change in a tab's state.
func (m *Manager) OnChanged(f func(Tab)) { m.onChanged = append(m.onChanged, f) }

// OnNavigationFinished subscribes to committed navigations.
func (m *Manager) OnNavigationFinished(f func(Tab)) { m.onFinished = append(m.onFinished, f) }

// OnTitleChanged subscribes to title changes.
func (m *Manager) OnTitleChanged(f func(Tab)) { m.onTitle = append(m.onTitle, f) }

// OnListChanged subscribes to a tab opening or closing, or a window's active tab changing.
func (m *Manager) OnListChanged(f func()) { m.onList = append(m.onList, f) }

// OnEmpty subscribes to a window's last tab closing.
func (m *Manager) OnEmpty(f func(engine.WindowID)) { m.onEmpty = append(m.onEmpty, f) }

// OnFrontChanged subscribes to another window coming to the front.
func (m *Manager) OnFrontChanged(f func(engine.WindowID)) { m.onFront = append(m.onFront, f) }

// OnMoved subscribes to a tab moving from its place in one window to another window.
func (m *Manager) OnMoved(f func(tab engine.TabID, from engine.WindowID, fromIndex int, to engine.WindowID)) {
	m.onMoved = append(m.onMoved, f)
}

// AddWindow adds a window with no tabs. It is the window in front.
func (m *Manager) AddWindow(w engine.WindowID) {
	m.windows[w] = &window{}
	m.toFront(w)
}

// toFront puts a window first in the order of use, and says so when it wasn't.
func (m *Manager) toFront(w engine.WindowID) {
	was := m.Front()
	m.used = slices.Insert(slices.DeleteFunc(m.used, func(u engine.WindowID) bool { return u == w }), 0, w)
	m.frontMaybeChanged(was)
}

func (m *Manager) frontMaybeChanged(was engine.WindowID) {
	if now := m.Front(); now != was && now != 0 {
		for _, f := range m.onFront {
			f(now)
		}
	}
}

// RemoveWindow closes a window's tabs and forgets it. The window used before it is in
// front now.
func (m *Manager) RemoveWindow(w engine.WindowID) {
	for _, id := range m.windows[w].order {
		delete(m.tabs, id)
		delete(m.windowOf, id)
		m.eng.CloseTab(id)
	}
	delete(m.windows, w)
	was := m.Front()
	m.used = slices.DeleteFunc(m.used, func(u engine.WindowID) bool { return u == w })
	m.listChanged()
	m.frontMaybeChanged(was)
}

// Focus says a window came to the front.
func (m *Manager) Focus(w engine.WindowID) {
	if m.windows[w] != nil {
		m.toFront(w)
	}
}

// Front is the window in front, the one used last, or 0 when there is none.
func (m *Manager) Front() engine.WindowID {
	if len(m.used) == 0 {
		return 0
	}
	return m.used[0]
}

// Windows returns the windows, the one in front first.
func (m *Manager) Windows() []engine.WindowID { return slices.Clone(m.used) }

// WindowOf is the window a tab is in, or 0 for no tab.
func (m *Manager) WindowOf(id engine.TabID) engine.WindowID { return m.windowOf[id] }

// front is the window in front's tabs.
func (m *Manager) front() *window { return m.windows[m.Front()] }

// Create is New Tab: it opens a new tab at the end of window w, on the blank page, in the
// workspace of w's active tab, or in the first one when w has none, and makes it active.
func (m *Manager) Create(w engine.WindowID) engine.TabID {
	if active, ok := m.Get(m.ActiveIn(w)); ok {
		return m.CreateIn(w, active.Workspace)
	}
	return m.CreateIn(w, m.first())
}

// FirstWorkspace is the first workspace in the profile's list, where a tab opens when
// nothing picks one.
func (m *Manager) FirstWorkspace() string { return m.first() }

// CreateIn opens a new tab at the end of window w, on the blank page, in a workspace, by
// its id, and makes it active.
func (m *Manager) CreateIn(w engine.WindowID, workspace string) engine.TabID {
	return m.Add(w, workspace, true)
}

// Add opens a new tab at the end of window w, on the blank page, in a workspace, and
// makes it active when foreground, or leaves it behind the active one.
func (m *Manager) Add(w engine.WindowID, workspace string, foreground bool) engine.TabID {
	id := m.create(w, workspace)
	m.shown(id, foreground)
	return id
}

// shown makes a new tab active, or tells of it behind the active one.
func (m *Manager) shown(id engine.TabID, foreground bool) {
	if foreground {
		m.Activate(id)
	} else {
		m.listChanged()
	}
}

// create opens a new tab at the end of window w, on the blank page, in a workspace.
func (m *Manager) create(w engine.WindowID, workspace string) engine.TabID {
	id := m.eng.CreateTab(workspace)
	m.add(id, w).Workspace = workspace
	m.eng.LoadURL(id, BlankURL)
	return id
}

// address is what a tab shows for a URL: nothing for the blank page.
func address(u string) string {
	if u == BlankURL {
		return ""
	}
	return u
}

func (m *Manager) add(id engine.TabID, w engine.WindowID) *Tab {
	t := &Tab{ID: id}
	m.tabs[id] = t
	m.windowOf[id] = w
	win := m.windows[w]
	win.order = append(win.order, id)
	win.recent = append(win.recent, id)
	return t
}

// Activate makes a tab the active one in its window.
func (m *Manager) Activate(id engine.TabID) {
	if m.tabs[id] == nil {
		return
	}
	win := m.windows[m.windowOf[id]]
	if id == win.active {
		return
	}
	win.active = id
	if win.cycle == nil {
		win.touch(id)
	}
	m.listChanged()
}

// touch makes a tab the most recent.
func (win *window) touch(id engine.TabID) {
	if i := slices.Index(win.recent, id); i >= 0 {
		win.recent = slices.Delete(win.recent, i, i+1)
	}
	win.recent = slices.Insert(win.recent, 0, id)
}

// CycleRecent shows the next tab back in recent order in the window in front, as the
// OS's app switcher does while its key is held: each press goes one further, past the
// oldest it comes round to the newest, and the order stays as it was until EndCycle.
// Reverse goes the other way.
func (m *Manager) CycleRecent(reverse bool) {
	win := m.front()
	if len(win.recent) < 2 {
		return
	}
	if win.cycle == nil {
		win.cycle = slices.Clone(win.recent)
		win.at = 0
	}
	n := len(win.cycle)
	if reverse {
		win.at = (win.at + n - 1) % n
	} else {
		win.at = (win.at + 1) % n
	}
	m.Activate(win.cycle[win.at])
}

// EndCycle ends a cycle through recent tabs: the tab it stopped on becomes the most recent.
func (m *Manager) EndCycle() {
	for _, win := range m.windows {
		win.endCycle()
	}
}

func (win *window) endCycle() {
	if win.cycle == nil {
		return
	}
	win.cycle = nil
	win.touch(win.active)
}

// ActivateAt shows the tab at a position in the window in front, counting from zero. Out
// of range does nothing.
func (m *Manager) ActivateAt(i int) {
	if win := m.front(); i >= 0 && i < len(win.order) {
		m.Activate(win.order[i])
	}
}

// ActivateLast shows the last tab of the window in front.
func (m *Manager) ActivateLast() { m.ActivateAt(len(m.front().order) - 1) }

// ActivateNext shows the tab after the active one in the window in front, or before it,
// wrapping around.
func (m *Manager) ActivateNext(reverse bool) {
	win := m.front()
	n := len(win.order)
	if n < 2 {
		return
	}
	i := slices.Index(win.order, win.active)
	if reverse {
		m.ActivateAt((i + n - 1) % n)
	} else {
		m.ActivateAt((i + 1) % n)
	}
}

// Close closes a tab. Closing the active one activates its right neighbour, or the
// left one at the end. Closing a window's last tab leaves it no tabs and fires OnEmpty.
func (m *Manager) Close(id engine.TabID) {
	if m.tabs[id] == nil {
		return
	}
	w := m.windowOf[id]
	m.leaveGroup(id)
	i := m.detach(id)
	delete(m.tabs, id)
	m.eng.CloseTab(id)
	m.left(w, id, i)
}

// Move moves a tab to window w, at index in its list, and makes it the active tab there.
// The tab keeps its id and its page goes on running. The window it left goes on as if the
// tab had closed there.
func (m *Manager) Move(id engine.TabID, w engine.WindowID, index int) {
	if m.tabs[id] == nil {
		return
	}
	from := m.windowOf[id]
	if from != w {
		m.leaveGroup(id)
	}
	i := m.detach(id)
	m.windowOf[id] = w
	win := m.windows[w]
	win.order = slices.Insert(win.order, index, id)
	win.recent = append(win.recent, id)
	win.active = id
	win.touch(id)
	for _, f := range m.onMoved {
		f(id, from, i, w)
	}
	m.left(from, id, i)
}

// detach takes a tab out of its window's lists, and says where it was in its order.
func (m *Manager) detach(id engine.TabID) int {
	win := m.windows[m.windowOf[id]]
	win.endCycle()
	i := slices.Index(win.order, id)
	delete(m.windowOf, id)
	win.order = slices.Delete(win.order, i, i+1)
	if j := slices.Index(win.recent, id); j >= 0 {
		win.recent = slices.Delete(win.recent, j, j+1)
	}
	return i
}

// left is what window w does once a tab at position i has left it: with no tabs it fires
// OnEmpty; if the tab was active, its right neighbour, or the left one at the end, is.
func (m *Manager) left(w engine.WindowID, id engine.TabID, i int) {
	win := m.windows[w]
	if len(win.order) == 0 {
		win.active = 0
		m.listChanged()
		for _, f := range m.onEmpty {
			f(w)
		}
		return
	}
	if id == win.active {
		win.active = 0
		m.Activate(win.order[min(i, len(win.order)-1)])
		return
	}
	m.listChanged()
}

// CloseActive closes the active tab of the window in front.
func (m *Manager) CloseActive() { m.Close(m.ActiveID()) }

// All returns copies of every tab, window by window in the order they were opened, each
// window's in order.
func (m *Manager) All() []Tab {
	ids := slices.Sorted(maps.Keys(m.windows))
	var out []Tab
	for _, w := range ids {
		out = append(out, m.AllIn(w)...)
	}
	return out
}

// AllIn returns copies of window w's tabs, in order.
func (m *Manager) AllIn(w engine.WindowID) []Tab {
	order := m.windows[w].order
	out := make([]Tab, len(order))
	for i, id := range order {
		out[i] = *m.tabs[id]
	}
	return out
}

// ActiveID returns the active tab of the window in front, or zero when there is none.
func (m *Manager) ActiveID() engine.TabID {
	if win := m.front(); win != nil {
		return win.active
	}
	return 0
}

// ActiveIn returns window w's active tab, or zero when it has none.
func (m *Manager) ActiveIn(w engine.WindowID) engine.TabID { return m.windows[w].active }

// Active returns a copy of the active tab of the window in front.
func (m *Manager) Active() Tab {
	if t, ok := m.tabs[m.ActiveID()]; ok {
		return *t
	}
	return Tab{}
}

// Navigate turns what the user typed into a URL and loads it in the active tab.
func (m *Manager) Navigate(text string) {
	u := NormalizeInput(text)
	if u == "" || m.tabs[m.ActiveID()] == nil {
		return
	}
	m.eng.LoadURL(m.ActiveID(), u)
}

// Back moves the active tab back in its history.
func (m *Manager) Back() {
	if t := m.tabs[m.ActiveID()]; t != nil && t.CanGoBack {
		m.eng.GoBack(m.ActiveID())
	}
}

// Forward moves the active tab forward in its history.
func (m *Manager) Forward() {
	if t := m.tabs[m.ActiveID()]; t != nil && t.CanGoForward {
		m.eng.GoForward(m.ActiveID())
	}
}

// Reload reloads the active tab.
func (m *Manager) Reload() {
	if t := m.tabs[m.ActiveID()]; t != nil && t.URL != "" {
		m.eng.Reload(m.ActiveID())
	}
}

// ReloadBypassingCache reloads the active tab without the cache.
func (m *Manager) ReloadBypassingCache() {
	if t := m.tabs[m.ActiveID()]; t != nil && t.URL != "" {
		m.eng.ReloadBypassingCache(m.ActiveID())
	}
}

// Stop stops the active tab loading.
func (m *Manager) Stop() {
	if t := m.tabs[m.ActiveID()]; t != nil && t.Loading {
		m.eng.Stop(m.ActiveID())
	}
}

// Duplicate opens a copy of the active tab, history and all, as the last tab, in its
// workspace, and shows it.
func (m *Manager) Duplicate() {
	t := m.tabs[m.ActiveID()]
	if t == nil {
		return
	}
	m.HandleTabOpened(m.ActiveID(), m.eng.DuplicateTab(m.ActiveID()), t.URL, t.Workspace, true)
}

// Open opens a URL in a new last tab of the window in front and shows it.
func (m *Manager) Open(u string) { m.OpenIn(u, true) }

// OpenIn opens a URL in a new last tab of the window in front, in the first workspace,
// shown or behind the active one.
func (m *Manager) OpenIn(u string, foreground bool) engine.TabID {
	return m.open(m.ActiveID(), u, m.first(), foreground)
}

// OpenFor opens a URL about a tab's page, such as its source, in a new last tab of that
// tab's window, in its workspace, and shows it. The URL loads the page again, so in another
// workspace it would load it without the tab's cookies.
func (m *Manager) OpenFor(tab engine.TabID, u string) {
	if t := m.tabs[tab]; t != nil {
		m.open(tab, u, t.Workspace, true)
	}
}

func (m *Manager) open(opener engine.TabID, u, workspace string, foreground bool) engine.TabID {
	id := m.eng.CreateTab(workspace)
	m.HandleTabOpened(opener, id, u, workspace, foreground)
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
	if m.tabs[m.ActiveID()] == nil {
		return
	}
	now := m.eng.ZoomOf(m.ActiveID())
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
	if m.tabs[m.ActiveID()] != nil {
		m.eng.Zoom(m.ActiveID(), f)
	}
}

// HandleTabOpened records a tab a page opened, in the opener's window, or in the window
// in front when no tab opened it, in the workspace the engine made it in, and in its
// opener's group, and shows it if the page asked for the foreground.
func (m *Manager) HandleTabOpened(opener, id engine.TabID, u, workspace string, foreground bool) {
	w := m.windowOf[opener]
	if w == 0 {
		w = m.Front()
	}
	t := m.add(id, w)
	t.URL, t.Workspace = address(u), workspace
	// A tab a page opens joins its opener's group, after it.
	if o := m.tabs[opener]; o != nil && o.Group != "" {
		m.join(id, o.Group, opener)
	}
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
