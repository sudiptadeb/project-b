// Package window owns what the app knows about its native window: where the window
// controls are, and how round its corners are.
package window

import "projectb/engine"

// Manager owns the window state.
type Manager struct {
	controls     engine.Rect
	cornerRadius float64
	onChanged    []func(engine.Rect, float64)
}

// New returns a manager that knows nothing yet. The engine reports the controls and
// the corner radius after it starts.
func New() *Manager { return &Manager{} }

// Controls returns where the native window controls are. Empty when there are none.
func (m *Manager) Controls() engine.Rect { return m.controls }

// CornerRadius returns the radius the OS rounds the window's corners with, in CSS
// pixels. 0 when the corners are square, as in full screen.
func (m *Manager) CornerRadius() float64 { return m.cornerRadius }

// OnControlsChanged subscribes to the controls moving, appearing or disappearing, and
// to the corner radius changing.
func (m *Manager) OnControlsChanged(f func(engine.Rect, float64)) {
	m.onChanged = append(m.onChanged, f)
}

// HandleControlsChanged records the engine's report.
func (m *Manager) HandleControlsChanged(r engine.Rect, cornerRadius float64) {
	if m.controls == r && m.cornerRadius == cornerRadius {
		return
	}
	m.controls = r
	m.cornerRadius = cornerRadius
	for _, f := range m.onChanged {
		f(r, cornerRadius)
	}
}
