//go:build extensions

package chromeapi

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"projectb/engine"
	"projectb/tabs"
)

// tab is chrome.tabs.Tab. Address, title and icon go only to an extension that may see them.
type tab struct {
	ID              engine.TabID `json:"id"`
	Index           int          `json:"index"`
	GroupID         int          `json:"groupId"`
	WindowID        int          `json:"windowId"`
	Selected        bool         `json:"selected"`
	LastAccessed    float64      `json:"lastAccessed"`
	Highlighted     bool         `json:"highlighted"`
	Active          bool         `json:"active"`
	Pinned          bool         `json:"pinned"`
	Frozen          bool         `json:"frozen"`
	Discarded       bool         `json:"discarded"`
	AutoDiscardable bool         `json:"autoDiscardable"`
	Incognito       bool         `json:"incognito"`
	Status          string       `json:"status"`
	URL             string       `json:"url,omitempty"`
	Title           string       `json:"title,omitempty"`
	FavIconURL      string       `json:"favIconUrl,omitempty"`
}

func status(t tabs.Tab) string {
	if t.Loading {
		return "loading"
	}
	return "complete"
}

// index is a tab's place in its window.
func (a *API) index(id engine.TabID) int {
	return slices.IndexFunc(a.tabs.AllIn(a.tabs.WindowOf(id)), func(o tabs.Tab) bool { return o.ID == id })
}

func (a *API) tabFor(e engine.Extension, t tabs.Tab) tab {
	w := a.tabs.WindowOf(t.ID)
	active := t.ID == a.tabs.ActiveIn(w)
	out := tab{
		ID: t.ID, Index: a.index(t.ID),
		GroupID: -1, WindowID: int(w), Selected: active, Highlighted: active, Active: active,
		LastAccessed:    float64(a.accessed[t.ID].UnixMilli()),
		AutoDiscardable: true, Status: status(t),
	}
	if seesTabs(e) {
		out.URL, out.Title, out.FavIconURL = t.URL, t.Title, t.Favicon
	}
	return out
}

// tabArg is the tab argument i names, or the active tab when the caller left it out.
func (a *API) tabArg(c call, i int) (tabs.Tab, error) {
	var id engine.TabID
	if !c.arg(i, &id) {
		id = a.tabs.ActiveID()
	}
	t, ok := a.tabs.Get(id)
	if !ok {
		return t, fmt.Errorf("No tab with id: %d.", id)
	}
	return t, nil
}

// resolve turns a URL an extension gave, which may be a path inside itself, into a
// whole URL.
func resolve(extension, u string) string {
	if strings.Contains(u, ":") {
		return u
	}
	return "chrome-extension://" + extension + "/" + strings.TrimPrefix(u, "/")
}

