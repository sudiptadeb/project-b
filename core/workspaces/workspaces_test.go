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
