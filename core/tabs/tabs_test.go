package tabs

import (
	"slices"
	"testing"

	"projectb/engine"
)

// fakeEngine records what the manager asks of it.
type fakeEngine struct {
	engine.Engine
	next   engine.TabID
	closed []engine.TabID
	// in is the workspace each tab was made in, by its id.
	in map[engine.TabID]string
}

func (f *fakeEngine) CreateTab(workspace string) engine.TabID {
	f.next++
	if f.in == nil {
		f.in = map[engine.TabID]string{}
	}
	f.in[f.next] = workspace
	return f.next
}

func (f *fakeEngine) DuplicateTab(engine.TabID) engine.TabID { f.next++; return f.next }
func (f *fakeEngine) WriteClipboard(string)                  {}
func (f *fakeEngine) CloseTab(id engine.TabID)               { f.closed = append(f.closed, id) }
func (f *fakeEngine) LoadURL(engine.TabID, string)           {}

// work is the first workspace in the tests' list.
const work = "0a1b2c3d"

// oneWindow is a manager with window 1, whose list has work first.
func oneWindow(e engine.Engine) *Manager {
	m := New(e, func() string { return work })
	m.AddWindow(1)
	return m
}

func ids(tabs []Tab) []engine.TabID {
	out := make([]engine.TabID, len(tabs))
	for i, t := range tabs {
		out[i] = t.ID
	}
	return out
}

func TestCreateCloseAndActivate(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	lists := 0
	m.OnListChanged(func() { lists++ })

	a, b, c := m.Create(1), m.Create(1), m.Create(1)
	if m.ActiveID() != c || !slices.Equal(ids(m.All()), []engine.TabID{a, b, c}) {
		t.Fatalf("after create: active %v all %v", m.ActiveID(), ids(m.All()))
	}

	m.Activate(a)
	if m.ActiveID() != a {
		t.Fatalf("activate: active %v", m.ActiveID())
	}
	m.ActivateNext(false)
	m.ActivateNext(false)
	m.ActivateNext(false)
	if m.ActiveID() != a {
		t.Fatalf("next wraps: active %v", m.ActiveID())
	}
	m.ActivateNext(true)
	if m.ActiveID() != c {
		t.Fatalf("previous wraps: active %v", m.ActiveID())
	}
	m.ActivateAt(1)
	if m.ActiveID() != b {
		t.Fatalf("at: active %v", m.ActiveID())
	}

	// Closing the active tab moves to its right neighbour; closing the last one, left.
	m.Close(b)
	if m.ActiveID() != c || !slices.Equal(e.closed, []engine.TabID{b}) {
		t.Fatalf("close active: active %v closed %v", m.ActiveID(), e.closed)
	}
	m.Close(c)
	if m.ActiveID() != a {
		t.Fatalf("close last in order: active %v", m.ActiveID())
	}
	if lists == 0 {
		t.Fatal("list changes never announced")
	}
}

func TestClosingTheLastTabEmptiesTheList(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	empty := false
	m.OnEmpty(func(w engine.WindowID) { empty = w == 1 })
	m.Create(1)
	m.CloseActive()
	if !empty || len(m.All()) != 0 || m.ActiveID() != 0 {
		t.Fatalf("empty %v all %v active %v", empty, m.All(), m.ActiveID())
	}
}

func TestATabAPageOpensIsShownOnlyInTheForeground(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	a := m.Create(1)
	m.HandleTabOpened(a, 7, "https://b.test/", "", false)
	if m.ActiveID() != a || m.All()[1].URL != "https://b.test/" {
		t.Fatalf("background open: active %v all %v", m.ActiveID(), m.All())
	}
	m.HandleTabOpened(a, 8, "https://c.test/", "", true)
	if m.ActiveID() != 8 {
		t.Fatalf("foreground open: active %v", m.ActiveID())
	}
}

func TestFaviconFollowsThePage(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	id := m.Create(1)
	m.HandleNavigationStarting(id, "https://a.test/")
	m.HandleFaviconChanged(id, "data:image/png;base64,AAAA")
	if m.Active().Favicon == "" {
		t.Fatal("favicon not recorded")
	}
	m.HandleNavigationStarting(id, "https://b.test/")
	if m.Active().Favicon != "" {
		t.Fatal("a new page keeps the old page's favicon")
	}
}