func init() {
	functions["tabs.get"] = func(a *API, c call) (any, error) {
		var id engine.TabID
		c.arg(0, &id)
		t, ok := a.tabs.Get(id)
		if !ok {
			return nil, fmt.Errorf("No tab with id: %d.", id)
		}
		return a.tabFor(a.caller(c), t), nil
	}
	// The tab the caller runs in. A popup or a background script is in none.
	functions["tabs.getCurrent"] = func(a *API, c call) (any, error) {
		t, ok := a.tabs.Get(c.Tab)
		if !ok {
			return nil, nil
		}
		return a.tabFor(a.caller(c), t), nil
	}
	// The engine takes the picture of the tab named here, if the caller may see its page.
	functions["tabs.captureVisibleTab"] = func(a *API, c call) (any, error) {
		w, err := a.windowArg(c, 0)
		if err != nil {
			return nil, err
		}
		active := a.tabs.ActiveIn(w.ID)
		if _, ok := a.tabs.Get(active); !ok {
			return nil, fmt.Errorf("No active web contents to capture")
		}
		return map[string]engine.TabID{"tab": active}, nil
	}
	functions["tabs.query"] = func(a *API, c call) (any, error) {
		var q struct {
			Active            *bool            `json:"active"`
			CurrentWindow     *bool            `json:"currentWindow"`
			LastFocusedWindow *bool            `json:"lastFocusedWindow"`
			Highlighted       *bool            `json:"highlighted"`
			Pinned            *bool            `json:"pinned"`
			Status            *string          `json:"status"`
			WindowID          *engine.WindowID `json:"windowId"`
			Index             *int             `json:"index"`
			Title             *string          `json:"title"`
			URL               any              `json:"url"`
		}
		c.arg(0, &q)
		e := a.caller(c)
		// url is one match pattern or a list of them.
		var urls []string
		switch v := q.URL.(type) {
		case string:
			urls = []string{v}
		case []any:
			for _, p := range v {
				if p, ok := p.(string); ok {
					urls = append(urls, p)
				}
			}
		}
		if q.WindowID != nil && *q.WindowID == currentWindow {
			*q.WindowID = a.current(c)
		}
		current, front := a.current(c), a.tabs.Front()
		out := []tab{}
		for _, t := range a.tabs.All() {
			w := a.tabs.WindowOf(t.ID)
			active := t.ID == a.tabs.ActiveIn(w)
			if q.Active != nil && *q.Active != active ||
				q.Highlighted != nil && *q.Highlighted != active ||
				q.CurrentWindow != nil && *q.CurrentWindow != (w == current) ||
				q.LastFocusedWindow != nil && *q.LastFocusedWindow != (w == front) ||
				q.Pinned != nil && *q.Pinned ||
				q.Status != nil && *q.Status != status(t) ||
				q.WindowID != nil && *q.WindowID != w ||
				q.Index != nil && *q.Index != a.index(t.ID) {
				continue
			}
			// An extension that can't see a tab's address and title can't find it by them.
			if (q.URL != nil || q.Title != nil) && !seesTabs(e) ||
				q.URL != nil && !slices.ContainsFunc(urls, func(p string) bool { return matchURL(p, t.URL) }) ||
				q.Title != nil && !matchTitle(*q.Title, t.Title) {
				continue
			}
			out = append(out, a.tabFor(e, t))
		}
		return out, nil
	}
	functions["tabs.create"] = func(a *API, c call) (any, error) {
		var p struct {
			URL      string           `json:"url"`
			Active   *bool            `json:"active"`
			WindowID *engine.WindowID `json:"windowId"`
		}
		c.arg(0, &p)
		u := tabs.BlankURL
		if p.URL != "" {
			u = resolve(c.Extension, p.URL)
		}
		// Into the window named, or the caller's current one.
		w := a.current(c)
		if p.WindowID != nil && *p.WindowID != currentWindow {
			if _, ok := a.windows.Get(*p.WindowID); !ok {
				return nil, fmt.Errorf("No window with id: %d.", *p.WindowID)
			}
			w = *p.WindowID
		}
		var id engine.TabID
		if w == a.tabs.Front() {
			id = a.tabs.OpenIn(u, p.Active == nil || *p.Active)
		} else {
			id = a.tabs.CreateIn(w)
			a.tabs.Load(id, u)
		}
		t, _ := a.tabs.Get(id)
		return a.tabFor(a.caller(c), t), nil
	}
	functions["tabs.duplicate"] = func(a *API, c call) (any, error) {
		t, err := a.tabArg(c, 0)
		if err != nil {
			return nil, err
		}
		a.tabs.Activate(t.ID)
		a.tabs.Duplicate()
		return a.tabFor(a.caller(c), a.tabs.Active()), nil
	}
	functions["tabs.update"] = func(a *API, c call) (any, error) {
		t, err := a.tabArg(c, 0)
		if err != nil {
			return nil, err
		}
		var p struct {
			URL         string `json:"url"`
			Active      *bool  `json:"active"`
			Highlighted *bool  `json:"highlighted"`
			Selected    *bool  `json:"selected"`
		}
		c.arg(1, &p)
		for _, show := range []*bool{p.Active, p.Highlighted, p.Selected} {
			if show != nil && *show {
				a.tabs.Activate(t.ID)
			}
		}
		if p.URL != "" {
			a.tabs.Load(t.ID, resolve(c.Extension, p.URL))
		}
		t, _ = a.tabs.Get(t.ID)
		return a.tabFor(a.caller(c), t), nil
	}
	functions["tabs.reload"] = func(a *API, c call) (any, error) {
		t, err := a.tabArg(c, 0)
		if err != nil {
			return nil, err
		}
		var p struct {
			BypassCache bool `json:"bypassCache"`
		}
		c.arg(1, &p)
		a.tabs.ReloadTab(t.ID, p.BypassCache)
		return nil, nil
	}
	// One id or a list of them.
	functions["tabs.remove"] = func(a *API, c call) (any, error) {
		var ids []engine.TabID
		var one engine.TabID
		if c.arg(0, &one) {
			ids = []engine.TabID{one}
		} else {
			c.arg(0, &ids)
		}
		for _, id := range ids {
			if _, ok := a.tabs.Get(id); !ok {
				return nil, fmt.Errorf("No tab with id: %d.", id)
			}
		}
		for _, id := range ids {
			a.tabs.Close(id)
		}
		return nil, nil
	}
}

