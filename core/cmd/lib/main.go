// Command lib is the Go core as a c-archive, linked statically into the main binary.
// It implements engine.Engine on the C function table and forwards the adapter's
// calls to app.App. Nothing else in core/ touches cgo, so everything else still builds
// and tests with no C++.
package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../../engine/abi
#include <stdlib.h>
#include "pb_abi.h"

static inline int64_t call_create_tab(const PBEngineCalls* c) { return c->create_tab(); }
static inline void call_load_url(const PBEngineCalls* c, int64_t tab, const char* url) { c->load_url(tab, url); }
static inline void call_go_back(const PBEngineCalls* c, int64_t tab) { c->go_back(tab); }
static inline void call_go_forward(const PBEngineCalls* c, int64_t tab) { c->go_forward(tab); }
static inline void call_reload(const PBEngineCalls* c, int64_t tab) { c->reload(tab); }
static inline void call_set_page_bounds(const PBEngineCalls* c, double x, double y, double w, double h, double vw, double vh) { c->set_page_bounds(x, y, w, h, vw, vh); }
static inline void call_set_shell_regions(const PBEngineCalls* c, const double* rects, size_t count) { c->set_shell_regions(rects, count); }
static inline void call_focus_page(const PBEngineCalls* c) { c->focus_page(); }
static inline void call_send_to_shell(const PBEngineCalls* c, const char* json, size_t length) { c->send_to_shell(json, length); }
static inline void call_post_wakeup(const PBEngineCalls* c) { c->post_wakeup(); }
*/
import "C"

import (
	"encoding/json"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"unsafe"

	"projectb/app"
	"projectb/engine"
)

// nativeEngine is the real engine: every method calls the C++ adapter.
type nativeEngine struct {
	calls *C.PBEngineCalls

	mu     sync.Mutex
	posted []func()
}

func (e *nativeEngine) Name() string { return "content" }

func (e *nativeEngine) CreateTab() engine.TabID {
	return engine.TabID(C.call_create_tab(e.calls))
}

func (e *nativeEngine) LoadURL(tab engine.TabID, url string) {
	c := C.CString(url)
	defer C.free(unsafe.Pointer(c))
	C.call_load_url(e.calls, C.int64_t(tab), c)
}

