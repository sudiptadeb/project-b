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
	shown  engine.TabID
	closed []engine.TabID
}

func (f *fakeEngine) CreateTab() engine.TabID      { f.next++; return f.next }
func (f *fakeEngine) CloseTab(id engine.TabID)     { f.closed = append(f.closed, id) }
func (f *fakeEngine) ShowTab(id engine.TabID)      { f.shown = id }
func (f *fakeEngine) LoadURL(engine.TabID, string) {}

func ids(tabs []Tab) []engine.TabID {
	out := make([]engine.TabID, len(tabs))
	for i, t := range tabs {
		out[i] = t.ID
	}
	return out
}

func TestCreateCloseAndActivate(t *testing.T) {
	e := &fakeEngine{}
	m := New(e)
	lists := 0
	m.OnListChanged(func() { lists++ })

	a, b, c := m.Create(), m.Create(), m.Create()
	if m.ActiveID() != c || e.shown != c || !slices.Equal(ids(m.All()), []engine.TabID{a, b, c}) {
		t.Fatalf("after create: active %v shown %v all %v", m.ActiveID(), e.shown, ids(m.All()))
	}

	m.Activate(a)
	if m.ActiveID() != a || e.shown != a {
		t.Fatalf("activate: active %v shown %v", m.ActiveID(), e.shown)
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
	m := New(e)
	empty := false
	m.OnEmpty(func() { empty = true })
	m.Create()
	m.CloseActive()
	if !empty || len(m.All()) != 0 || m.ActiveID() != 0 {
		t.Fatalf("empty %v all %v active %v", empty, m.All(), m.ActiveID())
	}
}

func TestATabAPageOpensIsShownOnlyInTheForeground(t *testing.T) {
	e := &fakeEngine{}
	m := New(e)
	a := m.Create()
	m.HandleTabOpened(a, 7, "https://b.test/", false)
	if m.ActiveID() != a || m.All()[1].URL != "https://b.test/" {
		t.Fatalf("background open: active %v all %v", m.ActiveID(), m.All())
	}
	m.HandleTabOpened(a, 8, "https://c.test/", true)
	if m.ActiveID() != 8 || e.shown != 8 {
		t.Fatalf("foreground open: active %v shown %v", m.ActiveID(), e.shown)
	}
}

func TestFaviconFollowsThePage(t *testing.T) {
	m := New(&fakeEngine{})
	id := m.Create()
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
	m := New(e)
	id := m.Create()
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
