package shells

import "testing"

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
