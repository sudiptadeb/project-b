// Package engine is the contract between our app and a browser engine.
//
// App code only ever sees this package. The real implementation wraps Chromium's
// content layer (see ../../engine); the mock is pure Go (see ../mockengine).
//
// Threading: every Engine call and every Listener call happens on the main thread.
// A worker goroutine that finishes something uses Post to get back onto it.
package engine

import "errors"

// TabID identifies a tab across the boundary. Content's own objects never cross it.
type TabID int64

// WindowID identifies a window across the boundary. The app makes them, 1 for the first.
type WindowID int64

// Rect is in window coordinates, CSS pixels.
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Contains reports whether the point is inside the rectangle.
func (r Rect) Contains(x, y float64) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Mods are the modifier keys held with a key. The values cross to the engine as they are.
type Mods int

const (
	ModCtrl Mods = 1 << iota
	ModAlt       // Option on a Mac.
	ModShift
	ModCmd // The Command key on a Mac, the Windows key elsewhere.
)

// Key is one key press: its Windows virtual-key code, which every engine reports on
// every platform, and the modifiers held with it.
type Key struct {
	Code int  `json:"code"`
	Mods Mods `json:"mods"`
}

// KeyPhase is when the engine asks about a key.
type KeyPhase int

const (
	// KeyBeforePage: the key is about to go to a page. Saying yes keeps it from the page.
	KeyBeforePage KeyPhase = iota + 1
	// KeyUnhandled: the page had the key and didn't use it.
	KeyUnhandled
)

// Surface is which of our pages had the keyboard when a key was pressed.
type Surface int

const (
	SurfacePage Surface = iota + 1
	SurfaceShell
	SurfaceSettings
)

// MenuItem is one entry of the native menu bar. A submenu has Items; a separator has
// none of the rest.
type MenuItem struct {
	Title     string     `json:"title,omitempty"`
	Action    string     `json:"action,omitempty"` // What choosing it runs, by id.
	Key       *Key       `json:"key,omitempty"`    // The key it shows; nil for none.
	Enabled   bool       `json:"enabled,omitempty"`
	Checked   bool       `json:"checked,omitempty"`
	Separator bool       `json:"separator,omitempty"`
	Role      string     `json:"role,omitempty"` // For a submenu the OS fills: "services".
	Items     []MenuItem `json:"items,omitempty"`
}

// Menu is the whole menu bar. Role says which menu the OS knows as which: "app",
// "window" and "help".
type Menu []MenuItem

// EditCommand is a text-editing command for whichever page has the keyboard.
type EditCommand int

const (
	EditUndo EditCommand = iota + 1
	EditRedo
	EditCut
	EditCopy
	EditPaste
	EditPasteAndMatchStyle
	EditDelete
	EditSelectAll
)

// AppCommand is something only the OS can do for the app.
type AppCommand int

const (
	AppAbout AppCommand = iota + 1
	AppHide
	AppHideOthers
	AppShowAll
	AppBringAllToFront
	AppQuit // Ends the app, every window at once.
)

// Engine is what the app can ask a browser engine to do.
type Engine interface {
	// CreateTab makes a hidden tab. A window's layout shows it.
	CreateTab() TabID
	CloseTab(tab TabID)
	LoadURL(tab TabID, url string)

	// GoBack, GoForward and Reload move within the tab's own session history. What
	// happens next arrives as ordinary navigation events.
	GoBack(tab TabID)
	GoForward(tab TabID)
	Reload(tab TabID)
	// ReloadBypassingCache reloads without the cache; Stop stops loading.
	ReloadBypassingCache(tab TabID)
	Stop(tab TabID)
	// Zoom sets the tab's page zoom as a factor, 1 for actual size; the engine keeps it
	// for the page's host, as browsers do. ZoomOf says what it is now.
	Zoom(tab TabID, factor float64)
	ZoomOf(tab TabID) float64
	// DuplicateTab makes a hidden copy of the tab, its history included.
	DuplicateTab(tab TabID) TabID
	// Edit runs a text-editing command in whichever page has the keyboard.
	Edit(c EditCommand)

	// CreateWindow makes window w, with its shell page, and puts it on screen with its
	// first layout, a little down and right of window near, or where a first window goes
	// when near is 0. It tells WindowChanged once this call has returned.
	CreateWindow(w, near WindowID, l WindowLayout)

	// RequestWindow asks a native window to close, minimise, zoom, go in or out of full
	// screen, or come to the front. What follows arrives as WindowClosed or
	// WindowChanged.
	RequestWindow(w WindowID, r WindowRequest)

	// SetWindowLayout is everything about how a window behaves, from the window
	// manager, whenever any of it changes: which tabs it shows, and where.
	SetWindowLayout(w WindowID, l WindowLayout)
	// FocusTab gives keyboard focus to a visible tab's page; FocusShell gives it to a
	// window's shell page, which then places it inside itself.
	FocusTab(tab TabID)
	FocusShell(w WindowID)

	// SendToShell delivers one message to a window's shell page.
	SendToShell(w WindowID, msg []byte)

	// OpenSettings puts the settings page over the whole of a window, in a page of its
	// own, and gives it the keyboard; CloseSettings takes it away. SendToSettings delivers
	// one message to it while it is open.
	OpenSettings(w WindowID)
	CloseSettings(w WindowID)
	SendToSettings(w WindowID, msg []byte)

	// SetMenu replaces the native menu bar, where the platform has one.
	SetMenu(m Menu)
	AppCommand(c AppCommand)

	// Post runs fn on the main thread, later.
	Post(fn func())
}

