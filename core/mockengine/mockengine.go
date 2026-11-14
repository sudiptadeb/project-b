// Package mockengine is a browser engine with no browser in it. Its page is a set of
// buttons that fire the events real content would, for the tab it shows: navigation,
// title, failure, crash, a page opening a tab, a key press, full screen. A window shows a
// mock page for each tab its layout lists.
package mockengine

import (
	"encoding/json"
	"maps"
	"net/url"
	"slices"
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

// Width and Height are a mock window's content size until its dev window reports its
// own, as the real window's first size.
const (
	Width  = 1100.0
	Height = 700.0
)

// Keys are the names the mock page uses for key presses, as a Mac keyboard has them.
var Keys = map[string]engine.Key{
	"focusAddressBar": {Code: 'L', Mods: engine.ModCmd},
	"newTab":          {Code: 'T', Mods: engine.ModCmd},
	"newWindow":       {Code: 'N', Mods: engine.ModCmd},
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
	"splitRight":      {Code: 'D', Mods: engine.ModCmd},
	"splitDown":       {Code: 'D', Mods: engine.ModCmd | engine.ModShift},
}

// tab is one mock tab's session history and what its page shows.
type tab struct {
	history []string // The committed URLs.
	pos     int      // The current one in history, or -1 before the first.
	navSeq  int
	state   string // What the mock page shows: empty, loading, loaded, blocked, crashed.
	current string
	zoom    float64
	// workspace is the app's id for the workspace the tab is in, "" for the profile's own storage.
	workspace string
}

// Engine implements engine.Engine. Run is the main thread: everything happens there,
// as it does with real content. Each window the app makes is a dev window of its own, the
// shell at /?window=<id> on the dev server.
type Engine struct {
	queue    chan func()
	listener engine.Listener
	nextTab  engine.TabID
	tabs     map[engine.TabID]*tab
	// first is the workspace the app last said is first; erased are the erases it asked
	// for, in order. The mock makes no tab on its own, so only tests read them.
	first  string
	erased []Erase

	// Written on the main thread, read by the dev server's goroutines.
	mu         sync.Mutex
	windows    map[engine.WindowID]*window
	feeds      map[engine.WindowID]*Feeds
	requests   []engine.WindowRequest
	modes      []engine.WindowMode
	closed     []engine.WindowID
	quit       bool
	shownColor string
	menu       engine.Menu
	dockColor  string
	edits      []engine.EditCommand
	clipboard  string
	// focused is the tab last given the keyboard, by FocusTab or a press in its page.
	focused engine.TabID
	// unshownFocus are the tabs FocusTab was asked for while no window showed them.
	unshownFocus []engine.TabID
	// showRegions is the Show window regions flag, as the app launched with it.
	showRegions bool

	// The fake extension runtime, in a build with extensions.
	extensions extensionRuntime
}

// window is one mock window.
type window struct {
	page   engine.Page
	layout engine.WindowLayout
	// visible are the tabs the layout shows, in its order. The window has a mock page for
	// each, by its place in the list, its slot, and pages is what each slot's shows.
	visible           []engine.TabID
	pages             [][]byte
	hidden            []engine.TabID // Tabs told they are hidden by the last layout.
	full              bool           // Full screen, so there are no window controls.
	w, h              float64        // The window's content size.
	bounds            engine.Rect    // Its frame on screen.
	minimised, zoomed bool
	settings          bool
	shellBack         bool               // The phone's back key goes to the shell.
	reported          engine.WindowState // The last one reported.
}

// Feeds are what one window's dev window hears.
type Feeds struct {
	Shell    Feed // Messages to the shell page.
	Settings Feed // Messages to the settings page.
	Page     Feed // State for the mock pages, each message for one slot.
	// What the dev window draws with the shell, where the real window has views of its
	// own: the page, the settings page and an extension's popup. Each message is
	// WindowState's.
	Window Feed
}

// New returns an engine. Call Run to start its main thread.
func New() *Engine {
	return &Engine{queue: make(chan func(), 4096), tabs: map[engine.TabID]*tab{},
		windows: map[engine.WindowID]*window{}, feeds: map[engine.WindowID]*Feeds{}}
}

// Run is the main loop. It never returns.
func (e *Engine) Run() {
	for fn := range e.queue {
		fn()
	}
}

// SetSwitches takes the switches the app launches with, from the flags saved in the
// profile, as the real engine finds them on its command line before content starts.
// Call it before the app starts.
func (e *Engine) SetSwitches(switches []string) {
	e.showRegions = slices.Contains(switches, "--pb-show-regions")
}

// Feeds are window w's feeds, whether or not the window exists yet. Any goroutine.
func (e *Engine) Feeds(w engine.WindowID) *Feeds {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.feeds[w] == nil {
		e.feeds[w] = &Feeds{}
	}
	return e.feeds[w]
}

// SetListener sets who hears the engine's events, and reports the windows the app has
// made so far to it, as the real engine does once the call that made each has returned.
func (e *Engine) SetListener(l engine.Listener) {
	e.listener = l
	for _, w := range slices.Sorted(maps.Keys(e.windows)) {
		e.reportWindow(w)
	}
}

func (e *Engine) reportWindow(w engine.WindowID) {
	win := e.windows[w]
	if win == nil {
		return
	}
	s := engine.WindowState{W: win.w, H: win.h, FullScreen: win.full, Bounds: win.bounds, Minimised: win.minimised, Zoomed: win.zoomed}
	if !win.full {
		s.Controls = Controls
		s.CornerRadius = CornerRadius
	}
	e.mu.Lock()
	win.reported = s
	e.mu.Unlock()
	e.listener.WindowChanged(w, s)
	e.Feeds(w).Window.Publish(e.WindowState(w))
}

// Resize is window w's dev window reporting its content size, as the real window reports
// a resize. One whose window closed reports to no one. Any goroutine.
func (e *Engine) Resize(w engine.WindowID, width, height float64) {
	e.Post(func() {
		if win := e.windows[w]; win != nil {
			win.w, win.h = width, height
			e.reportWindow(w)
		}
	})
}

// Reported is the state window w last reported.
func (e *Engine) Reported(w engine.WindowID) engine.WindowState {
	e.mu.Lock()
	defer e.mu.Unlock()
	if win := e.windows[w]; win != nil {
		return win.reported
	}
	return engine.WindowState{}
}

// CreateWindow makes a window. The first one's colour is what the real window paints
// when it first goes on screen.
func (e *Engine) CreateWindow(w, near engine.WindowID, page engine.Page, l engine.WindowLayout) {
	e.mu.Lock()
	if len(e.windows) == 0 && e.shownColor == "" {
		e.shownColor = l.FrameColor
	}
	// Every mock window starts where the real first one does.
	e.windows[w] = &window{page: page, layout: l, w: Width, h: Height, bounds: engine.Rect{X: 80, Y: 60, W: Width, H: Height}}
	e.mu.Unlock()
	if e.listener != nil {
		e.Post(func() { e.reportWindow(w) })
	}
}

// SetWindowPage loads another page in the window. Its dev window loads the shell its state
// names.
func (e *Engine) SetWindowPage(w engine.WindowID, page engine.Page) {
	e.mu.Lock()
	e.windows[w].page = page
	e.mu.Unlock()
	e.Feeds(w).Window.Publish(e.WindowState(w))
}

func (e *Engine) CreateTab(workspace string) engine.TabID {
	e.nextTab++
	e.tabs[e.nextTab] = &tab{pos: -1, state: "empty", workspace: workspace}
	return e.nextTab
}

// Workspace is the workspace a tab is in, "" for the profile's own storage. Main thread.
func (e *Engine) Workspace(id engine.TabID) string {
	if t := e.tabs[id]; t != nil {
		return t.workspace
	}
	return ""
}

func (e *Engine) SetFirstWorkspace(workspace string) { e.first = workspace }

// FirstWorkspace is the workspace the app last said is first. Main thread.
func (e *Engine) FirstWorkspace() string { return e.first }

// Erase is one erase the app asked for: the workspace, and how many of its tabs were still
// open then, which the app closes first.
type Erase struct {
	Workspace string
	Open      int
}

func (e *Engine) EraseWorkspace(workspace string) {
	open := 0
	for _, t := range e.tabs {
		if t.workspace == workspace {
			open++
		}
	}
	e.erased = append(e.erased, Erase{workspace, open})
}

// Erased are the erases the app asked for, in order. Main thread.
func (e *Engine) Erased() []Erase { return slices.Clone(e.erased) }

func (e *Engine) CloseTab(id engine.TabID) {
	delete(e.tabs, id)
	for w, win := range e.windows {
		for slot, v := range win.visible {
			if v == id {
				e.publishPage(w, slot, 0, "none", "")
			}
		}
	}
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

// RequestWindow does what the real window does with each request, as far as what it
// reports goes, and records every request. Closing says the window closed.
func (e *Engine) RequestWindow(w engine.WindowID, r engine.WindowRequest) {
	e.mu.Lock()
	e.requests = append(e.requests, r)
	e.mu.Unlock()
	switch r {
	case engine.WindowClose:
		// Like the real one, it closes on a later turn, after the call that asked.
		e.Post(func() {
			for slot := range max(1, len(e.windows[w].visible)) {
				e.publishPage(w, slot, 0, "closed", "")
			}
			e.mu.Lock()
			delete(e.windows, w)
			e.closed = append(e.closed, w)
			e.mu.Unlock()
			e.listener.WindowClosed(w)
		})
	case engine.WindowFullScreen:
		e.windows[w].full = !e.windows[w].full
		e.reportWindow(w)
	case engine.WindowMinimise:
		e.windows[w].minimised = true
		e.reportWindow(w)
	case engine.WindowZoom:
		e.windows[w].zoomed = !e.windows[w].zoomed
		e.reportWindow(w)
	case engine.WindowFront:
		e.windows[w].minimised = false
		e.reportWindow(w)
	}
}

// SetWindowMode puts the window into a mode, as the real one does, and reports it.
func (e *Engine) SetWindowMode(w engine.WindowID, m engine.WindowMode) {
	e.mu.Lock()
	e.modes = append(e.modes, m)
	e.mu.Unlock()
	win := e.windows[w]
	win.full, win.minimised, win.zoomed = m == engine.ModeFullScreen, m == engine.ModeMinimised, m == engine.ModeZoomed
	e.reportWindow(w)
}

// SetWindowBounds moves and sizes the window, as the real one does, and reports it.
func (e *Engine) SetWindowBounds(w engine.WindowID, r engine.Rect) {
	e.windows[w].bounds = r
	e.reportWindow(w)
}

// The real window puts each visible tab's view where the layout says, and tells a tab
// that leaves the list it is hidden. The mock says where each goes on the window's Window
// feed, and its dev window puts a mock page there for each, by slot: a slot that shows
// another tab now hears what that tab shows.
func (e *Engine) SetWindowLayout(w engine.WindowID, l engine.WindowLayout) {
	win := e.windows[w]
	var visible []engine.TabID
	for _, v := range l.Tabs {
		if e.tabs[v.Tab] != nil {
			visible = append(visible, v.Tab)
		}
	}
	e.mu.Lock()
	win.layout = l
	win.hidden = win.hidden[:0]
	for _, id := range win.visible {
		if !slices.Contains(visible, id) && e.tabs[id] != nil && !e.shownElsewhere(w, id) {
			win.hidden = append(win.hidden, id)
		}
	}
	old := win.visible
	win.visible = visible
	e.mu.Unlock()
	e.Feeds(w).Window.Publish(e.WindowState(w))
	for slot, id := range visible {
		if slot >= len(old) || old[slot] != id {
			t := e.tabs[id]
			e.publishPage(w, slot, id, t.state, t.current)
		}
	}
}

// shown is the tab in window w's slot, or 0.
func (e *Engine) shown(w engine.WindowID, slot int) engine.TabID {
	if win := e.windows[w]; win != nil && slot < len(win.visible) {
		return win.visible[slot]
	}
	return 0
}

// shownElsewhere says whether a window other than w shows a tab: one that moved there,
// which the real engine doesn't tell it is hidden as it leaves w.
func (e *Engine) shownElsewhere(w engine.WindowID, id engine.TabID) bool {
	for other, win := range e.windows {
		if other != w && slices.Contains(win.visible, id) {
			return true
		}
	}
	return false
}

// Page is where a layout puts the first visible tab's page, or nothing.
func Page(l engine.WindowLayout) engine.Rect {
	if len(l.Tabs) == 0 {
		return engine.Rect{}
	}
	return l.Tabs[0].Rect
}

// Layout is the last layout the app gave window w, or none once it closed.
func (e *Engine) Layout(w engine.WindowID) engine.WindowLayout {
	e.mu.Lock()
	defer e.mu.Unlock()
	if win := e.windows[w]; win != nil {
		return win.layout
	}
	return engine.WindowLayout{}
}

// Open says whether window w is open. Main thread.
func (e *Engine) Open(w engine.WindowID) bool { return e.windows[w] != nil }

// Windows are the windows open now.
func (e *Engine) Windows() []engine.WindowID {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Sorted(maps.Keys(e.windows))
}

// Requests are the window requests the app made, in order.
func (e *Engine) Requests() []engine.WindowRequest {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.WindowRequest(nil), e.requests...)
}

// Modes are the modes the app asked windows into, in order.
func (e *Engine) Modes() []engine.WindowMode {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.WindowMode(nil), e.modes...)
}

