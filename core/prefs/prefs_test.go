package prefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTabLayoutSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if m.TabLayout() != TabsVertical {
		t.Fatalf("default = %q", m.TabLayout())
	}
	var got []string
	m.OnTabLayoutChanged(func(l string) { got = append(got, l) })
	m.SetTabLayout("sideways")
	m.SetTabLayout(TabsHorizontal)
	m.SetTabLayout(TabsHorizontal)
	if len(got) != 1 || got[0] != TabsHorizontal {
		t.Fatalf("changes = %v", got)
	}

	if New(dir).TabLayout() != TabsHorizontal {
		t.Fatal("layout not saved")
	}
}

func TestAutoUpdateIsOnByDefaultAndSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	// A file from before the setting existed keeps the default.
	if err := os.WriteFile(filepath.Join(dir, "prefs.json"), []byte(`{"tabLayout":"horizontal"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(dir)
	if !m.AutoUpdate() || m.TabLayout() != TabsHorizontal {
		t.Fatalf("prefs = %+v", m.All())
	}
	var got []bool
	m.OnAutoUpdateChanged(func(on bool) { got = append(got, on) })
	m.SetAutoUpdate(true)
	m.SetAutoUpdate(false)
	if len(got) != 1 || got[0] {
		t.Fatalf("changes = %v", got)
	}
	if again := New(dir); again.AutoUpdate() || again.TabLayout() != TabsHorizontal {
		t.Fatalf("after restart = %+v", again.All())
	}
}

func TestFrameColorIsDustyTealByDefaultAndSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if m.FrameColor() != "#4f9e9e" {
		t.Fatalf("default = %q", m.FrameColor())
	}
	var got []string
	m.OnFrameColorChanged(func(c string) { got = append(got, c) })
	m.SetFrameColor("teal")
	m.SetFrameColor("#3470d0")
	m.SetFrameColor("#3470d0")
	if len(got) != 1 || got[0] != "#3470d0" {
		t.Fatalf("changes = %v", got)
	}
	if New(dir).FrameColor() != "#3470d0" {
		t.Fatal("colour not saved")
	}
}
