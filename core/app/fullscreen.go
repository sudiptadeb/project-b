package app

import (
	"projectb/engine"
)

// A page's full screen, as a video's button asks for (docs/daily-driver.md): Go puts the
// window in full screen, unless it is already, and tells its shell, which lays the page
// across the whole window and draws nothing else. Escape, another tab shown, the tab
// closing or the window leaving full screen ends it, and the window goes back to how it was.

// pageFull is one window's page in full screen: the tab, and whether the window was in
// full screen already.
type pageFull struct {
	tab engine.TabID
	was bool
	// shown says the window has been in full screen since, so leaving it ends the page's.
	shown bool
}

// escapeKey is Escape's key code.
const escapeKey = 0x1b

type pageFulls struct {
	a      *App
	e      engine.Engine
	byWind map[engine.WindowID]pageFull
}

func newPageFulls(a *App, e engine.Engine) *pageFulls {
	f := &pageFulls{a: a, e: e, byWind: map[engine.WindowID]pageFull{}}
	// Another tab shown, or the tab gone, ends it.
	a.Tabs.OnListChanged(func() {
		for w, p := range f.byWind {
			if a.Tabs.ActiveIn(w) != p.tab {
				f.e.ExitPageFullScreen(p.tab)
				f.end(w)
			}
		}
	})
	return f
}

func (f *pageFulls) send(w engine.WindowID) {
	f.a.Shell.SendTo(w, map[string]any{"t": "pageFullScreen", "pageFullScreen": f.byWind[w].tab})
}

func (f *pageFulls) changed(tab engine.TabID, on bool) {
	w := f.a.Tabs.WindowOf(tab)
	if w == 0 {
		return
	}
	if !on {
		if f.byWind[w].tab == tab {
			f.end(w)
		}
		return
	}
	if f.a.Tabs.ActiveIn(w) != tab {
		return
	}
	if _, already := f.byWind[w]; !already {
		was := f.a.Window.FullScreen(w)
		f.byWind[w] = pageFull{tab: tab, was: was, shown: was}
		if !was {
			f.a.Window.Request(w, engine.WindowFullScreen)
		}
	}
	f.send(w)
}

// end puts window w back as it was before its page filled the screen.
func (f *pageFulls) end(w engine.WindowID) {
	p, ok := f.byWind[w]
	if !ok {
		return
	}
	delete(f.byWind, w)
	if !p.was && f.a.Window.FullScreen(w) {
		f.a.Window.Request(w, engine.WindowFullScreen)
	}
	f.send(w)
}

// key takes Escape while the window in front shows a page in full screen.
func (f *pageFulls) key(k engine.Key) bool {
	p, ok := f.byWind[f.a.Tabs.Front()]
	if !ok || k.Code != escapeKey || k.Mods != 0 {
		return false
	}
	f.e.ExitPageFullScreen(p.tab)
	return true
}

// windowChanged ends a page's full screen when its window leaves full screen by itself,
// as through its green button.
func (f *pageFulls) windowChanged(w engine.WindowID, s engine.WindowState) {
	p, ok := f.byWind[w]
	switch {
	case !ok:
	case s.FullScreen:
		p.shown = true
		f.byWind[w] = p
	case p.shown:
		f.e.ExitPageFullScreen(p.tab)
	}
}

func (a *App) PageFullScreen(tab engine.TabID, on bool) { a.fulls.changed(tab, on) }

func (a *App) IsPageFullScreen(tab engine.TabID) bool {
	for _, p := range a.fulls.byWind {
		if p.tab == tab {
			return true
		}
	}
	return false
}
