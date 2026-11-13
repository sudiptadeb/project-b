//go:build extensions

package app_test

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"projectb/engine"
	"projectb/extensions"
	"projectb/flags"
	"projectb/mockengine"
)

// The test extension's source, and the id tools/pack-test-extension.py's key gives it.
const (
	testExtension   = "../../tools/test-extension"
	testExtensionID = "oepjjojhhaclfhmopocgmdfbeiffpheh"
)

// extRig is the rig with the app's extension manager; the mock engine has a fake
// extension runtime.
type extRig struct {
	*rig
	x *mockengine.Engine
	m *extensions.Manager
}

func startExtensions(t *testing.T, dir string) *extRig {
	t.Helper()
	flags.New(dir).Set(flags.Extensions, "on")
	r := start(t, dir)
	return &extRig{rig: r, x: r.e, m: r.a.Extensions()}
}

// With the Extensions flag off, the build's extension code doesn't run.
func TestExtensionsWaitForTheirFlag(t *testing.T) {
	r := start(t, t.TempDir())
	if r.a.Extensions() != nil {
		t.Fatal("the extension manager started with the flag off")
	}
}

// on runs fn on the main thread and waits for it.
func (r *extRig) on(fn func()) {
	done := make(chan struct{})
	r.e.Post(func() { fn(); close(done) })
	<-done
}

// until waits for cond, which runs on the main thread, to hold.
func (r *extRig) until(what string, cond func() bool) {
	r.t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		var ok bool
		r.on(func() { ok = cond() })
		if ok {
			return
		}
	}
	r.t.Fatalf("timed out waiting for %s", what)
}

func (r *extRig) call(name, args string, tab engine.TabID) mockengine.Answer {
	r.t.Helper()
	a := r.x.Call(testExtensionID, name, args, tab)
	if a.Error != "" {
		r.t.Fatalf("%s%s: %s", name, args, a.Error)
	}
	return a
}

func (r *extRig) events(name string) []mockengine.Event {
	var out []mockengine.Event
	for _, e := range r.x.ExtensionState().Events {
		if e.Name == name {
			out = append(out, e)
		}
	}
	return out
}

