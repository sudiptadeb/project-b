package app_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"projectb/app"
	"projectb/commands"
	"projectb/engine"
	"projectb/mockengine"
	"projectb/prefs"
	"projectb/profiles"
	"projectb/shells"
	"projectb/tabs"
	"projectb/timeline"
	"projectb/update"
	"projectb/window"
	"projectb/workspaces"
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
	return startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), BlockedHosts: blocked, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
}

// startWith starts the app as start does, on cfg.
func startWith(t *testing.T, cfg app.Config) *rig {
	t.Helper()
	e := mockengine.New()
	got, stop := e.Feeds(1).Shell.Subscribe()
	t.Cleanup(stop)
	pages, stopPages := e.Feeds(1).Page.Subscribe()
	t.Cleanup(stopPages)
	r := &rig{t: t, e: e, got: got, pages: pages}
	ready := make(chan struct{})
	e.Post(func() {
		r.a = app.Start(e, cfg)
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
			loaded = string(b) == `{"slot":0,"state":"loaded","tab":1,"url":"about:blank"}`
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

// The window has no band until its shell asks for one, and then the one it asked for.
func TestTheShellsLayoutSetsTheBand(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"ready"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if _, ok := m["frameWidth"]; ok || r.e.Layout(1).FrameWidth != 0 {
		t.Fatalf("before the shell asked: snapshot %v, engine %v", m["frameWidth"], r.e.Layout(1).FrameWidth)
	}
	r.shell(`{"t":"layout","shell":[],"drag":[],"tabs":[{"tab":1,"rect":{"x":4,"y":48,"w":792,"h":548}}],"viewW":800,"viewH":600,"frame":4}`)
	r.e.Settle()
	if r.e.Layout(1).FrameWidth != 4 {
		t.Fatalf("asked for 4: %+v", r.e.Layout(1))
	}
}

func TestShellRegionsDecideTheHitTest(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"layout","shell":[{"x":0,"y":0,"w":800,"h":48},{"x":100,"y":48,"w":400,"h":200}],"drag":[{"x":300,"y":0,"w":500,"h":32}],"tabs":[{"tab":1,"rect":{"x":0,"y":48,"w":800,"h":552}}],"viewW":800,"viewH":600}`)
	r.e.Settle()

	l := r.e.Layout(1)
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
	if got := r.e.Layout(1).At(400, 10); got != engine.AreaShell {
		t.Fatalf("in full screen the empty top row is %v", got)
	}
	r.shell(`{"t":"window","text":"fullScreen"}`)
	r.waitFor(func(m map[string]any) bool {
		return m["t"] == "controls" && m["cornerRadius"] == mockengine.CornerRadius
	})
	if got := r.e.Layout(1).At(400, 10); got != engine.AreaDrag {
		t.Fatalf("back from full screen the empty top row is %v", got)
	}
	r.shell(`{"t":"window","text":"zoom"}`)
	r.shell(`{"t":"window","text":"minimise"}`)
	r.shell(`{"t":"window","text":"close"}`)
	deadline := time.Now().Add(time.Second)
	for !r.e.Closed(1) {
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
	if hidden := r.e.Hidden(1); !slices.Equal(hidden, []engine.TabID{1}) {
		t.Fatalf("the first tab should have been told it is hidden: %v", hidden)
	}
	r.visit("https://b.test/")

	// The page opens a tab: the engine made it, Go put it in the list and showed it.
	r.e.Fire(1, 0, "open", "https://c.test/")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 3 })
	if m["active"].(float64) == second {
		t.Fatalf("opened tab should be active: %v", m)
	}
	r.waitForTab(func(tab map[string]any) bool { return tab["url"] == "https://c.test/" && tab["loading"] == false })

	r.e.Fire(1, 0, "key", "tab1")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && m["active"] == float64(1) })
	r.e.Fire(1, 0, "key", "nextTab")
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && m["active"] == second })
	r.e.Fire(1, 0, "key", "closeTab")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
	if m["active"] == second {
		t.Fatalf("closed tab still active: %v", m)
	}

	// Closing the rest closes the window.
	r.e.Fire(1, 0, "key", "closeTab")
	r.e.Fire(1, 0, "key", "closeTab")
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 0 })
	deadline := time.Now().Add(time.Second)
	for !r.e.Closed(1) {
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
	if m["windowColor"] != "#4f9e9e" || r.e.Layout(1).FrameColor != "#4f9e9e" {
		t.Fatalf("frame colour: shell %v, engine %v", m["windowColor"], r.e.Layout(1).FrameColor)
	}

	r.e.Fire(1, 0, "fullscreen", "")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "controls" })
	if m["controls"].(map[string]any)["w"] != float64(0) || m["cornerRadius"] != float64(0) {
		t.Fatalf("full screen should report no controls and square corners: %v", m)
	}
}

var _ engine.Listener = (*app.App)(nil)

// openSettings opens the settings page from the shell's gear and reads its snapshot.
func (r *rig) openSettings() (<-chan []byte, map[string]any) {
	r.t.Helper()
	got, stop := r.e.Feeds(1).Settings.Subscribe()
	r.t.Cleanup(stop)
	r.shell(`{"t":"run","text":"app.settings"}`)
	deadline := time.Now().Add(5 * time.Second)
	for !r.e.SettingsOpen(1) {
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
	b, _ := os.ReadFile(filepath.Join(dir, "updates", "settings.json"))
	if !strings.Contains(string(b), `"autoUpdate":false`) {
		t.Fatalf("the update folder's settings.json = %s", b)
	}
	// The theme: saved, the shell told, the window's own parts dark.
	if snap["theme"] != "system" || r.e.Layout(1).Theme != engine.ThemeSystem {
		t.Fatalf("theme starts as %v, window %v", snap["theme"], r.e.Layout(1).Theme)
	}
	r.settings(`{"t":"setTheme","text":"dark"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "theme" && m["theme"] == "dark" })
	if r.e.Layout(1).Theme != engine.ThemeDark {
		t.Fatalf("window theme = %v", r.e.Layout(1).Theme)
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
	if r.e.Layout(1).FrameColor != "#3470d0" {
		t.Fatalf("window = %v", r.e.Layout(1).FrameColor)
	}
	r.stop()

	r = start(t, dir)
	defer r.stop()
	r.shell(`{"t":"ready"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if m["windowColor"] != "#3470d0" || r.e.Layout(1).FrameColor != "#3470d0" || r.e.ShownColor() != "#3470d0" {
		t.Fatalf("after restart: shell %v, window %v, shown %v", m["windowColor"], r.e.Layout(1).FrameColor, r.e.ShownColor())
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

	r.e.Fire(1, 0, "key", "closeTab")
	deadline := time.Now().Add(5 * time.Second)
	for r.e.SettingsOpen(1) {
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
	r.e.Fire(1, 0, "key", "recentTab")
	active(2)
	r.e.Fire(1, 0, "key", "recentTab")
	active(1)
	r.e.Fire(1, 0, "release", "")
	// Recent: 1, 3, 2.
	r.e.Fire(1, 0, "key", "recentTab")
	active(3)
	r.e.Fire(1, 0, "release", "")
	// Recent: 3, 1, 2. Backward from the newest comes round to the oldest.
	r.e.Fire(1, 0, "key", "recentTabBack")
	active(2)
	r.e.Fire(1, 0, "release", "")
	r.e.Fire(1, 0, "key", "recentTab")
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

// isTabs matches a tabs message with n tabs.
func isTabs(n int) func(map[string]any) bool {
	return func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == n }
}

// openSecondWindow presses Cmd+N in window 1 and waits for window 2's shell to start.
func (r *rig) openSecondWindow() <-chan []byte {
	r.t.Helper()
	second, stop := r.e.Feeds(2).Shell.Subscribe()
	r.t.Cleanup(stop)
	r.e.Fire(1, 0, "key", "newWindow")
	r.e.Settle()
	if w := r.e.Windows(); !slices.Equal(w, []engine.WindowID{1, 2}) {
		r.t.Fatalf("windows after Cmd+N: %v", w)
	}
	// A new window's shell hears nothing until it says it's ready.
	select {
	case b := <-second:
		r.t.Fatalf("window 2's shell heard %s before it was ready", b)
	default:
	}
	r.e.Post(func() { r.a.ShellMessage(2, []byte(`{"t":"ready"}`)) })
	return second
}

// Cmd+N opens a second window with one new tab of its own and its own shell, in front.
// Commands act on the window in front, settings opens over the window it was asked in,
// closing a window's last tab closes it, and closing the last window quits.
func TestASecondWindow(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	second := r.openSecondWindow()
	snap := r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	tabs := snap["tabs"].([]any)
	if len(tabs) != 1 || tabs[0].(map[string]any)["id"] != snap["active"] || snap["active"] == float64(1) {
		t.Fatalf("window 2 starts with one new tab of its own: %v active %v", tabs, snap["active"])
	}

	// Cmd+T acts on the window in front, window 2.
	r.e.Fire(2, 0, "key", "newTab")
	r.waitOn(second, isTabs(2))
	// Window 1 comes to the front, and Cmd+T acts on it.
	r.e.Fire(1, 0, "focus", "")
	r.e.Fire(1, 0, "key", "newTab")
	r.waitFor(isTabs(2))

	// Settings opens over the window it was asked in.
	r.e.Fire(2, 0, "focus", "")
	r.e.Post(func() { r.a.ShellMessage(2, []byte(`{"t":"run","text":"app.settings"}`)) })
	r.e.Settle()
	if !r.e.SettingsOpen(2) || r.e.SettingsOpen(1) {
		t.Fatalf("settings open: window 1 %v, window 2 %v", r.e.SettingsOpen(1), r.e.SettingsOpen(2))
	}

	// Closing window 2's tabs closes window 2: its settings page goes first, then a tab
	// at a time. The app goes on with window 1.
	for range 3 {
		r.e.Fire(2, 0, "key", "closeTab")
	}
	r.e.Settle()
	if !r.e.Closed(2) || !slices.Equal(r.e.Windows(), []engine.WindowID{1}) || r.e.Quit() {
		t.Fatalf("after window 2's last tab: closed %v, windows %v, quit %v", r.e.Closed(2), r.e.Windows(), r.e.Quit())
	}

	// Window 1's own close button closes it, the last window, which quits.
	r.e.Fire(1, 0, "close", "")
	r.e.Settle()
	if !r.e.Quit() {
		t.Fatal("closing the last window didn't quit")
	}
}

// A second start's URLs open as new tabs in the window used last, which comes to the
// front.
func TestAHandoverOpensInTheWindowUsedLast(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	second := r.openSecondWindow()
	r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	r.e.Fire(1, 0, "focus", "")
	r.e.Post(func() { r.a.Handover([]string{"https://h.test/"}) })
	m := r.waitFor(isTabs(2))
	if url := m["tabs"].([]any)[1].(map[string]any)["url"]; url != "https://h.test/" {
		t.Fatalf("window 1's new tab is at %v", url)
	}
	r.waitForTab(func(tab map[string]any) bool { return tab["title"] == "Mock h.test" && tab["loading"] == false })
	var inSecond int
	done := make(chan struct{})
	r.e.Post(func() { inSecond = len(r.a.Tabs.AllIn(2)); close(done) })
	<-done
	if req := r.e.Requests(); inSecond != 1 || req[len(req)-1] != engine.WindowFront {
		t.Fatalf("window 2 has %d tabs; requests %v", inSecond, req)
	}
}

// A crashed shell waits to say it's ready again, and only its own window's does: the other
// window's shell goes on hearing.
func TestAShellCrashResetsOnlyItsWindow(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	second := r.openSecondWindow()
	r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	r.e.Post(func() { r.a.ShellCrashed(2) })
	r.e.Fire(2, 0, "key", "newTab")
	r.e.Settle()
	select {
	case b := <-second:
		t.Fatalf("window 2's crashed shell heard %s", b)
	default:
	}
	// Window 1's shell still hears what every shell hears.
	r.e.Post(func() { r.a.Prefs.SetTheme("dark") })
	r.waitFor(func(m map[string]any) bool { return m["t"] == "theme" })
	r.e.Post(func() { r.a.ShellMessage(2, []byte(`{"t":"ready"}`)) })
	snap := r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	if len(snap["tabs"].([]any)) != 2 {
		t.Fatalf("the reloaded shell's snapshot: %v", snap["tabs"])
	}
}

// moveTab asks Go to move a tab, as ProjectB.moveTab does over DevTools.
func (r *rig) moveTab(tab engine.TabID, window engine.WindowID, index int) {
	r.t.Helper()
	r.e.Post(func() {
		if _, err := r.a.DevToolsCommand("ProjectB.moveTab", []byte(fmt.Sprintf(`{"tab":%d,"window":%d,"index":%d}`, tab, window, index))); err != nil {
			r.t.Errorf("moveTab: %v", err)
		}
	})
	r.e.Settle()
}

// A tab moves to another window, where it is the active tab, and both shells hear their
// new lists; a window whose last tab moves away closes. A tab moved into a new window of
// its own is that window's only tab, and a window's only tab doesn't move.
func TestMovingATabToAnotherWindow(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	second := r.openSecondWindow()
	snap := r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	inSecond := snap["active"].(float64)
	r.e.Fire(1, 0, "focus", "")
	r.e.Fire(1, 0, "key", "newTab")
	r.waitFor(isTabs(2))

	r.moveTab(1, 2, 0)
	m := r.waitOn(second, isTabs(2))
	if m["active"] != float64(1) || m["tabs"].([]any)[0].(map[string]any)["id"] != float64(1) {
		t.Fatalf("window 2 after the move: %v", m)
	}
	r.waitFor(isTabs(1))

	// Into a new window of its own: window 3 starts with the tab, and no tab of its own.
	third, stop := r.e.Feeds(3).Shell.Subscribe()
	t.Cleanup(stop)
	var made engine.WindowID
	r.e.Post(func() { made = r.a.NewWindowWith(engine.TabID(inSecond)) })
	r.e.Settle()
	if made != 3 {
		t.Fatalf("the new window holding the tab is %v", made)
	}
	r.e.Post(func() { r.a.ShellMessage(3, []byte(`{"t":"ready"}`)) })
	snap = r.waitOn(third, func(m map[string]any) bool { return m["t"] == "snapshot" })
	if tabs := snap["tabs"].([]any); len(tabs) != 1 || snap["active"] != inSecond {
		t.Fatalf("window 3: %v active %v", tabs, snap["active"])
	}
	// Its only tab has a window already, which is the one Go answers with.
	var stays engine.WindowID
	r.e.Post(func() { stays = r.a.NewWindowWith(engine.TabID(inSecond)) })
	r.e.Settle()
	if w := r.e.Windows(); stays != 3 || !slices.Equal(w, []engine.WindowID{1, 2, 3}) {
		t.Fatalf("moving a window's only tab out answered %v; windows %v", stays, w)
	}

	// Window 3's last tab moves to window 1, and window 3 closes.
	r.moveTab(engine.TabID(inSecond), 1, 0)
	if !r.e.Closed(3) || !slices.Equal(r.e.Windows(), []engine.WindowID{1, 2}) {
		t.Fatalf("closed %v, windows %v", r.e.Closed(3), r.e.Windows())
	}

	// A new window answers with its id and its first tab's.
	var w engine.WindowID
	var tab engine.TabID
	var in []tabs.Tab
	r.e.Post(func() { w, tab = r.a.NewWindow(""); in = r.a.Tabs.AllIn(w) })
	r.e.Settle()
	if w != 4 || len(in) != 1 || in[0].ID != tab {
		t.Fatalf("NewWindow answered %v %v; its tabs %+v", w, tab, in)
	}
}

// Tab, Move Tab to New Window moves the active tab of the window in front into a window
// of its own.
func TestMoveTabToNewWindowFromTheMenu(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"newTab"}`)
	r.waitFor(isTabs(2))
	second, stop := r.e.Feeds(2).Shell.Subscribe()
	t.Cleanup(stop)
	r.e.Post(func() { r.a.MenuChosen("tab.moveToNewWindow") })
	r.e.Settle()
	r.e.Post(func() { r.a.ShellMessage(2, []byte(`{"t":"ready"}`)) })
	snap := r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	if tabs := snap["tabs"].([]any); len(tabs) != 1 || snap["active"] != float64(2) {
		t.Fatalf("window 2: %v active %v", tabs, snap["active"])
	}
	r.waitFor(isTabs(1))
}

// The window manager is the one place another manager hears of windows opening, closing,
// coming to the front and tabs moving between them, and reads each window's bounds and
// state; bounds and state change as asked.
func TestWindowsAsAnotherManagerSeesThem(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var events []window.Event
	r.e.Post(func() { r.a.Window.OnEvent(func(e window.Event) { events = append(events, e) }) })
	r.e.Settle()
	r.openSecondWindow()
	r.e.Fire(1, 0, "focus", "")
	r.e.Settle()
	r.moveTab(1, 2, 0)

	var list []window.Window
	r.e.Post(func() { r.a.Window.SetBounds(2, engine.Rect{X: 200, Y: 100, W: 900, H: 600}) })
	r.e.Settle()
	// A state is set outright, and setting the one it is in asks the engine nothing.
	r.e.Post(func() {
		r.a.Window.SetState(2, window.StateZoomed)
	})
	r.e.Settle()
	r.e.Post(func() {
		r.a.Window.SetState(2, window.StateZoomed)
		r.a.Window.SetState(2, window.StateMinimised)
	})
	r.e.Settle()
	r.e.Post(func() { list = r.a.Window.List() })
	r.e.Settle()
	want := []window.Window{
		{ID: 2, Bounds: engine.Rect{X: 200, Y: 100, W: 900, H: 600}, State: window.StateMinimised, Front: true},
	}
	// Window 1 closed when its only tab moved away, so window 2 is in front.
	if !reflect.DeepEqual(list, want) || !slices.Equal(r.e.Modes(), []engine.WindowMode{engine.ModeZoomed, engine.ModeMinimised}) {
		t.Fatalf("windows %+v, modes asked %v", list, r.e.Modes())
	}
	wantEvents := []window.Event{
		{Kind: window.Opened, Window: 2}, {Kind: window.FrontChanged, Window: 2},
		{Kind: window.BoundsChanged, Window: 2},
		{Kind: window.FrontChanged, Window: 1}, {Kind: window.TabMoved, Window: 2, Tab: 1, From: 1, FromIndex: 0},
		{Kind: window.Closed, Window: 1}, {Kind: window.FrontChanged, Window: 2},
		{Kind: window.BoundsChanged, Window: 2},
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events %+v", events)
	}
}

// A DevTools client's new target with newWindow opens in a window of its own, and asking
// to show a tab brings its window to the front.
func TestADevToolsTargetInANewWindow(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var tab engine.TabID
	r.e.Post(func() {
		tab = r.e.CreateTab("")
		r.a.TabOpened(0, tab, "https://n.test/", "", true, true)
	})
	r.e.Settle()
	var inSecond []tabs.Tab
	var front engine.WindowID
	r.e.Post(func() { inSecond, front = r.a.Tabs.AllIn(2), r.a.Tabs.Front() })
	r.e.Settle()
	if len(inSecond) != 1 || inSecond[0].ID != tab || front != 2 {
		t.Fatalf("window 2 %+v, in front %v", inSecond, front)
	}
	before := len(r.e.Requests())
	r.e.Post(func() { r.a.ActivateRequested(1) })
	r.e.Settle()
	if req := r.e.Requests()[before:]; !slices.Equal(req, []engine.WindowRequest{engine.WindowFront}) {
		t.Fatalf("requests %v", req)
	}
}

// The app is ready once the shell shows its first snapshot: app.ready, the line tests wait
// for, is marked then and only once, not again for a shell that comes back from a crash.
func TestAppReadyOnceTheShellShowsItsState(t *testing.T) {
	_, from := timeline.Since(^uint64(0) >> 1)
	readies := func() int {
		entries, _ := timeline.Since(from)
		n := 0
		for _, e := range entries {
			if e.Kind == timeline.KindMark && e.Name == "app.ready" {
				n++
			}
		}
		return n
	}
	r := start(t, t.TempDir())
	defer r.stop()
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	sync := func() {
		done := make(chan struct{})
		r.e.Post(func() { close(done) })
		<-done
	}
	sync()
	if n := readies(); n != 0 {
		t.Fatalf("app.ready marked %d times before the shell showed anything", n)
	}
	r.shell(`{"t":"shown"}`)
	sync()
	if n := readies(); n != 1 {
		t.Fatalf("app.ready marked %d times once the shell showed its state", n)
	}
	r.e.Post(func() { r.a.ShellCrashed(1) })
	r.shell(`{"t":"ready"}`)
	r.shell(`{"t":"shown"}`)
	sync()
	if n := readies(); n != 1 {
		t.Fatalf("app.ready marked %d times after the shell came back", n)
	}
}

// The Profiles menu lists every profile as the list has it when the menu opens, with a
// tick on this one. The profile's colour is its frame colour: on the Dock icon and in
// the list.
func TestTheProfilesMenuAndTheProfilesColour(t *testing.T) {
	root := t.TempDir()
	list := profiles.New(root)
	c, err := list.Choose("", false)
	if err != nil {
		t.Fatal(err)
	}
	e := mockengine.New()
	var a *app.App
	ready := make(chan struct{})
	e.Post(func() {
		a = app.Start(e, app.Config{ProfileDir: c.Dir, Profiles: list, Profile: c.ID})
		e.SetListener(a)
		close(ready)
	})
	go e.Run()
	<-ready
	e.Settle()
	if got := e.DockColor(); got != "#4f9e9e" {
		t.Fatalf("the Dock shows %q", got)
	}
	var items engine.Menu
	e.Post(func() { items = a.MenuList("profiles") })
	e.Settle()
	if len(items) != 1 || items[0].Title != "Personal" || !items[0].Checked {
		t.Fatalf("the Profiles menu holds %+v", items)
	}
	e.Post(func() { a.Prefs.SetFrameColor("#3470d0") })
	e.Settle()
	if p, _, _ := list.Get(c.ID); e.DockColor() != "#3470d0" || p.Color != "#3470d0" {
		t.Fatalf("the Dock shows %q and the list %q", e.DockColor(), p.Color)
	}
}

// Tab, New Tab in Workspace lists the profile's workspaces, read as the menu opens, and
// choosing one opens a new tab in it in the window in front: Go and the engine have the tab
// there, a tab its page opens is there too, and its visits go into the profile's one
// history, as every tab's do.
func TestNewTabInWorkspaceFromTheMenu(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var items engine.Menu
	r.e.Post(func() { items = r.a.MenuList("workspaces") })
	r.e.Settle()
	if len(items) != 1 || items[0].Title != "Main" || items[0].Action != "workspaces:" || !items[0].Enabled {
		t.Fatalf("a new profile's list holds %+v", items)
	}
	var work workspaces.Workspace
	r.e.Post(func() {
		work, _ = r.a.Workspaces.Create("Work", "#5b7fd6")
		items = r.a.MenuList("workspaces")
	})
	r.e.Settle()
	if len(items) != 2 || items[1].Title != "Work" || items[1].Action != "workspaces:"+work.ID {
		t.Fatalf("the list holds %+v", items)
	}
	r.e.Post(func() { r.a.MenuChosen(items[1].Action) })
	m := r.waitFor(isTabs(2))
	tab := m["tabs"].([]any)[1].(map[string]any)
	id := engine.TabID(tab["id"].(float64))
	var in string
	r.e.Post(func() { in = r.e.Workspace(id) })
	r.e.Settle()
	if m["active"] != tab["id"] || tab["workspace"] != work.ID || in != work.ID {
		t.Fatalf("the new tab %v, active %v, is in %q in the engine", tab, m["active"], in)
	}
	r.show(tab["id"].(float64))
	r.visit("https://work.test/")
	r.e.Fire(1, 0, "open", "https://work.test/more")
	m = r.waitFor(isTabs(3))
	if opened := m["tabs"].([]any)[2].(map[string]any); opened["workspace"] != work.ID {
		t.Fatalf("the tab its page opened is %v", opened)
	}
	r.waitForTab(func(t map[string]any) bool {
		return t["url"] == "https://work.test/more" && strings.HasPrefix(t["title"].(string), "Mock")
	})
	if items := r.suggestions("work"); len(items) != 2 {
		t.Fatal("a visit in a workspace isn't in the history")
	}
}

// New Tab, by Cmd+T or the shell's newTab, opens in the workspace of the window's active
// tab. The shell's newTab can name a workspace instead, Main by "".
func TestANewTabOpensInTheActiveTabsWorkspace(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var work workspaces.Workspace
	r.e.Post(func() { work, _ = r.a.Workspaces.Create("Work", "#5b7fd6") })
	r.e.Settle()
	// opens runs a step and checks it opened one tab, active, in want, in Go and in the
	// engine.
	n := 1
	opens := func(step string, do func(), want string) {
		t.Helper()
		do()
		r.e.Settle()
		var tab tabs.Tab
		var count int
		var in string
		r.e.Post(func() {
			tab, count = r.a.Tabs.Active(), len(r.a.Tabs.All())
			in = r.e.Workspace(tab.ID)
		})
		r.e.Settle()
		n++
		if count != n || tab.Workspace != want || in != want {
			t.Fatalf("%s: %d tabs, the active one in %q, in %q in the engine; want %q", step, count, tab.Workspace, in, want)
		}
	}
	cmdT := func() { r.e.Fire(1, 0, "key", "newTab") }
	shellNewTab := func() { r.shell(`{"t":"newTab"}`) }
	opens("the shell names a workspace", func() { r.shell(`{"t":"newTab","workspace":"` + work.ID + `"}`) }, work.ID)
	opens("Cmd+T from a tab in it", cmdT, work.ID)
	opens("the shell's newTab from a tab in it", shellNewTab, work.ID)
	opens("the shell names Main", func() { r.shell(`{"t":"newTab","workspace":""}`) }, "")
	opens("Cmd+T from a tab in Main", cmdT, "")
	opens("the shell's newTab from a tab in Main", shellNewTab, "")
}

// A tab nothing places opens in the first workspace in the user's order: the first window's
// at start, a new window's and a link's from another app. The engine hears which is first at
// start and whenever another comes to the top, for the tabs it makes on its own.
func TestATabNothingPlacesOpensInTheFirstWorkspace(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "workspaces.json"), []byte(`{"version":2,"workspaces":[{"id":"0a1b2c3d","name":"Work","color":"#5b7fd6"},{"id":"","name":"Main","color":"#3470d0"}]}`), 0o600)
	r := start(t, dir)
	defer r.stop()
	// in says where a tab is, in Go and in the engine.
	in := func(id engine.TabID) string {
		tab, _ := r.a.Tabs.Get(id)
		return tab.Workspace + "/" + r.e.Workspace(id)
	}
	var atStart, inWindow, handed, firstThen, firstNow, afterMove string
	r.e.Post(func() {
		atStart, firstThen = in(r.a.Tabs.AllIn(1)[0].ID), r.e.FirstWorkspace()
		r.a.Handover([]string{"https://elsewhere.test/"})
		handed = in(r.a.Tabs.ActiveID())
	})
	// The page loads, and its visit is saved, before anything else.
	r.waitForTab(func(t map[string]any) bool {
		return t["url"] == "https://elsewhere.test/" && t["loading"] == false && strings.HasPrefix(t["title"].(string), "Mock")
	})
	r.e.Post(func() {
		_, tab := r.a.NewWindow("")
		inWindow = in(tab)
		r.a.Workspaces.Move("", -1)
		firstNow = r.e.FirstWorkspace()
		_, tab = r.a.NewWindow("")
		afterMove = in(tab)
	})
	r.e.Settle()
	work := "0a1b2c3d/0a1b2c3d"
	if atStart != work || inWindow != work || handed != work || firstThen != "0a1b2c3d" {
		t.Fatalf("with Work first: the first tab is in %q, a new window's in %q, a handed link's in %q; the engine has %q first", atStart, inWindow, handed, firstThen)
	}
	if firstNow != "" || afterMove != "/" {
		t.Fatalf("with Main moved to the top the engine has %q first, and a new window's tab is in %q", firstNow, afterMove)
	}
}