func TestNormalizeInput(t *testing.T) {
	for in, want := range map[string]string{
		"":                      "",
		"  example.com ":        "https://example.com",
		"example.com:8080/a":    "https://example.com:8080/a",
		"https://example.com/a": "https://example.com/a",
		"http://example.com":    "http://example.com",
		"localhost:3000":        "http://localhost:3000",
		"about:blank":           "about:blank",
		"how do tabs work":      "https://duckduckgo.com/?q=how+do+tabs+work",
		"tabs":                  "https://duckduckgo.com/?q=tabs",
	} {
		if got := NormalizeInput(in); got != want {
			t.Errorf("NormalizeInput(%q) = %q, want %q", in, got, want)
		}
	}
}

// loads records what the manager asks the engine to load.
type loads struct {
	fakeEngine
	urls []string
}

func (l *loads) LoadURL(_ engine.TabID, u string) { l.urls = append(l.urls, u) }

func TestANewTabHoldsTheBlankPageAndReadsAsEmpty(t *testing.T) {
	e := &loads{}
	m := oneWindow(e)
	id := m.Create(1)
	if !slices.Equal(e.urls, []string{BlankURL}) {
		t.Fatalf("loaded %v", e.urls)
	}
	m.HandleNavigationStarting(id, BlankURL)
	m.HandleNavigationFinished(id, BlankURL)
	m.HandleTitleChanged(id, BlankURL) // Content's title for an untitled page.
	if tab := m.Active(); tab.URL != "" || tab.Title != "" {
		t.Fatalf("the blank page shows as url %q title %q", tab.URL, tab.Title)
	}
	m.HandleNavigationStarting(id, "https://example.com/")
	m.HandleNavigationFinished(id, "https://example.com/")
	m.HandleTitleChanged(id, "Example Domain")
	if tab := m.Active(); tab.URL != "https://example.com/" || tab.Title != "Example Domain" {
		t.Fatalf("a real page shows as url %q title %q", tab.URL, tab.Title)
	}
}

// Each window has its own tabs, its own active tab and its own recent order; commands act
// on the window in front, and a tab a page opens goes in its opener's window.
func TestWindowsKeepTheirOwnTabs(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	a, b := m.Create(1), m.Create(1)
	m.AddWindow(2)
	if m.Front() != 2 || m.ActiveID() != 0 {
		t.Fatalf("a new window is in front, with no tabs: front %v active %v", m.Front(), m.ActiveID())
	}
	c := m.Create(2)
	if m.WindowOf(c) != 2 || !slices.Equal(ids(m.AllIn(2)), []engine.TabID{c}) || !slices.Equal(ids(m.AllIn(1)), []engine.TabID{a, b}) {
		t.Fatalf("window 1 %v, window 2 %v", ids(m.AllIn(1)), ids(m.AllIn(2)))
	}
	if m.ActiveIn(1) != b || m.ActiveID() != c {
		t.Fatalf("active: window 1 %v, in front %v", m.ActiveIn(1), m.ActiveID())
	}

	// Next tab and Ctrl+Tab stay within the window in front.
	m.ActivateNext(false)
	m.CycleRecent(false)
	if m.ActiveID() != c || m.ActiveIn(1) != b {
		t.Fatalf("a window of one tab cycles to itself: %v, window 1 %v", m.ActiveID(), m.ActiveIn(1))
	}
	m.Focus(1)
	m.ActivateNext(false)
	if m.Front() != 1 || m.ActiveID() != a || m.ActiveIn(2) != c {
		t.Fatalf("after window 1 came to the front: front %v active %v, window 2 %v", m.Front(), m.ActiveID(), m.ActiveIn(2))
	}

	// A page in window 2 opens a tab: it goes in window 2, though window 1 is in front.
	m.HandleTabOpened(c, 9, "https://p.test/", "", true)
	if m.WindowOf(9) != 2 || m.ActiveIn(2) != 9 || m.ActiveID() != a {
		t.Fatalf("opened tab in window %v, window 2 active %v, in front %v", m.WindowOf(9), m.ActiveIn(2), m.ActiveID())
	}
	// One no page opened goes in the window in front.
	m.HandleTabOpened(0, 10, "https://q.test/", "", true)
	if m.WindowOf(10) != 1 {
		t.Fatalf("tab with no opener in window %v", m.WindowOf(10))
	}

	// Closing window 2's last tab empties window 2 only.
	var emptied []engine.WindowID
	m.OnEmpty(func(w engine.WindowID) { emptied = append(emptied, w) })
	m.Close(9)
	m.Close(c)
	if !slices.Equal(emptied, []engine.WindowID{2}) || len(m.AllIn(1)) != 3 {
		t.Fatalf("emptied %v, window 1 %v", emptied, ids(m.AllIn(1)))
	}
}

