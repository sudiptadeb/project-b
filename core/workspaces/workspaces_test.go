package workspaces

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

func TestANewProfileHasNoWorkspaces(t *testing.T) {
	if l := New(t.TempDir()).List(); len(l) != 0 {
		t.Fatalf("list %+v", l)
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
	want := []Workspace{{ID: work.ID, Name: "Office", Color: "#5b7fd6"}, {ID: shop.ID, Name: "Shopping", Color: "#c2410c"}}
	if got := New(dir).List(); !slices.Equal(got, want) || changes != 3 {
		t.Fatalf("read back %+v after %d changes", got, changes)
	}
	var f struct {
		Workspaces []map[string]string `json:"workspaces"`
	}
	b, _ := os.ReadFile(filepath.Join(dir, "workspaces.json"))
	if err := json.Unmarshal(b, &f); err != nil || len(f.Workspaces) != 2 || f.Workspaces[0]["name"] != "Office" || f.Workspaces[0]["color"] != "#5b7fd6" {
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
	if m.SetName(w.ID, "") == nil || m.SetName("nope", "Other") == nil || len(m.List()) != 1 || m.List()[0].Name != "Work" {
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
	if l := m.List(); len(l) != 2 || l[0].Name != "WORK" || l[1].Name != "Shopping" {
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
	if l := New(dir).List(); len(l) != 1 || l[0].Color != "#c2410c" || l[0].ID != w.ID || changes != 2 {
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
	if got := names(New(dir).List()); got != "CAB" {
		t.Fatalf("order %s", got)
	}
	if m.Move(c.ID, -1) == nil || m.Move(b.ID, 1) == nil || m.Move(a.ID, 2) == nil || m.Move("nope", 1) == nil {
		t.Fatal("moved past an end, more than one place, or an unknown id")
	}
	if got := names(m.List()); got != "CAB" {
		t.Fatalf("order %s", got)
	}
}
