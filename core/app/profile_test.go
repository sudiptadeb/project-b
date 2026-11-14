package app_test

import (
	"path/filepath"
	"testing"

	"projectb/app"
	"projectb/engine"
	"projectb/prefs"
	"projectb/profiles"
	"projectb/update"
)

// startListed starts the app on the first profile of a list of its own, as a first start
// does.
func startListed(t *testing.T) (*rig, *profiles.Manager, profiles.Choice) {
	t.Helper()
	list := profiles.New(t.TempDir())
	c, err := list.Choose("", false)
	if err != nil {
		t.Fatal(err)
	}
	cfg := app.Config{ProfileDir: c.Dir, Profiles: list, Profile: c.ID, Update: update.Config{StateDir: filepath.Join(list.Root(), "update")}}
	return startWith(t, cfg), list, c
}

// The Profile section shows the profile's name and its template, and a new name is the
// list's, which the Profiles menu reads when it next opens.
func TestTheProfileSectionRenamesTheProfile(t *testing.T) {
	r, list, c := startListed(t)
	defer r.stop()
	got, snap := r.openSettings()
	p, _ := snap["profile"].(map[string]any)
	if p["name"] != "Personal" || p["template"] != "Personal" || len(p["templates"].([]any)) != 2 {
		t.Fatalf("the snapshot's profile is %v", snap["profile"])
	}
	r.settings(`{"t":"setProfileName","text":"Home"}`)
	r.waitOn(got, func(m map[string]any) bool {
		p, _ := m["profile"].(map[string]any)
		return m["t"] == "profile" && p["name"] == "Home"
	})
	if listed, _, _ := list.Get(c.ID); listed.Name != "Home" {
		t.Fatalf("listed %+v", listed)
	}
	var items engine.Menu
	r.e.Post(func() { items = r.a.MenuList("profiles") })
	r.e.Settle()
	if len(items) != 1 || items[0].Title != "Home" {
		t.Fatalf("the Profiles menu holds %+v", items)
	}
}

// Reset to a template writes its values over the profile's prefs and nothing else: the
// shell, the window, the Dock and the list follow at once, and history stays.
func TestResetToATemplateTakesItsValuesAndKeepsTheRest(t *testing.T) {
	r, list, c := startListed(t)
	defer r.stop()
	r.visit("https://example.com/docs")
	r.shell(`{"t":"data","data":{"panes":[1]}}`)
	got, _ := r.openSettings()

	r.settings(`{"t":"resetProfile","text":"Work"}`)
	r.waitOn(got, func(m map[string]any) bool {
		p, _ := m["profile"].(map[string]any)
		return m["t"] == "profile" && p["template"] == "Work"
	})
	// The shell hears each as Apply sets it: the tabs, then the colour.
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabLayout" && m["tabLayout"] == "horizontal" })
	r.waitFor(func(m map[string]any) bool { return m["t"] == "windowColor" && m["windowColor"] == "#5b7fd6" })
	if r.e.Layout(1).FrameColor != "#5b7fd6" || r.e.DockColor() != "#5b7fd6" {
		t.Fatalf("the window is %s and the Dock %s", r.e.Layout(1).FrameColor, r.e.DockColor())
	}
	if listed, _, _ := list.Get(c.ID); listed.Color != "#5b7fd6" || listed.Template != "Work" || listed.Name != "Personal" {
		t.Fatalf("listed %+v", listed)
	}
	if got := prefs.New(c.Dir).All(); got != (prefs.Prefs{TabLayout: prefs.TabsHorizontal, FrameColor: "#5b7fd6", Theme: prefs.ThemeSystem, Shell: "glide", AddressBarPosition: prefs.AddressBottom}) {
		t.Fatalf("the prefs are %+v", got)
	}
	if items := r.suggestions("exam"); len(items) != 1 {
		t.Fatalf("history after the reset: %v", items)
	}
	// The shell's own data goes: reloaded, it starts from Go's data alone.
	r.shell(`{"t":"ready"}`)
	if m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" }); m["data"] != nil {
		t.Fatalf("the shell's data after the reset: %v", m["data"])
	}
}

// A copy on a folder named outright has no entry in the list, so no Profile section.
func TestAFolderNamedOutrightHasNoProfileSection(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	if _, snap := r.openSettings(); snap["profile"] != nil {
		t.Fatalf("the snapshot has a profile: %v", snap["profile"])
	}
}
