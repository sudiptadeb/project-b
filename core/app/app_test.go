package app_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"projectb/app"
	"projectb/engine"
	"projectb/mockengine"
)

// rig runs the real managers on the mock engine, as the dev server does.
type rig struct {
	t     *testing.T
	e     *mockengine.Engine
	a     *app.App
	got   <-chan []byte
	pages <-chan []byte // What the mock page is told to show.
}

func start(t *testing.T, dir string, blocked ...string) *rig {
	t.Helper()
	e := mockengine.New()
	got, stop := e.Shell.Subscribe()
	t.Cleanup(stop)
	pages, stopPages := e.Page.Subscribe()
	t.Cleanup(stopPages)
	r := &rig{t: t, e: e, got: got, pages: pages}
	ready := make(chan struct{})
	e.Post(func() {
		r.a = app.Start(e, app.Config{ProfileDir: dir, BlockedHosts: blocked})
		e.SetListener(r.a)
		close(ready)
	})
	go e.Run()
	<-ready
	// The shell says it's ready as it starts, and only then hears from Go.
	r.shell(`{"t":"ready"}`)
	return r
}

func (r *rig) shell(cmd string) { r.e.Post(func() { r.a.ShellMessage(1, []byte(cmd)) }) }

// show reports a layout from the shell that shows one tab, as the shell does for the
// active tab.
func (r *rig) show(tab float64) {
	r.shell(fmt.Sprintf(`{"t":"layout","tabs":[{"tab":%v,"rect":{"x":0,"y":48,"w":800,"h":552}}],"viewW":800,"viewH":600}`, tab))
}

// waitFor reads shell messages until one matches.
func (r *rig) waitFor(match func(map[string]any) bool) map[string]any {
	r.t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b := <-r.got:
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				r.t.Fatalf("bad message %s: %v", b, err)
			}
			if match(m) {
				return m
			}
		case <-deadline:
			r.t.Fatal("timed out waiting for shell message")
		}
	}
}

func (r *rig) waitForTab(match func(tab map[string]any) bool) map[string]any {
	r.t.Helper()
	return r.waitFor(func(m map[string]any) bool {
		tab, ok := m["tab"].(map[string]any)
		return ok && m["t"] == "tab" && match(tab)
	})["tab"].(map[string]any)
}

func (r *rig) visit(url string) {
	r.t.Helper()
	r.shell(`{"t":"navigate","text":"` + url + `"}`)
	r.waitForTab(func(t map[string]any) bool {
		return t["url"] == url && t["loading"] == false && strings.HasPrefix(t["title"].(string), "Mock")
	})
}

func (r *rig) suggestions(text string) []any {
	r.t.Helper()
	r.shell(`{"t":"suggest","id":7,"text":"` + text + `"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "suggestions" })
	if m["id"] != float64(7) {
		r.t.Fatalf("id = %v", m["id"])
	}
	items, _ := m["items"].([]any)
	return items
}

func (r *rig) stop() {
	done := make(chan struct{})
	r.e.Post(func() { r.a.Stop(); close(done) })
	<-done
}

func TestVisitBecomesSuggestion(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://example.com/docs")

	items := r.suggestions("exam")
	if len(items) != 1 {
		t.Fatalf("items = %v", items)
	}
	item := items[0].(map[string]any)
	if item["url"] != "https://example.com/docs" || item["title"] != "Mock example.com" {
		t.Fatalf("item = %v", item)
	}
}

func TestLoadProgressIsReportedOnTheTab(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"navigate","text":"https://example.com/"}`)
	r.waitForTab(func(tab map[string]any) bool { return tab["progress"] == 0.5 && tab["loading"] == true })
	r.waitForTab(func(tab map[string]any) bool { return tab["progress"] == 1.0 })
}

func TestHistorySurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	r.visit("https://example.com/")
	r.stop()

	if _, err := os.Stat(filepath.Join(dir, "history.jsonl")); err != nil {
		t.Fatal(err)
	}

	r2 := start(t, dir)
	defer r2.stop()
	deadline := time.Now().Add(5 * time.Second)
	for len(r2.suggestions("example")) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("history did not load")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBlockedURLNeverLoadsOrEntersHistory(t *testing.T) {
	r := start(t, t.TempDir(), "blocked.test")
	defer r.stop()
	r.shell(`{"t":"navigate","text":"https://www.blocked.test/x"}`)
	r.waitForTab(func(tab map[string]any) bool { return tab["blocked"] == true })
	if items := r.suggestions("blocked"); len(items) != 0 {
		t.Fatalf("items = %v", items)
	}
}

