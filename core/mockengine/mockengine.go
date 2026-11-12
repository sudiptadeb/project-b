// Package mockengine is a browser engine with no browser in it. Its page is a set of
// buttons that fire the events real content would: navigation, title, failure, crash,
// a page opening a tab, a key press, full screen.
package mockengine

import (
	"encoding/json"
	"net/url"
	"sync"
	"time"

	"projectb/engine"
)

// loadDelay is how long a mock page takes to load.
const loadDelay = 100 * time.Millisecond

// Controls is where the mock's window controls sit: what macOS 26 gives a window with
// a unified toolbar, whose buttons centre 26px down. Full screen hides them.
var Controls = engine.Rect{X: 19, Y: 19, W: 60, H: 14}

// CornerRadius is what macOS 26 rounds that window's corners with. Full screen has none.
const CornerRadius = 26.0

// Width and Height are the mock window's content size, the real window's first size.
const (
	Width  = 1100.0
	Height = 700.0
)

// Keys are the names the mock page uses for key presses, as a Mac keyboard has them.
var Keys = map[string]engine.Key{
	"focusAddressBar": {Code: 'L', Mods: engine.ModCmd},
	"newTab":          {Code: 'T', Mods: engine.ModCmd},
	"closeTab":        {Code: 'W', Mods: engine.ModCmd},
	"nextTab":         {Code: 0xDD, Mods: engine.ModCmd | engine.ModShift},
	"previousTab":     {Code: 0xDB, Mods: engine.ModCmd | engine.ModShift},
	"recentTab":       {Code: 0x09, Mods: engine.ModCtrl},
	"recentTabBack":   {Code: 0x09, Mods: engine.ModCtrl | engine.ModShift},
	"tab1":            {Code: '1', Mods: engine.ModCmd},
	"tab2":            {Code: '2', Mods: engine.ModCmd},
	"tab3":            {Code: '3', Mods: engine.ModCmd},
	"tab9":            {Code: '9', Mods: engine.ModCmd},
	"settings":        {Code: 0xBC, Mods: engine.ModCmd},
}

// tab is one mock tab's session history and what its page shows.
type tab struct {
	history []string // The committed URLs.
	pos     int      // The current one in history, or -1 before the first.
	navSeq  int
	state   string // What the mock page shows: empty, loading, loaded, blocked, crashed.
	current string
	zoom    float64
}

// Engine implements engine.Engine. Run is the main thread: everything happens there,
// as it does with real content.
type Engine struct {
	queue    chan func()
	listener engine.Listener
	nextTab  engine.TabID
	tabs     map[engine.TabID]*tab
	shown    engine.TabID
	full     bool // Full screen, so there are no window controls.

	// Written on the main thread, read by the dev server's goroutines.
	mu         sync.Mutex
	layout     engine.WindowLayout
	requests   []engine.WindowRequest
	closed     bool
	hidden     []engine.TabID // Tabs told they are hidden since the last ShowTab.
	shownColor string
	settings   bool
	menu       engine.Menu
	edits      []engine.EditCommand

	// The fake extension runtime, in a build with extensions.
	extensions extensionRuntime

	Settings Feed // Messages to the settings page.

	Shell Feed // Messages to the shell page.
	Page  Feed // State for the mock page.

	// What the dev window draws over the shell, where the real window has views of its
	// own: the settings page and an extension's popup. Each message is WindowState's.
	Window Feed
}

// New returns an engine. Call Run to start its main thread.
func New() *Engine {
	return &Engine{queue: make(chan func(), 4096), tabs: map[engine.TabID]*tab{}}
}

// Run is the main loop. It never returns.
func (e *Engine) Run() {
	for fn := range e.queue {
		fn()
	}
}

// SetListener sets who hears the engine's events, and reports the window to it, as the
// real engine does right after it starts. That is also when the real engine puts its
// window on screen, so the colour the window has now is its first frame's.
func (e *Engine) SetListener(l engine.Listener) {
	e.mu.Lock()
	e.shownColor = e.layout.FrameColor
	e.mu.Unlock()
	e.listener = l
	e.reportWindow()
}

func (e *Engine) reportWindow() {
	s := engine.WindowState{W: Width, H: Height, FullScreen: e.full}
	if !e.full {
		s.Controls = Controls
		s.CornerRadius = CornerRadius
	}
	e.listener.WindowChanged(s)
}

func (e *Engine) Name() string { return "mock" }

func (e *Engine) CreateTab() engine.TabID {
	e.nextTab++
	e.tabs[e.nextTab] = &tab{pos: -1, state: "empty"}
	return e.nextTab
}

func (e *Engine) CloseTab(id engine.TabID) {
	delete(e.tabs, id)
	if e.shown == id {
		e.shown = 0
		e.publishPage(0, "none", "")
	}
}

