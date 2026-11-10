package window

import (
	"reflect"
	"testing"

	"projectb/engine"
)

// recorder is an engine that keeps what the window manager told it.
type recorder struct {
	engine.Engine
	layouts  []engine.WindowLayout
	requests []engine.WindowRequest
}

func (r *recorder) SetWindowLayout(l engine.WindowLayout) { r.layouts = append(r.layouts, l) }
func (r *recorder) RequestWindow(q engine.WindowRequest)  { r.requests = append(r.requests, q) }
func (r *recorder) last() engine.WindowLayout             { return r.layouts[len(r.layouts)-1] }

var (
	mac    = engine.Rect{X: 9, Y: 9, W: 60, H: 14}
	report = ShellLayout{
		Shell: []engine.Rect{{X: 0, Y: 0, W: 1100, H: 48}, {X: 0, Y: 48, W: 220, H: 652}},
		Drag:  []engine.Rect{{X: 80, Y: 4, W: 40, H: 40}, {X: 900, Y: 4, W: 196, H: 40}},
		Page:  engine.Rect{X: 224, Y: 48, W: 872, H: 648},
		ViewW: 1100,
		ViewH: 700,
	}
)

func TestTheFirstLayoutCarriesTheFrame(t *testing.T) {
	e := &recorder{}
	New(e, "#4f9e9e", engine.ThemeSystem)
	if len(e.layouts) != 1 || e.last().FrameColor != "#4f9e9e" || e.last().FrameWidth != 4 || len(e.last().Regions) != 0 {
		t.Fatalf("layouts %+v", e.layouts)
	}
}

func TestWhatEachPointDoes(t *testing.T) {
	e := &recorder{}
	m := New(e, "#4f9e9e", engine.ThemeSystem)
	m.HandleWindowChanged(engine.WindowState{W: 1100, H: 700, Controls: mac, CornerRadius: 26})
	m.HandleShellLayout(report)
	l := e.last()
	if l.Page != report.Page || l.ViewW != 1100 || l.ViewH != 700 {
		t.Fatalf("layout %+v", l)
	}
	for _, c := range []struct {
		x, y float64
		want engine.Area
	}{
		{100, 20, engine.AreaDrag},   // The empty space after the window buttons, over the shell's top row.
		{1000, 20, engine.AreaDrag},  // The empty right end of the top row.
		{300, 20, engine.AreaShell},  // A control in the top row.
		{100, 600, engine.AreaShell}, // The sidebar, which never drags.
		{600, 300, engine.AreaPage},  // The page.
	} {
		if got := l.At(c.x, c.y); got != c.want {
			t.Errorf("At(%v, %v) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestFullScreenSendsNoDragAreas(t *testing.T) {
	e := &recorder{}
	m := New(e, "#4f9e9e", engine.ThemeSystem)
	m.HandleShellLayout(report)
	m.HandleWindowChanged(engine.WindowState{W: 1440, H: 900, FullScreen: true})
	if got := e.last().At(100, 20); got != engine.AreaShell {
		t.Fatalf("in full screen the top row's empty space is %v", got)
	}
	for _, g := range e.last().Regions {
		if g.Area == engine.AreaDrag {
			t.Fatalf("a drag area in full screen: %+v", e.last().Regions)
		}
	}
	m.HandleWindowChanged(engine.WindowState{W: 1100, H: 700, Controls: mac, CornerRadius: 26})
	if got := e.last().At(100, 20); got != engine.AreaDrag {
		t.Fatalf("back from full screen the top row's empty space is %v", got)
	}
}

func TestAnUnchangedLayoutIsNotSentTwice(t *testing.T) {
	e := &recorder{}
	m := New(e, "#4f9e9e", engine.ThemeSystem)
	m.HandleShellLayout(report)
	m.HandleShellLayout(report)
	// A resize changes the window's size, but not what the shell last reported.
	m.HandleWindowChanged(engine.WindowState{W: 1101, H: 700, Controls: mac, CornerRadius: 26})
	m.HandleWindowChanged(engine.WindowState{W: 1102, H: 700, Controls: mac, CornerRadius: 26})
	m.SetColor("#4f9e9e")
	if len(e.layouts) != 2 {
		t.Fatalf("sent %d layouts: %+v", len(e.layouts), e.layouts)
	}
}

func TestColorRepaintsTheWindowAndTellsSubscribers(t *testing.T) {
	e := &recorder{}
	m := New(e, "#4f9e9e", engine.ThemeSystem)
	m.HandleShellLayout(report)
	var got []string
	m.OnColorChanged(func(c string) { got = append(got, c) })
	m.SetColor("#3470d0")
	m.SetColor("#3470d0")
	if e.last().FrameColor != "#3470d0" || len(e.last().Regions) != 4 || m.Color() != "#3470d0" || !reflect.DeepEqual(got, []string{"#3470d0"}) {
		t.Fatalf("window %+v, manager %v, changes %v", e.last(), m.Color(), got)
	}
}

func TestControlsChangeOnlyWhenDifferent(t *testing.T) {
	m := New(&recorder{}, "#4f9e9e", engine.ThemeSystem)
	var got []engine.Rect
	var radii []float64
	m.OnControlsChanged(func(r engine.Rect, radius float64) {
		got = append(got, r)
		radii = append(radii, radius)
	})
	m.HandleWindowChanged(engine.WindowState{W: 1100, H: 700, Controls: mac, CornerRadius: 26})
	m.HandleWindowChanged(engine.WindowState{W: 1200, H: 700, Controls: mac, CornerRadius: 26})
	m.HandleWindowChanged(engine.WindowState{W: 1440, H: 900, FullScreen: true})
	if !reflect.DeepEqual(got, []engine.Rect{mac, {}}) || !reflect.DeepEqual(radii, []float64{26, 0}) {
		t.Fatalf("got %v radii %v", got, radii)
	}
	if m.Controls() != (engine.Rect{}) || m.CornerRadius() != 0 {
		t.Fatalf("controls = %v radius = %v", m.Controls(), m.CornerRadius())
	}
}

func TestAShellCrashGivesThePageEveryPress(t *testing.T) {
	e := &recorder{}
	m := New(e, "#4f9e9e", engine.ThemeSystem)
	m.HandleShellLayout(report)
	m.HandleShellCrashed()
	if l := e.last(); len(l.Regions) != 0 || l.Page != report.Page || l.At(300, 20) != engine.AreaPage {
		t.Fatalf("after the crash %+v", l)
	}
	// The reloaded shell reports the same layout as before, and it goes out again.
	m.HandleShellLayout(report)
	if got := e.last().At(300, 20); got != engine.AreaShell {
		t.Fatalf("the reloaded shell's control is %v", got)
	}
}

func TestRequestsGoToTheEngine(t *testing.T) {
	e := &recorder{}
	m := New(e, "#4f9e9e", engine.ThemeSystem)
	m.Request(engine.WindowFullScreen)
	m.Request(engine.WindowZoom)
	m.Request(engine.WindowMinimise)
	m.Request(engine.WindowClose)
	want := []engine.WindowRequest{engine.WindowFullScreen, engine.WindowZoom, engine.WindowMinimise, engine.WindowClose}
	if !reflect.DeepEqual(e.requests, want) {
		t.Fatalf("requests %v", e.requests)
	}
}
