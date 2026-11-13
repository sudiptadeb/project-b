package content

import (
	"encoding/json"
	"errors"

	"projectb/engine"
	"projectb/timeline"
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
	eventProcessCrashed Hook = 11
	eventDevTools       Hook = 12
	eventWindowClosed   Hook = 13
	eventWindowFocused  Hook = 14
	eventMenuList       Hook = 15
	eventPickerMessage  Hook = 16
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
	eventProcessCrashed: "engine.ProcessCrashed",
	eventDevTools:       "engine.DevToolsCommand",
	eventWindowClosed:   "engine.WindowClosed",
	eventWindowFocused:  "engine.WindowFocused",
	eventMenuList:       "engine.MenuList",
	eventPickerMessage:  "engine.PickerMessage",
}

// terminationNames are how a child process ended, as engine.ProcessCrash says it. The
// engine reports only these.
var terminationNames = map[TerminationStatus]string{
	TerminationStatusAbnormalTermination: "exited abnormally",
	TerminationStatusProcessWasKilled:    "was killed",
	TerminationStatusProcessCrashed:      "crashed",
	TerminationStatusLaunchFailed:        "failed to launch",
	TerminationStatusOom:                 "ran out of memory",
	TerminationStatusEvictedForMemory:    "was evicted for memory",
}

// Events is what the adapter itself reports. Always on; there is nothing to subscribe.
type Events struct {
	OnShellMessage    func(w engine.WindowID, msg []byte)
	OnTabOpened       func(opener, tab engine.TabID, url, workspace string, foreground, newWindow bool)
	OnFaviconChanged  func(tab engine.TabID, dataURL string)
	OnWindowChanged   func(w engine.WindowID, s engine.WindowState)
	OnShellCrashed    func(w engine.WindowID)
	OnWindowClosed    func(w engine.WindowID)
	OnWindowFocused   func(w engine.WindowID)
	OnSettingsMessage func(w engine.WindowID, msg []byte)
	OnMenuChosen      func(action string)
	OnMenuList        func(name string) engine.Menu
	OnPickerMessage   func(w engine.WindowID, msg []byte)
	OnOSBindings      func(byTitle map[string]engine.Key)
	OnModsChanged     func(m engine.Mods)
	OnProcessCrashed  func(p engine.ProcessCrash)
	OnDevTools        func(method string, params []byte) (result any, err error)

	// more are the events a build tag adds, by number.
	more map[Hook]func(r *reader) (Value, bool)
}

func (e *Events) dispatch(id Hook, r *reader) (Value, bool) {
	switch id {
	case eventShellMessage:
		if e.OnShellMessage != nil {
			e.OnShellMessage(engine.WindowID(r.int()), []byte(r.str()))
		}
	case eventPickerMessage:
		if e.OnPickerMessage != nil {
			e.OnPickerMessage(engine.WindowID(r.int()), []byte(r.str()))
		}
	case eventTabOpened:
		if e.OnTabOpened != nil {
			e.OnTabOpened(engine.TabID(r.int()), engine.TabID(r.int()), r.str(), r.str(), r.bool(), r.bool())
		}
	case eventFaviconChanged:
		if e.OnFaviconChanged != nil {
			e.OnFaviconChanged(engine.TabID(r.int()), r.str())
		}
	case eventWindowChanged:
		if e.OnWindowChanged != nil {
			e.OnWindowChanged(engine.WindowID(r.int()), engine.WindowState{
				W:            r.float(),
				H:            r.float(),
				Controls:     engine.Rect{X: r.float(), Y: r.float(), W: r.float(), H: r.float()},
				CornerRadius: r.float(),
				FullScreen:   r.bool(),
				Bounds:       engine.Rect{X: r.float(), Y: r.float(), W: r.float(), H: r.float()},
				Minimised:    r.bool(),
				Zoomed:       r.bool(),
			})
		}
	case eventShellCrashed:
		if e.OnShellCrashed != nil {
			e.OnShellCrashed(engine.WindowID(r.int()))
		}
	case eventWindowClosed:
		if e.OnWindowClosed != nil {
			e.OnWindowClosed(engine.WindowID(r.int()))
		}
	case eventWindowFocused:
		if e.OnWindowFocused != nil {
			e.OnWindowFocused(engine.WindowID(r.int()))
		}
	case eventSettingsMsg:
		if e.OnSettingsMessage != nil {
			e.OnSettingsMessage(engine.WindowID(r.int()), []byte(r.str()))
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
			timeline.Warnf("content: bad OS bindings: %v", err)
		} else if e.OnOSBindings != nil {
			e.OnOSBindings(byTitle)
		}
	default:
		if f := e.more[id]; f != nil {
			return f(r)
		}
	case eventProcessCrashed:
		if e.OnProcessCrashed != nil {
			e.OnProcessCrashed(engine.ProcessCrash{
				Type:     r.str(),
				Status:   terminationNames[TerminationStatus(r.int())],
				ExitCode: int(r.int()),
				PID:      int(r.int()),
				URL:      r.str(),
			})
		}
	case eventMenuList:
		// The answer is the list's items now, as the menu bar's JSON has them.
		if e.OnMenuList == nil {
			return Value{}, false
		}
		b, err := json.Marshal(e.OnMenuList(r.str()))
		if err != nil {
			timeline.Errorf("content: a menu list: %v", err)
			return Value{}, false
		}
		return Value{S: string(b)}, true
	case eventDevTools:
		// The answer is the reply's body as DevTools has it: its result or its error.
		if e.OnDevTools == nil {
			return Value{}, false
		}
		method, params := r.str(), r.str()
		result, err := e.OnDevTools(method, []byte(params))
		if errors.Is(err, engine.ErrUnknownCommand) {
			return Value{}, false
		}
		body := map[string]any{"result": result}
		if err != nil {
			body = map[string]any{"error": map[string]any{"code": -32000, "message": err.Error()}}
		}
		b, err := json.Marshal(body)
		if err != nil {
			b, _ = json.Marshal(map[string]any{"error": map[string]any{"code": -32603, "message": err.Error()}})
		}
		return Value{S: string(b)}, true
	}
	return Value{}, false
}
