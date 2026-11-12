// Package window owns how each native window behaves: which areas drag it, which belong
// to its shell, which tabs it shows and where, its frame, and requests to close, minimise, zoom or
// go full screen. It is the only manager that tells the engine about the windows.
package window

import (
	"maps"
	"slices"

	"projectb/engine"
)

// ShellLayout is what the shell reports about itself: the rectangles its controls take,
// the empty spaces in the top row that drag the window, the tabs it shows and where,
// and the window size it measured against.
type ShellLayout struct {
	Shell []engine.Rect       `json:"shell"`
	Drag  []engine.Rect       `json:"drag"`
	Tabs  []engine.VisibleTab `json:"tabs"`
	ViewW float64             `json:"viewW"`
	ViewH float64             `json:"viewH"`
}

// Manager owns the windows.
type Manager struct {
	eng        engine.Engine
	color      string
	theme      engine.Theme
	frameWidth float64
	windows    map[engine.WindowID]*window
	onChanged  []func(engine.WindowID, engine.Rect, float64)
	onColor    []func(string)
}

// window is what the manager keeps for one window: the shell's last report, the
// engine's last state, and the layout last sent.
type window struct {
	shell ShellLayout
	state engine.WindowState
	sent  engine.WindowLayout
}

// New returns a manager with no windows. Every window it opens has the frame, color,
// "#rrggbb", 4px wide, and the theme.
func New(eng engine.Engine, color string, theme engine.Theme) *Manager {
	return &Manager{eng: eng, color: color, theme: theme, frameWidth: 4, windows: map[engine.WindowID]*window{}}
}

// Open makes window id, a little down and right of window near, or where a first window
// goes when near is 0. Its first layout carries the frame. The engine reports its state
// after it starts.
func (m *Manager) Open(id, near engine.WindowID) {
	w := &window{}
	m.windows[id] = w
	w.sent = m.layout(w)
	m.eng.CreateWindow(id, near, w.sent)
}

// Close forgets a window that closed.
func (m *Manager) Close(id engine.WindowID) { delete(m.windows, id) }

// Windows returns the open windows, in the order they were opened.
func (m *Manager) Windows() []engine.WindowID {
	ids := slices.Collect(maps.Keys(m.windows))
	slices.Sort(ids)
	return ids
}

// layout is the one decision: drag areas first, then the shell's, and the pages under
// everything else. In full screen there is no window to move, so nothing drags.
func (m *Manager) layout(w *window) engine.WindowLayout {
	l := engine.WindowLayout{
		ViewW:      w.shell.ViewW,
		ViewH:      w.shell.ViewH,
		Tabs:       w.shell.Tabs,
		FrameColor: m.color,
		FrameWidth: m.frameWidth,
		Theme:      m.theme,
	}
	if !w.state.FullScreen {
		for _, r := range w.shell.Drag {
			l.Regions = append(l.Regions, engine.Region{Rect: r, Area: engine.AreaDrag})
		}
	}
	for _, r := range w.shell.Shell {
		l.Regions = append(l.Regions, engine.Region{Rect: r, Area: engine.AreaShell})
	}
	return l
}

// update sends a window's layout when it differs from the last one sent.
func (m *Manager) update(id engine.WindowID) {
	w := m.windows[id]
	l := m.layout(w)
	if l.ViewW == w.sent.ViewW && l.ViewH == w.sent.ViewH && slices.Equal(l.Tabs, w.sent.Tabs) &&
		l.FrameColor == w.sent.FrameColor && l.FrameWidth == w.sent.FrameWidth && l.Theme == w.sent.Theme &&
		slices.Equal(l.Regions, w.sent.Regions) {
		return
	}
	w.sent = l
	m.eng.SetWindowLayout(id, l)
}

// updateAll sends every window's layout that changed.
func (m *Manager) updateAll() {
	for _, id := range m.Windows() {
		m.update(id)
	}
}

// HandleShellLayout takes a window's shell's report.
func (m *Manager) HandleShellLayout(id engine.WindowID, l ShellLayout) {
	m.windows[id].shell = l
	m.update(id)
}

// HandleShellCrashed drops what the dead shell claimed, so the pages take every press
// until the reloaded shell reports again. The pages stay where they were.
func (m *Manager) HandleShellCrashed(id engine.WindowID) {
	w := m.windows[id]
	w.shell.Shell = nil
	w.shell.Drag = nil
	m.update(id)
}

// HandleWindowChanged takes the engine's report about a window.
func (m *Manager) HandleWindowChanged(id engine.WindowID, s engine.WindowState) {
	w := m.windows[id]
	old := w.state
	w.state = s
	m.update(id)
	if s.Controls != old.Controls || s.CornerRadius != old.CornerRadius {
		for _, f := range m.onChanged {
			f(id, s.Controls, s.CornerRadius)
		}
	}
}

// FullScreen says whether a window is in full screen. No window, as after the last one
// closed, is not.
func (m *Manager) FullScreen(id engine.WindowID) bool {
	w := m.windows[id]
	return w != nil && w.state.FullScreen
}

// Request asks a window to close, minimise, zoom, or go in or out of full screen.
// The engine reports what came of it.
func (m *Manager) Request(id engine.WindowID, r engine.WindowRequest) { m.eng.RequestWindow(id, r) }

// Color is the one colour every window paints natively and the shell paints its frame
// with, "#rrggbb".
func (m *Manager) Color() string { return m.color }

// OnColorChanged subscribes to the frame colour changing.
func (m *Manager) OnColorChanged(f func(string)) { m.onColor = append(m.onColor, f) }

// SetColor repaints every window's frame in color, and tells whoever subscribed, so the
// windows and the shells change together.
func (m *Manager) SetColor(color string) {
	if color == m.color {
		return
	}
	m.color = color
	m.updateAll()
	for _, f := range m.onColor {
		f(color)
	}
}

// SetTheme draws the windows' own parts light, dark or as the system does.
func (m *Manager) SetTheme(t engine.Theme) {
	m.theme = t
	m.updateAll()
}

// FrameWidth is the width of the frame band, in CSS pixels, for the windows and the
// shells alike.
func (m *Manager) FrameWidth() float64 { return m.frameWidth }

// Controls returns where a window's native controls are. Empty when there are none.
func (m *Manager) Controls(id engine.WindowID) engine.Rect { return m.windows[id].state.Controls }

// CornerRadius returns the radius the OS rounds a window's corners with, in CSS
// pixels. 0 when the corners are square, as in full screen.
func (m *Manager) CornerRadius(id engine.WindowID) float64 { return m.windows[id].state.CornerRadius }

// OnControlsChanged subscribes to a window's controls moving, appearing or disappearing,
// and to its corner radius changing.
func (m *Manager) OnControlsChanged(f func(engine.WindowID, engine.Rect, float64)) {
	m.onChanged = append(m.onChanged, f)
}