// View Source opens in the workspace of the tab whose source it shows, in Go and in the
// engine: the source is the page fetched again, with that workspace's cookies.
func TestViewSourceOpensInItsTabsWorkspace(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var work workspaces.Workspace
	r.e.Post(func() {
		work, _ = r.a.Workspaces.Create("Work", "#5b7fd6")
		r.a.Tabs.CreateIn(1, work.ID)
	})
	m := r.waitFor(isTabs(2))
	r.show(m["tabs"].([]any)[1].(map[string]any)["id"].(float64))
	r.visit("https://work.test/")
	r.e.Post(func() { r.a.MenuChosen("dev.viewSource") })
	m = r.waitFor(isTabs(3))
	source := m["tabs"].([]any)[2].(map[string]any)
	r.waitForTab(func(t map[string]any) bool {
		return t["url"] == "view-source:https://work.test/" && strings.HasPrefix(t["title"].(string), "Mock")
	})
	var in string
	r.e.Post(func() { in = r.e.Workspace(engine.TabID(source["id"].(float64))) })
	r.e.Settle()
	if source["workspace"] != work.ID || in != work.ID || m["active"] != source["id"] {
		t.Fatalf("the source's tab %v, active %v, is in %q in the engine", source, m["active"], in)
	}
}

// Every shell has the workspaces from its snapshot on, and hears of one made or renamed at
// once, so the dot on a tab in one is in its colour.
func TestTheShellHearsTheWorkspaces(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if l, ok := snap["workspaces"].([]any); !ok || len(l) != 1 || l[0].(map[string]any)["name"] != "Main" || l[0].(map[string]any)["id"] != "" {
		t.Fatalf("the snapshot's workspaces: %v", snap["workspaces"])
	}
	if tab := snap["tabs"].([]any)[0].(map[string]any); tab["workspace"] != "" {
		t.Fatalf("the first tab is %v", tab)
	}
	r.e.Post(func() { r.a.Workspaces.Create("Work", "#5b7fd6") })
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "workspaces" })
	w := m["workspaces"].([]any)[1].(map[string]any)
	if w["name"] != "Work" || w["color"] != "#5b7fd6" || w["id"] == "" {
		t.Fatalf("the shell heard %v", m)
	}
	r.e.Post(func() { r.a.Workspaces.SetName(w["id"].(string), "Office") })
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "workspaces" })
	if m["workspaces"].([]any)[1].(map[string]any)["name"] != "Office" {
		t.Fatalf("after the rename the shell heard %v", m)
	}
}