// ShownColor is the colour the first window painted when it went on screen.
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

// DuplicateTab copies the tab's history into a new hidden tab, in its workspace.
func (e *Engine) DuplicateTab(id engine.TabID) engine.TabID {
	n := e.CreateTab(e.Workspace(id))
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

func (e *Engine) WriteClipboard(text string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clipboard = text
}

// Clipboard is the text the app last put on the clipboard.
func (e *Engine) Clipboard() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.clipboard
}

// Edits are the editing commands the app ran, in order.
func (e *Engine) Edits() []engine.EditCommand {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.EditCommand(nil), e.edits...)
}

// The real window puts the settings page in a view over everything. The mock says so on
// the window's Window feed, and its dev window draws it there.
func (e *Engine) OpenSettings(w engine.WindowID) {
	e.mu.Lock()
	e.windows[w].settings = true
	e.mu.Unlock()
	e.Feeds(w).Window.Publish(e.WindowState(w))
}

func (e *Engine) CloseSettings(w engine.WindowID) {
	e.mu.Lock()
	e.windows[w].settings = false
	e.mu.Unlock()
	e.Feeds(w).Settings.Publish([]byte(`{"t":"closed"}`))
	e.Feeds(w).Window.Publish(e.WindowState(w))
}

// WindowState is what window w holds besides the shell, and how it draws and hit-tests,
// as its Window feed sends it:
//
//	{"t":"window", "shell":the id of the shell the window loads,
//	 "pages":[{"tab":id,"rect":{x,y,w,h}}] where each mock page goes, by slot,
//	 "regions":[{x,y,w,h,"area":"drag"|"shell"}],
//	 "frameColor":"#rrggbb", "frameWidth":px, "controls":the window buttons or null,
//	 "cornerRadius":px, "settings":open, "popup":the extension popup or null,
//	 "showRegions":whether to outline every rectangle the window manager tracks}
//
// The rectangles are the layout's, in the shell's coordinates, which on
// the mock are the dev window's. The regions are in order: the first that holds a point
// decides. A window that closed holds nothing. Any goroutine.
func (e *Engine) WindowState(w engine.WindowID) []byte {
	type region struct {
		engine.Rect
		Area string `json:"area"`
	}
	e.mu.Lock()
	var l engine.WindowLayout
	var s engine.WindowState
	var page engine.Page
	pages := []engine.VisibleTab{}
	settings := false
	if win := e.windows[w]; win != nil {
		l, s, page, settings = win.layout, win.reported, win.page, win.settings
		for _, v := range l.Tabs {
			if slices.Contains(win.visible, v.Tab) {
				pages = append(pages, v)
			}
		}
	}
	e.mu.Unlock()
	regions := []region{}
	for _, r := range l.Regions {
		regions = append(regions, region{r.Rect, AreaNames[r.Area]})
	}
	var controls *engine.Rect
	if s.Controls != (engine.Rect{}) {
		controls = &s.Controls
	}
	b, _ := json.Marshal(map[string]any{"t": "window", "shell": page.Shell(), "pages": pages, "regions": regions,
		"frameColor": l.FrameColor, "frameWidth": l.FrameWidth, "controls": controls, "cornerRadius": s.CornerRadius,
		"settings": settings, "popup": e.openPopup(w), "showRegions": e.showRegions})
	return b
}

