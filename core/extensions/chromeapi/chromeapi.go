//go:build extensions

// Package chromeapi answers the chrome.* calls that are about the browser itself: tabs,
// windows, action and commands. Chromium's extension runtime carries a call here with
// its name and its arguments as JSON, and this package answers from Go's own tabs and
// windows, in the shapes Chrome's API documents. Everything shaped by Chrome's API is in
// this package and nowhere else in core/.
package chromeapi

import (
	"encoding/json"
	"slices"
	"time"

	"projectb/engine"
	"projectb/extensions"
	"projectb/tabs"
	"projectb/window"
)

// Chrome's API takes -2 for "the current window".
const currentWindow = -2

// Opener opens windows, as the app does.
type Opener interface {
	NewWindow(u string) (engine.WindowID, engine.TabID)
	NewWindowWith(tab engine.TabID) engine.WindowID
}

// API answers calls and fires events.
type API struct {
	eng     engine.Extensions
	tabs    *tabs.Manager
	windows *window.Manager
	open    Opener
	ext     *extensions.Manager

	// What the extensions were last told about the tabs: each tab, the window it is in,
	// and each window's active tab.
	known    map[engine.TabID]tabs.Tab
	knownIn  map[engine.TabID]engine.WindowID
	active   map[engine.WindowID]engine.TabID
	accessed map[engine.TabID]time.Time
}

// New returns an API. The caller forwards the tab manager's changes to TabChanged and
// TabsChanged, the window manager's events to WindowEvent, and button clicks to Clicked.
func New(eng engine.Extensions, t *tabs.Manager, w *window.Manager, open Opener, ext *extensions.Manager) *API {
	return &API{
		eng: eng, tabs: t, windows: w, open: open, ext: ext,
		known: map[engine.TabID]tabs.Tab{}, knownIn: map[engine.TabID]engine.WindowID{},
		active: map[engine.WindowID]engine.TabID{}, accessed: map[engine.TabID]time.Time{},
	}
}

// call is one API function: the caller, and its arguments, each nil when left out.
type call struct {
	engine.ExtensionCall
	args []json.RawMessage
}

// arg decodes argument i into v, and says whether the caller gave it.
func (c call) arg(i int, v any) bool {
	if i >= len(c.args) || c.args[i] == nil || string(c.args[i]) == "null" {
		return false
	}
	return json.Unmarshal(c.args[i], v) == nil
}

var functions = map[string]func(a *API, c call) (any, error){}

// Call answers one call: its result as JSON, "" for none, or an error. A function this
// package doesn't have gets an error, as a call Chrome refuses does.
func (a *API) Call(c engine.ExtensionCall) (string, string) {
	f := functions[c.Name]
	if f == nil {
		return "", c.Name + " is not available in Project B"
	}
	var args []json.RawMessage
	json.Unmarshal([]byte(c.Args), &args)
	result, err := f(a, call{c, args})
	if err != nil {
		return "", err.Error()
	}
	if result == nil {
		return "", ""
	}
	b, _ := json.Marshal(result)
	return string(b), ""
}

// dispatch fires an event in one extension.
func (a *API) dispatch(id, event string, args ...any) {
	b, _ := json.Marshal(args)
	a.eng.DispatchExtensionEvent(id, event, string(b))
}

// seesTabs says whether an extension may read tabs' addresses and titles: it has the
// "tabs" permission or asks for any host. The engine itself, which asks with no
// extension for the tab a message came from, sees them too.
func seesTabs(e engine.Extension) bool {
	return e.ID == "" || slices.Contains(e.Permissions, "tabs") || len(e.Hosts) > 0
}

// enabled is the extensions events go to.
func (a *API) enabled() []engine.Extension {
	var out []engine.Extension
	for _, e := range a.ext.Installed() {
		if e.Enabled {
			out = append(out, e)
		}
	}
	return out
}

func (a *API) caller(c call) engine.Extension {
	e, _ := a.ext.Get(c.Extension)
	return e
}