// The settings page's Workspaces section lists the workspaces, makes one with a name and a
// colour and renames one: Go keeps them in the profile's list, and the page, the shells and
// the Tab menu's list have them at once.
func TestTheSettingsPageMakesAndRenamesWorkspaces(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	got, snap := r.openSettings()
	if l, ok := snap["workspaces"].([]any); !ok || len(l) != 1 || l[0].(map[string]any)["name"] != "Main" {
		t.Fatalf("the snapshot's workspaces: %v", snap["workspaces"])
	}
	r.settings(`{"t":"createWorkspace","text":"Work","color":"#c2410c"}`)
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "workspaces" })
	w := m["workspaces"].([]any)[1].(map[string]any)
	if w["name"] != "Work" || w["color"] != "#c2410c" {
		t.Fatalf("the page heard %v", m)
	}
	r.waitFor(func(m map[string]any) bool { return m["t"] == "workspaces" })
	r.settings(`{"t":"renameWorkspace","id":"` + w["id"].(string) + `","text":"Office"}`)
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "workspaces" })
	var items engine.Menu
	r.e.Post(func() { items = r.a.MenuList("workspaces") })
	r.e.Settle()
	if len(items) != 2 || items[1].Title != "Office" {
		t.Fatalf("the Tab menu's list holds %+v", items)
	}
	if l := workspaces.New(dir).List(); len(l) != 2 || l[1].ID != w["id"] || l[1].Name != "Office" {
		t.Fatalf("the profile's list holds %+v", l)
	}
}

