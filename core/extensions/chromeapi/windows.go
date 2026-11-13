//go:build extensions

package chromeapi

import (
	"fmt"
	"math"

	"projectb/engine"
	"projectb/window"
)

// windowView is chrome.windows.Window. Its place on screen is in points, from the top left of
// the main screen, as Chrome's is.
type windowView struct {
	ID          engine.WindowID `json:"id"`
	Focused     bool            `json:"focused"`
	Incognito   bool            `json:"incognito"`
	AlwaysOnTop bool            `json:"alwaysOnTop"`
	Type        string          `json:"type"`
	State       string          `json:"state"`
	Left        int             `json:"left"`
	Top         int             `json:"top"`
	Width       int             `json:"width"`
	Height      int             `json:"height"`
	Tabs        []tab           `json:"tabs,omitempty"`
}

// Chrome's words for a window's state, and Go's for each.
var (
	chromeState = map[window.State]string{
		window.StateNormal:     "normal",
		window.StateMinimised:  "minimized",
		window.StateZoomed:     "maximized",
		window.StateFullScreen: "fullscreen",
	}
	goState = map[string]window.State{
		"normal":            window.StateNormal,
		"minimized":         window.StateMinimised,
		"maximized":         window.StateZoomed,
		"fullscreen":        window.StateFullScreen,
		"locked-fullscreen": window.StateFullScreen,
	}
)

func (a *API) windowFor(e engine.Extension, w window.Window, populate bool) windowView {
	r := w.Bounds
	out := windowView{
		ID: w.ID, Focused: w.Front, Type: "normal", State: chromeState[w.State],
		Left: int(math.Round(r.X)), Top: int(math.Round(r.Y)),
		Width: int(math.Round(r.W)), Height: int(math.Round(r.H)),
	}
	if populate {
		for _, t := range a.tabs.AllIn(w.ID) {
			out.Tabs = append(out.Tabs, a.tabFor(e, t))
		}
	}
	return out
}

// current is the caller's current window: the one its tab is in, or for a popup or a
// background script the window in front.
func (a *API) current(c call) engine.WindowID {
	if w := a.tabs.WindowOf(c.Tab); w != 0 {
		return w
	}
	return a.tabs.Front()
}

// windowArg is the window argument i names, the current one for -2, or the current one
// when the caller left it out.
func (a *API) windowArg(c call, i int) (window.Window, error) {
	id := engine.WindowID(currentWindow)
	c.arg(i, &id)
	if id == currentWindow {
		id = a.current(c)
	}
	w, ok := a.windows.Get(id)
	if !ok {
		return w, fmt.Errorf("No window with id: %d.", id)
	}
	return w, nil
}

type getInfo struct {
	Populate bool `json:"populate"`
}

// place is where windows.create and windows.update put a window; each part left out
// stays as it is.
type place struct {
	Left    *float64 `json:"left"`
	Top     *float64 `json:"top"`
	Width   *float64 `json:"width"`
	Height  *float64 `json:"height"`
	Focused *bool    `json:"focused"`
	State   string   `json:"state"`
}

// apply moves, sizes and shows window w as p says. A size or place goes only with the
// normal state, as in Chrome.
func (a *API) apply(w engine.WindowID, p place) error {
	moves := p.Left != nil || p.Top != nil || p.Width != nil || p.Height != nil
	s, ok := goState[p.State]
	if p.State != "" && !ok {
		return fmt.Errorf("Invalid value for state: %s.", p.State)
	}
	if moves && p.State != "" && s != window.StateNormal {
		return fmt.Errorf("Invalid value for state")
	}
	if p.State != "" {
		a.windows.SetState(w, s)
	}
	if moves {
		cur, _ := a.windows.Get(w)
		r := cur.Bounds
		for _, f := range []struct {
			from *float64
			to   *float64
		}{{p.Left, &r.X}, {p.Top, &r.Y}, {p.Width, &r.W}, {p.Height, &r.H}} {
			if f.from != nil {
				*f.to = *f.from
			}
		}
		a.windows.SetBounds(w, r)
	}
	if p.Focused != nil && *p.Focused {
		a.windows.Request(w, engine.WindowFront)
	}
	return nil
}

