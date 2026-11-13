# One window manager

A design note, 2026-11-10, with the owner's decisions at the end. Built for macOS the
same day; Windows and Linux are described here only.

Every desktop asks one question about a mouse press in our window: does it move the
window, go to the shell, or go to the page? Each platform asks it differently. macOS asks
our views and then moves the window itself, in the window server, the system process
that owns every window on screen. Windows asks us about every point with a message
called WM_NCHITTEST and acts on our answer. Linux either lets the window manager do it
all, or, when the app draws its own title bar, the app tells the system "start moving"
or "start resizing" on a press. Today no single piece of our code owns that question.
This note proposes that the Go window manager does.

## Today

- The shell measures itself in `shell/src/regions.js` and sends three messages:
  `regions` (the rectangles its controls take, marked `v-shell`), `dragRegions` (the
  empty shell marked `v-drag`) and `pageBounds` (where the page goes, with the window
  size it measured against).
- The shell manager passes each straight to the engine, one call per message:
  `SetShellRegions`, `SetDragRegions` and `SetPageBounds` (`core/shell/shell.go`, lines
  125 to 130).
- The window manager, `core/window/window.go`, owns the frame colour, the frame width,
  where the native window controls are and the corner radius. It never talks to the
  engine.
- `core/app/app.go` line 49 calls `SetWindowFrame` once at startup with the colour and
  width it reads from the window manager.
- The engine reports `WindowControlsChanged(rect, radius)` after start, on resize and on
  full screen (`core/engine/engine.go`, line 133). Full screen is never said outright:
  it shows up as no controls and a radius of 0. The window's size is never reported.
- The rule itself, drag beats shell and shell beats the page, lives only in the engines:
  `PBContainerView hitTest:` in `engine/browser/window_mac.mm` and `Hit` in
  `core/mockengine/mockengine.go`, each written by hand. Go never sees it.

So four engine calls, three senders, and nobody owns the whole answer.

## Proposed

The window manager becomes the one owner of how the window behaves.

- **From the shell** it takes one layout report: the shell's rectangles, the empty
  spaces in the top row that drag the window, and the tabs it shows, each with its
  rectangle ([windows.md](windows.md)).
- **From the engine** it takes the window's size, where the controls are, the corner
  radius and whether it is in full screen.
- **It decides, in one place,** which areas drag, which belong to the shell, where the
  pages go, and the frame colour and width. In full screen it sends no drag areas,
  because there is no window to move.
- **It gives the engine one description** of the window, through one call that replaces
  the four.

The types, in `core/engine`:

```go
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

// WindowLayout is everything the engine needs to make the window behave.
type WindowLayout struct {
	ViewW, ViewH float64      // The window size this was made for; an engine ignores a stale one.
	Tabs         []VisibleTab // The tabs whose pages show, and where. Every other tab is hidden.
	Regions      []Region     // In order. The first that holds a point decides.
	FrameColor   string   // "#rrggbb", painted under the shell and the page.
	FrameWidth   float64  // The band drawn around the window's edge.
}

// At is the one hit test: what a press at x, y does.
func (l WindowLayout) At(x, y float64) Area

// WindowState is what the engine reports about its native window.
type WindowState struct {
	W, H         float64 // The content area, CSS pixels.
	Controls     Rect    // The native window buttons; empty when there are none.
	CornerRadius float64 // 0 when the corners are square.
	FullScreen   bool
}
```

On `Engine`, `SetWindowLayout(w WindowID, l WindowLayout)` replaces `SetPageBounds`,
`SetShellRegions`, `SetDragRegions` and `SetWindowFrame`. On `Listener`,
`WindowChanged(w WindowID, s WindowState)` replaces `WindowControlsChanged`. Each names
the window by Go's id for it ([windows.md](windows.md)). The order of `Regions`
is the decision: the manager puts the controls inside the drag bars first, as shell areas,
then the drag areas, then the other shell areas. Every engine only walks the list, so none
of them knows the rule.

## Per platform

- **macOS, today's code mapped.** `hitTest:` walks `Regions`: a drag area answers nil,
  so the press falls through to AppKit's frame; a shell area answers the shell's view;
  anything else answers the page. The window server moves the window by itself from the
  parts of the title bar no view claims (`_opaqueRectForWindowMoveWhenInTitlebar`), so
  the container claims everything but the drag areas, less what an earlier region holds,
  with one empty view per rectangle of the rest: the shell and the page are the app's, the
  drag areas the system's. The server looks only at the title bar, so below it, as in the
  toolbar under a tab strip, a press that falls through to the frame would do nothing:
  there the window hands the press to the server with `performWindowDragWithEvent:`, as
  Chrome's frame does, so a drag area moves the window wherever it is. Double-click to
  zoom and the resize edges stay with AppKit, and a drag area below the title bar zooms
  on a double-click as the title bar does. `ReportControls` grows into the `WindowState` report.