func (e *Engine) ShowTab(id engine.TabID) {
	t := e.tabs[id]
	if t == nil {
		return
	}
	e.mu.Lock()
	e.hidden = e.hidden[:0]
	for other := range e.tabs {
		if other != id {
			e.hidden = append(e.hidden, other)
		}
	}
	e.mu.Unlock()
	e.shown = id
	e.publishPage(id, t.state, t.current)
}

func (e *Engine) LoadURL(id engine.TabID, u string) { e.navigate(id, u) }

func (e *Engine) GoBack(id engine.TabID) {
	if t := e.tabs[id]; t != nil && t.pos > 0 {
		e.moveTo(id, t.pos-1)
	}
}

func (e *Engine) GoForward(id engine.TabID) {
	if t := e.tabs[id]; t != nil && t.pos < len(t.history)-1 {
		e.moveTo(id, t.pos+1)
	}
}

func (e *Engine) Reload(id engine.TabID) {
	if t := e.tabs[id]; t != nil && t.pos >= 0 {
		e.moveTo(id, t.pos)
	}
}

// RequestWindow closes the window, goes in or out of full screen, and records every
// request. A mock window has nothing to minimise or zoom.
func (e *Engine) RequestWindow(r engine.WindowRequest) {
	e.mu.Lock()
	e.requests = append(e.requests, r)
	e.closed = e.closed || r == engine.WindowClose
	e.mu.Unlock()
	switch r {
	case engine.WindowClose:
		e.publishPage(0, "closed", "")
	case engine.WindowFullScreen:
		e.full = !e.full
		e.reportWindow()
	}
}

func (e *Engine) SetWindowLayout(l engine.WindowLayout) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.layout = l
}

// Layout is the last layout the app gave the window.
func (e *Engine) Layout() engine.WindowLayout {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.layout
}

// Requests are the window requests the app made, in order.
func (e *Engine) Requests() []engine.WindowRequest {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.WindowRequest(nil), e.requests...)
}

// ShownColor is the colour the window painted when it went on screen.
func (e *Engine) ShownColor() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.shownColor
}

func (e *Engine) ReloadBypassingCache(id engine.TabID) { e.Reload(id) }

func (e *Engine) Stop(id engine.TabID) {
	if t := e.tabs[id]; t != nil && t.state == "loading" {
		t.navSeq++
		t.state = "loaded"
		e.listener.LoadingChanged(id, false)
	}
}

func (e *Engine) Zoom(id engine.TabID, factor float64) {
	if t := e.tabs[id]; t != nil {
		t.zoom = factor
	}
}

func (e *Engine) ZoomOf(id engine.TabID) float64 {
	if t := e.tabs[id]; t != nil && t.zoom != 0 {
		return t.zoom
	}
	return 1
}

// DuplicateTab copies the tab's history into a new hidden tab.
func (e *Engine) DuplicateTab(id engine.TabID) engine.TabID {
	n := e.CreateTab()
	if t := e.tabs[id]; t != nil {
		c := *t
		c.history = append([]string(nil), t.history...)
		e.tabs[n] = &c
	}
	return n
}

func (e *Engine) Edit(c engine.EditCommand) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.edits = append(e.edits, c)
}

// Edits are the editing commands the app ran, in order.
func (e *Engine) Edits() []engine.EditCommand {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.EditCommand(nil), e.edits...)
}

// The real window puts the settings page in a view over everything. The mock says so on
// its Window feed, and the dev window draws the page there.
func (e *Engine) OpenSettings() {
	e.mu.Lock()
	e.settings = true
	e.mu.Unlock()
	e.Window.Publish(e.WindowState())
}

func (e *Engine) CloseSettings() {
	e.mu.Lock()
	e.settings = false
	e.mu.Unlock()
	e.Settings.Publish([]byte(`{"t":"closed"}`))
	e.Window.Publish(e.WindowState())
}

// WindowState is what goes over the shell now, as the Window feed sends it:
// {"t":"window","settings":open,"popup":the extension popup or null}. Any goroutine.
func (e *Engine) WindowState() []byte {
	e.mu.Lock()
	settings := e.settings
	e.mu.Unlock()
	b, _ := json.Marshal(map[string]any{"t": "window", "settings": settings, "popup": e.openPopup()})
	return b
}

// SettingsOpen says whether the app has the settings page open.
func (e *Engine) SettingsOpen() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.settings
}

func (e *Engine) SendToSettings(msg []byte) { e.Settings.Publish(msg) }

func (e *Engine) SetMenu(m engine.Menu) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.menu = m
}

// Menu is the last menu the app gave the window.
func (e *Engine) Menu() engine.Menu {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.menu
}

func (e *Engine) AppCommand(engine.AppCommand) {}

func (e *Engine) FocusPage() {}

func (e *Engine) FocusShell() {}

func (e *Engine) SendToShell(msg []byte) { e.Shell.Publish(msg) }