func (e *nativeEngine) GoBack(tab engine.TabID) { C.call_go_back(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) GoForward(tab engine.TabID) { C.call_go_forward(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) Reload(tab engine.TabID) { C.call_reload(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) SetPageBounds(r engine.Rect, viewW, viewH float64) {
	C.call_set_page_bounds(e.calls, C.double(r.X), C.double(r.Y), C.double(r.W), C.double(r.H), C.double(viewW), C.double(viewH))
}

func (e *nativeEngine) SetShellRegions(rects []engine.Rect) {
	flat := make([]C.double, 0, len(rects)*4+1)
	for _, r := range rects {
		flat = append(flat, C.double(r.X), C.double(r.Y), C.double(r.W), C.double(r.H))
	}
	flat = append(flat, 0) // Never empty, so there's always an address to take.
	C.call_set_shell_regions(e.calls, &flat[0], C.size_t(len(rects)))
}

func (e *nativeEngine) FocusPage() { C.call_focus_page(e.calls) }

func (e *nativeEngine) SendToShell(msg []byte) {
	if len(msg) == 0 {
		return
	}
	C.call_send_to_shell(e.calls, (*C.char)(unsafe.Pointer(&msg[0])), C.size_t(len(msg)))
}

// Post runs fn on the main thread. Any goroutine may call it.
func (e *nativeEngine) Post(fn func()) {
	e.mu.Lock()
	e.posted = append(e.posted, fn)
	first := len(e.posted) == 1
	e.mu.Unlock()
	if first {
		C.call_post_wakeup(e.calls)
	}
}

var (
	native *nativeEngine
	theApp *app.App
)

//export PBStart
func PBStart(calls *C.PBEngineCalls, profileDir, blockedHosts *C.char) {
	// The table is C memory that lives as long as the process.
	native = &nativeEngine{calls: calls}
	var blocked []string
	if s := C.GoString(blockedHosts); s != "" {
		blocked = strings.Split(s, ",")
	}
	theApp = app.Start(native, app.Config{ProfileDir: C.GoString(profileDir), BlockedHosts: blocked})
}

//export PBStop
func PBStop() { theApp.Stop() }

//export PBRunPosted
func PBRunPosted() {
	native.mu.Lock()
	fns := native.posted
	native.posted = nil
	native.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

//export PBShellMessage
func PBShellMessage(msg *C.char, length C.size_t) {
	theApp.ShellMessage(C.GoBytes(unsafe.Pointer(msg), C.int(length)))
}

//export PBNavigationStarting
func PBNavigationStarting(tab C.int64_t, url *C.char) C.int {
	if theApp.NavigationStarting(engine.TabID(tab), C.GoString(url)) {
		return 1
	}
	return 0
}

//export PBNavigationFinished
func PBNavigationFinished(tab C.int64_t, url *C.char) {
	theApp.NavigationFinished(engine.TabID(tab), C.GoString(url))
}

//export PBNavigationFailed
func PBNavigationFailed(tab C.int64_t, url, reason *C.char) {
	theApp.NavigationFailed(engine.TabID(tab), C.GoString(url), C.GoString(reason))
}

//export PBHistoryChanged
func PBHistoryChanged(tab C.int64_t, canGoBack, canGoForward C.int) {
	theApp.HistoryChanged(engine.TabID(tab), canGoBack != 0, canGoForward != 0)
}

//export PBTitleChanged
func PBTitleChanged(tab C.int64_t, title *C.char) {
	theApp.TitleChanged(engine.TabID(tab), C.GoString(title))
}

//export PBLoadingChanged
func PBLoadingChanged(tab C.int64_t, loading C.int) {
	theApp.LoadingChanged(engine.TabID(tab), loading != 0)
}

//export PBPageCrashed
func PBPageCrashed(tab C.int64_t) { theApp.PageCrashed(engine.TabID(tab)) }

//export PBFocusAddressBarRequested
func PBFocusAddressBarRequested() { theApp.FocusAddressBarRequested() }

//export PBWindowControlsChanged
func PBWindowControlsChanged(x, y, w, h C.double) {
	theApp.WindowControlsChanged(engine.Rect{X: float64(x), Y: float64(y), W: float64(w), H: float64(h)})
}

// PBPing does nothing. The driver times it to measure the boundary.
//
//export PBPing
func PBPing() {}

// PBForceGC runs a full collection. The driver times it on the main thread.
//
//export PBForceGC
func PBForceGC() { runtime.GC() }

// PBRecoverNilDeref checks that a nil dereference in Go still becomes a panic that can
// be recovered, with Chromium's signal handlers installed in the same process.
//
//export PBRecoverNilDeref
func PBRecoverNilDeref() (result C.int) {
	defer func() {
		if recover() != nil {
			result = 1
		}
	}()
	var p *int
	*p = 1
	return 0
}

// PBGoStats returns runtime numbers as JSON. The caller frees the string.
//
//export PBGoStats
func PBGoStats() *C.char {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	var gc debug.GCStats
	debug.ReadGCStats(&gc)
	var maxPauseNs int64
	for _, p := range gc.Pause {
		if p.Nanoseconds() > maxPauseNs {
			maxPauseNs = p.Nanoseconds()
		}
	}
	b, _ := json.Marshal(map[string]any{
		"goroutines":  runtime.NumGoroutine(),
		"heapAllocKB": m.HeapAlloc / 1024,
		"sysKB":       m.Sys / 1024,
		"numGC":       m.NumGC,
		"maxPauseUs":  maxPauseNs / 1000,
		"cgoCalls":    runtime.NumCgoCall(),
	})
	return C.CString(string(b))
}

func main() {}
