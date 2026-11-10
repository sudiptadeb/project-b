package content

import "projectb/engine"

// The adapter's own events: things our C++ does that content has no hook for. The
// numbers and value layouts match PBEvent in engine/abi/pb_abi.h.
const (
	eventShellMessage   Hook = 1
	eventTabOpened      Hook = 2
	eventFaviconChanged Hook = 3
	eventWindowChanged  Hook = 4
	eventPing           Hook = 5
	eventShellCrashed   Hook = 6
)

var eventNames = map[Hook]string{
	eventShellMessage:   "engine.ShellMessage",
	eventTabOpened:      "engine.TabOpened",
	eventFaviconChanged: "engine.FaviconChanged",
	eventWindowChanged:  "engine.WindowChanged",
	eventPing:           "engine.Ping",
	eventShellCrashed:   "engine.ShellCrashed",
}

// Events is what the adapter itself reports. Always on; there is nothing to subscribe.
type Events struct {
	OnShellMessage   func(msg []byte)
	OnTabOpened      func(opener, tab engine.TabID, url string, foreground bool)
	OnFaviconChanged func(tab engine.TabID, dataURL string)
	OnWindowChanged  func(s engine.WindowState)
	OnShellCrashed   func()
}

func (e *Events) dispatch(id Hook, r *reader) (Value, bool) {
	switch id {
	case eventShellMessage:
		if e.OnShellMessage != nil {
			e.OnShellMessage([]byte(r.str()))
		}
	case eventTabOpened:
		if e.OnTabOpened != nil {
			e.OnTabOpened(engine.TabID(r.int()), engine.TabID(r.int()), r.str(), r.bool())
		}
	case eventFaviconChanged:
		if e.OnFaviconChanged != nil {
			e.OnFaviconChanged(engine.TabID(r.int()), r.str())
		}
	case eventWindowChanged:
		if e.OnWindowChanged != nil {
			e.OnWindowChanged(engine.WindowState{
				W:            r.float(),
				H:            r.float(),
				Controls:     engine.Rect{X: r.float(), Y: r.float(), W: r.float(), H: r.float()},
				CornerRadius: r.float(),
				FullScreen:   r.bool(),
			})
		}
	case eventShellCrashed:
		if e.OnShellCrashed != nil {
			e.OnShellCrashed()
		}
	}
	return Value{}, false
}
