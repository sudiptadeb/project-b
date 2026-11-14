package workspaces

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// A new profile has one workspace, Main, the profile's own storage, which is first.
func TestANewProfileHasMain(t *testing.T) {
	m := New(t.TempDir())
	if l := m.List(); !slices.Equal(l, []Workspace{{ID: "", Name: "Main", Color: "#3470d0"}}) || m.First() != "" {
		t.Fatalf("list %+v, first %q", l, m.First())
	}
}

// A list from before Main gets it at the top, once: the file says so from the next change
// on, and Main moved down stays where it was put.
func TestAnOldListGetsMainOnce(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "workspaces.json"), []byte(`{"workspaces":[{"id":"0a1b2c3d","name":"Work","color":"#5b7fd6"}]}`), 0o600)
	main, work := Workspace{ID: "", Name: "Main", Color: "#3470d0"}, Workspace{ID: "0a1b2c3d", Name: "Work", Color: "#5b7fd6"}
	m := New(dir)
	if l := m.List(); !slices.Equal(l, []Workspace{main, work}) || m.First() != "" {
		t.Fatalf("list %+v", l)
	}
	if got := New(dir).List(); !slices.Equal(got, []Workspace{main, work}) {
		t.Fatalf("read again before any change: %+v", got)
	}
	if err := m.Move("", 1); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "workspaces.json"))
	var f struct {
		Version int `json:"version"`
	}
	json.Unmarshal(b, &f)
	again := New(dir)
	if got := again.List(); f.Version != 2 || !slices.Equal(got, []Workspace{work, main}) || again.First() != work.ID {
		t.Fatalf("after Main moved down the file is %s and reads back as %+v", b, got)
	}
}

// A workspace made gets an id Go makes, keeps it through a rename, and the list is the
// file beside the prefs, read again by the next start.
func TestTheListIsKeptInTheProfile(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	changes := 0
	m.OnChanged(func() { changes++ })
	work, err := m.Create(" Work ", "#5b7fd6")
	if err != nil {
		t.Fatal(err)
	}
	shop, _ := m.Create("Shopping", "#c2410c")
	if !regexp.MustCompile(`^[0-9a-f]{8}$`).MatchString(work.ID) || work.ID == shop.ID || work.Name != "Work" {
		t.Fatalf("made %+v and %+v", work, shop)
	}
	if err := m.SetName(work.ID, "Office"); err != nil {
		t.Fatal(err)
	}
	want := []Workspace{{ID: "", Name: "Main", Color: "#3470d0"}, {ID: work.ID, Name: "Office", Color: "#5b7fd6"}, {ID: shop.ID, Name: "Shopping", Color: "#c2410c"}}
	if got := New(dir).List(); !slices.Equal(got, want) || changes != 3 {
		t.Fatalf("read back %+v after %d changes", got, changes)
	}
	var f struct {
		Version    int                 `json:"version"`
		Workspaces []map[string]string `json:"workspaces"`
	}
	b, _ := os.ReadFile(filepath.Join(dir, "workspaces.json"))
	if err := json.Unmarshal(b, &f); err != nil || f.Version != 2 || len(f.Workspaces) != 3 || f.Workspaces[1]["name"] != "Office" || f.Workspaces[1]["color"] != "#5b7fd6" {
		t.Fatalf("the file holds %s", b)
	}
}

func TestAWorkspaceNeedsANameAndAColour(t *testing.T) {
	m := New(t.TempDir())
	if _, err := m.Create("  ", "#5b7fd6"); err == nil {
		t.Fatal("made one with no name")
	}
	if _, err := m.Create("Work", "blue"); err == nil {
		t.Fatal("made one with no colour")
	}
	w, _ := m.Create("Work", "#5b7fd6")
	if m.SetName(w.ID, "") == nil || m.SetName("nope", "Other") == nil || len(m.List()) != 2 || m.List()[1].Name != "Work" {
		t.Fatalf("list %+v", m.List())
	}
}