// Removing a window closes its tabs, and the window used before it is in front again.
func TestRemovingAWindowClosesItsTabs(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	a := m.Create(1)
	m.AddWindow(2)
	b, c := m.Create(2), m.Create(2)
	m.RemoveWindow(2)
	if m.Front() != 1 || m.ActiveID() != a || !slices.Equal(m.Windows(), []engine.WindowID{1}) {
		t.Fatalf("front %v active %v windows %v", m.Front(), m.ActiveID(), m.Windows())
	}
	if !slices.Equal(e.closed, []engine.TabID{b, c}) || len(m.All()) != 1 || m.WindowOf(b) != 0 {
		t.Fatalf("closed %v, all %v", e.closed, ids(m.All()))
	}
}

// A tab moves to a place in another window's list, keeps its id and is active there; the
// window it left goes on as if it had closed there, and one left empty is emptied.
func TestMovingATabBetweenWindows(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	a, b := m.Create(1), m.Create(1)
	m.AddWindow(2)
	c := m.Create(2)
	var emptied []engine.WindowID
	m.OnEmpty(func(w engine.WindowID) { emptied = append(emptied, w) })

	m.Move(b, 2, 0)
	if m.WindowOf(b) != 2 || !slices.Equal(ids(m.AllIn(2)), []engine.TabID{b, c}) || m.ActiveIn(2) != b {
		t.Fatalf("window 2 %v active %v", ids(m.AllIn(2)), m.ActiveIn(2))
	}
	if !slices.Equal(ids(m.AllIn(1)), []engine.TabID{a}) || m.ActiveIn(1) != a || len(e.closed) != 0 {
		t.Fatalf("window 1 %v active %v, closed %v", ids(m.AllIn(1)), m.ActiveIn(1), e.closed)
	}
	m.Move(a, 2, 2)
	if !slices.Equal(ids(m.AllIn(2)), []engine.TabID{b, c, a}) || !slices.Equal(emptied, []engine.WindowID{1}) || len(e.closed) != 0 {
		t.Fatalf("window 2 %v, emptied %v, closed %v", ids(m.AllIn(2)), emptied, e.closed)
	}
}

// A tab is in the workspace it was made in for its life: a new one, one a page opened, a
// copy, and one moved to another window.
func TestATabKeepsItsWorkspace(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	plain := m.CreateIn(1, "")
	work := m.CreateIn(1, "0a1b2c3d")
	m.HandleTabOpened(work, 9, "https://p.test/", "0a1b2c3d", true)
	m.Duplicate()
	workspace := func(id engine.TabID) string { t, _ := m.Get(id); return t.Workspace }
	got := []string{workspace(plain), workspace(work), workspace(9), workspace(m.ActiveID())}
	if !slices.Equal(got, []string{"", "0a1b2c3d", "0a1b2c3d", "0a1b2c3d"}) || m.ActiveID() == 9 {
		t.Fatalf("workspaces %q, the copy %v", got, m.ActiveID())
	}
	m.AddWindow(2)
	m.Move(work, 2, 0)
	if m.WindowOf(work) != 2 || workspace(work) != "0a1b2c3d" || m.AllIn(2)[0].Workspace != "0a1b2c3d" {
		t.Fatalf("after the move: window %v, %+v", m.WindowOf(work), m.AllIn(2))
	}
}

// New Tab opens in the workspace of its window's active tab, and in the first one in a
// window with no tab.
func TestANewTabTakesTheActiveTabsWorkspace(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	workspace := func(id engine.TabID) string { t, _ := m.Get(id); return t.Workspace }
	first := m.Create(1)
	main := m.CreateIn(1, "")
	fromMain := m.Create(1)
	m.Activate(first)
	fromFirst := m.Create(1)
	m.AddWindow(2)
	inNewWindow := m.Create(2)
	got := []string{workspace(first), workspace(main), workspace(fromMain), workspace(fromFirst), workspace(inNewWindow)}
	if !slices.Equal(got, []string{work, "", "", work, work}) {
		t.Fatalf("workspaces %q", got)
	}
}

// A URL opened with no tab to take a workspace from, as a link from another app is, opens
// in the first workspace, whichever tab is active.
func TestAURLOpensInTheFirstWorkspace(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	m.CreateIn(1, "")
	id := m.OpenIn("https://p.test/", true)
	if tab, _ := m.Get(id); tab.Workspace != work || e.in[id] != work || tab.URL != "https://p.test/" {
		t.Fatalf("the tab is %+v, in %q in the engine", tab, e.in[id])
	}
}
