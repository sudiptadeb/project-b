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
	to       []engine.WindowID // The window each layout went to.
	created  []engine.WindowID // Each window made, and the one it was put near.
	requests []engine.WindowRequest
	pages    []engine.Page // Each page a window was given after it was made.
}

func (r *recorder) CreateWindow(w, near engine.WindowID, page engine.Page, l engine.WindowLayout) {
	r.created = append(r.created, w, near)
	r.SetWindowLayout(w, l)
}
func (r *recorder) SetWindowLayout(w engine.WindowID, l engine.WindowLayout) {
	r.layouts = append(r.layouts, l)
	r.to = append(r.to, w)
}
func (r *recorder) RequestWindow(w engine.WindowID, q engine.WindowRequest) {
	r.requests = append(r.requests, q)
}
func (r *recorder) SetWindowPage(w engine.WindowID, page engine.Page) {
	r.pages = append(r.pages, page)
}
func (r *recorder) last() engine.WindowLayout { return r.layouts[len(r.layouts)-1] }

// one is a manager with window 1 open.
func one(e *recorder) *Manager {
	m := New(e, engine.ShellPage("standard"), "#4f9e9e", engine.ThemeSystem, func() engine.WindowID { return 1 })
	m.Open(1, 0)
	return m
}

var (
	mac    = engine.Rect{X: 9, Y: 9, W: 60, H: 14}
	report = ShellLayout{
		Shell: []engine.Rect{{X: 0, Y: 0, W: 1100, H: 48}, {X: 0, Y: 48, W: 220, H: 652}},
		Drag:  []engine.Rect{{X: 80, Y: 4, W: 40, H: 40}, {X: 900, Y: 4, W: 196, H: 40}},
		Tabs:  []engine.VisibleTab{{Tab: 1, Rect: engine.Rect{X: 224, Y: 48, W: 872, H: 648}}},
		ViewW: 1100,
		ViewH: 700,
	}
)

func TestTheFirstLayoutCarriesTheFrame(t *testing.T) {
	e := &recorder{}
	one(e)
	if len(e.layouts) != 1 || e.last().FrameColor != "#4f9e9e" || e.last().FrameWidth != 4 || len(e.last().Regions) != 0 {
		t.Fatalf("layouts %+v", e.layouts)
	}
}