// Listener is what an engine tells the app. There is exactly one.
type Listener interface {
	// ShellMessage is one message from a window's shell page.
	ShellMessage(w WindowID, msg []byte)

	// NavigationStarting fires at the start and on each redirect. Returning false
	// blocks the navigation.
	NavigationStarting(tab TabID, url string) bool
	NavigationFinished(tab TabID, url string)
	NavigationFailed(tab TabID, url, reason string)
	// HistoryChanged says whether the tab can currently go back and forward.
	HistoryChanged(tab TabID, canGoBack, canGoForward bool)
	TitleChanged(tab TabID, title string)
	LoadingChanged(tab TabID, loading bool)
	// LoadProgressChanged says how far the page's load has got, 0 to 1.
	LoadProgressChanged(tab TabID, progress float64)
	PageCrashed(tab TabID)
	// FaviconChanged carries the page's icon as a data: URL, or "" when the page has
	// none the engine could fetch.
	FaviconChanged(tab TabID, dataURL string)

	// TabOpened says a page opened a new tab (target=_blank, window.open). The engine
	// has already created it, hidden; the app decides whether to show it.
	TabOpened(opener, tab TabID, url string, foreground bool)

	// TabFocused says the keyboard went into a visible tab's page: a click, Tab from the
	// shell, or FocusTab.
	TabFocused(tab TabID)

	// CloseRequested says something asked to close a tab: its page called
	// window.close(), or a DevTools client closed its target. ActivateRequested asks
	// for a tab to be shown, as a DevTools client's Target.activateTarget does. The app
	// decides; the engine has done nothing yet.
	CloseRequested(tab TabID)
	ActivateRequested(tab TabID)

	// Key asks about one key press in any of our pages, at either phase. Returning true
	// means the app used it.
	Key(k Key, phase KeyPhase, in Surface) bool
	// ModsChanged says which modifiers are held now, whenever that changes.
	ModsChanged(m Mods)
	// MenuChosen says the user chose a menu item, by its action id.
	MenuChosen(action string)
	// OSBindings carries the keys the OS lets people give our menu items, by item
	// title, as macOS App Shortcuts does. It fires after start and whenever they change.
	OSBindings(byTitle map[string]Key)

	// SettingsMessage is one message from a window's settings page.
	SettingsMessage(w WindowID, msg []byte)

	// WindowClosed says a window closed, asked by the app or by its own close button.
	// Its shell and settings pages are gone with it; its tabs are still there.
	WindowClosed(w WindowID)
	// WindowFocused says a window became the one in front, the one keys go to.
	WindowFocused(w WindowID)

	// WindowChanged says what a native window is now. It fires after start, on
	// resize and on entering or leaving full screen.
	WindowChanged(w WindowID, s WindowState)

	// ShellCrashed says a window's shell page's renderer died. The engine reloads it, and
	// the new shell reports its layout again.
	ShellCrashed(w WindowID)

	// ProcessCrashed says a child process crashed.
	ProcessCrashed(p ProcessCrash)

	// DevToolsCommand is one of our own DevTools commands, ProjectB.*, from a client of
	// the debugging port, with its params as JSON. The result is the reply's result,
	// as JSON. ErrUnknownCommand leaves the reply to content, which says it has no such
	// method.
	DevToolsCommand(method string, params []byte) (result any, err error)
}

// ErrUnknownCommand is DevToolsCommand's answer for a method it doesn't have.
var ErrUnknownCommand = errors.New("unknown command")

// Area is what a press at a point does.
type Area int

const (
	AreaPage  Area = iota // No region holds the point: it goes to the visible tab's page under it.
	AreaShell             // It goes to the shell.
	AreaDrag              // It moves the window.
)

// Region is one rectangle and what a press inside it does.
type Region struct {
	Rect Rect
	Area Area
}

// VisibleTab is a tab whose page a window shows, and where.
type VisibleTab struct {
	Tab  TabID `json:"tab"`
	Rect Rect  `json:"rect"`
}

// WindowLayout is everything the engine needs to make a window behave.
type WindowLayout struct {
	ViewW, ViewH float64      // The window size the shell measured; an engine places the pages only when it still has that size.
	Tabs         []VisibleTab // The tabs whose pages show, and where. Every other tab is hidden.
	Regions      []Region     // In order. The first that holds a point decides.
	FrameColor   string       // "#rrggbb", painted under the shell and the page.
	FrameWidth   float64      // The band drawn around the window's edge, over both.
	Theme        Theme        // Light or dark for the window's own parts, or the system's.
}

// Theme is how the window's own parts, its buttons and title bar, are drawn.
type Theme int

const (
	ThemeSystem Theme = iota // Whatever the system is.
	ThemeLight
	ThemeDark
)

// At is the one hit test: what a press at x, y does.
func (l WindowLayout) At(x, y float64) Area {
	for _, r := range l.Regions {
		if r.Rect.Contains(x, y) {
			return r.Area
		}
	}
	return AreaPage
}

// ProcessCrash is a child process that died when it shouldn't have: a renderer, the GPU
// process or a utility process. The engine starts another when one is needed.
type ProcessCrash struct {
	Type     string // The kind of process, as the engine names it.
	Status   string // How it ended: crashed, killed, out of memory, and so on.
	ExitCode int
	PID      int
	URL      string // The page a renderer showed, or "".
}

// WindowState is what the engine reports about its native window.
type WindowState struct {
	W, H         float64 // The content area, CSS pixels.
	Controls     Rect    // The native window buttons; empty when there are none.
	CornerRadius float64 // 0 when the corners are square.
	FullScreen   bool
}

// WindowRequest is what the window is asked to do. The first four are what pressing that
// window button does, so zoom and full screen go in or out. WindowFront brings the window
// to the front and makes the app the active one.
type WindowRequest int

const (
	WindowClose WindowRequest = iota + 1
	WindowMinimise
	WindowZoom
	WindowFullScreen
	WindowFront
)
