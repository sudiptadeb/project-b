package app_test

import (
	"encoding/json"
	"testing"

	"projectb/app"
	"projectb/engine"
	"projectb/mockengine"
	"projectb/profiles"
	"projectb/timeline"
)

var _ engine.Listener = (*app.Picker)(nil)

// picker starts a picker copy on the root of list, and returns what its page hears.
func picker(t *testing.T, list *profiles.Manager, open func(id string)) (*mockengine.Engine, *app.Picker, <-chan []byte) {
	t.Helper()
	e := mockengine.New()
	got, stop := e.Feeds(1).Shell.Subscribe()
	t.Cleanup(stop)
	var p *app.Picker
	e.Post(func() {
		p = app.StartPicker(e, list, open)
		e.SetListener(p)
	})
	go e.Run()
	e.Settle()
	return e, p, got
}

func state(t *testing.T, got <-chan []byte) map[string]any {
	t.Helper()
	for b := range got {
		var m map[string]any
		json.Unmarshal(b, &m)
		if m["t"] == "state" {
			return m
		}
	}
	return nil
}

func TestThePickerShowsTheProfilesAndTheTemplates(t *testing.T) {
	list := profiles.New(t.TempDir())
	list.Choose("", false)
	e, p, got := picker(t, list, nil)
	if e.PageOf(1) != engine.PagePicker {
		t.Fatalf("window 1 holds %q", e.PageOf(1))
	}
	e.Post(func() { p.Message(1, []byte(`{"t":"ready"}`)) })
	s := state(t, got)
	b, _ := json.Marshal([]any{s["profiles"], s["templates"]})
	if string(b) != `[[{"color":"#4f9e9e","id":"default","name":"Personal"}],["Personal","Work"]]` {
		t.Fatalf("the picker shows %s", b)
	}
	if e.DockColor() != "" {
		t.Fatal("a picker copy put a dot on the Dock icon")
	}
}

func TestChoosingAProfileOpensItInACopyOfItsOwnAndQuits(t *testing.T) {
	list := profiles.New(t.TempDir())
	list.Choose("", false)
	var opened []string
	e, p, _ := picker(t, list, func(id string) { opened = append(opened, id) })
	e.Post(func() { p.Message(1, []byte(`{"t":"open","id":"default"}`)) })
	e.Settle()
	if len(opened) != 1 || opened[0] != "default" || !e.Quit() {
		t.Fatalf("opened %v, quit %v", opened, e.Quit())
	}
}

func TestNewProfileMakesOneFromATemplateAndOpensIt(t *testing.T) {
	list := profiles.New(t.TempDir())
	list.Choose("", false)
	var opened []string
	e, p, _ := picker(t, list, func(id string) { opened = append(opened, id) })
	e.Post(func() { p.Message(1, []byte(`{"t":"create","name":"Office","template":"Work"}`)) })
	e.Settle()
	made, _ := list.List()
	if len(made) != 2 || made[1].Name != "Office" || made[1].Color != "#5b7fd6" {
		t.Fatalf("New Profile made %+v", made)
	}
	if len(opened) != 1 || opened[0] != made[1].ID || !e.Quit() {
		t.Fatalf("opened %v, quit %v", opened, e.Quit())
	}
}

func TestAProfileThatIsntListedOpensNothing(t *testing.T) {
	list := profiles.New(t.TempDir())
	list.Choose("", false)
	called := false
	e, p, _ := picker(t, list, func(string) { called = true })
	e.Post(func() { p.Message(1, []byte(`{"t":"open","id":"nobody"}`)) })
	e.Settle()
	if called || e.Quit() {
		t.Fatalf("opened %v, quit %v", called, e.Quit())
	}
}

func TestClosingThePickerWithoutAChoiceQuits(t *testing.T) {
	list := profiles.New(t.TempDir())
	e, _, _ := picker(t, list, nil)
	e.Post(func() { e.RequestWindow(1, engine.WindowClose) })
	e.Settle()
	if !e.Quit() {
		t.Fatal("the picker closed and the copy went on")
	}
}

// A picker copy is ready once its page shows the profiles: app.ready, the line tests wait
// for, is marked then and only once.
func TestAPickerCopyIsReadyOnceItsPageShowsTheProfiles(t *testing.T) {
	_, from := timeline.Since(^uint64(0) >> 1)
	readies := func() int {
		entries, _ := timeline.Since(from)
		n := 0
		for _, e := range entries {
			if e.Kind == timeline.KindMark && e.Name == "app.ready" {
				n++
			}
		}
		return n
	}
	list := profiles.New(t.TempDir())
	list.Choose("", false)
	e, p, _ := picker(t, list, nil)
	e.Post(func() { p.Message(1, []byte(`{"t":"ready"}`)) })
	e.Settle()
	if n := readies(); n != 0 {
		t.Fatalf("app.ready marked %d times before the page showed anything", n)
	}
	e.Post(func() { p.Message(1, []byte(`{"t":"shown"}`)) })
	e.Post(func() { p.Message(1, []byte(`{"t":"shown"}`)) })
	e.Settle()
	if n := readies(); n != 1 {
		t.Fatalf("app.ready marked %d times once the page showed the profiles", n)
	}
}
