package shells

import (
	"os"
	"slices"
	"testing"
)

func TestTheListedShellsAreAllButTheExample(t *testing.T) {
	var listed []string
	for _, s := range Listed() {
		listed = append(listed, s.ID)
	}
	if len(listed) == 0 || listed[0] != "standard" || !Has("example") || Has("Example") {
		t.Fatalf("listed %v", listed)
	}
	for _, s := range Listed() {
		if s.Example {
			t.Fatalf("an example is listed: %+v", s)
		}
	}
}

// Every folder of shell/src/apps/shell is a shell the registry has, and every shell the
// registry has is a folder there.
func TestTheRegistryAndTheFoldersNameTheSameShells(t *testing.T) {
	entries, err := os.ReadDir("../../shell/src/apps/shell")
	if err != nil {
		t.Fatal(err)
	}
	var folders, registered []string
	for _, e := range entries {
		if e.IsDir() {
			folders = append(folders, e.Name())
		}
	}
	for _, s := range All() {
		registered = append(registered, s.ID)
	}
	slices.Sort(registered)
	if !slices.Equal(folders, registered) {
		t.Fatalf("the folders are %v, the registry has %v", folders, registered)
	}
}