// AreaNames are what the dev window and its tests call each area.
var AreaNames = [...]string{engine.AreaPage: "page", engine.AreaShell: "shell", engine.AreaDrag: "drag"}

// SettingsOpen says whether window w has its settings page open.
func (e *Engine) SettingsOpen(w engine.WindowID) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	win := e.windows[w]
	return win != nil && win.settings
}

func (e *Engine) SendToSettings(w engine.WindowID, msg []byte) { e.Feeds(w).Settings.Publish(msg) }

func (e *Engine) SetMenu(m engine.Menu) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.menu = m
}

// PageOf is which page covers window w, or "" for one that isn't open.
func (e *Engine) PageOf(w engine.WindowID) engine.Page {
	e.mu.Lock()
	defer e.mu.Unlock()
	if win := e.windows[w]; win != nil {
		return win.page
	}
	return ""
}

// Menu is the last menu the app gave the window.
func (e *Engine) Menu() engine.Menu {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.menu
}

// AppCommand records a quit; the rest are the OS's.
func (e *Engine) SetDockColor(color string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dockColor = color
}

// DockColor is the colour the app last put on its Dock icon, or "".
func (e *Engine) DockColor() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.dockColor
}

func (e *Engine) AppCommand(c engine.AppCommand) {
	if c == engine.AppQuit {
		e.mu.Lock()
		e.quit = true
		e.mu.Unlock()
	}
}