func TestTheFirstTabHoldsTheBlankPage(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.show(1)
	deadline := time.After(5 * time.Second)
	for loaded := false; !loaded; {
		select {
		case b := <-r.pages:
			loaded = string(b) == `{"state":"loaded","tab":1,"url":"about:blank"}`
		case <-deadline:
			t.Fatal("the first tab never loaded the blank page")
		}
	}
	r.shell(`{"t":"ready"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := m["tabs"].([]any)[0].(map[string]any)
	if tab["url"] != "" || tab["title"] != "" {
		t.Fatalf("the blank page shows as url %q title %q", tab["url"], tab["title"])
	}
	if items := r.suggestions("blank"); len(items) != 0 {
		t.Fatalf("items = %v", items)
	}
}

func TestTheShellNeverLoadsInATab(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"navigate","text":"chrome://shell/"}`)
	r.waitForTab(func(tab map[string]any) bool { return tab["url"] == "chrome://shell/" && tab["blocked"] == true })
	r.shell(`{"t":"navigate","text":"chrome://gpu/"}`)
	r.waitForTab(func(tab map[string]any) bool {
		return tab["url"] == "chrome://gpu/" && tab["blocked"] == false && tab["title"] == "Mock gpu"
	})
}

func TestStaleNavigationDoesNotCommit(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"navigate","text":"https://first.test/"}`)
	r.shell(`{"t":"navigate","text":"https://second.test/"}`)
	r.waitForTab(func(tab map[string]any) bool { return tab["url"] == "https://second.test/" && tab["loading"] == false })
	if items := r.suggestions("first"); len(items) != 0 {
		t.Fatalf("items = %v", items)
	}
}

func TestShellRegionsDecideTheHitTest(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"layout","shell":[{"x":0,"y":0,"w":800,"h":48},{"x":100,"y":48,"w":400,"h":200}],"drag":[{"x":300,"y":0,"w":500,"h":32}],"tabs":[{"tab":1,"rect":{"x":0,"y":48,"w":800,"h":552}}],"viewW":800,"viewH":600}`)
	r.e.Settle()

	l := r.e.Layout()
	if !slices.Equal(l.Tabs, []engine.VisibleTab{{Tab: 1, Rect: engine.Rect{X: 0, Y: 48, W: 800, H: 552}}}) || l.FrameColor == "" {
		t.Fatalf("layout %+v", l)
	}
	for _, c := range []struct {
		x, y float64
		want engine.Area
	}{{10, 10, engine.AreaShell}, {120, 100, engine.AreaShell}, {10, 100, engine.AreaPage}, {600, 300, engine.AreaPage}, {400, 10, engine.AreaDrag}, {400, 40, engine.AreaShell}} {
		if got := l.At(c.x, c.y); got != c.want {
			t.Errorf("At(%v,%v) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestShellWindowRequestsReachTheEngine(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"layout","shell":[{"x":0,"y":0,"w":800,"h":48}],"drag":[{"x":300,"y":0,"w":500,"h":32}],"tabs":[{"tab":1,"rect":{"x":0,"y":48,"w":800,"h":552}}],"viewW":800,"viewH":600}`)
	r.shell(`{"t":"window","text":"fullScreen"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "controls" && m["cornerRadius"] == float64(0) })
	if got := r.e.Layout().At(400, 10); got != engine.AreaShell {
		t.Fatalf("in full screen the empty top row is %v", got)
	}
	r.shell(`{"t":"window","text":"fullScreen"}`)
	r.waitFor(func(m map[string]any) bool {
		return m["t"] == "controls" && m["cornerRadius"] == mockengine.CornerRadius
	})
	if got := r.e.Layout().At(400, 10); got != engine.AreaDrag {
		t.Fatalf("back from full screen the empty top row is %v", got)
	}
	r.shell(`{"t":"window","text":"zoom"}`)
	r.shell(`{"t":"window","text":"minimise"}`)
	r.shell(`{"t":"window","text":"close"}`)
	deadline := time.Now().Add(time.Second)
	for !r.e.Closed() {
		if time.Now().After(deadline) {
			t.Fatal("window never closed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	want := []engine.WindowRequest{engine.WindowFullScreen, engine.WindowFullScreen, engine.WindowZoom, engine.WindowMinimise, engine.WindowClose}
	if got := r.e.Requests(); !slices.Equal(got, want) {
		t.Fatalf("requests %v", got)
	}
}

func TestTabsThroughTheShellAndKeys(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")

	r.shell(`{"t":"newTab"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
	second := m["active"].(float64)
	if second == 1 {
		t.Fatalf("new tab should be active: %v", m)
	}
	// The shell shows the active tab, and its layout hides the one it showed before.
	r.show(1)
	r.show(second)
	r.e.Settle()
	if hidden := r.e.Hidden(); !slices.Equal(hidden, []engine.TabID{1}) {
		t.Fatalf("the first tab should have been told it is hidden: %v", hidden)
	}
	r.visit("https://b.test/")

	// The page opens a tab: the engine made it, Go put it in the list and showed it.
	r.e.Fire("open", "https://c.test/")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 3 })
	if m["active"].(float64) == second {
		t.Fatalf("opened tab should be active: %v", m)
	}
	r.waitForTab(func(tab map[string]any) bool { return tab["url"] == "https://c.test/" && tab["loading"] == false })

	r.e.Fire("key", "tab1")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && m["active"] == float64(1) })
	r.e.Fire("key", "nextTab")
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && m["active"] == second })
	r.e.Fire("key", "closeTab")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
	if m["active"] == second {
		t.Fatalf("closed tab still active: %v", m)
	}

	// Closing the rest closes the window.
	r.e.Fire("key", "closeTab")
	r.e.Fire("key", "closeTab")
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 0 })
	deadline := time.Now().Add(time.Second)
	for !r.e.Closed() {
		if time.Now().After(deadline) {
			t.Fatal("window never closed")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestTabLayoutIsAnnouncedAndSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	r.shell(`{"t":"setTabLayout","text":"horizontal"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabLayout" && m["tabLayout"] == "horizontal" })
	r.stop()

	r2 := start(t, dir)
	defer r2.stop()
	r2.shell(`{"t":"ready"}`)
	m := r2.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if m["tabLayout"] != "horizontal" {
		t.Fatalf("snapshot after restart: %v", m["tabLayout"])
	}
}

func TestSnapshotCarriesTheWindowControls(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"ready"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	controls := m["controls"].(map[string]any)
	if controls["x"] != float64(mockengine.Controls.X) || controls["w"] != float64(mockengine.Controls.W) {
		t.Fatalf("controls = %v", controls)
	}
	if m["cornerRadius"] != mockengine.CornerRadius {
		t.Fatalf("cornerRadius = %v", m["cornerRadius"])
	}
	if m["windowColor"] != "#4f9e9e" || r.e.Layout().FrameColor != "#4f9e9e" || m["frameWidth"] != 4.0 || r.e.Layout().FrameWidth != 4 {
		t.Fatalf("window frame: shell %v %v, engine %v %v", m["windowColor"], m["frameWidth"], r.e.Layout().FrameColor, r.e.Layout().FrameWidth)
	}

	r.e.Fire("fullscreen", "")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "controls" })
	if m["controls"].(map[string]any)["w"] != float64(0) || m["cornerRadius"] != float64(0) {
		t.Fatalf("full screen should report no controls and square corners: %v", m)
	}
}

var _ engine.Listener = (*app.App)(nil)

// openSettings opens the settings page from the shell's gear and reads its snapshot.
func (r *rig) openSettings() (<-chan []byte, map[string]any) {
	r.t.Helper()
	got, stop := r.e.Settings.Subscribe()
	r.t.Cleanup(stop)
	r.shell(`{"t":"run","text":"app.settings"}`)
	deadline := time.Now().Add(5 * time.Second)
	for !r.e.SettingsOpen() {
		if time.Now().After(deadline) {
			r.t.Fatal("settings never opened")
		}
		time.Sleep(5 * time.Millisecond)
	}
	r.settings(`{"t":"ready"}`)
	return got, r.waitOn(got, func(m map[string]any) bool { return m["t"] == "snapshot" })
}

func (r *rig) settings(cmd string) { r.e.Post(func() { r.a.SettingsMessage(1, []byte(cmd)) }) }

func (r *rig) waitOn(ch <-chan []byte, match func(map[string]any) bool) map[string]any {
	r.t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b := <-ch:
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				r.t.Fatalf("bad message %s: %v", b, err)
			}
			if match(m) {
				return m
			}
		case <-deadline:
			r.t.Fatal("timed out waiting for a settings message")
		}
	}
}

// Settings is a page of its own: the snapshot carries what it shows, it sets the prefs,
// and Go says when they change.
func TestTheSettingsPageReadsAndSetsTheSettings(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	got, snap := r.openSettings()
	if snap["autoUpdate"] != true || snap["running"] != "" || snap["keyboard"] == nil {
		t.Fatalf("snapshot = %v", snap)
	}
	r.settings(`{"t":"setAutoUpdate","on":false}`)
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "autoUpdate" && m["autoUpdate"] == false })
	b, _ := os.ReadFile(filepath.Join(dir, "prefs.json"))
	if !strings.Contains(string(b), `"autoUpdate":false`) {
		t.Fatalf("prefs.json = %s", b)
	}
	// The theme: saved, the shell told, the window's own parts dark.
	if snap["theme"] != "system" || r.e.Layout().Theme != engine.ThemeSystem {
		t.Fatalf("theme starts as %v, window %v", snap["theme"], r.e.Layout().Theme)
	}
	r.settings(`{"t":"setTheme","text":"dark"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "theme" && m["theme"] == "dark" })
	if r.e.Layout().Theme != engine.ThemeDark {
		t.Fatalf("window theme = %v", r.e.Layout().Theme)
	}
	b, _ = os.ReadFile(filepath.Join(dir, "prefs.json"))
	if !strings.Contains(string(b), `"theme":"dark"`) {
		t.Fatalf("prefs.json = %s", b)
	}
	r.settings(`{"t":"close"}`)
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "closed" })
}