// The section's dialog edits a workspace's name and colour in one message, a name another
// has is refused whole, and Move up and Move down reorder the list for the Tab menu too.
// The page has the number of tabs open in each workspace, and hears it again when a tab
// opens or closes in one.
func TestTheSettingsPageEditsMovesAndCountsWorkspaces(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	var work, shop string
	r.e.Post(func() {
		w, _ := r.a.Workspaces.Create("Work", "#5b7fd6")
		s, _ := r.a.Workspaces.Create("Shopping", "#c2410c")
		work, shop = w.ID, s.ID
	})
	r.e.Settle()
	got, snap := r.openSettings()
	if c, ok := snap["workspaceTabs"].(map[string]any); !ok || len(c) != 1 || c[""] != float64(1) {
		t.Fatalf("the snapshot's counts: %v", snap["workspaceTabs"])
	}
	r.settings(`{"t":"editWorkspace","id":"` + work + `","text":"shopping","color":"#2f9e44"}`)
	r.settings(`{"t":"editWorkspace","id":"` + work + `","text":"Office","color":"#2f9e44"}`)
	r.settings(`{"t":"moveWorkspace","id":"` + shop + `","by":-1}`)
	r.waitOn(got, func(m map[string]any) bool {
		return m["t"] == "workspaces" && m["workspaces"].([]any)[1].(map[string]any)["id"] == shop
	})
	l := workspaces.New(dir).List()
	if len(l) != 3 || l[0].ID != "" || l[1].ID != shop || l[2].ID != work || l[2].Name != "Office" || l[2].Color != "#2f9e44" {
		t.Fatalf("the profile's list holds %+v", l)
	}
	var items engine.Menu
	r.e.Post(func() { items = r.a.MenuList("workspaces") })
	r.e.Settle()
	if len(items) != 3 || items[0].Title != "Main" || items[1].Title != "Shopping" || items[2].Title != "Office" {
		t.Fatalf("the Tab menu's list holds %+v", items)
	}
	var tab engine.TabID
	r.e.Post(func() { tab = r.a.Tabs.CreateIn(r.a.Tabs.Front(), work) })
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "workspaceTabs" })
	if c := m["workspaceTabs"].(map[string]any); len(c) != 2 || c[work] != float64(1) || c[""] != float64(1) {
		t.Fatalf("after a tab opened in Office the page heard %v", m)
	}
	r.e.Post(func() { r.a.Tabs.Close(tab) })
	m = r.waitOn(got, func(m map[string]any) bool { return m["t"] == "workspaceTabs" })
	if c := m["workspaceTabs"].(map[string]any); len(c) != 1 || c[work] != nil {
		t.Fatalf("after it closed the page heard %v", m)
	}
}

