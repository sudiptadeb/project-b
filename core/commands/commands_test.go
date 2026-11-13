package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"projectb/engine"
	"projectb/mockengine"
)

func manager(t *testing.T, dir string, full *bool) (*Manager, map[string]int) {
	t.Helper()
	ran := map[string]int{}
	m := New(mockengine.New(), dir, func() bool { return *full })
	m.platform = Mac
	for _, id := range []string{"tab.new", "tab.close", "tab.select1", "app.quit", "app.settings", "view.fullScreen", "nav.reload", "app.hide"} {
		m.Register(Action{ID: id, Title: id, Run: func() { ran[id]++ }})
	}
	m.Load()
	return m, ran
}

func key(t *testing.T, s string) engine.Key {
	t.Helper()
	k, err := ParseKeys(s)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestKeysRoundTrip(t *testing.T) {
	for _, s := range []string{"Cmd+T", "Ctrl+Shift+Tab", "Option+Cmd+Left", "Ctrl+Cmd+F", "Cmd+,", "Shift+Cmd+]", "F6"} {
		if got := FormatKeys(Mac, key(t, s)); got != s {
			t.Errorf("%s came back as %s", s, got)
		}
	}
	if FormatKeys(Windows, key(t, "Option+Shift+Q")) != "Alt+Shift+Q" {
		t.Error("Option is Alt on Windows")
	}
	if _, err := ParseKeys("Cmd+Nope"); err == nil {
		t.Error("an unknown key was read")
	}
}

// Before the page only reserved keys run; windowed ones give way in full screen; any
// bound key runs once the page didn't use it.
func TestWhoWinsAKey(t *testing.T) {
	full := false
	m, ran := manager(t, t.TempDir(), &full)
	if !m.Key(key(t, "Cmd+T"), engine.KeyBeforePage) || ran["tab.new"] != 1 {
		t.Fatal("Cmd+T is reserved outside full screen")
	}
	full = true
	if m.Key(key(t, "Cmd+T"), engine.KeyBeforePage) {
		t.Fatal("in full screen the page gets Cmd+T first")
	}
	if !m.Key(key(t, "Cmd+T"), engine.KeyUnhandled) || ran["tab.new"] != 2 {
		t.Fatal("an unused Cmd+T still opens a tab")
	}
	if !m.Key(key(t, "Cmd+Q"), engine.KeyBeforePage) || ran["app.quit"] != 1 {
		t.Fatal("the locked Cmd+Q is always the browser's")
	}
	if m.Key(key(t, "Cmd+R"), engine.KeyBeforePage) {
		t.Fatal("Cmd+R is the page's first")
	}
	m.SetRecording(true)
	if m.Key(key(t, "Cmd+Q"), engine.KeyBeforePage) {
		t.Fatal("recording a key runs nothing")
	}
}

// Until the first edit the file only names its template; the edit copies it in.
func TestTheFirstEditOwnsTheKeymap(t *testing.T) {
	dir := t.TempDir()
	full := false
	m, ran := manager(t, dir, &full)
	if _, err := os.Stat(filepath.Join(dir, "keymap.json")); !os.IsNotExist(err) {
		t.Fatal("loading wrote a keymap")
	}
	if err := m.AddBinding("tab.new", "Cmd+Shift+Y", ReserveAlways); err != nil {
		t.Fatal(err)
	}
	var f Keymap
	b, _ := os.ReadFile(filepath.Join(dir, "keymap.json"))
	json.Unmarshal(b, &f)
	if f.Follows != "" || f.Template != DefaultTemplate || len(f.Actions["tab.new"].Mac) != 2 || f.Actions["nav.reload"] == nil {
		t.Fatalf("owned keymap = %s", b)
	}
	m.Key(key(t, "Cmd+Shift+Y"), engine.KeyBeforePage)
	if ran["tab.new"] != 1 {
		t.Fatal("the new key didn't run")
	}
	m.UseTemplate("chrome")
	b, _ = os.ReadFile(filepath.Join(dir, "keymap.json"))
	if strings.TrimSpace(string(b)) != "{\n  \"follows\": \"chrome\"\n}" {
		t.Fatalf("after a reset: %s", b)
	}
}

// An update's new action joins an owned keymap, without a key the user already uses.
func TestAnUpdateMergesNewActions(t *testing.T) {
	dir := t.TempDir()
	owned := keymapTemplates()[DefaultTemplate].clone()
	delete(owned.Actions, "tab.new")
	delete(owned.Actions, "nav.reload")
	owned.Actions["tab.select1"].Mac = []Binding{{Keys: "Cmd+T", Reserve: ReservePage}}
	writeJSON(filepath.Join(dir, "keymap.json"), owned)

	full := false
	m, _ := manager(t, dir, &full)
	if m.keys.Actions["nav.reload"] == nil || len(m.keys.Actions["nav.reload"].Mac) != 1 {
		t.Fatal("nav.reload wasn't merged with its key")
	}
	if a := m.keys.Actions["tab.new"]; a == nil || len(a.Mac) != 0 {
		t.Fatalf("tab.new should be merged without the taken Cmd+T: %+v", a)
	}
	if len(m.missing) != 1 || m.missing[0].ID != "tab.new" || m.missing[0].Taken[0] != "Cmd+T" {
		t.Fatalf("missing = %+v", m.missing)
	}
}

func TestABadFileIsSetAside(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keymap.json"), []byte("{nope"), 0o644)
	full := false
	m, _ := manager(t, dir, &full)
	if !m.Key(key(t, "Cmd+T"), engine.KeyBeforePage) {
		t.Fatal("a bad file should leave the template in charge")
	}
	if _, err := os.Stat(filepath.Join(dir, "keymap.json.bad")); err != nil {
		t.Fatal("the bad file wasn't kept")
	}
	if len(m.View().Problems) != 1 {
		t.Fatal("the problem isn't shown")
	}
}

// macOS App Shortcuts arrive by title and become bindings marked fromOS.
func TestOSBindingsJoinTheKeymap(t *testing.T) {
	full := false
	m, ran := manager(t, t.TempDir(), &full)
	m.OSBindings(map[string]engine.Key{"->File->tab.new": key(t, "Option+Cmd+N")})
	bs := m.keys.Actions["tab.new"].Mac
	if len(bs) != 2 || !bs[1].FromOS || bs[1].Keys != "Option+Cmd+N" || bs[1].Reserve != ReserveWindowed {
		t.Fatalf("bindings = %+v", bs)
	}
	m.Key(key(t, "Option+Cmd+N"), engine.KeyBeforePage)
	if ran["tab.new"] != 1 {
		t.Fatal("the OS key didn't run")
	}
	m.RemoveBinding("tab.new", 1)
	if len(m.keys.Actions["tab.new"].Mac) != 2 {
		t.Fatal("an OS key was removed here")
	}
	m.OSBindings(nil)
	if len(m.keys.Actions["tab.new"].Mac) != 1 {
		t.Fatal("an OS key the OS dropped stayed")
	}
}

// A list's items are read each time its menu opens, and choosing one hands its key back.
func TestAMenuListIsReadWhenItsMenuOpens(t *testing.T) {
	e := mockengine.New()
	m := New(e, t.TempDir(), func() bool { return false })
	names := []string{"Personal"}
	var chosen []string
	m.RegisterList(List{
		ID: "profiles",
		Items: func() []ListItem {
			var items []ListItem
			for i, n := range names {
				items = append(items, ListItem{Key: strings.ToLower(n), Title: n, Checked: i == 0})
			}
			return items
		},
		Choose: func(key string) { chosen = append(chosen, key) },
	})
	m.Load()
	var profiles *engine.MenuItem
	for i, top := range e.Menu() {
		if top.Title == "Profiles" {
			profiles = &e.Menu()[i]
		}
	}
	if profiles == nil || len(profiles.Items) == 0 || profiles.Items[0].List != "profiles" {
		t.Fatalf("the Profiles menu is %+v", profiles)
	}
	names = append(names, "Work")
	items := m.ListItems("profiles")
	if len(items) != 2 || items[1].Title != "Work" || !items[0].Checked || items[1].Checked {
		t.Fatalf("items = %+v", items)
	}
	m.Run(items[1].Action)
	if len(chosen) != 1 || chosen[0] != "work" {
		t.Fatalf("chose %v", chosen)
	}
}