// pack writes the test extension as a package the mock runtime reads: a CRX3 with the
// id in its header and no signature, which only the real engine checks.
func pack(t *testing.T, file, version string) {
	t.Helper()
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	entries, err := os.ReadDir(testExtension)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, _ := os.ReadFile(filepath.Join(testExtension, entry.Name()))
		if entry.Name() == "manifest.json" {
			data = bytes.Replace(data, []byte(`"version": "1.0"`), []byte(`"version": "`+version+`"`), 1)
		}
		w, _ := z.Create(entry.Name())
		w.Write(data)
	}
	z.Close()
	id := make([]byte, 16)
	for i := range id {
		id[i] = (testExtensionID[2*i]-'a')<<4 | (testExtensionID[2*i+1] - 'a')
	}
	// Field 10000, the signed data, holding field 1, the id.
	header := append([]byte{0x82, 0xf1, 0x04, 18, 0x0a, 16}, id...)
	out := append([]byte("Cr24"), binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint32(nil, 3), uint32(len(header)))...)
	out = append(append(out, header...), archive.Bytes()...)
	if err := os.WriteFile(file, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// install installs the test extension from a package file, as Install package does, and
// says yes to the prompt.
func (r *extRig) install() {
	r.t.Helper()
	r.beginInstall()
	r.on(func() { r.m.AnswerPrompt(true) })
	r.until("the install", func() bool { return len(r.m.Installed()) == 1 })
}

// beginInstall picks a package of the test extension and waits for the engine to ask.
func (r *extRig) beginInstall() {
	r.t.Helper()
	file := filepath.Join(r.t.TempDir(), "test.crx")
	pack(r.t, file, "1.0")
	r.x.FireExtension("choose", "", file)
	r.on(func() { r.m.InstallPackage(1) })
	r.until("the install prompt", func() bool { return r.m.Prompt() != nil })
}

func TestInstallAsksFirst(t *testing.T) {
	r := startExtensions(t, t.TempDir())
	r.beginInstall()

	var v extensions.View
	r.on(func() { v = r.m.View() })
	if p := v.Prompt; p.Name != "Project B test extension" || p.Version != "1.0" || !strings.Contains(strings.Join(p.Permissions, " "), "tabs") {
		t.Fatalf("prompt: %+v", p)
	}
	if len(v.Installed) != 0 {
		t.Fatalf("installed before the answer: %d", len(v.Installed))
	}

	// The shell has the prompt too, for an install a store's page begins, and answers it.
	// It asks again, as a reloaded shell would; the snapshot from its start came before the prompt.
	r.shell(`{"t":"ready"}`)
	m := r.waitFor(func(m map[string]any) bool {
		x, _ := m["extensions"].(map[string]any)
		return m["t"] == "snapshot" && x["prompt"] != nil
	})
	if p, _ := m["extensions"].(map[string]any)["prompt"].(map[string]any); p["name"] != "Project B test extension" {
		t.Fatalf("the shell's prompt: %v", m["extensions"])
	}
	r.shell(`{"t":"extensionAnswerPrompt","on":true}`)
	r.until("the install", func() bool { return len(r.m.Installed()) == 1 })
	r.on(func() { v = r.m.View() })
	if e := v.Installed[0]; e.ID != testExtensionID || !e.Enabled || e.ActionPopup != "chrome-extension://"+testExtensionID+"/popup.html" {
		t.Fatalf("installed: %+v", e)
	}
	if v.Prompt != nil || v.Error != "" {
		t.Fatalf("after the install: %+v", v)
	}
}

func TestDecliningThePromptInstallsNothing(t *testing.T) {
	r := startExtensions(t, t.TempDir())
	r.beginInstall()
	r.on(func() { r.m.AnswerPrompt(false) })
	r.until("the prompt to go", func() bool { return r.m.Prompt() == nil })
	r.on(func() {
		if v := r.m.View(); len(v.Installed) != 0 || v.Error != "" {
			t.Fatalf("after declining: %+v", v)
		}
	})
}

func TestLoadUnpackedAndInstallAPackageFile(t *testing.T) {
	r := startExtensions(t, t.TempDir())
	folder, _ := filepath.Abs(testExtension)
	r.x.FireExtension("choose", "", folder)
	r.on(func() { r.m.LoadUnpacked(1) })
	r.until("the unpacked extension", func() bool { return len(r.m.Installed()) == 1 })
	r.on(func() {
		if e := r.m.Installed()[0]; !e.Unpacked || e.Path != folder || e.ID == testExtensionID {
			t.Fatalf("unpacked: %+v", e)
		}
	})

	file := filepath.Join(t.TempDir(), "test.crx")
	pack(t, file, "1.1")
	r.x.FireExtension("choose", "", file)
	r.on(func() { r.m.InstallPackage(1) })
	r.until("the install prompt", func() bool { return r.m.View().Prompt != nil })
	r.on(func() { r.m.AnswerPrompt(true) })
	r.until("the packaged extension", func() bool { return len(r.m.Installed()) == 2 })
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("a package the user picked was removed: %v", err)
	}

	// Picking nothing does nothing, and a folder with no manifest is an error to show.
	r.on(func() { r.m.LoadUnpacked(1) })
	r.x.FireExtension("choose", "", t.TempDir())
	r.on(func() { r.m.LoadUnpacked(1) })
	r.until("the error", func() bool { return r.m.View().Error != "" })
	r.on(func() {
		if n := len(r.m.Installed()); n != 2 {
			t.Fatalf("%d installed", n)
		}
	})
}

