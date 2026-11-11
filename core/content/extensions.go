//go:build extensions

package content

import (
	"encoding/json"
	"log"

	"projectb/engine"
)

// The adapter's extension events. The numbers and value layouts match PBExtensionEvent in
// engine/abi/pb_extensions_abi.h.
const (
	eventExtensionsChanged      Hook = 50
	eventExtensionInstallPrompt Hook = 51
	eventExtensionInstallDone   Hook = 52
	eventExtensionCall          Hook = 53
	eventExtensionBlockedCount  Hook = 54
	eventExtensionPopupClosed   Hook = 55
	eventPathChosen             Hook = 56
)

// WireExtensions turns the adapter's extension events into the listener's calls.
func WireExtensions(h *Hooks, l engine.ExtensionsListener) {
	on := func(id Hook, name string, f func(r *reader)) {
		eventNames[id] = name
		h.Events.more[id] = func(r *reader) (Value, bool) {
			f(r)
			return Value{}, false
		}
	}
	h.Events.more = map[Hook]func(r *reader) (Value, bool){}
	on(eventExtensionsChanged, "engine.ExtensionsChanged", func(*reader) { l.ExtensionsChanged() })
	on(eventExtensionInstallPrompt, "engine.ExtensionInstallPrompt", func(r *reader) {
		p := engine.InstallPrompt{ID: r.int()}
		if err := json.Unmarshal([]byte(r.str()), &p); err != nil {
			log.Printf("content: bad install prompt: %v", err)
			return
		}
		l.ExtensionInstallPrompt(p)
	})
	on(eventExtensionInstallDone, "engine.ExtensionInstallDone", func(r *reader) {
		l.ExtensionInstallDone(engine.InstallResult{Path: r.str(), Extension: r.str(), Error: r.str(), Cancelled: r.bool()})
	})
	on(eventExtensionBlockedCount, "engine.ExtensionBlockedCount", func(r *reader) {
		l.ExtensionBlockedCount(r.str(), engine.TabID(r.int()), int(r.int()))
	})
	on(eventExtensionPopupClosed, "engine.ExtensionPopupClosed", func(*reader) { l.ExtensionPopupClosed() })
	on(eventPathChosen, "engine.PathChosen", func(r *reader) { l.PathChosen(r.str()) })

	// The one event with an answer: {"result": ...}, {} for none, or {"error": "..."}.
	eventNames[eventExtensionCall] = "engine.ExtensionCall"
	h.Events.more[eventExtensionCall] = func(r *reader) (Value, bool) {
		result, failure := l.ExtensionCall(engine.ExtensionCall{Extension: r.str(), Name: r.str(), Args: r.str(), Tab: engine.TabID(r.int())})
		answer := struct {
			Result json.RawMessage `json:"result,omitempty"`
			Error  string          `json:"error,omitempty"`
		}{json.RawMessage(result), failure}
		b, _ := json.Marshal(answer)
		return Value{S: string(b)}, true
	}
}
