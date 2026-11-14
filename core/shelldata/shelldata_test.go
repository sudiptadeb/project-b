package shelldata

import (
	"testing"
)

func TestKeepsEachShellsDataAcrossManagers(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if got := string(m.Get("glide")); got != "null" {
		t.Fatalf("nothing saved should be null, got %s", got)
	}
	if err := m.Set("glide", []byte(`{"favorites":{"":[{"url":"https://a.test/"}]}}`)); err != nil {
		t.Fatal(err)
	}
	if err := m.Set("terminal", []byte(`[1]`)); err != nil {
		t.Fatal(err)
	}
	again := New(dir)
	if got := string(again.Get("glide")); got != `{"favorites":{"":[{"url":"https://a.test/"}]}}` {
		t.Fatalf("glide: %s", got)
	}
	if got := string(again.Get("terminal")); got != `[1]` {
		t.Fatalf("terminal: %s", got)
	}
	if err := m.Set("glide", []byte(`{nope`)); err == nil {
		t.Fatal("bad JSON should be refused")
	}
}
