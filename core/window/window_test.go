package window

import (
	"testing"

	"projectb/engine"
	"projectb/mockengine"
)

func TestControlsChangeOnlyWhenDifferent(t *testing.T) {
	m := New(mockengine.New(), "#4f9e9e")
	var got []engine.Rect
	var radii []float64
	m.OnControlsChanged(func(r engine.Rect, radius float64) {
		got = append(got, r)
		radii = append(radii, radius)
	})

	mac := engine.Rect{X: 9, Y: 9, W: 60, H: 14}
	m.HandleControlsChanged(mac, 20)
	m.HandleControlsChanged(mac, 20)
	m.HandleControlsChanged(engine.Rect{}, 0) // Full screen: no controls, square corners.

	if len(got) != 2 || got[0] != mac || got[1] != (engine.Rect{}) {
		t.Fatalf("got %v", got)
	}
	if radii[0] != 20 || radii[1] != 0 {
		t.Fatalf("radii %v", radii)
	}
	if m.Controls() != (engine.Rect{}) || m.CornerRadius() != 0 {
		t.Fatalf("controls = %v radius = %v", m.Controls(), m.CornerRadius())
	}
}

func TestColorRepaintsTheWindowAndTellsSubscribers(t *testing.T) {
	e := mockengine.New()
	m := New(e, "#4f9e9e")
	if e.WindowColor() != "#4f9e9e" || e.FrameWidth() != 4 {
		t.Fatalf("window frame %v %v", e.WindowColor(), e.FrameWidth())
	}
	var got []string
	m.OnColorChanged(func(c string) { got = append(got, c) })
	m.SetColor("#3470d0")
	m.SetColor("#3470d0")
	if e.WindowColor() != "#3470d0" || m.Color() != "#3470d0" || len(got) != 1 || got[0] != "#3470d0" {
		t.Fatalf("window %v, manager %v, changes %v", e.WindowColor(), m.Color(), got)
	}
}
