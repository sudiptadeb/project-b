package flags

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Every row of registry.json holds together, and ids and switches are each used once.
func TestTheRegistry(t *testing.T) {
	ids, switches := map[string]bool{}, map[string]bool{}
	for _, f := range All {
		if ids[f.ID] || switches[f.Switch] {
			t.Errorf("%s: id or switch used twice", f.ID)
		}
		ids[f.ID], switches[f.Switch] = true, true
	}
	for _, bad := range []string{
		`[{"id":"a","name":"A","description":"d","switch":"a","type":"bool"}]`,
		`[{"id":"a","name":"A","description":"d","switch":"a","type":"enum"}]`,
		`[{"id":"a","name":"A","description":"d","switch":"a","type":"string","choices":["x"]}]`,
		`[{"id":"a","name":"A","description":"d","switch":"a","type":"string","min":1}]`,
		`[{"id":"a","name":"A","description":"d","switch":"a","type":"number","pattern":"x"}]`,
		`[{"id":"a","name":"A","description":"d","switch":"a","type":"string","pattern":"("}]`,
		`[{"id":"a","name":"A","switch":"a","type":"boolean"}]`,
		`[{"id":"a","name":"A","description":"d","switch":"a","type":"boolean","defualt":"on"}]`,
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

// Each type takes what it should, refuses the rest, and becomes its switch.
func TestTheTypes(t *testing.T) {
	rows, err := Parse([]byte(`[
		{"id":"b","name":"B","description":"d","switch":"b","type":"boolean"},
		{"id":"n","name":"N","description":"d","switch":"n","type":"number","min":1,"max":10},
		{"id":"e","name":"E","description":"d","switch":"e","type":"enum","choices":["low","high"]},
		{"id":"s","name":"S","description":"d","switch":"s","type":"string","pattern":"^[a-z]+$"},
		{"id":"ns","name":"NS","description":"d","switch":"ns","type":"numbers","min":0},
		{"id":"ss","name":"SS","description":"d","switch":"ss","type":"strings"}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct{ good, bad []string }{
		"b":  {[]string{"on"}, []string{"off", "true"}},
		"n":  {[]string{"1", "10"}, []string{"0", "11", "x", "1.5"}},
		"e":  {[]string{"low", "high"}, []string{"mid", "Low"}},
		"s":  {[]string{"abc"}, []string{"ABC", "a b"}},
		"ns": {[]string{"0", "1, 2,3"}, []string{"1,,2", "-1", "1,x"}},
		"ss": {[]string{"A", "A, B"}, []string{"A,", ",B"}},
	}
	for _, f := range rows {
		for _, v := range cases[f.ID].good {
			if _, ok := f.normal(v); !ok {
				t.Errorf("%s refused %q", f.ID, v)
			}
		}
		for _, v := range cases[f.ID].bad {
			if _, ok := f.normal(v); ok {
				t.Errorf("%s took %q", f.ID, v)
			}
		}
	}
	if v, _ := rows[4].normal(" 1, 2 ,3 "); v != "1,2,3" {
		t.Errorf("a list is saved as %q", v)
	}
	if rows[0].toSwitch("on") != "--b" || rows[1].toSwitch("5") != "--n=5" {
		t.Error("switches")
	}
}

func pending(m *Manager, id string) bool {
	for _, f := range m.View() {
		if f.ID == id {
			return f.Pending
		}
	}
	return false
}

func TestAChangeWaitsForTheNextLaunch(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if len(m.Switches()) != 0 || pending(m, RemoteDebugging) {
		t.Fatalf("a new profile has flags: %v", m.Switches())
	}
	changes := 0
	m.OnChanged(func() { changes++ })
	m.Set(RemoteDebugging, "9333")
	m.Set(RemoteDebugging, "9333")
	if changes != 1 || !pending(m, RemoteDebugging) || pending(m, Features) || m.IsOn(RemoteDebugging) || len(m.Switches()) != 0 {
		t.Fatalf("changes %d, view %+v, switches %v", changes, m.View(), m.Switches())
	}
	m.Set(RemoteDebugging, "")
	if pending(m, RemoteDebugging) {
		t.Fatal("relaunch offered with nothing changed")
	}

	m.Set(RemoteDebugging, "9333")
	m.Set(Features, "Foo, Bar:x/y")
	m.Set(Features, "Foo, Bar")
	next := New(dir)
	want := []string{"--remote-debugging-port=9333", "--enable-features=Foo,Bar"}
	if !slices.Equal(next.Switches(), want) || !next.IsOn(RemoteDebugging) || pending(next, RemoteDebugging) {
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

func TestBuildFlagsNeedTheBuild(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	m.Set(Extensions, "on")
	var view FlagView
	for _, f := range m.View() {
		if f.ID == Extensions {
			view = f
		}
	}
	if view.Built != builds[Extensions] || (view.Value == "on") != builds[Extensions] {
		t.Fatalf("view %+v", view)
	}
	if builds[Extensions] && !slices.Contains(New(dir).Switches(), "--pb-extensions") {
		t.Fatal("no --pb-extensions")
	}
	// Only a build flag has a tagged file saying it is built.
	for id := range builds {
		if f, ok := byID(id); !ok || !f.Build {
			t.Errorf("%s is built but isn't a build flag", id)
		}
	}
}