func TestWhatEachPointDoes(t *testing.T) {
	e := &recorder{}
	m := one(e)
	m.HandleWindowChanged(1, engine.WindowState{W: 1100, H: 700, Controls: mac, CornerRadius: 26})
	m.HandleShellLayout(1, report)
	l := e.last()
	if !reflect.DeepEqual(l.Tabs, report.Tabs) || l.ViewW != 1100 || l.ViewH != 700 {
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

// The shell decides which tabs show and where; the layout carries every one of them, so
// a shell that shows two tabs side by side gets both.
func TestTheLayoutCarriesEveryVisibleTab(t *testing.T) {
	e := &recorder{}
	m := one(e)
	split := report
	split.Tabs = []engine.VisibleTab{
		{Tab: 1, Rect: engine.Rect{X: 224, Y: 48, W: 436, H: 648}},
		{Tab: 2, Rect: engine.Rect{X: 660, Y: 48, W: 436, H: 648}},
	}
	m.HandleShellLayout(1, split)
	if !reflect.DeepEqual(e.last().Tabs, split.Tabs) {
		t.Fatalf("tabs %+v", e.last().Tabs)
	}
	m.HandleShellLayout(1, report)
	if !reflect.DeepEqual(e.last().Tabs, report.Tabs) {
		t.Fatalf("back to one tab: %+v", e.last().Tabs)
	}
}

func TestFullScreenSendsNoDragAreas(t *testing.T) {
	e := &recorder{}
	m := one(e)
	m.HandleShellLayout(1, report)
	m.HandleWindowChanged(1, engine.WindowState{W: 1440, H: 900, FullScreen: true})
	if got := e.last().At(100, 20); got != engine.AreaShell {
		t.Fatalf("in full screen the top row's empty space is %v", got)
	}
	for _, g := range e.last().Regions {
		if g.Area == engine.AreaDrag {
			t.Fatalf("a drag area in full screen: %+v", e.last().Regions)
		}
	}
	m.HandleWindowChanged(1, engine.WindowState{W: 1100, H: 700, Controls: mac, CornerRadius: 26})
	if got := e.last().At(100, 20); got != engine.AreaDrag {
		t.Fatalf("back from full screen the top row's empty space is %v", got)
	}
}

func TestAnUnchangedLayoutIsNotSentTwice(t *testing.T) {
	e := &recorder{}
	m := one(e)
	m.HandleShellLayout(1, report)
	m.HandleShellLayout(1, report)
	// A resize changes the window's size, but not what the shell last reported.
	m.HandleWindowChanged(1, engine.WindowState{W: 1101, H: 700, Controls: mac, CornerRadius: 26})
	m.HandleWindowChanged(1, engine.WindowState{W: 1102, H: 700, Controls: mac, CornerRadius: 26})
	m.SetColor("#4f9e9e")
	if len(e.layouts) != 2 {
		t.Fatalf("sent %d layouts: %+v", len(e.layouts), e.layouts)
	}
}

func TestColorRepaintsTheWindowAndTellsSubscribers(t *testing.T) {
	e := &recorder{}
	m := one(e)
	m.HandleShellLayout(1, report)
	var got []string
	m.OnColorChanged(func(c string) { got = append(got, c) })
	m.SetColor("#3470d0")
	m.SetColor("#3470d0")
	if e.last().FrameColor != "#3470d0" || len(e.last().Regions) != 4 || m.Color() != "#3470d0" || !reflect.DeepEqual(got, []string{"#3470d0"}) {
		t.Fatalf("window %+v, manager %v, changes %v", e.last(), m.Color(), got)
	}
}

func TestControlsChangeOnlyWhenDifferent(t *testing.T) {
	m := one(&recorder{})
	var got []engine.Rect
	var radii []float64
	m.OnControlsChanged(func(_ engine.WindowID, r engine.Rect, radius float64) {
		got = append(got, r)
		radii = append(radii, radius)
	})
	m.HandleWindowChanged(1, engine.WindowState{W: 1100, H: 700, Controls: mac, CornerRadius: 26})
	m.HandleWindowChanged(1, engine.WindowState{W: 1200, H: 700, Controls: mac, CornerRadius: 26})
	m.HandleWindowChanged(1, engine.WindowState{W: 1440, H: 900, FullScreen: true})
	if !reflect.DeepEqual(got, []engine.Rect{mac, {}}) || !reflect.DeepEqual(radii, []float64{26, 0}) {
		t.Fatalf("got %v radii %v", got, radii)
	}
	if m.Controls(1) != (engine.Rect{}) || m.CornerRadius(1) != 0 {
		t.Fatalf("controls = %v radius = %v", m.Controls(1), m.CornerRadius(1))
	}
}

func TestAShellCrashGivesThePageEveryPress(t *testing.T) {
	e := &recorder{}
	m := one(e)
	m.HandleShellLayout(1, report)
	m.HandleShellCrashed(1)
	if l := e.last(); len(l.Regions) != 0 || !reflect.DeepEqual(l.Tabs, report.Tabs) || l.At(300, 20) != engine.AreaPage {
		t.Fatalf("after the crash %+v", l)
	}
	// The reloaded shell reports the same layout as before, and it goes out again.
	m.HandleShellLayout(1, report)
	if got := e.last().At(300, 20); got != engine.AreaShell {
		t.Fatalf("the reloaded shell's control is %v", got)
	}
}

func TestRequestsGoToTheEngine(t *testing.T) {
	e := &recorder{}
	m := one(e)
	m.Request(1, engine.WindowFullScreen)
	m.Request(1, engine.WindowZoom)
	m.Request(1, engine.WindowMinimise)
	m.Request(1, engine.WindowClose)
	want := []engine.WindowRequest{engine.WindowFullScreen, engine.WindowZoom, engine.WindowMinimise, engine.WindowClose}
	if !reflect.DeepEqual(e.requests, want) {
		t.Fatalf("requests %v", e.requests)
	}
}

// Each window has its own layout, made from its own shell's report and its own state; the
// frame colour is every window's.
func TestEachWindowHasItsOwnLayout(t *testing.T) {
	e := &recorder{}
	m := one(e)
	m.HandleShellLayout(1, report)
	m.Open(2, 1)
	m.HandleWindowChanged(2, engine.WindowState{W: 1440, H: 900, FullScreen: true})
	m.HandleShellLayout(2, ShellLayout{Shell: report.Shell, Drag: report.Drag, ViewW: 1440, ViewH: 900})
	if !reflect.DeepEqual(e.created, []engine.WindowID{1, 0, 2, 1}) {
		t.Fatalf("created %v", e.created)
	}
	if e.to[len(e.to)-1] != 2 || len(e.last().Tabs) != 0 || e.last().At(100, 20) != engine.AreaShell {
		t.Fatalf("window 2's layout %+v", e.last())
	}
	if m.FullScreen(1) || !m.FullScreen(2) || m.Windows()[0] != 1 || len(m.Windows()) != 2 {
		t.Fatalf("full screen %v %v, windows %v", m.FullScreen(1), m.FullScreen(2), m.Windows())
	}
	sent := len(e.layouts)
	m.SetColor("#3470d0")
	if !reflect.DeepEqual(e.to[sent:], []engine.WindowID{1, 2}) {
		t.Fatalf("the colour went to %v", e.to[sent:])
	}
	m.Close(2)
	sent = len(e.layouts)
	m.SetColor("#4f9e9e")
	if !reflect.DeepEqual(e.to[sent:], []engine.WindowID{1}) || !reflect.DeepEqual(m.Windows(), []engine.WindowID{1}) {
		t.Fatalf("after window 2 closed the colour went to %v", e.to[sent:])
	}
}

// Another manager reads each window's bounds, state and whether it is in front, and hears
// windows open and close.
func TestWhatAnotherManagerReadsAndHears(t *testing.T) {
	front := engine.WindowID(1)
	m := New(&recorder{}, engine.ShellPage("standard"), "#4f9e9e", engine.ThemeSystem, func() engine.WindowID { return front })
	var events []Event
	m.OnEvent(func(e Event) { events = append(events, e) })
	m.Open(1, 0)
	m.Open(2, 1)
	m.HandleWindowChanged(1, engine.WindowState{Bounds: engine.Rect{X: 80, Y: 60, W: 1280, H: 720}, Zoomed: true})
	m.HandleWindowChanged(2, engine.WindowState{Bounds: engine.Rect{X: 109, Y: 89, W: 1280, H: 720}, Minimised: true})
	front = 2
	m.HandleFrontChanged(2)
	m.HandleTabMoved(7, 1, 3, 2)
	m.Close(1)
	want := []Window{{ID: 2, Bounds: engine.Rect{X: 109, Y: 89, W: 1280, H: 720}, State: StateMinimised, Front: true}}
	if got := m.List(); !reflect.DeepEqual(got, want) {
		t.Fatalf("windows %+v", got)
	}
	wantEvents := []Event{{Kind: Opened, Window: 1}, {Kind: Opened, Window: 2}, {Kind: BoundsChanged, Window: 1},
		{Kind: BoundsChanged, Window: 2}, {Kind: FrontChanged, Window: 2},
		{Kind: TabMoved, Window: 2, Tab: 7, From: 1, FromIndex: 3}, {Kind: Closed, Window: 1}}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events %+v", events)
	}
}

// Another page in every window drops what the old one claimed, and leaves the tabs where
// they were; a window opened after it gets the new page too.
func TestAnotherPageDropsWhatTheOldOneClaimed(t *testing.T) {
	e := &recorder{}
	m := one(e)
	m.HandleShellLayout(1, report)
	m.SetPage(engine.ShellPage("example"))
	if l := e.last(); !reflect.DeepEqual(e.pages, []engine.Page{"shell/example"}) || len(l.Regions) != 0 || !reflect.DeepEqual(l.Tabs, report.Tabs) {
		t.Fatalf("pages %v, layout %+v", e.pages, l)
	}
	if m.page != "shell/example" {
		t.Fatalf("a new window gets %q", m.page)
	}
}