func TestPinsButtonsAndThePopup(t *testing.T) {
	dir := t.TempDir()
	r := startExtensions(t, dir)
	r.install()

	var buttons []extensions.Button
	r.on(func() { buttons = r.m.Buttons() })
	if len(buttons) != 1 || buttons[0].Pinned || !buttons[0].Enabled || buttons[0].Title != "Project B test extension" || !strings.HasPrefix(buttons[0].Icon, "data:image/png") {
		t.Fatalf("buttons: %+v", buttons)
	}

	r.shell(`{"t":"extensionPin","extension":"` + testExtensionID + `","on":true}`)
	m := r.waitFor(func(m map[string]any) bool {
		x, ok := m["extensions"].(map[string]any)
		if !ok || m["t"] != "extensions" || len(x["buttons"].([]any)) == 0 {
			return false
		}
		return x["buttons"].([]any)[0].(map[string]any)["pinned"] == true
	})
	if m == nil {
		t.Fatal("no pinned button")
	}
	saved, _ := os.ReadFile(filepath.Join(dir, "extensions.json"))
	if !strings.Contains(string(saved), testExtensionID) {
		t.Fatalf("the pin was not saved: %s", saved)
	}

	// A press opens the popup under the button; a second press closes it.
	click := `{"t":"extensionClick","extension":"` + testExtensionID + `","rect":{"x":900,"y":8,"w":28,"h":28}}`
	r.shell(click)
	r.until("the popup", func() bool { return r.x.ExtensionState().Popup != nil })
	if p := r.x.ExtensionState().Popup; p.URL != "chrome-extension://"+testExtensionID+"/popup.html" || p.Anchor.X != 900 || p.Anchor.W != 28 {
		t.Fatalf("popup: %+v", p)
	}
	r.shell(click)
	r.until("the popup to close", func() bool { return r.x.ExtensionState().Popup == nil && r.m.OpenPopup() == "" })

	// The user clicking elsewhere closes it too.
	r.shell(click)
	r.until("the popup", func() bool { return r.m.OpenPopup() == testExtensionID })
	r.x.FireExtension("closePopup", "", "")
	r.until("the popup to close", func() bool { return r.m.OpenPopup() == "" })

	// With no popup, a press tells the extension instead.
	r.call("action.setPopup", `[{"popup":""}]`, 0)
	r.shell(click)
	r.until("action.onClicked", func() bool { return len(r.events("action.onClicked")) == 1 })

	r.shell(`{"t":"extensionPin","extension":"` + testExtensionID + `","on":false}`)
	r.until("the unpin", func() bool { return !r.m.Pinned(testExtensionID) })

	// Removing it takes its button away.
	r.on(func() { r.m.Pin(testExtensionID, true); r.m.Remove(testExtensionID) })
	r.until("the removal", func() bool { return len(r.m.Buttons()) == 0 && len(r.m.View().Pinned) == 0 })
}

func TestDisabledExtensionHasNoButton(t *testing.T) {
	r := startExtensions(t, t.TempDir())
	r.install()
	r.on(func() { r.m.SetEnabled(testExtensionID, false) })
	r.until("the button to go", func() bool { return len(r.m.Buttons()) == 0 && len(r.m.Installed()) == 1 })
	r.on(func() { r.m.SetEnabled(testExtensionID, true) })
	r.until("the button to return", func() bool { return len(r.m.Buttons()) == 1 })
}

