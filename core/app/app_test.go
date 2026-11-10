package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	return r
}

func (r *rig) shell(cmd string) { r.e.Post(func() { r.a.ShellMessage([]byte(cmd)) }) }

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
	r.shell(`{"t":"regions","rects":[{"x":0,"y":0,"w":800,"h":48},{"x":100,"y":48,"w":400,"h":200}]}`)
	r.shell(`{"t":"dragRegions","rects":[{"x":300,"y":0,"w":500,"h":32}]}`)
	r.shell(`{"t":"ready"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })

	for _, c := range []struct {
		x, y float64
		want string
	}{{10, 10, "shell"}, {120, 100, "shell"}, {10, 100, "page"}, {600, 300, "page"}, {400, 10, "drag"}, {400, 40, "shell"}} {
		if got := r.e.Hit(c.x, c.y); got != c.want {
			t.Errorf("Hit(%v,%v) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestTabsThroughTheShellAndShortcuts(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")

	r.shell(`{"t":"newTab"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
	second := m["active"].(float64)
	if second == 1 {
		t.Fatalf("new tab should be active: %v", m)
	}
	if hidden := r.e.Hidden(); len(hidden) != 1 || hidden[0] != 1 {
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

	r.e.Fire("shortcut", "tab1")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && m["active"] == float64(1) })
	r.e.Fire("shortcut", "nextTab")
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && m["active"] == second })
	r.e.Fire("shortcut", "closeTab")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
	if m["active"] == second {
		t.Fatalf("closed tab still active: %v", m)
	}

	// Closing the rest closes the window.
	r.e.Fire("shortcut", "closeTab")
	r.e.Fire("shortcut", "closeTab")
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
	if m["windowColor"] != "#4f9e9e" || r.e.WindowColor() != "#4f9e9e" || m["frameWidth"] != 4.0 || r.e.FrameWidth() != 4 {
		t.Fatalf("window frame: shell %v %v, engine %v %v", m["windowColor"], m["frameWidth"], r.e.WindowColor(), r.e.FrameWidth())
	}

	r.e.Fire("fullscreen", "")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "controls" })
	if m["controls"].(map[string]any)["w"] != float64(0) || m["cornerRadius"] != float64(0) {
		t.Fatalf("full screen should report no controls and square corners: %v", m)
	}
}

var _ engine.Listener = (*app.App)(nil)

// The settings panel is the shell's: the snapshot carries what it shows, the shell sets
// the prefs, and Go says when they change.
func TestTheShellReadsAndSetsTheSettings(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	r.shell(`{"t":"ready"}`)
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if snap["autoUpdate"] != true || snap["running"] != "" {
		t.Fatalf("snapshot = %v", snap)
	}
	r.shell(`{"t":"setAutoUpdate","on":false}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "autoUpdate" && m["autoUpdate"] == false })
	b, _ := os.ReadFile(filepath.Join(dir, "prefs.json"))
	if !strings.Contains(string(b), `"autoUpdate":false`) {
		t.Fatalf("prefs.json = %s", b)
	}
}

// The frame colour is a pref: setting it from the shell repaints the window and tells the
// shell, together, and it survives a restart.
func TestTheFrameColorChangesTheWindowAndTheShellLive(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	r.shell(`{"t":"setFrameColor","text":"#3470d0"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "windowColor" && m["windowColor"] == "#3470d0" })
	if r.e.WindowColor() != "#3470d0" {
		t.Fatalf("window = %v", r.e.WindowColor())
	}
	r.stop()

	r = start(t, dir)
	defer r.stop()
	r.shell(`{"t":"ready"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if m["windowColor"] != "#3470d0" || r.e.WindowColor() != "#3470d0" || r.e.ShownColor() != "#3470d0" {
		t.Fatalf("after restart: shell %v, window %v, shown %v", m["windowColor"], r.e.WindowColor(), r.e.ShownColor())
	}
}