// TabsChanged tells the extensions which tabs opened and closed and which is active in
// each window now, from what they were last told.
func (a *API) TabsChanged() {
	now := map[engine.TabID]tabs.Tab{}
	in := map[engine.TabID]engine.WindowID{}
	left := map[engine.WindowID]int{}
	for _, t := range a.tabs.All() {
		now[t.ID] = t
		in[t.ID] = a.tabs.WindowOf(t.ID)
		left[in[t.ID]]++
	}
	active := map[engine.WindowID]engine.TabID{}
	for _, w := range a.tabs.Windows() {
		if id := a.tabs.ActiveIn(w); id != 0 {
			active[w] = id
			if a.active[w] != id {
				a.accessed[id] = time.Now()
			}
		}
	}
	for _, e := range a.enabled() {
		for _, t := range a.tabs.All() {
			if _, was := a.known[t.ID]; !was {
				a.dispatch(e.ID, "tabs.onCreated", a.tabFor(e, t))
			}
		}
		for id := range a.known {
			if _, is := now[id]; !is {
				w := a.knownIn[id]
				a.dispatch(e.ID, "tabs.onRemoved", id, map[string]any{"windowId": w, "isWindowClosing": left[w] == 0})
			}
		}
		for w, id := range active {
			if a.active[w] != id {
				a.dispatch(e.ID, "tabs.onActivated", map[string]any{"tabId": id, "windowId": w})
			}
		}
	}
	for id := range a.known {
		if _, is := now[id]; !is {
			delete(a.accessed, id)
			a.ext.ForgetTab(id)
		}
	}
	a.known, a.knownIn, a.active = now, in, active
}

// TabChanged tells the extensions what changed in one tab: its status, and for those
// that may see them its address, title and icon.
func (a *API) TabChanged(t tabs.Tab) {
	was, ok := a.known[t.ID]
	if !ok {
		return
	}
	a.known[t.ID] = t
	// A tab that went to another page starts again with what its buttons show everywhere.
	if t.URL != was.URL {
		a.ext.ForgetTab(t.ID)
	}
	for _, e := range a.ext.Installed() {
		if !e.Enabled {
			continue
		}
		change := map[string]any{}
		if status(t) != status(was) {
			change["status"] = status(t)
		}
		if seesTabs(e) {
			if t.URL != was.URL {
				change["url"] = t.URL
			}
			if t.Title != was.Title {
				change["title"] = t.Title
			}
			if t.Favicon != was.Favicon {
				change["favIconUrl"] = t.Favicon
			}
		}
		if len(change) > 0 {
			a.dispatch(e.ID, "tabs.onUpdated", t.ID, change, a.tabFor(e, t))
		}
	}
}