// FocusTab gives the tab the keyboard, which the mock only remembers. The real window
// gives it only to a tab it shows: a page view it hides can't take it, and the keyboard
// is left where no key reaches Go. So a tab no window shows doesn't get it here either,
// and the mock remembers that it was asked.
func (e *Engine) FocusTab(tab engine.TabID) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.tabs[tab] != nil {
		shown := false
		for _, win := range e.windows {
			shown = shown || slices.Contains(win.visible, tab)
		}
		if !shown {
			e.unshownFocus = append(e.unshownFocus, tab)
			return
		}
	}
	e.focused = tab
}

// UnshownFocus are the tabs FocusTab was asked for while no window showed them.
func (e *Engine) UnshownFocus() []engine.TabID {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.TabID{}, e.unshownFocus...)
}

// Focused is the tab last given the keyboard, by FocusTab or a press in its page.
func (e *Engine) Focused() engine.TabID {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.focused
}

func (e *Engine) FocusShell(engine.WindowID) {}

func (e *Engine) SendToShell(w engine.WindowID, msg []byte) { e.Feeds(w).Shell.Publish(msg) }

func (e *Engine) SetShellBack(w engine.WindowID, on bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if win := e.windows[w]; win != nil {
		win.shellBack = on
	}
}

func (e *Engine) Post(fn func()) { e.queue <- fn }