// Names are unique whatever their case; a rename may change only the case of its own.
func TestANameIsUniqueInTheList(t *testing.T) {
	m := New(t.TempDir())
	work, _ := m.Create("Work", "#5b7fd6")
	shop, _ := m.Create("Shopping", "#c2410c")
	if _, err := m.Create(" work ", "#2f9e44"); err == nil {
		t.Fatal("made a second Work")
	}
	if m.SetName(shop.ID, "WORK") == nil {
		t.Fatal("renamed Shopping to another's name")
	}
	if err := m.SetName(work.ID, "WORK"); err != nil {
		t.Fatalf("renaming to its own name in another case: %v", err)
	}
	if _, err := m.Create("main", "#2f9e44"); err == nil {
		t.Fatal("made a second Main")
	}
	if l := m.List(); len(l) != 3 || l[1].Name != "WORK" || l[2].Name != "Shopping" {
		t.Fatalf("list %+v", l)
	}
}

// A colour is changed in place and kept; one that isn't #rrggbb, or for no workspace, isn't.
func TestAWorkspaceCanBeRecoloured(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	changes := 0
	m.OnChanged(func() { changes++ })
	w, _ := m.Create("Work", "#5b7fd6")
	if err := m.SetColor(w.ID, "#c2410c"); err != nil {
		t.Fatal(err)
	}
	if m.SetColor(w.ID, "red") == nil || m.SetColor("nope", "#2f9e44") == nil {
		t.Fatal("took a bad colour or an unknown id")
	}
	// The same colour again is no change.
	m.SetColor(w.ID, "#c2410c")
	if l := New(dir).List(); len(l) != 2 || l[1].Color != "#c2410c" || l[1].ID != w.ID || changes != 2 {
		t.Fatalf("read back %+v after %d changes", l, changes)
	}
}

// Move swaps a workspace with the one above or below it, and the order is kept; it moves
// one place only, and not past either end.
func TestAWorkspaceMovesUpAndDown(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	a, _ := m.Create("A", "#5b7fd6")
	b, _ := m.Create("B", "#c2410c")
	c, _ := m.Create("C", "#2f9e44")
	names := func(l []Workspace) string {
		s := ""
		for _, w := range l {
			s += w.Name
		}
		return s
	}
	if err := m.Move(c.ID, -1); err != nil {
		t.Fatal(err)
	}
	if err := m.Move(a.ID, 1); err != nil {
		t.Fatal(err)
	}
	if got := names(New(dir).List()); got != "MainCAB" {
		t.Fatalf("order %s", got)
	}
	if m.Move("", -1) == nil || m.Move(b.ID, 1) == nil || m.Move(a.ID, 2) == nil || m.Move("nope", 1) == nil {
		t.Fatal("moved past an end, more than one place, or an unknown id")
	}
	if got := names(m.List()); got != "MainCAB" {
		t.Fatalf("order %s", got)
	}
	// Main moves as any workspace does, and the first is whichever is at the top.
	m.Move(c.ID, -1)
	if got := names(New(dir).List()); got != "CMainAB" || m.First() != c.ID {
		t.Fatalf("order %s, first %q", got, m.First())
	}
}

// A workspace leaves the list and stays out of it; Main is deleted as any other, and a list
// that had it doesn't get it back. The last one can't be deleted, nor one the list doesn't
// hold.
func TestAWorkspaceIsDeletedButNotTheLast(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "workspaces.json"), []byte(`{"workspaces":[{"id":"0a1b2c3d","name":"Work","color":"#5b7fd6"}]}`), 0o600)
	m := New(dir)
	changes := 0
	m.OnChanged(func() { changes++ })
	if err := m.Delete(""); err != nil {
		t.Fatal(err)
	}
	if m.Delete("") == nil || m.Delete("0a1b2c3d") == nil {
		t.Fatal("deleted one the list doesn't hold, or the last")
	}
	again := New(dir)
	if l := again.List(); len(l) != 1 || l[0].ID != "0a1b2c3d" || again.First() != "0a1b2c3d" || changes != 1 {
		t.Fatalf("read back %+v, first %q, after %d changes", l, again.First(), changes)
	}
}