func init() {
	functions["windows.get"] = func(a *API, c call) (any, error) {
		w, err := a.windowArg(c, 0)
		if err != nil {
			return nil, err
		}
		var q getInfo
		c.arg(1, &q)
		return a.windowFor(a.caller(c), w, q.Populate), nil
	}
	functions["windows.getCurrent"] = func(a *API, c call) (any, error) {
		var q getInfo
		c.arg(0, &q)
		w, _ := a.windows.Get(a.current(c))
		return a.windowFor(a.caller(c), w, q.Populate), nil
	}
	functions["windows.getLastFocused"] = func(a *API, c call) (any, error) {
		var q getInfo
		c.arg(0, &q)
		w, _ := a.windows.Get(a.tabs.Front())
		return a.windowFor(a.caller(c), w, q.Populate), nil
	}
	functions["windows.getAll"] = func(a *API, c call) (any, error) {
		var q getInfo
		c.arg(0, &q)
		e := a.caller(c)
		out := []windowView{}
		for _, w := range a.windows.List() {
			out = append(out, a.windowFor(e, w, q.Populate))
		}
		return out, nil
	}
	// A new window holds a tab moved out of another, or the pages asked for, or a new tab.
	// Every window is a normal one: a popup type opens as a normal window.
	functions["windows.create"] = func(a *API, c call) (any, error) {
		var p struct {
			place
			URL   any           `json:"url"`
			TabID *engine.TabID `json:"tabId"`
		}
		c.arg(0, &p)
		var urls []string
		switch u := p.URL.(type) {
		case string:
			urls = []string{resolve(c.Extension, u)}
		case []any:
			for _, one := range u {
				if s, ok := one.(string); ok {
					urls = append(urls, resolve(c.Extension, s))
				}
			}
		}
		var w engine.WindowID
		switch {
		case p.TabID != nil:
			if _, ok := a.tabs.Get(*p.TabID); !ok {
				return nil, fmt.Errorf("No tab with id: %d.", *p.TabID)
			}
			w = a.open.NewWindowWith(*p.TabID)
		case len(urls) == 0:
			w, _ = a.open.NewWindow("")
		default:
			w, _ = a.open.NewWindow(urls[0])
			for _, u := range urls[1:] {
				a.tabs.Load(a.tabs.CreateIn(w, ""), u)
			}
		}
		if err := a.apply(w, p.place); err != nil {
			return nil, err
		}
		win, _ := a.windows.Get(w)
		return a.windowFor(a.caller(c), win, true), nil
	}
	functions["windows.update"] = func(a *API, c call) (any, error) {
		w, err := a.windowArg(c, 0)
		if err != nil {
			return nil, err
		}
		var p place
		c.arg(1, &p)
		if err := a.apply(w.ID, p); err != nil {
			return nil, err
		}
		w, _ = a.windows.Get(w.ID)
		return a.windowFor(a.caller(c), w, false), nil
	}
	functions["windows.remove"] = func(a *API, c call) (any, error) {
		w, err := a.windowArg(c, 0)
		if err != nil {
			return nil, err
		}
		a.windows.Request(w.ID, engine.WindowClose)
		return nil, nil
	}
}

// WindowEvent tells the extensions what happened to the windows: one opened, closed, came
// to the front or moved, or a tab moved from one window to another.
func (a *API) WindowEvent(ev window.Event) {
	w, _ := a.windows.Get(ev.Window)
	for _, e := range a.enabled() {
		switch ev.Kind {
		case window.Opened:
			a.dispatch(e.ID, "windows.onCreated", a.windowFor(e, w, false))
		case window.Closed:
			a.dispatch(e.ID, "windows.onRemoved", ev.Window)
		case window.FrontChanged:
			a.dispatch(e.ID, "windows.onFocusChanged", ev.Window)
		case window.BoundsChanged:
			a.dispatch(e.ID, "windows.onBoundsChanged", a.windowFor(e, w, false))
		case window.TabMoved:
			a.dispatch(e.ID, "tabs.onDetached", ev.Tab, map[string]any{"oldWindowId": ev.From, "oldPosition": ev.FromIndex})
			a.dispatch(e.ID, "tabs.onAttached", ev.Tab, map[string]any{"newWindowId": ev.Window, "newPosition": a.index(ev.Tab)})
		}
	}
}