// Settle returns once the engine has run everything posted to it so far, and everything
// that posted in turn: the queue has drained. Any goroutine but the main thread.
func (e *Engine) Settle() {
	done := make(chan struct{})
	var check func()
	check = func() {
		if len(e.queue) > 0 {
			e.Post(check)
			return
		}
		close(done)
	}
	e.Post(check)
	<-done
}

// Closed reports whether window w closed.
func (e *Engine) Closed(w engine.WindowID) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Contains(e.closed, w)
}

// Quit reports whether the app quit.
func (e *Engine) Quit() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.quit
}

// Hidden returns the tabs in window w told they are hidden by its last layout.
func (e *Engine) Hidden(w engine.WindowID) []engine.TabID {
	e.mu.Lock()
	defer e.mu.Unlock()
	if win := e.windows[w]; win != nil {
		return append([]engine.TabID(nil), win.hidden...)
	}
	return nil
}

// Fire makes the engine do what a button on a mock page of window w says, to the tab the
// page shows, or to the first tab the window shows for 0. Any goroutine.
func (e *Engine) Fire(w engine.WindowID, tab engine.TabID, kind, value string) {
	e.Post(func() {
		win := e.windows[w]
		if win == nil {
			return
		}
		shown := tab
		if shown == 0 {
			shown = e.shown(w, 0)
		}
		t := e.tabs[shown]
		switch kind {
		case "navigate":
			e.navigate(shown, value)
		case "title":
			e.listener.TitleChanged(shown, value)
		case "backKey":
			// The phone's back key, as Android's window takes it: settings, then the
			// shell's own, then the page's history.
			switch {
			case win.settings:
				e.listener.SettingsMessage(w, []byte(`{"t":"close"}`))
			case win.shellBack:
				e.listener.ShellMessage(w, []byte(`{"t":"backKey"}`))
			default:
				e.listener.ShellMessage(w, []byte(`{"t":"back"}`))
			}
		case "contextMenu":
			// A long press on a link to value, as content reports it.
			e.listener.ContextMenu(shown, engine.ContextMenu{X: 40, Y: 60, Link: value, LinkText: "a link"})
		case "scroll":
			// value is down or up, as a page that scrolls makes content say.
			e.listener.ScrollDirectionChanged(shown, value == "down")
		case "fail":
			if t != nil {
				t.navSeq++
				e.listener.LoadingChanged(shown, false)
				e.listener.NavigationFailed(shown, t.current, value)
			}
		case "crash":
			if t != nil {
				t.navSeq++
				t.state = "crashed"
				e.listener.PageCrashed(shown)
				e.showPage(shown, t.state, t.current)
			}
		case "favicon":
			e.listener.FaviconChanged(shown, value)
		case "focusTab":
			// A press in the page, which takes the keyboard, as a click does in the real
			// window's.
			if t != nil {
				e.FocusTab(shown)
				e.listener.TabFocused(shown)
			}
		case "menu":
			// A menu bar item chosen, by its action id, as the real menu bar says.
			e.listener.MenuChosen(value)
		case "open":
			// What a page's target=_blank or window.open does: the engine makes the
			// tab, in the page's workspace, tells the app, then loads it.
			id := e.CreateTab(e.Workspace(shown))
			e.listener.TabOpened(shown, id, value, e.Workspace(shown), true, false)
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
			win.full = !win.full
			e.reportWindow(w)
		case "focus":
			// The window comes to the front, as a click on it does.
			e.listener.WindowFocused(w)
		case "close":
			// Its own close button, which asks no one.
			e.mu.Lock()
			delete(e.windows, w)
			e.closed = append(e.closed, w)
			e.mu.Unlock()
			e.listener.WindowClosed(w)
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
		e.showPage(id, t.state, u)
		return
	}
	e.listener.LoadingChanged(id, true)
	e.listener.LoadProgressChanged(id, 0.5)
	t.state = "loading"
	e.showPage(id, t.state, u)
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
			e.showPage(id, t.state, u)
		})
	})
}

// publishPage tells the mock page in window w's slot what to show: a tab, or 0 for none.
// The window keeps it for a page that connects later.
func (e *Engine) publishPage(w engine.WindowID, slot int, id engine.TabID, state, u string) {
	b, _ := json.Marshal(map[string]any{"slot": slot, "tab": id, "state": state, "url": u})
	if win := e.windows[w]; win != nil {
		e.mu.Lock()
		for len(win.pages) <= slot {
			win.pages = append(win.pages, nil)
		}
		win.pages[slot] = b
		e.mu.Unlock()
	}
	e.Feeds(w).Page.Publish(b)
}

// PageState is the last message for the mock page in window w's slot, or nil before any.
// Any goroutine.
func (e *Engine) PageState(w engine.WindowID, slot int) []byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	if win := e.windows[w]; win != nil && slot < len(win.pages) {
		return win.pages[slot]
	}
	return nil
}

// showPage tells the mock page of each slot that shows a tab what it shows now.
func (e *Engine) showPage(id engine.TabID, state, u string) {
	for w, win := range e.windows {
		for slot, v := range win.visible {
			if v == id {
				e.publishPage(w, slot, id, state, u)
			}
		}
	}
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
