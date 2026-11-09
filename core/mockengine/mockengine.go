// Package mockengine is a browser engine with no browser in it. Its page is a set of
// buttons that fire the events real content would: navigation, title, failure, crash.
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

// Engine implements engine.Engine. Run is the main thread: everything happens there,
// as it does with real content.
type Engine struct {
	queue    chan func()
	listener engine.Listener
	nextTab  engine.TabID
	tab      engine.TabID
	navSeq   int
	history  []string // The committed URLs of the one tab.
	pos      int      // The current one in history, or -1 before the first.
	full     bool     // Full screen, so there are no window controls.

	// Written on the main thread, read by the dev server's goroutines.
	mu      sync.Mutex
	regions []engine.Rect
	bounds  engine.Rect
	current string

	Shell Feed // Messages to the shell page.
	Page  Feed // State for the mock page.
}

// New returns an engine. Call Run to start its main thread.
func New() *Engine { return &Engine{queue: make(chan func(), 4096), pos: -1} }

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
	e.tab = e.nextTab
	return e.tab
}

func (e *Engine) LoadURL(tab engine.TabID, u string) { e.navigate(tab, u) }

func (e *Engine) GoBack(tab engine.TabID) {
	if e.pos > 0 {
		e.moveTo(tab, e.pos-1)
	}
}

func (e *Engine) GoForward(tab engine.TabID) {
	if e.pos < len(e.history)-1 {
		e.moveTo(tab, e.pos+1)
	}
}

func (e *Engine) Reload(tab engine.TabID) {
	if e.pos >= 0 {
		e.moveTo(tab, e.pos)
	}
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

func (e *Engine) FocusPage() {}

func (e *Engine) SendToShell(msg []byte) { e.Shell.Publish(msg) }

func (e *Engine) Post(fn func()) { e.queue <- fn }

// Regions returns the shell's rectangles and the page's bounds.
func (e *Engine) Regions() ([]engine.Rect, engine.Rect) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.Rect(nil), e.regions...), e.bounds
}

// HitShell answers what the real engine's hit test answers.
func (e *Engine) HitShell(x, y float64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range e.regions {
		if r.Contains(x, y) {
			return true
		}
	}
	return false
}

// Fire makes the engine do what a button on the mock page says. Any goroutine.
func (e *Engine) Fire(kind, value string) {
	e.Post(func() {
		switch kind {
		case "navigate":
			e.navigate(e.tab, value)
		case "title":
			e.listener.TitleChanged(e.tab, value)
		case "fail":
			e.navSeq++
			e.listener.LoadingChanged(e.tab, false)
			e.listener.NavigationFailed(e.tab, e.currentURL(), value)
		case "crash":
			e.navSeq++
			e.listener.PageCrashed(e.tab)
			e.publishPage("crashed", e.currentURL())
		case "shortcut":
			e.listener.FocusAddressBarRequested()
		case "fullscreen":
			e.full = !e.full
			e.reportControls()
		}
	})
}

// navigate is a new page: it replaces whatever was ahead of the current one.
func (e *Engine) navigate(tab engine.TabID, u string) {
	e.load(tab, u, func() {
		e.history = append(e.history[:e.pos+1], u)
		e.pos = len(e.history) - 1
	})
}

// moveTo loads a page already in the history.
func (e *Engine) moveTo(tab engine.TabID, pos int) {
	e.load(tab, e.history[pos], func() { e.pos = pos })
}

// load behaves like a real load: gates first, then later the commit, unless a newer
// navigation replaced it.
func (e *Engine) load(tab engine.TabID, u string, commit func()) {
	e.navSeq++
	seq := e.navSeq
	if !e.listener.NavigationStarting(tab, u) {
		e.publishPage("blocked", u)
		return
	}
	e.listener.LoadingChanged(tab, true)
	e.publishPage("loading", u)
	time.AfterFunc(loadDelay, func() {
		e.Post(func() {
			if seq != e.navSeq {
				return
			}
			commit()
			e.setCurrent(u)
			e.listener.HistoryChanged(tab, e.pos > 0, e.pos < len(e.history)-1)
			e.listener.NavigationFinished(tab, u)
			e.listener.TitleChanged(tab, "Mock "+hostOf(u))
			e.listener.LoadingChanged(tab, false)
			e.publishPage("loaded", u)
		})
	})
}

func (e *Engine) publishPage(state, u string) {
	b, _ := json.Marshal(map[string]string{"state": state, "url": u})
	e.Page.Publish(b)
}

func (e *Engine) setCurrent(u string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.current = u
}

func (e *Engine) currentURL() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.current
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
