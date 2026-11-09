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

	// CloseWindow closes the window, which ends the app.
	CloseWindow()

	// SetPageBounds places the shown tab's view behind the shell. viewW and viewH are
	// the window size the shell measured against, so an engine can ignore a report
	// made for a size the window no longer has.
	SetPageBounds(r Rect, viewW, viewH float64)
	// SetShellRegions replaces the rectangles the shell owns. Mouse events inside
	// them go to the shell; everything else goes to the page.
	SetShellRegions(rects []Rect)
	// SetDragRegions replaces the rectangles where a mouse down moves the window
	// instead: the empty parts of the shell. They win over shell regions.
	SetDragRegions(rects []Rect)
	// SetWindowFrame is the frame the window itself paints: its colour, "#rrggbb",
	// under the page and the shell, so any gap the shell leaves shows it and not white,
	// and the width of the band it draws around its edge over both.
	SetWindowFrame(hex string, width float64)
	// FocusPage gives keyboard focus to the shown tab's page; FocusShell gives it to
	// the shell page, which then places it inside itself.
	FocusPage()
	FocusShell()

	// SendToShell delivers one message to the shell page.
	SendToShell(msg []byte)
	// SendToTab delivers one message to our own page in a tab, if the tab holds one with
	// its bridge. Anything else in the tab gets nothing.
	SendToTab(tab TabID, msg []byte)

	// Post runs fn on the main thread, later.
	Post(fn func())
}

// Listener is what an engine tells the app. There is exactly one.
type Listener interface {
	// ShellMessage is a message from one of our own pages. from says which: 0 for the
	// shell, or the id of the tab the page is in. The engine sets it; the page can't.
	ShellMessage(from TabID, msg []byte)

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

	// WindowControlsChanged says where the native window controls (close, minimise,
	// zoom) sit, so the shell can leave that space, and the radius the OS rounds the
	// window's corners with, so the shell can follow the curve. It fires after start,
	// on resize and on entering or leaving full screen. An empty Rect means there are
	// none to leave space for; a radius of 0 means square corners.
	WindowControlsChanged(r Rect, cornerRadius float64)
}
