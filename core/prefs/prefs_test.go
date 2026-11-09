package prefs

import "testing"

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
