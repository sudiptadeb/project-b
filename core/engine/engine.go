// Package engine is the contract between our app and a browser engine.
//
// App code only ever sees this package. The real implementation wraps Chromium's
// content layer (see ../../engine); the mock is pure Go (see ../mockengine).
//
// Threading: every Engine call and every Listener call happens on the main thread.
// A worker goroutine that finishes something uses Post to get back onto it.
package engine

import (
	"errors"
	"strings"
)

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

// Page is which of our pages covers a window: a shell, or the picker in a copy that
// hasn't chosen its profile. Each is an app of its own, chrome://<page>/: a shell's page
// is "shell/<id>", at chrome://shell/<id>/.
type Page string

// PagePicker is the picker's page.
const PagePicker Page = "picker"

// ShellPage is the page of the shell called id.
func ShellPage(id string) Page { return Page("shell/" + id) }

// Shell is the id of the shell the page is, or "" for a page that isn't a shell.
func (p Page) Shell() string {
	id, ok := strings.CutPrefix(string(p), "shell/")
	if !ok {
		return ""
	}
	return id
}

// MenuItem is one entry of the native menu bar. A submenu has Items; a separator has
// none of the rest.
type MenuItem struct {
	Title     string `json:"title,omitempty"`
	Action    string `json:"action,omitempty"` // What choosing it runs, by id.
	Key       *Key   `json:"key,omitempty"`    // The key it shows; nil for none.
	Enabled   bool   `json:"enabled,omitempty"`
	Checked   bool   `json:"checked,omitempty"`
	Separator bool   `json:"separator,omitempty"`
	Role      string `json:"role,omitempty"` // For a submenu the OS fills: "services".
	// List stands in for the items of a list that changes, which the engine asks for,
	// by this name, each time its menu opens (Listener.MenuList).
	List  string     `json:"list,omitempty"`
	Items []MenuItem `json:"items,omitempty"`
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
	// CreateTab makes a hidden tab in a workspace, by the app's id for it: "" is the
	// profile's own storage, which a profile's first workspace, Main, has. A window's layout
	// shows it. A workspace keeps its own cookies and site storage, and a tab stays in its
	// workspace for its life, as does every tab it opens.
	CreateTab(workspace string) TabID
	// SetFirstWorkspace says which workspace a tab the engine makes on its own goes in, one
	// a client of the debugging port or an extension asks for: the first in the profile's
	// list. The app says so at start, before any such tab, and whenever the first changes.
	SetFirstWorkspace(workspace string)
	// EraseWorkspace erases a workspace's site data and cache: its cookies, every kind of
	// site storage, service workers and the HTTP cache. The app asks once it has closed
	// the workspace's tabs. For "", the profile's own storage, extensions keep their own.
	EraseWorkspace(workspace string)
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
	// DuplicateTab makes a hidden copy of the tab, its history included, in its workspace.
	DuplicateTab(tab TabID) TabID
	// Edit runs a text-editing command in whichever page has the keyboard.
	Edit(c EditCommand)
	// WriteClipboard puts plain text on the system clipboard.
	WriteClipboard(text string)
	// DownloadURL downloads url as tab's page would, through the Save panel when saveAs.
	DownloadURL(tab TabID, url string, saveAs bool)
	// DownloadCommand acts on a running download.
	DownloadCommand(id DownloadID, c DownloadCommand)
	// OpenFile opens a file as the OS would; RevealFile shows it in the file manager.
	OpenFile(path string)
	RevealFile(path string)

	// CreateWindow makes window w, with its page, and puts it on screen with its first
	// layout, a little down and right of window near, or where a first window goes when
	// near is 0. It tells WindowChanged once this call has returned.
	CreateWindow(w, near WindowID, page Page, l WindowLayout)
	// SetWindowPage loads another page in window w in place of the one covering it, which
	// is gone with all it had. The new page starts as a new window's does. The tabs stay
	// where the window's layout last put them.
	SetWindowPage(w WindowID, page Page)

	// RequestWindow asks a native window to close, minimise, zoom, go in or out of full
	// screen, or come to the front. What follows arrives as WindowClosed or
	// WindowChanged.
	RequestWindow(w WindowID, r WindowRequest)
	// SetWindowBounds puts a window's frame on screen at r, y down from the top of the
	// main screen. What follows arrives as WindowChanged.
	SetWindowBounds(w WindowID, r Rect)
	// SetWindowMode puts a window into a mode from whatever it is in now, where the
	// requests above toggle. What follows arrives as WindowChanged.
	SetWindowMode(w WindowID, m WindowMode)

	// SetWindowLayout is everything about how a window behaves, from the window
	// manager, whenever any of it changes: which tabs it shows, and where.
	SetWindowLayout(w WindowID, l WindowLayout)
	// FocusTab gives keyboard focus to a visible tab's page; FocusShell gives it to a
	// window's shell page, which then places it inside itself.
	FocusTab(tab TabID)
	FocusShell(w WindowID)

	// SendToShell delivers one message to a window's shell page.
	SendToShell(w WindowID, msg []byte)
	// SetShellBack says whether the phone's back key goes to the shell first, while it
	// has something open that the key closes: the engine then sends {"t":"backKey"} on
	// the shell's channel, as if the shell had, instead of going back in the page.
	SetShellBack(w WindowID, on bool)

	// OpenSettings puts the settings page over the whole of a window, in a page of its
	// own, and gives it the keyboard; CloseSettings takes it away. SendToSettings delivers
	// one message to it while it is open.
	OpenSettings(w WindowID)
	CloseSettings(w WindowID)
	SendToSettings(w WindowID, msg []byte)

	// SetMenu replaces the native menu bar, where the platform has one.
	SetMenu(m Menu)
	AppCommand(c AppCommand)
	// SetDockColor draws the app's Dock icon with a small dot in color, "#rrggbb", the
	// profile's, so each running profile's copy tells itself apart there.
	SetDockColor(color string)

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
	// ContextMenu says a long press or a right click in a tab's page was on a link, an
	// image or a selection, which a shell may offer to act on.
	ContextMenu(tab TabID, m ContextMenu)
	// ScrollDirectionChanged says the page began to scroll down, or back up: a phone
	// shell hides its bars while the page scrolls down.
	ScrollDirectionChanged(tab TabID, down bool)
	PageCrashed(tab TabID)
	// FaviconChanged carries the page's icon as a data: URL, or "" when the page has
	// none the engine could fetch.
	FaviconChanged(tab TabID, dataURL string)

	// TabOpened says a page opened a new tab (target=_blank, window.open). The engine
	// has already created it, hidden, in workspace, its opener's; the app decides whether
	// to show it. A tab a client of the debugging port asks for is in the first
	// workspace, and may be in a window of its own, newWindow; a page's never is, so its
	// tab goes in its window.
	TabOpened(opener, tab TabID, url, workspace string, foreground, newWindow bool)

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
	// MenuList is the items of the menu list name (MenuItem.List) as they are now. The
	// engine asks each time a menu holding it opens.
	MenuList(name string) Menu
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

	// DownloadTarget asks where a download goes, before any of it is saved. A target
	// with no path refuses it.
	DownloadTarget(r DownloadRequest) DownloadTarget
	// DownloadUpdated says what a download is now: on each change once it has a path.
	DownloadUpdated(d Download)

	// FileChooser asks whether a page may open the file panel, and where it opens. The
	// panel is the system's own.
	FileChooser(r FileChooserRequest) FileChooserAnswer
	// FilesChosen says the user chose count files in the panel, from folder.
	FilesChosen(tab TabID, count int, folder string)
}

