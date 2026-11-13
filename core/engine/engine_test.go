package engine

import "testing"

func TestTheFirstRegionThatHoldsAPointDecides(t *testing.T) {
	l := WindowLayout{Regions: []Region{
		{Rect{X: 100, Y: 0, W: 50, H: 40}, AreaDrag},
		{Rect{X: 0, Y: 0, W: 800, H: 40}, AreaShell},
	}}
	for _, c := range []struct {
		x, y float64
		want Area
	}{{120, 10, AreaDrag}, {10, 10, AreaShell}, {150, 10, AreaShell}, {10, 40, AreaPage}} {
		if got := l.At(c.x, c.y); got != c.want {
			t.Errorf("At(%v, %v) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestAShellsPageNamesItsShell(t *testing.T) {
	if p := ShellPage("standard"); p != "shell/standard" || p.Shell() != "standard" {
		t.Errorf("ShellPage(standard) = %q, its shell %q", p, p.Shell())
	}
	if PagePicker.Shell() != "" {
		t.Errorf("the picker is shell %q", PagePicker.Shell())
	}
}