// The frame colour is a pref: setting it from the settings page repaints the window and
// tells the shell, together, and it survives a restart.
func TestTheFrameColorChangesTheWindowAndTheShellLive(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	r.openSettings()
	r.settings(`{"t":"setFrameColor","text":"#3470d0"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "windowColor" && m["windowColor"] == "#3470d0" })
	if r.e.Layout().FrameColor != "#3470d0" {
		t.Fatalf("window = %v", r.e.Layout().FrameColor)
	}
	r.stop()

	r = start(t, dir)
	defer r.stop()
	r.shell(`{"t":"ready"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if m["windowColor"] != "#3470d0" || r.e.Layout().FrameColor != "#3470d0" || r.e.ShownColor() != "#3470d0" {
		t.Fatalf("after restart: shell %v, window %v, shown %v", m["windowColor"], r.e.Layout().FrameColor, r.e.ShownColor())
	}
}

// A key added in the settings page runs its action, and the keymap now belongs to the
// user. Closing a tab while settings is open closes settings.
func TestAKeyAddedInSettingsRunsItsAction(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	got, _ := r.openSettings()
	r.settings(`{"t":"addKey","id":"tab.new","text":"Cmd+Shift+Y","reserve":"always"}`)
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "keyboard" })
	b, _ := os.ReadFile(filepath.Join(dir, "keymap.json"))
	if !strings.Contains(string(b), `"Shift+Cmd+Y"`) || strings.Contains(string(b), `"follows"`) {
		t.Fatalf("keymap.json = %s", b)
	}

	r.e.Fire("key", "closeTab")
	deadline := time.Now().Add(5 * time.Second)
	for r.e.SettingsOpen() {
		if time.Now().After(deadline) {
			t.Fatal("Cmd+W didn't close settings")
		}
		time.Sleep(5 * time.Millisecond)
	}

	r.e.Post(func() {
		r.a.Key(engine.Key{Code: 'Y', Mods: engine.ModCmd | engine.ModShift}, engine.KeyBeforePage, engine.SurfacePage)
	})
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
}

