// Package window owns how the native window behaves: which areas drag it, which belong
// to the shell, where the page goes, its frame, and requests to close, minimise, zoom or
// go full screen. It is the only manager that tells the engine about the window.
package window

import (
	"slices"

	"projectb/engine"
)

// ShellLayout is what the shell reports about itself: the rectangles its controls take,
// the empty spaces in the top row that drag the window, where the page goes, and the
// window size it measured against.
type ShellLayout struct {
	Shell []engine.Rect `json:"shell"`
	Drag  []engine.Rect `json:"drag"`
	Page  engine.Rect   `json:"page"`
	ViewW float64       `json:"viewW"`
	ViewH float64       `json:"viewH"`
}

// Manager owns the window.
type Manager struct {
	eng        engine.Engine
	color      string
	frameWidth float64
	shell      ShellLayout
	state      engine.WindowState
	sent       engine.WindowLayout
	onChanged  []func(engine.Rect, float64)
	onColor    []func(string)
}

// New returns a manager and gives the window its first layout, which carries the
// frame: color, "#rrggbb", 4px wide. The engine reports its state after it starts.
func New(eng engine.Engine, color string) *Manager {
	m := &Manager{eng: eng, color: color, frameWidth: 4}
	m.sent = m.layout()
	eng.SetWindowLayout(m.sent)
	return m
}

// layout is the one decision: drag areas first, then the shell's, and the page under
// everything else. In full screen there is no window to move, so nothing drags.
func (m *Manager) layout() engine.WindowLayout {
	l := engine.WindowLayout{
		ViewW:      m.shell.ViewW,
		ViewH:      m.shell.ViewH,
		Page:       m.shell.Page,
		FrameColor: m.color,
		FrameWidth: m.frameWidth,
	}
	if !m.state.FullScreen {
		for _, r := range m.shell.Drag {
			l.Regions = append(l.Regions, engine.Region{Rect: r, Area: engine.AreaDrag})
		}
	}
	for _, r := range m.shell.Shell {
		l.Regions = append(l.Regions, engine.Region{Rect: r, Area: engine.AreaShell})
	}
	return l
}

// update sends the layout when it differs from the last one sent.
func (m *Manager) update() {
	l := m.layout()
	if l.ViewW == m.sent.ViewW && l.ViewH == m.sent.ViewH && l.Page == m.sent.Page &&
		l.FrameColor == m.sent.FrameColor && l.FrameWidth == m.sent.FrameWidth &&
		slices.Equal(l.Regions, m.sent.Regions) {
		return
	}
	m.sent = l
	m.eng.SetWindowLayout(l)
}

// HandleShellLayout takes the shell's report.
func (m *Manager) HandleShellLayout(l ShellLayout) {
	m.shell = l
	m.update()
}

// HandleShellCrashed drops what the dead shell claimed, so the page takes every press
// until the reloaded shell reports again. The page stays where it was.
func (m *Manager) HandleShellCrashed() {
	m.shell.Shell = nil
	m.shell.Drag = nil
	m.update()
}

// HandleWindowChanged takes the engine's report.
func (m *Manager) HandleWindowChanged(s engine.WindowState) {
	old := m.state
	m.state = s
	m.update()
	if s.Controls != old.Controls || s.CornerRadius != old.CornerRadius {
		for _, f := range m.onChanged {
			f(s.Controls, s.CornerRadius)
		}
	}
}

// Request asks the window to close, minimise, zoom, or go in or out of full screen.
// The engine reports what came of it.
func (m *Manager) Request(r engine.WindowRequest) { m.eng.RequestWindow(r) }

// Color is the one colour the window paints natively and the shell paints its frame
// with, "#rrggbb".
func (m *Manager) Color() string { return m.color }

// OnColorChanged subscribes to the frame colour changing.
func (m *Manager) OnColorChanged(f func(string)) { m.onColor = append(m.onColor, f) }

// SetColor repaints the window's frame in color, and tells whoever subscribed, so the
// window and the shell change together.
func (m *Manager) SetColor(color string) {
	if color == m.color {
		return
	}
	m.color = color
	m.update()
	for _, f := range m.onColor {
		f(color)
	}
}

// FrameWidth is the width of the frame band, in CSS pixels, for the window and the
// shell alike.
func (m *Manager) FrameWidth() float64 { return m.frameWidth }

// Controls returns where the native window controls are. Empty when there are none.
func (m *Manager) Controls() engine.Rect { return m.state.Controls }

// CornerRadius returns the radius the OS rounds the window's corners with, in CSS
// pixels. 0 when the corners are square, as in full screen.
func (m *Manager) CornerRadius() float64 { return m.state.CornerRadius }

// OnControlsChanged subscribes to the controls moving, appearing or disappearing, and
// to the corner radius changing.
func (m *Manager) OnControlsChanged(f func(engine.Rect, float64)) {
	m.onChanged = append(m.onChanged, f)
}
