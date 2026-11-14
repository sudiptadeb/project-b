package tones

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Every palette is what shell/src/tones.js made, light and dark, for the presets, the
// workspace colours and a spread of custom colours: testdata/tones.json, written from that
// JavaScript by testdata/gen.mjs before Go took it over.
func TestTheSameTonesAsTheJavaScript(t *testing.T) {
	b, err := os.ReadFile("testdata/tones.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]Both
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	if len(want) < 600 {
		t.Fatalf("only %d frames", len(want))
	}
	for frame, w := range want {
		got := Of(frame)
		if !reflect.DeepEqual(got.Light, w.Light) {
			t.Errorf("%s light:\n got %v\nwant %v", frame, got.Light, w.Light)
		}
		if !reflect.DeepEqual(got.Dark, w.Dark) {
			t.Errorf("%s dark:\n got %v\nwant %v", frame, got.Dark, w.Dark)
		}
	}
}

// No colour, or one that isn't #rrggbb, has nothing to paint.
func TestNoTonesWithoutAColour(t *testing.T) {
	for _, frame := range []string{"", "#fff", "4f9e9e00", "#4f9e9g"} {
		if Of(frame) != nil {
			t.Errorf("%q has tones", frame)
		}
	}
}