// New tab in workspace, from a workspace's row menu, opens a tab in it in the page's own
// window and closes the page, so the tab is what shows. An id the list doesn't hold opens
// nothing.
func TestTheSettingsPageOpensATabInAWorkspace(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var work string
	r.e.Post(func() {
		w, _ := r.a.Workspaces.Create("Work", "#5b7fd6")
		work = w.ID
	})
	r.e.Settle()
	r.openSettings()
	var before int
	r.e.Post(func() { before = len(r.a.Tabs.All()) })
	r.e.Settle()
	r.settings(`{"t":"openInWorkspace","id":"nothere"}`)
	r.settings(`{"t":"openInWorkspace","id":"` + work + `"}`)
	r.e.Settle()
	var in []string
	var open bool
	r.e.Post(func() {
		for _, tab := range r.a.Tabs.All()[before:] {
			in = append(in, tab.Workspace)
		}
		open = r.a.Settings.IsOpen(r.a.Tabs.Front())
	})
	r.e.Settle()
	if len(in) != 1 || in[0] != work {
		t.Fatalf("new tabs are in %q, want one in %s", in, work)
	}
	if open {
		t.Fatal("the settings page is still open")
	}
}

// Deleting a workspace from the settings page takes it out of the list everyone has, the Tab
// menu's too, and tells the engine the new first; closes its tabs in every window, a window
// that held only those getting a new tab in the first workspace; and only then has the
// engine erase its storage. The last one can't be deleted, nor can one the list doesn't
// hold.
func TestTheSettingsPageDeletesAWorkspace(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	var work string
	var w2 engine.WindowID
	r.e.Post(func() {
		w, _ := r.a.Workspaces.Create("Work", "#5b7fd6")
		work = w.ID
		r.a.Workspaces.Move(work, -1)
		r.a.Tabs.CreateIn(1, work)
		w2, _ = r.a.NewWindow("")
		r.a.WindowFocused(1)
	})
	r.e.Settle()
	got, _ := r.openSettings()
	var first string
	r.e.Post(func() { first = r.e.FirstWorkspace() })
	r.e.Settle()
	if first != work {
		t.Fatalf("with Work first the engine has %q", first)
	}
	r.settings(`{"t":"deleteWorkspace","id":"nothere"}`)
	r.settings(`{"t":"deleteWorkspace","id":"` + work + `"}`)
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "workspaces" })
	if l := m["workspaces"].([]any); len(l) != 1 || l[0].(map[string]any)["id"] != "" {
		t.Fatalf("the page heard %v", m)
	}
	r.waitOn(got, func(m map[string]any) bool {
		c, ok := m["workspaceTabs"].(map[string]any)
		return m["t"] == "workspaceTabs" && ok && len(c) == 1 && c[""] == float64(2)
	})
	r.e.Settle()
	var left, inW2 []tabs.Tab
	var erased []mockengine.Erase
	var items engine.Menu
	r.e.Post(func() {
		left, inW2, erased, first, items = r.a.Tabs.AllIn(1), r.a.Tabs.AllIn(w2), r.e.Erased(), r.e.FirstWorkspace(), r.a.MenuList("workspaces")
	})
	r.e.Settle()
	if len(left) != 1 || left[0].Workspace != "" || len(inW2) != 1 || inW2[0].Workspace != "" || r.e.Closed(w2) || r.e.Closed(1) {
		t.Fatalf("window 1 has %+v and window 2 %+v; window 2 closed %v, window 1 %v", left, inW2, r.e.Closed(w2), r.e.Closed(1))
	}
	if !reflect.DeepEqual(erased, []mockengine.Erase{{Workspace: work}}) || first != "" || len(items) != 1 || items[0].Title != "Main" {
		t.Fatalf("the engine erased %+v and has %q first; the Tab menu lists %+v", erased, first, items)
	}
	if l := workspaces.New(dir).List(); len(l) != 1 || l[0].ID != "" {
		t.Fatalf("the profile's list holds %+v", l)
	}
	r.settings(`{"t":"deleteWorkspace","id":""}`)
	r.e.Settle()
	r.e.Post(func() { left, erased = r.a.Tabs.All(), r.e.Erased() })
	r.e.Settle()
	if len(left) != 2 || len(erased) != 1 || len(workspaces.New(dir).List()) != 1 {
		t.Fatalf("deleting the last left %+v and erased %+v", left, erased)
	}
}

