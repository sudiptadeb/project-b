package window

import "projectb/engine"

// State is how a window sits on screen.
type State string

const (
	StateNormal     State = "normal"
	StateMinimised  State = "minimised"
	StateZoomed     State = "zoomed"
	StateFullScreen State = "fullScreen"
)

// Window is what Go knows of one window, for another manager to read.
type Window struct {
	ID     engine.WindowID `json:"id"`
	Bounds engine.Rect     `json:"bounds"` // Its frame on screen, points, y down from the top of the main screen.
	State  State           `json:"state"`
	Front  bool            `json:"front"` // It is the window in front, the one commands act on.
}

// EventKind is what happened to a window.
type EventKind int

const (
	Opened        EventKind = iota + 1 // Window opened.
	Closed                             // Window closed.
	FrontChanged                       // Window is the one in front now.
	TabMoved                           // Tab moved to Window from window From, where it was at FromIndex.
	BoundsChanged                      // Window's frame on screen is somewhere else, or another size.
)

// Event is one thing that happened to the windows.
type Event struct {
	Kind      EventKind
	Window    engine.WindowID
	Tab       engine.TabID
	From      engine.WindowID
	FromIndex int
}

// OnEvent subscribes to windows opening and closing, the window in front changing, tabs
// moving between windows, and a window's frame changing: the one place to hear of them.
// A frame dragged across the screen changes as often as the engine reports it.
func (m *Manager) OnEvent(f func(Event)) { m.onEvent = append(m.onEvent, f) }

func (m *Manager) event(e Event) {
	for _, f := range m.onEvent {
		f(e)
	}
}

// HandleFrontChanged takes the tabs manager's word that window w is in front now.
func (m *Manager) HandleFrontChanged(w engine.WindowID) {
	m.event(Event{Kind: FrontChanged, Window: w})
}

// HandleTabMoved takes the tabs manager's word that a tab moved from its place in one
// window to another window.
func (m *Manager) HandleTabMoved(tab engine.TabID, from engine.WindowID, fromIndex int, to engine.WindowID) {
	m.event(Event{Kind: TabMoved, Window: to, Tab: tab, From: from, FromIndex: fromIndex})
}

// List returns the open windows, in the order they were opened.
func (m *Manager) List() []Window {
	var out []Window
	for _, id := range m.Windows() {
		w, _ := m.Get(id)
		out = append(out, w)
	}
	return out
}

// Get returns one window, and whether it is open.
func (m *Manager) Get(id engine.WindowID) (Window, bool) {
	w := m.windows[id]
	if w == nil {
		return Window{}, false
	}
	s := w.state
	state := StateNormal
	switch {
	case s.FullScreen:
		state = StateFullScreen
	case s.Minimised:
		state = StateMinimised
	case s.Zoomed:
		state = StateZoomed
	}
	return Window{ID: id, Bounds: s.Bounds, State: state, Front: id == m.front()}, true
}

// SetBounds puts a window's frame on screen at r. The engine reports what came of it.
func (m *Manager) SetBounds(id engine.WindowID, r engine.Rect) { m.eng.SetWindowBounds(id, r) }

// modes are the engine's word for each state.
var modes = map[State]engine.WindowMode{
	StateNormal:     engine.ModeNormal,
	StateMinimised:  engine.ModeMinimised,
	StateZoomed:     engine.ModeZoomed,
	StateFullScreen: engine.ModeFullScreen,
}

// SetState puts a window into a state outright, from whatever it is in now; nothing when
// it is in that state already. The engine reports what came of it.
func (m *Manager) SetState(id engine.WindowID, s State) {
	if w, _ := m.Get(id); w.State != s {
		m.eng.SetWindowMode(id, modes[s])
	}
}
