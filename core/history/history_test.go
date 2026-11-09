package history

import "testing"

func newManager(t *testing.T) *Manager {
	t.Helper()
	m := New(t.TempDir(), func(fn func()) { fn() })
	t.Cleanup(m.Close)
	return m
}

func TestOnlyWebPagesAreRecorded(t *testing.T) {
	m := newManager(t)
	m.AddVisit("about:blank", "")
	m.AddVisit("chrome://shell/", "")
	m.AddVisit("https://example.com/", "")
	if got := m.Search("e", 10); len(got) != 1 || got[0].URL != "https://example.com/" {
		t.Fatalf("got %v", got)
	}
}

func TestHostPrefixRanksAboveTitleMatch(t *testing.T) {
	m := newManager(t)
	m.AddVisit("https://notes.test/", "docs about go")
	m.AddVisit("https://www.go.dev/", "")
	got := m.Search("go", 10)
	if len(got) != 2 || got[0].URL != "https://www.go.dev/" {
		t.Fatalf("got %v", got)
	}
}

func TestVisitsAddUp(t *testing.T) {
	m := newManager(t)
	m.AddVisit("https://example.com/", "")
	m.AddVisit("https://example.com/", "Example")
	got := m.Search("example", 10)
	if len(got) != 1 || got[0].Visits != 2 || got[0].Title != "Example" {
		t.Fatalf("got %v", got)
	}
}