// A delete never closes a window: one whose every tab is in the workspace deleted, with
// the settings page open in it, stays, with one blank tab in the new first workspace, the
// page still open, and the engine is asked to erase the workspace.
func TestDeletingAWorkspaceKeepsAWindowOfOnlyItsTabs(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "workspaces.json"), []byte(`{"version":2,"workspaces":[{"id":"0a1b2c3d","name":"Work","color":"#5b7fd6"},{"id":"","name":"Main","color":"#3470d0"}]}`), 0o600)
	r := start(t, dir)
	defer r.stop()
	r.e.Post(func() { r.a.Tabs.Create(1) })
	r.openSettings()
	r.settings(`{"t":"deleteWorkspace","id":"0a1b2c3d"}`)
	r.e.Settle()
	var left []tabs.Tab
	var erased []mockengine.Erase
	var open bool
	in := "none"
	r.e.Post(func() {
		left, erased, open = r.a.Tabs.AllIn(1), r.e.Erased(), r.a.Settings.IsOpen(1)
		if len(left) == 1 {
			in = r.e.Workspace(left[0].ID)
		}
	})
	r.e.Settle()
	if r.e.Closed(1) || r.e.Quit() || !open || len(left) != 1 || left[0].Workspace != "" || left[0].URL != "" || in != "" {
		t.Fatalf("window 1 closed %v, the app quit %v, settings open %v; the window has %+v", r.e.Closed(1), r.e.Quit(), open, left)
	}
	if !reflect.DeepEqual(erased, []mockengine.Erase{{Workspace: "0a1b2c3d"}}) {
		t.Fatalf("the engine erased %+v", erased)
	}
}