// Ctrl+Tab goes back through recent tabs while Ctrl is held, as Cmd+Tab does through apps,
// and letting go settles on the tab it reached.
func TestCtrlTabCyclesRecentTabs(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"newTab"}`)
	r.shell(`{"t":"newTab"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 3 })
	active := func(id float64) {
		t.Helper()
		r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && m["active"] == id })
	}
	// Recent: 3, 2, 1.
	r.e.Fire("key", "recentTab")
	active(2)
	r.e.Fire("key", "recentTab")
	active(1)
	r.e.Fire("release", "")
	// Recent: 1, 3, 2.
	r.e.Fire("key", "recentTab")
	active(3)
	r.e.Fire("release", "")
	// Recent: 3, 1, 2. Backward from the newest comes round to the oldest.
	r.e.Fire("key", "recentTabBack")
	active(2)
	r.e.Fire("release", "")
	r.e.Fire("key", "recentTab")
	active(3)
}

// The menu comes from the tree: Edit has Redo, View has Full Screen with its macOS key,
// and the hidden Select Tab submenu isn't there.
func TestTheMenuComesFromTheTree(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var menu engine.Menu
	done := make(chan struct{})
	r.e.Post(func() { menu = r.e.Menu(); close(done) })
	<-done
	find := func(items []engine.MenuItem, title string) *engine.MenuItem {
		for i := range items {
			if items[i].Title == title {
				return &items[i]
			}
		}
		return nil
	}
	edit, view, tab := find(menu, "Edit"), find(menu, "View"), find(menu, "Tab")
	if edit == nil || view == nil || tab == nil {
		t.Fatalf("menu = %+v", menu)
	}
	if find(edit.Items, "Redo") == nil {
		t.Fatal("no Redo")
	}
	full := find(view.Items, "Full Screen")
	if full == nil || full.Key == nil || *full.Key != (engine.Key{Code: 'F', Mods: engine.ModCtrl | engine.ModCmd}) {
		t.Fatalf("Full Screen = %+v", full)
	}
	if find(tab.Items, "Select Tab") != nil {
		t.Fatal("the hidden Select Tab submenu is in the menu")
	}
}
