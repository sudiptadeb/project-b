package window

import (
	"testing"

	"projectb/engine"
)

func TestControlsChangeOnlyWhenDifferent(t *testing.T) {
	m := New()
	var got []engine.Rect
	m.OnControlsChanged(func(r engine.Rect) { got = append(got, r) })

	mac := engine.Rect{X: 9, Y: 9, W: 60, H: 14}
	m.HandleControlsChanged(mac)
	m.HandleControlsChanged(mac)
	m.HandleControlsChanged(engine.Rect{})

	if len(got) != 2 || got[0] != mac || got[1] != (engine.Rect{}) {
		t.Fatalf("got %v", got)
	}
	if m.Controls() != (engine.Rect{}) {
		t.Fatalf("controls = %v", m.Controls())
	}
}
