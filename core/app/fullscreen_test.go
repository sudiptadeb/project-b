package app_test

import (
	"testing"
	"time"

	"projectb/engine"
)

func TestAPageFillsTheScreenAndEscapeEndsIt(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	full := func(tab float64) {
		r.t.Helper()
		r.waitFor(func(m map[string]any) bool { return m["t"] == "pageFullScreen" && m["pageFullScreen"] == tab })
	}
	windowFull := func(on bool) {
		r.t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			got := make(chan bool)
			r.e.Post(func() { got <- r.a.Window.FullScreen(1) })
			if <-got == on {
				return
			}
			if time.Now().After(deadline) {
				r.t.Fatalf("the window's full screen should be %v", on)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	r.e.Fire(1, 0, "pageFullscreen", "")
	full(1)
	windowFull(true)
	asked := make(chan bool)
	r.e.Post(func() { asked <- r.a.IsPageFullScreen(1) })
	if !<-asked {
		t.Fatal("content should hear the page is in full screen")
	}

	// Escape, before the page has it.
	taken := make(chan bool)
	r.e.Post(func() { taken <- r.a.Key(engine.Key{Code: 0x1b}, engine.KeyBeforePage, engine.SurfacePage) })
	if !<-taken {
		t.Fatal("Escape should be taken")
	}
	full(0)
	windowFull(false)

	// Already in full screen, the window stays so after.
	r.e.Fire(1, 0, "fullscreen", "")
	windowFull(true)
	r.e.Fire(1, 0, "pageFullscreen", "")
	full(1)
	r.shell(`{"t":"newTab"}`)
	full(0)
	windowFull(true)
}
