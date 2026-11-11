//go:build extensions

package chromeapi

import "fmt"

// window is chrome.windows.Window, for the one window there is.
type window struct {
	ID          int    `json:"id"`
	Focused     bool   `json:"focused"`
	Incognito   bool   `json:"incognito"`
	AlwaysOnTop bool   `json:"alwaysOnTop"`
	Type        string `json:"type"`
	State       string `json:"state"`
	Tabs        []tab  `json:"tabs,omitempty"`
}

func (a *API) window(c call, populate bool) window {
	w := window{ID: windowID, Focused: true, Type: "normal", State: "normal"}
	if a.fullScreen() {
		w.State = "fullscreen"
	}
	if populate {
		e := a.caller(c)
		for _, t := range a.tabs.All() {
			w.Tabs = append(w.Tabs, a.tabFor(e, t))
		}
	}
	return w
}

// windowArg checks that argument i names the one window.
func windowArg(c call, i int) error {
	var id int
	if c.arg(i, &id) && id != windowID && id != currentWindow {
		return fmt.Errorf("No window with id: %d.", id)
	}
	return nil
}

type getInfo struct {
	Populate bool `json:"populate"`
}

func init() {
	functions["windows.get"] = func(a *API, c call) (any, error) {
		if err := windowArg(c, 0); err != nil {
			return nil, err
		}
		var q getInfo
		c.arg(1, &q)
		return a.window(c, q.Populate), nil
	}
	current := func(a *API, c call) (any, error) {
		var q getInfo
		c.arg(0, &q)
		return a.window(c, q.Populate), nil
	}
	functions["windows.getCurrent"] = current
	functions["windows.getLastFocused"] = current
	functions["windows.getAll"] = func(a *API, c call) (any, error) {
		var q getInfo
		c.arg(0, &q)
		return []window{a.window(c, q.Populate)}, nil
	}
	// There is one window, so a new window's pages open as tabs in it.
	functions["windows.create"] = func(a *API, c call) (any, error) {
		var p struct {
			URL any `json:"url"`
		}
		c.arg(0, &p)
		switch u := p.URL.(type) {
		case string:
			a.tabs.OpenIn(resolve(c.Extension, u), true)
		case []any:
			for _, one := range u {
				if s, ok := one.(string); ok {
					a.tabs.OpenIn(resolve(c.Extension, s), true)
				}
			}
		}
		return a.window(c, true), nil
	}
	// The one window is always focused, and its size and state are the user's.
	functions["windows.update"] = func(a *API, c call) (any, error) {
		if err := windowArg(c, 0); err != nil {
			return nil, err
		}
		return a.window(c, false), nil
	}
}
