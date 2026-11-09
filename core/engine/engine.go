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

// Engine is what the app can ask a browser engine to do.
type Engine interface {
	// Name says which engine this is, so the shell can show the mock page.
	Name() string

	CreateTab() TabID
	LoadURL(tab TabID, url string)

	// SetPageBounds places the active tab's view behind the shell. viewW and viewH are
	// the window size the shell measured against, so an engine can ignore a report
	// made for a size the window no longer has.
	SetPageBounds(r Rect, viewW, viewH float64)
	// SetShellRegions replaces the rectangles the shell owns. Mouse events inside
	// them go to the shell; everything else goes to the page.
	SetShellRegions(rects []Rect)
	FocusPage()

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
	TitleChanged(tab TabID, title string)
	LoadingChanged(tab TabID, loading bool)
	PageCrashed(tab TabID)

	// FocusAddressBarRequested fires on the shortcut while the page has focus.
	FocusAddressBarRequested()
}
