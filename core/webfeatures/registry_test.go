package webfeatures

import "testing"

// Every row of registry.json holds together, and a row that doesn't is refused.
func TestTheRegistry(t *testing.T) {
	if len(All) == 0 {
		t.Fatal("no rows")
	}
	const row = `"name":"A","api":"a()","why":"w"`
	for _, bad := range []string{
		`[{"id":"a",` + row + `,"status":{"mac":"works"}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"fine"}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works","linux":"works"}}]`,
		`[{"id":"a","name":"A","api":"a()","status":{"mac":"works","android":"works"}}]`,
		`[{"id":"A",` + row + `,"status":{"mac":"works","android":"works"}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"owner":"x","ownr":"y"}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"}},{"id":"a",` + row + `,"status":{"mac":"works","android":"works"}}]`,
		// Off says what turns it off, and what is turned off is off everywhere.
		`[{"id":"a",` + row + `,"status":{"mac":"off","android":"off"}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"off","android":"works"},"disable_blink_features":["A"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"quiet","android":"quiet"},"disable_features":["A"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"off","android":"off"},"disable_features":["A,B"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"interface":{"name":"x.mojom.X","scope":["page"]}}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"delegate":"RunFileChooser"}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"needs":["luck"]}]`,
		// A test names a probe of its own row, a basics check by number, or a check.
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"test":["probe:b"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"test":["basics:x"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"test":["drive"]}]`,
		`[{"id":"a",` + row + `,"status":{"mac":"works","android":"works"},"probe":[{"name":"p","call":"p()"}]},` +
			`{"id":"b",` + row + `,"status":{"mac":"works","android":"works"},"probe":[{"name":"p","call":"q()"}]}]`,
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	good := `[{"id":"a",` + row + `,"status":{"mac":"off","android":"off"},"disable_blink_features":["A"],` +
		`"probe":[{"name":"p","call":"p()"}],"test":["probe:p","basics:3","check:a page keeps its renderer"]}]`
	if _, err := Parse([]byte(good)); err != nil {
		t.Errorf("refused %s: %v", good, err)
	}
}
