package app

import (
	"encoding/json"

	"projectb/engine"
	"projectb/shell"
	"projectb/tabs"
)

// Find in page (docs/daily-driver.md): each tab keeps its own search while its find bar is
// open; Go runs it through the engine and tells the window's shell the active tab's, with
// how many matches there are and which is shown. The shell draws the bar and sends what is
// typed, Enter or Shift+Enter, and its closing.

// tabFind is one tab's open search.
type tabFind struct {
	Tab     engine.TabID `json:"tab"`
	Text    string       `json:"text"`
	Matches int          `json:"matches"`
	Active  int          `json:"active"`
	// request is the number of the search whose replies count.
	request int
}

type pageFinds struct {
	a     *App
	e     engine.Engine
	byTab map[engine.TabID]*tabFind
	// last is the text searched for last, in any tab, for a bar opened afresh.
	last    string
	request int
}

func newPageFinds(a *App, e engine.Engine) *pageFinds {
	f := &pageFinds{a: a, e: e, byTab: map[engine.TabID]*tabFind{}}
	// Another tab shown shows its own search, or none; a closed tab's goes.
	a.Tabs.OnListChanged(func() {
		for tab := range f.byTab {
			if _, ok := a.Tabs.Get(tab); !ok {
				delete(f.byTab, tab)
			}
		}
		for _, w := range a.Tabs.Windows() {
			f.send(w)
		}
	})
	// A page that loads anew is searched again.
	a.Tabs.OnNavigationFinished(func(t tabs.Tab) {
		if tf := f.byTab[t.ID]; tf != nil && tf.Text != "" {
			f.search(tf, true, false)
		}
	})
	return f
}

// send tells window w's shell its active tab's search, or null.
func (f *pageFinds) send(w engine.WindowID) {
	var shown any
	if tf := f.byTab[f.a.Tabs.ActiveIn(w)]; tf != nil {
		shown = tf
	}
	f.a.Shell.SendTo(w, map[string]any{"t": "find", "find": shown})
}

func (f *pageFinds) search(tf *tabFind, forward, next bool) {
	f.request++
	tf.request = f.request
	if tf.Text == "" {
		tf.Matches, tf.Active = 0, 0
		f.e.StopFinding(tf.Tab)
		f.send(f.a.Tabs.WindowOf(tf.Tab))
		return
	}
	f.last = tf.Text
	f.e.Find(tf.Tab, tf.request, tf.Text, forward, next)
}

// open opens the active tab's bar in window w, searching for the last text when it opens
// afresh, and asks the shell to put the keyboard in it.
func (f *pageFinds) open(w engine.WindowID) *tabFind {
	tab := f.a.Tabs.ActiveIn(w)
	if tab == 0 {
		return nil
	}
	tf := f.byTab[tab]
	if tf == nil {
		tf = &tabFind{Tab: tab, Text: f.last}
		f.byTab[tab] = tf
		f.send(w)
		if tf.Text != "" {
			f.search(tf, true, false)
		}
	}
	f.a.Shell.SendTo(w, map[string]any{"t": "focusFind"})
	return tf
}

// step shows the next match, or the one before, opening the bar first when it is closed.
func (f *pageFinds) step(w engine.WindowID, forward bool) {
	if tf := f.byTab[f.a.Tabs.ActiveIn(w)]; tf != nil && tf.Text != "" {
		f.search(tf, forward, true)
		return
	}
	f.open(w)
}

func (f *pageFinds) closeTab(tab engine.TabID) {
	delete(f.byTab, tab)
	f.e.StopFinding(tab)
	f.send(f.a.Tabs.WindowOf(tab))
}

func (f *pageFinds) reply(tab engine.TabID, request, matches, active int) {
	tf := f.byTab[tab]
	if tf == nil || tf.request != request {
		return
	}
	tf.Matches, tf.Active = matches, active
	f.send(f.a.Tabs.WindowOf(tab))
}

func (f *pageFinds) shellPart() shell.Part {
	return shell.Part{
		Snapshot: func(map[string]any) {},
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			var r struct {
				Tab     engine.TabID `json:"tab"`
				Text    string       `json:"text"`
				Forward bool         `json:"forward"`
			}
			json.Unmarshal(msg, &r)
			// Each names its tab, and goes nowhere once that tab's bar has closed, so a
			// key typed as another tab came forward doesn't search that one.
			tf := f.byTab[r.Tab]
			switch t {
			case "findText":
				if tf != nil {
					tf.Text = r.Text
					f.search(tf, true, false)
				}
			case "findNext":
				if tf != nil && tf.Text != "" {
					f.search(tf, r.Forward, true)
				}
			case "findClose":
				if tf != nil {
					f.closeTab(r.Tab)
				}
			default:
				return false
			}
			return true
		},
	}
}

func (a *App) FindReply(tab engine.TabID, request, matches, active int, _ bool) {
	a.finds.reply(tab, request, matches, active)
}
