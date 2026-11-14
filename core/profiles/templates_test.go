package profiles

import (
	"os"
	"path/filepath"
	"testing"

	"projectb/prefs"
)

func TestPersonalAndWorkShip(t *testing.T) {
	var names []string
	for _, tm := range Templates() {
		names = append(names, tm.Name)
	}
	if len(names) != 2 || names[0] != "Personal" || names[1] != "Work" {
		t.Fatalf("templates = %v", names)
	}
}

// prefsFrom is the prefs a profile made from a template has.
func prefsFrom(t *testing.T, name string) prefs.Prefs {
	tm, ok := TemplateNamed(name)
	if !ok {
		t.Fatalf("no template %s", name)
	}
	dir := t.TempDir()
	if err := tm.writePrefs(dir); err != nil {
		t.Fatal(err)
	}
	return prefs.New(dir).All()
}

func TestPersonalIsTodaysDefaults(t *testing.T) {
	if got, want := prefsFrom(t, "Personal"), prefs.New(t.TempDir()).All(); got != want {
		t.Fatalf("Personal = %+v, the defaults are %+v", got, want)
	}
}

func TestWorkHasItsOwnColourAndTheTabsOnTheSide(t *testing.T) {
	if p := prefsFrom(t, "Work"); p.TabLayout != prefs.TabsVertical || p.FrameColor != "#5b7fd6" {
		t.Fatalf("Work = %+v", p)
	}
}

func TestTheFirstProfileIsMadeFromPersonal(t *testing.T) {
	root := t.TempDir()
	m := New(root)
	if _, err := m.Choose("", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, First, "prefs.json")); err != nil {
		t.Fatalf("no prefs written: %v", err)
	}
	if p, _, _ := m.Get(First); p.Template != "Personal" || p.Color != "#4f9e9e" {
		t.Fatalf("first profile = %+v", p)
	}
}