// A shell changes the workspaces with the settings page's messages, naming a workspace by
// "workspace", under the same rules: it makes, edits and moves one, and every shell hears
// the list; its delete closes the workspace's tabs and only then has the engine erase it;
// and the last one isn't deleted.
func TestAShellChangesTheWorkspaces(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	list := func(m map[string]any) []any { l, _ := m["workspaces"].([]any); return l }
	r.shell(`{"t":"createWorkspace","text":"Work","color":"#5b7fd6"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "workspaces" && len(list(m)) == 2 })
	work := list(m)[1].(map[string]any)["id"].(string)
	r.shell(`{"t":"editWorkspace","workspace":"` + work + `","text":"Office","color":"#c2410c"}`)
	r.shell(`{"t":"moveWorkspace","workspace":"` + work + `","by":-1}`)
	r.waitFor(func(m map[string]any) bool {
		if m["t"] != "workspaces" {
			return false
		}
		w := list(m)[0].(map[string]any)
		return w["id"] == work && w["name"] == "Office" && w["color"] == "#c2410c"
	})
	var tab engine.TabID
	r.e.Post(func() { tab = r.a.Tabs.CreateIn(1, work) })
	r.shell(`{"t":"deleteWorkspace","workspace":"` + work + `"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "workspaces" && len(list(m)) == 1 })
	r.shell(`{"t":"deleteWorkspace","workspace":""}`)
	r.e.Settle()
	var left []tabs.Tab
	var erased []mockengine.Erase
	var open bool
	r.e.Post(func() { left, erased = r.a.Tabs.All(), r.e.Erased(); _, open = r.a.Tabs.Get(tab) })
	r.e.Settle()
	if open || len(left) != 1 || left[0].Workspace != "" || !reflect.DeepEqual(erased, []mockengine.Erase{{Workspace: work}}) {
		t.Fatalf("left %+v, the engine erased %+v", left, erased)
	}
	if l := workspaces.New(dir).List(); len(l) != 1 || l[0].ID != "" {
		t.Fatalf("the profile's list holds %+v", l)
	}
}