// FileChooserMode is what a page's file panel picks.
type FileChooserMode int

const (
	FileChooserOpen FileChooserMode = iota
	FileChooserOpenMultiple
	FileChooserFolder
	FileChooserSave
)

// FileChooserRequest is a page asking for files: its tab, what it picks, and the types
// it accepts, such as ".pdf" or "image/*", none for any.
type FileChooserRequest struct {
	Tab    TabID
	Mode   FileChooserMode
	Accept []string
}

// FileChooserAnswer says whether the panel opens, and in which folder, "" for the
// system's choice.
type FileChooserAnswer struct {
	Allow  bool   `json:"allow"`
	Folder string `json:"folder"`
}

// DownloadID is the engine's number for a download, for this run only.
type DownloadID uint32

// DownloadRequest is a download about to start: its tab, 0 for none, its address, the
// file name the engine suggests from its headers, its type, and whether it asked for the
// Save panel, as Save Link As does.
type DownloadRequest struct {
	ID     DownloadID
	Tab    TabID
	URL    string
	Name   string
	Mime   string
	SaveAs bool
}

// DownloadTarget is where a download goes, and whether the Save panel shows first,
// starting there.
type DownloadTarget struct {
	Path   string `json:"path"`
	Prompt bool   `json:"prompt"`
}

// DownloadState is where a download is.
type DownloadState int

const (
	DownloadProgress DownloadState = iota
	DownloadDone
	DownloadCancelled
	DownloadFailed
)

// Download is a download as the engine reports it. Total is -1 when unknown.
type Download struct {
	ID       DownloadID
	Path     string
	URL      string
	Mime     string
	State    DownloadState
	Received int64
	Total    int64
	Paused   bool
}

// DownloadCommand is what can be done to a running download.
type DownloadCommand int

const (
	DownloadCancel DownloadCommand = iota + 1
	DownloadPause
	DownloadResume
	DownloadRemove
)

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
	Bounds       Rect // The window's frame on screen, points, y down from the top of the main screen.
	Minimised    bool
	Zoomed       bool
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

// WindowMode is how a window sits on screen.
type WindowMode int

const (
	ModeNormal WindowMode = iota
	ModeMinimised
	ModeZoomed
	ModeFullScreen
)

// ContextMenu is what a long press or a right click in a page was on, at X, Y in the
// page: a link's address and words, an image's address (Media "image") or another
// medium's, and the selected text.
type ContextMenu struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Link      string  `json:"link"`
	LinkText  string  `json:"linkText"`
	Src       string  `json:"src"`
	Media     string  `json:"media"`
	Selection string  `json:"selection"`
	// Editable says the press was in a field.
	Editable bool `json:"editable"`
}
