package profiles

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestFirstStartMakesTheListHoldingPersonal(t *testing.T) {
	root := t.TempDir()
	m := New(root)
	c, err := m.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != First || c.Dir != filepath.Join(root, First) {
		t.Fatalf("first start chose %+v", c)
	}
	list, err := m.List()
	if err != nil || len(list) != 1 || list[0].ID != First || list[0].Name != "Personal" || list[0].Color != "#4f9e9e" {
		t.Fatalf("list = %+v, %v", list, err)
	}
}

func TestAProfileFromBeforeTheListIsKeptAsItIs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, First)
	os.MkdirAll(dir, 0o755)
	prefs := []byte(`{"tabLayout":"horizontal","frameColor":"#123456"}`)
	if err := os.WriteFile(filepath.Join(dir, "prefs.json"), prefs, 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(root)
	if _, err := m.Choose(""); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "prefs.json")); string(b) != string(prefs) {
		t.Fatalf("prefs.json became %s", b)
	}
	if p, ok, _ := m.Get(First); !ok || p.Color != "#123456" {
		t.Fatalf("the list's colour is %+v, not the profile's", p)
	}
}

func TestTheStartChoosesTheNamedProfileElseTheOneUsedLast(t *testing.T) {
	root := t.TempDir()
	m := New(root)
	m.Choose("")
	m.change(func(list []Profile) []Profile {
		return append(list, Profile{ID: "work", Name: "Work", Color: "#5b7fd6"})
	})
	if c, err := m.Choose("work"); err != nil || c.ID != "work" || c.Dir != filepath.Join(root, "work") {
		t.Fatalf("named work: %+v, %v", c, err)
	}
	if _, err := m.Choose("nobody"); err == nil {
		t.Fatal("a profile that isn't listed was chosen")
	}
	m.MarkUsed(First)
	m.MarkUsed("work")
	if c, _ := m.Choose(""); c.ID != "work" {
		t.Fatalf("used last is work, chose %+v", c)
	}
	m.MarkUsed(First)
	if c, _ := m.Choose(""); c.ID != First {
		t.Fatalf("used last is default, chose %+v", c)
	}
}

func TestAFolderNamedOutrightIsNotListed(t *testing.T) {
	root := t.TempDir()
	if c := Named("/somewhere/else"); c.Dir != "/somewhere/else" || c.ID != "" {
		t.Fatalf("named folder = %+v", c)
	}
	if _, err := os.Stat(filepath.Join(root, "profiles.json")); !os.IsNotExist(err) {
		t.Fatal("naming a folder touched the list")
	}
}

func TestChangesFromManyCopiesAreAllKept(t *testing.T) {
	root := t.TempDir()
	m := New(root)
	m.Choose("")
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each is a copy of the app of its own, with its own manager.
			New(root).change(func(list []Profile) []Profile {
				return append(list, Profile{ID: string(rune('a' + i))})
			})
		}()
	}
	wg.Wait()
	if list, _ := m.List(); len(list) != 21 {
		t.Fatalf("%d profiles, want 21", len(list))
	}
}

func TestTheListKeepsTheProfilesColour(t *testing.T) {
	m := New(t.TempDir())
	m.Choose("")
	m.SetColor(First, "#3470d0")
	if p, _, _ := New(m.Root()).Get(First); p.Color != "#3470d0" {
		t.Fatalf("colour = %q", p.Color)
	}
}