// View, Shell lists the shells the user can choose, with a tick on the one in use. Choosing
// another makes it the profile's and loads it in the window, which hears nothing until it
// says it's ready; what the old shell claimed of the window goes, and the tab stays shown.
func TestViewShellSwitchesTheShellLive(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	r.shell(`{"t":"layout","shell":[{"x":0,"y":0,"w":800,"h":48}],"tabs":[{"tab":1,"rect":{"x":0,"y":48,"w":800,"h":552}}],"viewW":800,"viewH":600}`)
	var items engine.Menu
	r.e.Post(func() { items = r.a.MenuList("shells") })
	r.e.Settle()
	listed := shells.Listed()
	if len(items) != len(listed) {
		t.Fatalf("View, Shell lists %+v", items)
	}
	for i, s := range listed {
		if items[i].Title != s.Name || items[i].Action != "shells:"+s.ID || items[i].Checked != (s.ID == "standard") {
			t.Fatalf("View, Shell lists %+v", items)
		}
	}
	for len(r.got) > 0 {
		<-r.got
	}
	r.e.Post(func() { r.a.MenuChosen("shells:minimal") })
	r.e.Settle()
	l := r.e.Layout(1)
	if r.e.PageOf(1) != "shell/minimal" || prefs.New(dir).Shell() != "minimal" || len(l.Regions) != 0 || len(l.Tabs) != 1 {
		t.Fatalf("the window loads %q, the pref is %q, the layout %+v", r.e.PageOf(1), prefs.New(dir).Shell(), l)
	}
	r.e.Post(func() { r.a.Shell.TabsChanged() })
	r.e.Settle()
	select {
	case b := <-r.got:
		t.Fatalf("the new shell heard %s before it said it's ready", b)
	default:
	}
	r.shell(`{"t":"ready"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
}

// A shell's own data is kept per window, as it last saved it, and comes back in the
// snapshot: after its shell crashed and reloaded too. Another window's shell has none. It
// goes when the shell changes.
func TestAShellsOwnDataIsKeptPerWindow(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	snapshot := func() map[string]any {
		r.e.Settle()
		for len(r.got) > 0 {
			<-r.got
		}
		r.shell(`{"t":"ready"}`)
		return r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	}
	if m := snapshot(); m["data"] != nil {
		t.Fatalf("a new window's shell has data %v", m["data"])
	}
	r.shell(`{"t":"data","data":{"panes":[1]}}`)
	r.shell(`{"t":"data","data":{"panes":[1,2]}}`)
	r.e.Post(func() { r.a.ShellCrashed(1) })
	if m := snapshot(); !reflect.DeepEqual(m["data"], map[string]any{"panes": []any{1.0, 2.0}}) {
		t.Fatalf("after a crash the shell has %v", m["data"])
	}
	var w2 engine.WindowID
	r.e.Post(func() { w2, _ = r.a.NewWindow("") })
	r.e.Settle()
	got, stop := r.e.Feeds(w2).Shell.Subscribe()
	defer stop()
	r.e.Post(func() { r.a.ShellMessage(w2, []byte(`{"t":"ready"}`)) })
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "snapshot" }); m["data"] != nil {
		t.Fatalf("another window's shell has %v", m["data"])
	}
	r.e.Post(func() { r.a.MenuChosen("shells:minimal") })
	if m := snapshot(); m["data"] != nil {
		t.Fatalf("the next shell has %v", m["data"])
	}
}

// A shell's own action runs only while its shell is the profile's, and sends that shell
// action with its id. With another shell on, its key isn't taken, so the page keeps it, and
// the shell hears nothing.
func TestAShellsOwnActionRunsOnlyWhileItsShellIsOn(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	took := func(k engine.Key) bool {
		var ok bool
		r.e.Post(func() { ok = r.a.Key(k, engine.KeyUnhandled, engine.SurfacePage) })
		r.e.Settle()
		return ok
	}
	cmdD := engine.Key{Code: 'D', Mods: engine.ModCmd}
	shiftCmdD := engine.Key{Code: 'D', Mods: engine.ModCmd | engine.ModShift}
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if took(cmdD) || took(shiftCmdD) {
		t.Fatal("with the standard shell on, the terminal's keys were taken")
	}
	r.e.Post(func() { r.a.MenuChosen("terminal.splitRight") })
	r.e.Settle()
	for len(r.got) > 0 {
		var m map[string]any
		json.Unmarshal(<-r.got, &m)
		if m["t"] == "action" {
			t.Fatalf("the standard shell heard %v", m)
		}
	}

	r.e.Post(func() { r.a.MenuChosen("shells:terminal") })
	r.shell(`{"t":"ready"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if !took(cmdD) {
		t.Fatal("with the terminal shell on, Cmd+D wasn't taken")
	}
	if m := r.waitFor(func(m map[string]any) bool { return m["t"] == "action" }); m["id"] != "terminal.splitRight" {
		t.Fatalf("Cmd+D sent %v", m)
	}
	if !took(shiftCmdD) {
		t.Fatal("with the terminal shell on, Shift+Cmd+D wasn't taken")
	}
	if m := r.waitFor(func(m map[string]any) bool { return m["t"] == "action" }); m["id"] != "terminal.splitDown" {
		t.Fatalf("Shift+Cmd+D sent %v", m)
	}
}

// The tab groups through the shell's messages: a new tab beside one starts a group of the
// two, which every tabs message carries; a tab a page opens joins its opener's group;
// editGroup renames it; closeGroup closes its tabs.
func TestGroupsThroughTheShell(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	groups := func(m map[string]any) []any { g, _ := m["groups"].([]any); return g }

	r.shell(`{"t":"newTab","groupWith":1}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(groups(m)) == 1 })
	g := groups(m)[0].(map[string]any)
	if ids := g["tabs"].([]any); len(ids) != 2 || ids[0].(float64) != 1 || g["color"] != tabs.GroupColors[0] {
		t.Fatalf("group %v", g)
	}
	r.e.Fire(1, 1, "open", "https://c.test/")
	m = r.waitFor(func(m map[string]any) bool {
		return m["t"] == "tabs" && len(groups(m)) == 1 && len(groups(m)[0].(map[string]any)["tabs"].([]any)) == 3
	})
	r.shell(`{"t":"editGroup","group":"` + g["id"].(string) + `","name":"Release"}`)
	r.waitFor(func(m map[string]any) bool {
		return m["t"] == "tabs" && len(groups(m)) == 1 && groups(m)[0].(map[string]any)["name"] == "Release"
	})
	r.shell(`{"t":"newTab"}`)
	r.shell(`{"t":"closeGroup","group":"` + g["id"].(string) + `"}`)
	r.waitFor(func(m map[string]any) bool {
		return m["t"] == "tabs" && len(m["tabs"].([]any)) == 1 && len(groups(m)) == 0
	})
}

// What a shell saves for the profile reaches every window's shell, and comes back in the
// snapshot after a restart.
func TestProfileDataOutlivesARestart(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	r.shell(`{"t":"profileData","data":{"pinned":["https://a.test/"]}}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "profileData" })
	if p := m["profileData"].(map[string]any)["pinned"].([]any); len(p) != 1 {
		t.Fatalf("profileData %v", m)
	}
	r.stop()
	r = start(t, dir)
	defer r.stop()
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if p, _ := m["profileData"].(map[string]any); p == nil || len(p["pinned"].([]any)) != 1 {
		t.Fatalf("snapshot profileData %v", m["profileData"])
	}
}

// A shell's snapshot lists the actions it can run, by title.
func TestTheShellHearsTheActionsByTitle(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	found := false
	for _, a := range m["actions"].([]any) {
		found = found || a.(map[string]any)["id"] == "app.settings"
	}
	if !found {
		t.Fatalf("no app.settings in the snapshot's actions: %v", m["actions"])
	}
}

// The glide keymap gives Cmd+T to the Command Bar, Cmd+D to pinning and Option+Cmd+Right to
// the next Space; Ctrl+1 is Space 1 in every keymap. Each reaches Glide as its action.
func TestTheGlideKeymap(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	r.e.Post(func() { r.a.MenuChosen("shells:glide") })
	r.shell(`{"t":"ready"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	press := func(keys, want string) {
		t.Helper()
		k, err := commands.ParseKeys(keys)
		if err != nil {
			t.Fatal(err)
		}
		var took bool
		r.e.Post(func() {
			took = r.a.Key(k, engine.KeyBeforePage, engine.SurfacePage) || r.a.Key(k, engine.KeyUnhandled, engine.SurfacePage)
		})
		r.e.Settle()
		if !took {
			t.Fatalf("%v wasn't taken", k)
		}
		if m := r.waitFor(func(m map[string]any) bool { return m["t"] == "action" }); m["id"] != want {
			t.Fatalf("%v sent %v, want %s", k, m, want)
		}
	}
	press("Ctrl+1", "glide.space1")
	r.e.Post(func() { r.a.Commands.UseTemplate("glide") })
	r.e.Settle()
	press("Cmd+T", "glide.commandBar")
	press("Cmd+D", "glide.pin")
	press("Option+Cmd+Right", "glide.spaceNext")
}

// The settings page's Install update hands a ready release to the installer, as the
// shell's button does, on a phone build: it once relaunched the app instead, which on a
// phone does nothing.
func TestTheSettingsPageInstallsAReadyUpdate(t *testing.T) {
	apk := []byte("an APK, as far as the updater cares")
	sum := sha256.Sum256(apk)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/asset" {
			w.Write(apk)
			return
		}
		fmt.Fprintf(w, `{"spec":1,"project":"project-b","published":"2026-11-14T00:00:00Z","channels":{"stable":"0.9.0"},
		  "releases":{"0.9.0":{"assets":{"%s/%s":{"url":"%s/asset","sha256":"%s","size":%d}}}}}`,
			runtime.GOOS, runtime.GOARCH, srv.URL, hex.EncodeToString(sum[:]), len(apk))
	}))
	defer srv.Close()
	dir := t.TempDir()
	installed := make(chan string, 4)
	r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Update: update.Config{
		Project: "project-b", Version: "0.1.0", Releases: srv.URL, StateDir: filepath.Join(dir, "updates"),
		Install: func(apk string) { installed <- apk }}})
	defer r.stop()
	r.e.Post(func() { r.a.Update.CheckNow() })
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var s update.State
		done := make(chan struct{})
		r.e.Post(func() { s = r.a.Update.State(); close(done) })
		<-done
		if s.Phase == update.Ready {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	r.settings(`{"t":"relaunch"}`)
	select {
	case got := <-installed:
		if b, err := os.ReadFile(got); err != nil || string(b) != string(apk) {
			t.Fatalf("installed %q: %q %v", got, b, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the settings page's Install update never reached the installer")
	}
}
