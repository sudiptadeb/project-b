package mockengine

import "projectb/engine"

// ExitPageFullScreen ends a page's full screen as content does: the page hears it, and
// the app is told.
func (e *Engine) ExitPageFullScreen(tab engine.TabID) {
	e.Post(func() { e.listener.PageFullScreen(tab, false) })
}
