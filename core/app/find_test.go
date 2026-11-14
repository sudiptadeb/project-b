package app_test

import (
	"slices"
	"testing"

	"projectb/engine"
)

// findState is the find message's find, or nil for none.
func (r *rig) find(match func(f map[string]any) bool) map[string]any {
	r.t.Helper()
	m := r.waitFor(func(m map[string]any) bool {
		if m["t"] != "find" {
			return false
		}
		f, _ := m["find"].(map[string]any)
		return match(f)
	})
	f, _ := m["find"].(map[string]any)
	return f
}

func TestFindInPage(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	r.e.Fire(1, 0, "menu", "find.open")
	r.find(func(f map[string]any) bool { return f != nil && f["text"] == "" })
	r.waitFor(func(m map[string]any) bool { return m["t"] == "focusFind" })

	r.shell(`{"t":"findText","tab":1,"text":"mock"}`)
	f := r.find(func(f map[string]any) bool { return f != nil && f["matches"] == float64(3) })
	if f["active"] != float64(1) || f["text"] != "mock" {
		t.Fatalf("find = %v", f)
	}
	r.shell(`{"t":"findNext","tab":1,"forward":true}`)
	r.find(func(f map[string]any) bool { return f != nil && f["active"] == float64(2) })
	r.shell(`{"t":"findNext","tab":1,"forward":false}`)
	r.shell(`{"t":"findNext","tab":1,"forward":false}`)
	r.find(func(f map[string]any) bool { return f != nil && f["active"] == float64(3) })

	r.shell(`{"t":"findText","tab":1,"text":"nothing here"}`)
	r.find(func(f map[string]any) bool {
		return f != nil && f["matches"] == float64(0) && f["active"] == float64(0)
	})

	// A find for a tab whose bar isn't open goes nowhere.
	r.shell(`{"t":"findText","tab":7,"text":"other"}`)
	r.shell(`{"t":"findClose","tab":1}`)
	r.find(func(f map[string]any) bool { return f == nil })
	r.e.Settle()
	if got := r.e.FindsStopped(); !slices.Equal(got, []engine.TabID{1}) {
		t.Fatalf("stopped = %v", got)
	}

	// Opened again, it searches for the last text, and Find Next with it closed opens it.
	r.e.Fire(1, 0, "menu", "find.next")
	r.find(func(f map[string]any) bool { return f != nil && f["text"] == "nothing here" })
}