func TestTabsAPI(t *testing.T) {
	r := startExtensions(t, t.TempDir())
	r.install()
	r.visit("https://example.com/")

	var all []map[string]any
	json.Unmarshal(r.call("tabs.query", `[{}]`, 0).Result, &all)
	if len(all) != 1 || all[0]["url"] != "https://example.com/" || all[0]["active"] != true || all[0]["windowId"] != 1.0 || all[0]["status"] != "complete" {
		t.Fatalf("tabs.query: %v", all)
	}
	first := engine.TabID(all[0]["id"].(float64))

	// A tab made in the background stays there; the events say what happened.
	var made map[string]any
	json.Unmarshal(r.call("tabs.create", `[{"url":"https://example.org/","active":false}]`, 0).Result, &made)
	second := engine.TabID(made["id"].(float64))
	if made["active"] != false || made["index"] != 1.0 {
		t.Fatalf("tabs.create: %v", made)
	}
	r.until("tabs.onCreated", func() bool { return len(r.events("tabs.onCreated")) >= 1 })
	r.until("the new tab to load", func() bool {
		for _, e := range r.events("tabs.onUpdated") {
			if strings.Contains(string(e.Args), `"status":"complete"`) && strings.Contains(string(e.Args), "example.org") {
				return true
			}
		}
		return false
	})
	json.Unmarshal(r.call("tabs.query", `[{"active":true,"currentWindow":true}]`, 0).Result, &all)
	if len(all) != 1 || engine.TabID(all[0]["id"].(float64)) != first {
		t.Fatalf("the active tab: %v", all)
	}
	for query, want := range map[string]int{
		`[{"url":"*://*.example.org/*"}]`:                   1,
		`[{"url":["https://example.com/*","*://*.org/*"]}]`: 2,
		`[{"url":"http://*/*"}]`:                            0,
		`[{"title":"*"}]`:                                   2,
		`[{"title":"no such title"}]`:                       0,
	} {
		json.Unmarshal(r.call("tabs.query", query, 0).Result, &all)
		if len(all) != want {
			t.Fatalf("tabs.query %s: %v", query, all)
		}
	}

	// A page inside the extension is named by its path.
	json.Unmarshal(r.call("tabs.create", `[{"url":"popup.html"}]`, 0).Result, &made)
	if made["url"] != "chrome-extension://"+testExtensionID+"/popup.html" {
		t.Fatalf("tabs.create with a path: %v", made)
	}
	third := engine.TabID(made["id"].(float64))

	before := len(r.events("tabs.onActivated"))
	r.call("tabs.update", `[`+itoa(second)+`,{"active":true}]`, 0)
	r.until("tabs.onActivated", func() bool { return len(r.events("tabs.onActivated")) == before+1 })
	r.on(func() {
		if r.a.Tabs.ActiveID() != second {
			t.Fatalf("active is %d, not %d", r.a.Tabs.ActiveID(), second)
		}
	})

	// getCurrent is the tab the caller's page is in, and nothing for a popup.
	var current map[string]any
	json.Unmarshal(r.call("tabs.getCurrent", `[]`, second).Result, &current)
	if engine.TabID(current["id"].(float64)) != second {
		t.Fatalf("tabs.getCurrent: %v", current)
	}
	if a := r.call("tabs.getCurrent", `[]`, 0); len(a.Result) != 0 {
		t.Fatalf("tabs.getCurrent from a popup: %s", a.Result)
	}

	r.call("tabs.remove", `[[`+itoa(second)+`,`+itoa(third)+`]]`, 0)
	r.until("tabs.onRemoved", func() bool { return len(r.events("tabs.onRemoved")) == 2 })
	if a := r.x.Call(testExtensionID, "tabs.get", `[`+itoa(second)+`]`, 0); !strings.Contains(a.Error, "No tab with id") {
		t.Fatalf("tabs.get of a closed tab: %+v", a)
	}
	if a := r.x.Call(testExtensionID, "tabs.move", `[1, {"index": 0}]`, 0); !strings.Contains(a.Error, "not available") {
		t.Fatalf("a function Go doesn't have: %+v", a)
	}

	var w map[string]any
	json.Unmarshal(r.call("windows.getCurrent", `[{"populate":true}]`, 0).Result, &w)
	if w["id"] != 1.0 || w["focused"] != true || len(w["tabs"].([]any)) != 1 {
		t.Fatalf("windows.getCurrent: %v", w)
	}
}

