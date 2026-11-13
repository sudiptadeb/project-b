package prefs

import (
	"encoding/json"
	"strings"
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

// A template's values go through the setters, so each listener hears as of a change by
// hand; a setting the values don't hold keeps its value.
func TestApplySetsWhatTheValuesHoldThroughTheSetters(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	m.SetTheme(ThemeDark)
	var heard []string
	m.OnTabLayoutChanged(func(l string) { heard = append(heard, l) })
	m.OnFrameColorChanged(func(c string) { heard = append(heard, c) })
	m.OnThemeChanged(func(th string) { heard = append(heard, th) })
	if err := m.Apply(json.RawMessage(`{"tabLayout":"horizontal","frameColor":"#5b7fd6"}`)); err != nil {
		t.Fatal(err)
	}
	if strings.Join(heard, " ") != "horizontal #5b7fd6" {
		t.Fatalf("heard %v", heard)
	}
	if got := New(dir).All(); got != (Prefs{TabLayout: TabsHorizontal, FrameColor: "#5b7fd6", Theme: ThemeDark}) {
		t.Fatalf("saved %+v", got)
	}
}
