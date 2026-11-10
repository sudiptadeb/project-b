package content

import (
	"encoding/json"
	"log"

	"projectb/engine"
)

// The adapter's own events: things our C++ does that content has no hook for. The
// numbers and value layouts match PBEvent in engine/abi/pb_abi.h.
const (
	eventShellMessage   Hook = 1
	eventTabOpened      Hook = 2
	eventFaviconChanged Hook = 3
	eventWindowChanged  Hook = 4
	eventPing           Hook = 5
	eventShellCrashed   Hook = 6
	eventSettingsMsg    Hook = 7
	eventMenuChosen     Hook = 8
	eventOSBindings     Hook = 9
	eventModsChanged    Hook = 10
)

var eventNames = map[Hook]string{
	eventShellMessage:   "engine.ShellMessage",
	eventTabOpened:      "engine.TabOpened",
	eventFaviconChanged: "engine.FaviconChanged",
	eventWindowChanged:  "engine.WindowChanged",
	eventPing:           "engine.Ping",
	eventShellCrashed:   "engine.ShellCrashed",
	eventSettingsMsg:    "engine.SettingsMessage",
	eventMenuChosen:     "engine.MenuChosen",
	eventOSBindings:     "engine.OSBindings",
	eventModsChanged:    "engine.ModsChanged",
}

// Events is what the adapter itself reports. Always on; there is nothing to subscribe.
type Events struct {
	OnShellMessage    func(msg []byte)
	OnTabOpened       func(opener, tab engine.TabID, url string, foreground bool)
	OnFaviconChanged  func(tab engine.TabID, dataURL string)
	OnWindowChanged   func(s engine.WindowState)
	OnShellCrashed    func()
	OnSettingsMessage func(msg []byte)
	OnMenuChosen      func(action string)
	OnOSBindings      func(byTitle map[string]engine.Key)
	OnModsChanged     func(m engine.Mods)
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
	case eventSettingsMsg:
		if e.OnSettingsMessage != nil {
			e.OnSettingsMessage([]byte(r.str()))
		}
	case eventMenuChosen:
		if e.OnMenuChosen != nil {
			e.OnMenuChosen(r.str())
		}
	case eventModsChanged:
		if e.OnModsChanged != nil {
			e.OnModsChanged(engine.Mods(r.int()))
		}
	case eventOSBindings:
		// JSON: each title with the key's Windows virtual-key code and our modifiers.
		var byTitle map[string]engine.Key
		if err := json.Unmarshal([]byte(r.str()), &byTitle); err != nil {
			log.Printf("content: bad OS bindings: %v", err)
		} else if e.OnOSBindings != nil {
			e.OnOSBindings(byTitle)
		}
	}
	return Value{}, false
}
