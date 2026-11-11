package flags

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestAChangeWaitsForTheNextLaunch(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if len(m.Switches()) != 0 || m.View().Relaunch {
		t.Fatalf("a new profile has flags: %v", m.Switches())
	}
	changes := 0
	m.OnChanged(func() { changes++ })
	m.Set(RemoteDebugging, "9333")
	m.Set(RemoteDebugging, "9333")
	if changes != 1 || !m.View().Relaunch || m.On(RemoteDebugging) || len(m.Switches()) != 0 {
		t.Fatalf("changes %d, view %+v, switches %v", changes, m.View(), m.Switches())
	}
	// Set back, nothing waits.
	m.Set(RemoteDebugging, "")
	if m.View().Relaunch {
		t.Fatal("relaunch offered with nothing changed")
	}

	m.Set(RemoteDebugging, "9333")
	m.Set(Features, "Foo,Bar:x/y")
	m.Set(Features, "Foo,Bar")
	next := New(dir)
	want := []string{"--remote-debugging-port=9333", "--enable-features=Foo,Bar"}
	if !slices.Equal(next.Switches(), want) || !next.On(RemoteDebugging) || next.View().Relaunch {
		t.Fatalf("next launch: %v", next.Switches())
	}
}

func TestValuesThatCantBeUsedAreDropped(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	for _, v := range []string{"80", "port", "70000"} {
		m.Set(RemoteDebugging, v)
	}
	m.Set("nope", "on")
	if len(m.saved) != 0 {
		t.Fatalf("saved %v", m.saved)
	}
	os.WriteFile(filepath.Join(dir, "flags.json"), []byte(`{"remoteDebugging":"22","features":"A","nope":"on"}`), 0o600)
	if got := New(dir).Switches(); !slices.Equal(got, []string{"--enable-features=A"}) {
		t.Fatalf("switches %v", got)
	}
}

func TestExtensionsNeedTheBuild(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	m.Set(Extensions, "on")
	var built bool
	for _, f := range m.View().Flags {
		if f.ID == Extensions {
			built = f.Built
		}
	}
	if built != builds["extensions"] || (m.saved[Extensions] == "on") != builds["extensions"] {
		t.Fatalf("built %v, saved %v", built, m.saved)
	}
	if builds["extensions"] && !slices.Contains(New(dir).Switches(), "--pb-extensions") {
		t.Fatal("no --pb-extensions")
	}
}
