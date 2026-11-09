// Package mockengine is a browser engine with no browser in it. Its page is a set of
// buttons that fire the events real content would: navigation, title, failure, crash,
// a page opening a tab, a shortcut, full screen.
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

// Controls is where the mock's window controls sit: what macOS gives a window with a
// 32px title bar. Full screen hides them.
var Controls = engine.Rect{X: 9, Y: 9, W: 60, H: 14}

// Shortcuts are the names the mock page uses for engine.Shortcut.
var Shortcuts = map[string]engine.Shortcut{
	"focusAddressBar": engine.ShortcutFocusAddressBar,
	"newTab":          engine.ShortcutNewTab,
	"closeTab":        engine.ShortcutCloseTab,
	"nextTab":         engine.ShortcutNextTab,
	"previousTab":     engine.ShortcutPreviousTab,
	"tab1":            engine.ShortcutTab1,
	"tab2":            engine.ShortcutTab1 + 1,
	"tab3":            engine.ShortcutTab1 + 2,
	"tab9":            engine.ShortcutTab1 + 8,
}

// tab is one mock tab's session history and what its page shows.
type tab struct {
	history []string // The committed URLs.
	pos     int      // The current one in history, or -1 before the first.
	navSeq  int
	state   string // What the mock page shows: empty, loading, loaded, blocked, crashed.
	current string
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
	mu      sync.Mutex
	regions []engine.Rect
	drag    []engine.Rect
	bounds  engine.Rect
	closed  bool
	hidden  []engine.TabID // Tabs told they are hidden since the last ShowTab.

	Shell Feed // Messages to the shell page.
	Page  Feed // State for the mock page.
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

// SetListener sets who hears the engine's events, and reports the window controls to
// it, as the real engine does right after it starts.
func (e *Engine) SetListener(l engine.Listener) {
	e.listener = l
	e.reportControls()
}

func (e *Engine) reportControls() {
	if e.full {
		e.listener.WindowControlsChanged(engine.Rect{})
	} else {
		e.listener.WindowControlsChanged(Controls)
	}
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

func (e *Engine) CloseWindow() {
	e.mu.Lock()
	e.closed = true
	e.mu.Unlock()
	e.publishPage(0, "closed", "")
}

func (e *Engine) SetPageBounds(r engine.Rect, viewW, viewH float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bounds = r
}

func (e *Engine) SetShellRegions(rects []engine.Rect) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.regions = rects
}

func (e *Engine) SetDragRegions(rects []engine.Rect) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.drag = rects
}

func (e *Engine) FocusPage() {}

func (e *Engine) FocusShell() {}

func (e *Engine) SendToShell(msg []byte) { e.Shell.Publish(msg) }

func (e *Engine) Post(fn func()) { e.queue <- fn }

// Regions returns the shell's rectangles, the drag rectangles and the page's bounds.
func (e *Engine) Regions() (shell, drag []engine.Rect, bounds engine.Rect) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.Rect(nil), e.regions...), append([]engine.Rect(nil), e.drag...), e.bounds
}

// Hit answers what the real engine's hit test answers for a point: "drag", "shell" or
// "page".
func (e *Engine) Hit(x, y float64) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range e.drag {
		if r.Contains(x, y) {
			return "drag"
		}
	}
	for _, r := range e.regions {
		if r.Contains(x, y) {
			return "shell"
		}
	}
	return "page"
}

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
		case "shortcut":
			if s, ok := Shortcuts[value]; ok {
				e.listener.Shortcut(s)
			}
		case "fullscreen":
			e.full = !e.full
			e.reportControls()
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
			e.listener.TitleChanged(id, "Mock "+hostOf(u))
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