- **Windows.** The window procedure answers WM_NCHITTEST from the same walk: HTCAPTION
  for a drag area, which tells Windows "this is title bar", so it moves the window, snaps
  it and maximises on a double-click by itself; HTCLIENT for the shell and the page.
  The resize edges stay in the system frame. Chromium does this in
  `HWNDMessageHandler::OnNCHitTest` (`ui/views/win/hwnd_message_handler.cc`).
- **Linux, X11.** With the window manager's own title bar, there is nothing to do. When
  we draw our own, a press in a drag area sends the window manager a
  `_NET_WM_MOVERESIZE` message, which hands it the move. Chromium: `DoWMMoveResize` in
  `ui/base/x/x11_util.cc`.
- **Linux, Wayland.** A press in a drag area calls `xdg_toplevel.move`, and one on an
  edge `xdg_toplevel.resize`, each with the serial, the number the compositor gave that
  exact press, so it knows the request is real. Chromium:
  `ui/ozone/platform/wayland/host/xdg_toplevel.cc`. On Linux, a double-click on a drag
  area is ours to turn into maximise.

## What moves where

- **`core/shell/shell.go`:** the `regions`, `dragRegions` and `pageBounds` cases become
  one `layout` case, which hands the report to the window manager. The shell manager
  stops calling the engine for the window. It still sends the shell its snapshot and the
  `controls` message, with what the window manager tells it.
- **`core/app/app.go`:** `window.New` takes the engine and sends the first layout
  itself, so the frame colour and width still reach the window before it shows. The
  listener's `WindowChanged` forwards to the window manager, and so does `ShellCrashed`:
  the manager drops what the dead shell claimed, so the page takes every press until the
  reloaded shell reports again. The engine used to clear the shell's regions itself, out
  of Go's sight.
- **`core/window/window.go`:** keeps the last shell report and the last window state,
  builds a `WindowLayout` from both whenever either changes, and sends it only when it
  differs from the last one sent.
- **The shell:** `regions.js` sends one `layout` message instead of three. It knows
  nothing about platforms. Following your decision, only the empty spaces in the top row
  keep `v-drag`; the left, right and bottom edge strips in `App.vue` lose it, and so do
  the sidebar's empty part and, with the tabs on top, the bands beside the toolbar. The
  settings overlay is shell, so its rectangles are shell areas like any other.
- **The native engine:** one C entry, `set_window_layout(window, layout, regions,
  count, tabs, tab_count)`, replaces the four in `engine/abi/pb_abi.h`: a
  `PBWindowLayout` with the view size and the frame, and beside it the `PBRegion` list
  (x, y, w, h and a `PBArea`) and the `PBVisibleTab` list (a tab, x, y, w, h). `request_window` replaces
  `close_window`. `PB_EVENT_WINDOW_CHANGED` carries the full `WindowState`, and
  `PB_EVENT_SHELL_CRASHED` says the shell's renderer died. The window applies the frame
  only when it changed, and places the pages only when the view size still matches.
- **The mock engine:** keeps the last `WindowLayout`. Its `Hit` becomes `At` on that
  layout, so the mock and the tests use the same rule the real engines mirror. Its
  full-screen button reports a `WindowState`.
- **Tests:** `core/window` tests the decisions with a recording engine and no shell:
  a drag bar's controls come before the drag areas and those before the other shell areas,
  the first layout carries the frame, an unchanged
  layout isn't sent twice, and full screen sends no drag areas. `core/engine` tests `At`.
  `core/app`'s hit-test test sends one `layout` message and checks the same points.

## Decided (2026-11-10)

1. **Full screen and zoom are the window manager's.** The shell asks the manager, the
   manager tells the engine, and the engine reports the result in `WindowState`. There is
   one path; the shell never talks to the engine about the window. The same goes for
   minimise and close.
2. **The resize edges are the system's,** wherever it provides them. On macOS and Windows
   the manager never marks an edge. The one exception is Linux with our own title bar,
   where the Linux engine turns a press on an edge into "resize from this edge". The
   manager's contract is the same everywhere.
3. **On Linux we draw our own title bar,** everywhere, with one look and one path through
   the standard move and resize requests on X11 and Wayland. GNOME draws no frames for
   apps, so we would have to anyway.
4. **Window buttons are the system's** where it has them: macOS keeps its traffic lights,
   Windows its system-drawn buttons, snap layouts included. The engine reports where they
   are, or that there are none. Where there are none (Linux), the shell draws them, and
   they send close, minimise and zoom requests through the manager.

So the requests get one call of their own beside the description:

```go
// WindowRequest is what the user asked the window to do. Each is what pressing that
// window button does, so zoom and full screen go in or out.
type WindowRequest int

const (
	WindowClose WindowRequest = iota + 1
	WindowMinimise
	WindowZoom
	WindowFullScreen
)
```

`RequestWindow(w WindowID, r WindowRequest)` on `Engine` replaces `CloseWindow`: the tabs closing the
last tab and the updater relaunching ask the window manager too. The shell sends
`{"t":"window","text":"close" | "minimise" | "zoom" | "fullScreen"}`. On macOS nothing in
the shell sends one yet, since the system's buttons do the job.