func (e *Engine) Post(fn func()) { e.queue <- fn }

// Closed reports whether the app closed its window.
func (e *Engine) Closed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.closed
}

// Hidden returns the tabs told they are hidden by the last ShowTab.
func (e *Engine) Hidden() []engine.TabID {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.TabID(nil), e.hidden...)
}

// Fire makes the engine do what a button on the mock page says, to the shown tab. Any
// goroutine.
func (e *Engine) Fire(kind, value string) {
	e.Post(func() {
		t := e.tabs[e.shown]
		switch kind {
		case "navigate":
			e.navigate(e.shown, value)
		case "title":
			e.listener.TitleChanged(e.shown, value)
		case "fail":
			if t != nil {
				t.navSeq++
				e.listener.LoadingChanged(e.shown, false)
				e.listener.NavigationFailed(e.shown, t.current, value)
			}
		case "crash":
			if t != nil {
				t.navSeq++
				t.state = "crashed"
				e.listener.PageCrashed(e.shown)
				e.publishPage(e.shown, t.state, t.current)
			}
		case "favicon":
			e.listener.FaviconChanged(e.shown, value)
		case "open":
			// What a page's target=_blank or window.open does: the engine makes the
			// tab, tells the app, then loads it.
			id := e.CreateTab()
			e.listener.TabOpened(e.shown, id, value, true)
			e.navigate(id, value)
		case "key":
			// A key pressed in the page, which uses none of them.
			if k, ok := Keys[value]; ok && !e.listener.Key(k, engine.KeyBeforePage, engine.SurfacePage) {
				e.listener.Key(k, engine.KeyUnhandled, engine.SurfacePage)
			}
		case "release":
			// Every modifier let go.
			e.listener.ModsChanged(0)
		case "fullscreen":
			// The system's own button, which asks no one.
			e.full = !e.full
			e.reportWindow()
		}
	})
}

// navigate is a new page: it replaces whatever was ahead of the current one.
func (e *Engine) navigate(id engine.TabID, u string) {
	e.load(id, u, func(t *tab) {
		t.history = append(t.history[:t.pos+1], u)
		t.pos = len(t.history) - 1
	})
}

// moveTo loads a page already in the history.
func (e *Engine) moveTo(id engine.TabID, pos int) {
	e.load(id, e.tabs[id].history[pos], func(t *tab) { t.pos = pos })
}

// load behaves like a real load: gates first, then later the commit, unless a newer
// navigation replaced it.
func (e *Engine) load(id engine.TabID, u string, commit func(*tab)) {
	t := e.tabs[id]
	if t == nil {
		return
	}
	// A load asked for while the app is still starting begins once it listens.
	if e.listener == nil {
		e.Post(func() { e.load(id, u, commit) })
		return
	}
	t.navSeq++
	seq := t.navSeq
	if !e.listener.NavigationStarting(id, u) {
		t.state = "blocked"
		e.publishPage(id, t.state, u)
		return
	}
	e.listener.LoadingChanged(id, true)
	e.listener.LoadProgressChanged(id, 0.5)
	t.state = "loading"
	e.publishPage(id, t.state, u)
	time.AfterFunc(loadDelay, func() {
		e.Post(func() {
			if e.tabs[id] != t || seq != t.navSeq {
				return
			}
			commit(t)
			t.current = u
			t.state = "loaded"
			e.listener.HistoryChanged(id, t.pos > 0, t.pos < len(t.history)-1)
			e.listener.NavigationFinished(id, u)
			e.listener.TitleChanged(id, titleOf(u))
			e.listener.LoadProgressChanged(id, 1)
			e.listener.LoadingChanged(id, false)
			e.publishPage(id, t.state, u)
		})
	})
}

// publishPage tells the mock page what to show. The page shows the shown tab only.
func (e *Engine) publishPage(id engine.TabID, state, u string) {
	if id != e.shown && id != 0 {
		return
	}
	b, _ := json.Marshal(map[string]any{"tab": id, "state": state, "url": u})
	e.Page.Publish(b)
}

// titleOf is a mock page's title. The blank page has none, so content titles it with
// its address, as it does any untitled page.
func titleOf(u string) string {
	if u == "about:blank" {
		return u
	}
	return "Mock " + hostOf(u)
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

// Feed sends messages to whoever is connected right now.
type Feed struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

// Subscribe returns a channel of messages and a function to stop.
func (f *Feed) Subscribe() (<-chan []byte, func()) {
	c := make(chan []byte, 256)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.subs == nil {
		f.subs = map[chan []byte]struct{}{}
	}
	f.subs[c] = struct{}{}
	return c, func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		delete(f.subs, c)
	}
}

// Publish sends to everyone connected and never blocks the main thread.
func (f *Feed) Publish(msg []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for c := range f.subs {
		select {
		case c <- msg:
		default:
		}
	}
}
