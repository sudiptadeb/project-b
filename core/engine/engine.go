// Package engine is the contract between our app and a browser engine.
//
// App code only ever sees this package. The real implementation wraps Chromium's
// content layer (see ../../engine); the mock is pure Go (see ../mockengine).
//
// Threading: every Engine call and every Listener call happens on the main thread.
// A worker goroutine that finishes something uses Post to get back onto it.
package engine

// TabID identifies a tab across the boundary. Content's own objects never cross it.
type TabID int64

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

// Shortcut is a key combination the engine keeps from pages and hands to the app.
// Which keys those are is decided in Go (core/content/wire.go for the real engine).
type Shortcut int

const (
	ShortcutFocusAddressBar Shortcut = iota + 1
	ShortcutNewTab
	ShortcutCloseTab
	ShortcutNextTab
	ShortcutPreviousTab
	// ShortcutTab1 and the eight after it pick a tab by position; the ninth picks the
	// last tab, as every browser does.
	ShortcutTab1
)

// Engine is what the app can ask a browser engine to do.
type Engine interface {
	// Name says which engine this is, so the shell can show the mock page.
	Name() string

	// CreateTab makes a hidden tab. ShowTab shows it.
	CreateTab() TabID
	CloseTab(tab TabID)
	// ShowTab shows this tab's page and hides every other one. Hidden pages are told
	// they are hidden.
	ShowTab(tab TabID)
	LoadURL(tab TabID, url string)

	// GoBack, GoForward and Reload move within the tab's own session history. What
	// happens next arrives as ordinary navigation events.
	GoBack(tab TabID)
	GoForward(tab TabID)
	Reload(tab TabID)

	// RequestWindow asks the native window to close, which ends the app, minimise,
	// zoom, or go in or out of full screen. What follows arrives as WindowChanged.
	RequestWindow(r WindowRequest)

	// SetWindowLayout is everything about how the window behaves, from the window
	// manager, whenever any of it changes.
	SetWindowLayout(l WindowLayout)
	// FocusPage gives keyboard focus to the shown tab's page; FocusShell gives it to
	// the shell page, which then places it inside itself.
	FocusPage()
	FocusShell()

	// SendToShell delivers one message to the shell page.
	SendToShell(msg []byte)

	// Post runs fn on the main thread, later.
	Post(fn func())
}

// Listener is what an engine tells the app. There is exactly one.
type Listener interface {
	ShellMessage(msg []byte)

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

	// CloseRequested says something asked to close a tab: its page called
	// window.close(), or a DevTools client closed its target. ActivateRequested asks
	// for a tab to be shown, as a DevTools client's Target.activateTarget does. The app
	// decides; the engine has done nothing yet.
	CloseRequested(tab TabID)
	ActivateRequested(tab TabID)

	// Shortcut fires for a key combination the engine reserves, wherever focus was.
	Shortcut(s Shortcut)

	// WindowChanged says what the native window is now. It fires after start, on
	// resize and on entering or leaving full screen.
	WindowChanged(s WindowState)

	// ShellCrashed says the shell page's renderer died. The engine reloads it, and the
	// new shell reports its layout again.
	ShellCrashed()
}

// Area is what a press at a point does.
type Area int

const (
	AreaPage  Area = iota // No region holds the point: it goes to the shown page.
	AreaShell             // It goes to the shell.
	AreaDrag              // It moves the window.
)

// Region is one rectangle and what a press inside it does.
type Region struct {
	Rect Rect
	Area Area
}

// WindowLayout is everything the engine needs to make the window behave.
type WindowLayout struct {
	ViewW, ViewH float64  // The window size the shell measured; an engine places the page only when it still has that size.
	Page         Rect     // Where the shown tab's page goes.
	Regions      []Region // In order. The first that holds a point decides.
	FrameColor   string   // "#rrggbb", painted under the shell and the page.
	FrameWidth   float64  // The band drawn around the window's edge, over both.
}

// At is the one hit test: what a press at x, y does.
func (l WindowLayout) At(x, y float64) Area {
	for _, r := range l.Regions {
		if r.Rect.Contains(x, y) {
			return r.Area
		}
	}
	return AreaPage
}

// WindowState is what the engine reports about its native window.
type WindowState struct {
	W, H         float64 // The content area, CSS pixels.
	Controls     Rect    // The native window buttons; empty when there are none.
	CornerRadius float64 // 0 when the corners are square.
	FullScreen   bool
}

// WindowRequest is what the user asked the window to do. Each is what pressing that
// window button does, so zoom and full screen go in or out.
type WindowRequest int

const (
	WindowClose WindowRequest = iota + 1
	WindowMinimise
	WindowZoom
	WindowFullScreen
)
