package app_test

import (
	"reflect"
	"testing"

	"projectb/engine"
)

// A page's file panel opens where files were last chosen, and not for a blocked address.
func TestTheFilePanelOpensWhereFilesWereLastChosen(t *testing.T) {
	r := start(t, t.TempDir(), "blocked.test")
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "chooseFile", "/Users/me/Documents")
	r.e.Fire(1, 0, "chooseFile", "")
	r.e.Settle()
	want := []engine.FileChooserAnswer{{Allow: true}, {Allow: true, Folder: "/Users/me/Documents"}}
	if got := r.e.FileChoosers(); !reflect.DeepEqual(got, want) {
		t.Fatalf("answers = %+v", got)
	}

	r.shell(`{"t":"navigate","text":"https://blocked.test/"}`)
	r.waitForTab(func(tab map[string]any) bool { return tab["blocked"] == true })
	r.e.Fire(1, 0, "chooseFile", "/tmp")
	r.e.Settle()
	if got := r.e.FileChoosers(); got[len(got)-1].Allow {
		t.Fatalf("a blocked page shouldn't get the panel: %+v", got)
	}
}