// Each window is one of Go's: the calls find, open, move, size and close them, and the
// events say what happened to each.
func TestWindowsAPI(t *testing.T) {
	r := startExtensions(t, t.TempDir())
	r.install()
	r.visit("https://example.com/")
	var first engine.TabID
	var w1, w2 engine.WindowID
	r.on(func() {
		first, w1 = r.a.Tabs.ActiveID(), r.a.Tabs.Front()
		w2, _ = r.a.NewWindow("https://example.org/")
	})
	r.until("windows.onFocusChanged", func() bool { return len(r.events("windows.onFocusChanged")) > 0 })
	if len(r.events("windows.onCreated")) != 1 {
		t.Fatalf("windows.onCreated: %v", r.events("windows.onCreated"))
	}
	id := func(v any) engine.WindowID { return engine.WindowID(v.(float64)) }

	var all []map[string]any
	json.Unmarshal(r.call("windows.getAll", `[{"populate":true}]`, 0).Result, &all)
	if len(all) != 2 || id(all[0]["id"]) != w1 || all[0]["focused"] != false || all[1]["focused"] != true || len(all[1]["tabs"].([]any)) != 1 {
		t.Fatalf("windows.getAll: %v", all)
	}

	// The current window is the caller's tab's, or for a popup the one in front.
	var w map[string]any
	json.Unmarshal(r.call("windows.getCurrent", `[]`, first).Result, &w)
	if id(w["id"]) != w1 {
		t.Fatalf("windows.getCurrent from a tab in the first window: %v", w)
	}
	var tabs []map[string]any
	json.Unmarshal(r.call("tabs.query", `[{"active":true,"currentWindow":true}]`, 0).Result, &tabs)
	if len(tabs) != 1 || tabs[0]["url"] != "https://example.org/" || id(tabs[0]["windowId"]) != w2 || tabs[0]["index"] != 0.0 {
		t.Fatalf("the active tab in the window in front: %v", tabs)
	}
	json.Unmarshal(r.call("tabs.query", `[{"active":true}]`, 0).Result, &tabs)
	if len(tabs) != 2 {
		t.Fatalf("each window has an active tab: %v", tabs)
	}

	// A tab made in a window behind goes there.
	var made map[string]any
	json.Unmarshal(r.call("tabs.create", `[{"url":"https://example.net/","windowId":`+itoa(engine.TabID(w1))+`}]`, 0).Result, &made)
	if id(made["windowId"]) != w1 || made["index"] != 1.0 {
		t.Fatalf("tabs.create in the first window: %v", made)
	}

	// A size and a state.
	r.call("windows.update", `[`+itoa(engine.TabID(w2))+`,{"left":10,"top":20,"width":800,"height":600}]`, 0)
	r.until("windows.onBoundsChanged", func() bool {
		got, _ := r.a.Window.Get(w2)
		return got.Bounds == engine.Rect{X: 10, Y: 20, W: 800, H: 600}
	})
	if len(r.events("windows.onBoundsChanged")) == 0 {
		t.Fatal("no windows.onBoundsChanged")
	}
	r.call("windows.update", `[`+itoa(engine.TabID(w2))+`,{"state":"maximized"}]`, 0)
	json.Unmarshal(r.call("windows.get", `[`+itoa(engine.TabID(w2))+`]`, 0).Result, &w)
	if w["state"] != "maximized" {
		t.Fatalf("windows.update to maximized: %v", w)
	}
	if a := r.x.Call(testExtensionID, "windows.update", `[`+itoa(engine.TabID(w2))+`,{"state":"minimized","left":0}]`, 0); a.Error == "" {
		t.Fatal("a place went with the minimized state")
	}

	// A tab moved into a window of its own, and that window closed.
	tab := engine.TabID(made["id"].(float64))
	json.Unmarshal(r.call("windows.create", `[{"tabId":`+itoa(tab)+`}]`, 0).Result, &w)
	w3 := id(w["id"])
	if w3 == w1 || w3 == w2 || len(w["tabs"].([]any)) != 1 {
		t.Fatalf("windows.create with a tab: %v", w)
	}
	if d, at := r.events("tabs.onDetached"), r.events("tabs.onAttached"); len(d) != 1 || len(at) != 1 ||
		string(d[0].Args) != `[`+itoa(tab)+`,{"oldPosition":1,"oldWindowId":`+itoa(engine.TabID(w1))+`}]` ||
		string(at[0].Args) != `[`+itoa(tab)+`,{"newPosition":0,"newWindowId":`+itoa(engine.TabID(w3))+`}]` {
		t.Fatalf("the move's events: %v %v", d, at)
	}
	r.call("windows.remove", `[`+itoa(engine.TabID(w3))+`]`, 0)
	r.until("windows.onRemoved", func() bool { return len(r.events("windows.onRemoved")) == 1 })
	r.until("tabs.onRemoved", func() bool { return len(r.events("tabs.onRemoved")) == 1 })
	if a := r.events("tabs.onRemoved")[0]; string(a.Args) != `[`+itoa(tab)+`,{"isWindowClosing":true,"windowId":`+itoa(engine.TabID(w3))+`}]` {
		t.Fatalf("tabs.onRemoved: %s", a.Args)
	}
	if a := r.x.Call(testExtensionID, "windows.get", `[`+itoa(engine.TabID(w3))+`]`, 0); !strings.Contains(a.Error, "No window with id") {
		t.Fatalf("windows.get of a closed window: %+v", a)
	}
}

