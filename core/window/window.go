// Package window owns what the app knows about its native window: today, where the
// window controls are.
package window

import "projectb/engine"

// Manager owns the window state.
type Manager struct {
	controls  engine.Rect
	onChanged []func(engine.Rect)
}

// New returns a manager that knows nothing yet. The engine reports the controls after
// it starts.
func New() *Manager { return &Manager{} }

// Controls returns where the native window controls are. Empty when there are none.
func (m *Manager) Controls() engine.Rect { return m.controls }

// OnControlsChanged subscribes to the controls moving, appearing or disappearing.
func (m *Manager) OnControlsChanged(f func(engine.Rect)) { m.onChanged = append(m.onChanged, f) }

// HandleControlsChanged records the engine's report.
func (m *Manager) HandleControlsChanged(r engine.Rect) {
	if m.controls == r {
		return
	}
	m.controls = r
	for _, f := range m.onChanged {
		f(r)
	}
}