func itoa(id engine.TabID) string {
	b, _ := json.Marshal(id)
	return string(b)
}

func TestActionAPI(t *testing.T) {
	r := startExtensions(t, t.TempDir())
	r.install()
	r.visit("https://example.com/")
	var tab engine.TabID
	r.on(func() { tab = r.a.Tabs.ActiveID() })
	button := func() extensions.Button {
		var b extensions.Button
		r.on(func() { b = r.m.Buttons()[0] })
		return b
	}

	r.call("action.setBadgeText", `[{"text":"7"}]`, 0)
	r.call("action.setBadgeBackgroundColor", `[{"color":[255,0,0,255]}]`, 0)
	r.call("action.setTitle", `[{"title":"Seven"}]`, 0)
	if b := button(); b.Badge != "7" || b.BadgeColor != "#ff0000ff" || b.Title != "Seven" {
		t.Fatalf("button: %+v", b)
	}
	if a := r.call("action.getBadgeBackgroundColor", `[{}]`, 0); string(a.Result) != "[255,0,0,255]" {
		t.Fatalf("getBadgeBackgroundColor: %s", a.Result)
	}

	// What is set for one tab shows in that tab only, until it goes to another page.
	r.call("action.setBadgeText", `[{"text":"here","tabId":`+itoa(tab)+`}]`, 0)
	if b := button(); b.Badge != "here" {
		t.Fatalf("button in the tab: %+v", b)
	}
	if a := r.call("action.getBadgeText", `[{}]`, 0); string(a.Result) != `"7"` {
		t.Fatalf("getBadgeText: %s", a.Result)
	}
	r.visit("https://example.org/")
	if b := button(); b.Badge != "7" {
		t.Fatalf("button after leaving the page: %+v", b)
	}

	// A count of blocked requests shows where the tab has no text of its own.
	r.call("action.setBadgeText", `[{"text":null}]`, 0)
	r.show(float64(tab))
	r.x.Settle()
	r.x.FireExtension("blocked", testExtensionID, "3")
	r.until("the count", func() bool { return r.m.Buttons()[0].Badge == "3" })

	r.call("action.disable", `[]`, 0)
	if b := button(); b.Enabled {
		t.Fatalf("a disabled button: %+v", b)
	}
	r.call("action.enable", `[]`, 0)
	r.call("action.setIcon", `[{"icon":"data:image/png;base64,BBBB"}]`, 0)
	if b := button(); !b.Enabled || b.Icon != "data:image/png;base64,BBBB" {
		t.Fatalf("button: %+v", b)
	}
}
