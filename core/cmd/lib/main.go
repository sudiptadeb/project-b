// Command lib is the Go core as a c-archive, linked statically into the main binary.
// It implements engine.Engine on the C function table and hands every event from the
// adapter to the content package. Nothing else in core/ touches cgo, so everything
// else still builds and tests with no C++.
package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../../engine/abi
#include <stdlib.h>
#include "pb_abi.h"

static inline int64_t call_create_tab(const PBEngineCalls* c) { return c->create_tab(); }
static inline void call_close_tab(const PBEngineCalls* c, int64_t tab) { c->close_tab(tab); }
static inline void call_show_tab(const PBEngineCalls* c, int64_t tab) { c->show_tab(tab); }
static inline void call_close_window(const PBEngineCalls* c) { c->close_window(); }
static inline void call_load_url(const PBEngineCalls* c, int64_t tab, const char* url) { c->load_url(tab, url); }
static inline void call_go_back(const PBEngineCalls* c, int64_t tab) { c->go_back(tab); }
static inline void call_go_forward(const PBEngineCalls* c, int64_t tab) { c->go_forward(tab); }
static inline void call_reload(const PBEngineCalls* c, int64_t tab) { c->reload(tab); }
static inline void call_set_page_bounds(const PBEngineCalls* c, double x, double y, double w, double h, double vw, double vh) { c->set_page_bounds(x, y, w, h, vw, vh); }
static inline void call_set_shell_regions(const PBEngineCalls* c, const double* rects, size_t count) { c->set_shell_regions(rects, count); }
static inline void call_set_drag_regions(const PBEngineCalls* c, const double* rects, size_t count) { c->set_drag_regions(rects, count); }
static inline void call_set_window_frame(const PBEngineCalls* c, int32_t rgb, double width) { c->set_window_frame(rgb, width); }
static inline void call_focus_page(const PBEngineCalls* c) { c->focus_page(); }
static inline void call_focus_shell(const PBEngineCalls* c) { c->focus_shell(); }
static inline void call_send_to_shell(const PBEngineCalls* c, const char* json, size_t length) { c->send_to_shell(json, length); }
static inline void call_set_hook_subscribed(const PBEngineCalls* c, int32_t hook, int on) { c->set_hook_subscribed(hook, on); }
static inline void call_post_wakeup(const PBEngineCalls* c) { c->post_wakeup(); }
*/
import "C"

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"projectb/app"
	"projectb/content"
	"projectb/engine"
	"projectb/update"
)

// Where releases are published and how to recognise one of ours. The project and the URL
// are the ones in amarit.json at the repo root, which tools/release.sh publishes with; the
// key is the certificate tools/package.sh signs with (its SHA-1, as in the trust profile).
const (
	releaseProject  = "project-b"
	releaseManifest = "https://github.com/sudiptadeb/project-b/releases/latest/download/releases.json"
	signingLeafSHA1 = "59b20dfe02ee393a05173eec84daa8c662c845be"
)

// updateConfig describes this build to the updater. A binary running from a build directory
// rather than an installed bundle has no version, and never updates.
func updateConfig(profileDir string) update.Config {
	exe, err := os.Executable()
	if err != nil {
		return update.Config{}
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	bundle := update.BundleOf(exe)
	if bundle == "" {
		return update.Config{}
	}
	version, err := update.Version(bundle)
	if err != nil {
		log.Printf("update: %v; updates are off", err)
		return update.Config{}
	}
	return update.Config{
		Project:  releaseProject,
		Version:  version,
		Releases: releaseManifest,
		StateDir: filepath.Join(profileDir, "updates"),
		App:      bundle,
		Verify:   update.SignedBy(signingLeafSHA1),
	}
}

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

func (e *nativeEngine) CloseTab(tab engine.TabID) { C.call_close_tab(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) ShowTab(tab engine.TabID) { C.call_show_tab(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) CloseWindow() { C.call_close_window(e.calls) }

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
	flat := flatten(rects)
	C.call_set_shell_regions(e.calls, &flat[0], C.size_t(len(rects)))
}

func (e *nativeEngine) SetWindowFrame(hex string, width float64) {
	rgb, err := strconv.ParseInt(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil {
		log.Printf("window colour %q is not #rrggbb", hex)
		return
	}
	C.call_set_window_frame(e.calls, C.int32_t(rgb), C.double(width))
}

func (e *nativeEngine) SetDragRegions(rects []engine.Rect) {
	flat := flatten(rects)
	C.call_set_drag_regions(e.calls, &flat[0], C.size_t(len(rects)))
}

// flatten lays rectangles out as x, y, w, h, with a spare value so there is always an
// address to take.
func flatten(rects []engine.Rect) []C.double {
	flat := make([]C.double, 0, len(rects)*4+1)
	for _, r := range rects {
		flat = append(flat, C.double(r.X), C.double(r.Y), C.double(r.W), C.double(r.H))
	}
	return append(flat, 0)
}

func (e *nativeEngine) FocusPage() { C.call_focus_page(e.calls) }

func (e *nativeEngine) FocusShell() { C.call_focus_shell(e.calls) }

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
	hooks  *content.Hooks
	theApp *app.App
)

//export PBStart
func PBStart(calls *C.PBEngineCalls, profileDir, blockedHosts, hookTable *C.char) {
	if table := C.GoString(hookTable); table != content.TableHash {
		log.Fatalf("the library's hook table is %s, Go's is %s: regenerate with tools/gen_hooks.py and rebuild", table, content.TableHash)
	}
	// The table is C memory that lives as long as the process.
	native = &nativeEngine{calls: calls}
	hooks = content.New(func(h content.Hook) { C.call_set_hook_subscribed(native.calls, C.int32_t(h), 1) })
	var blocked []string
	if s := C.GoString(blockedHosts); s != "" {
		blocked = strings.Split(s, ",")
	}
	profile := C.GoString(profileDir)
	theApp = app.Start(native, app.Config{ProfileDir: profile, BlockedHosts: blocked, Update: updateConfig(profile)})
	content.Wire(hooks, theApp)
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

// PBHook is every event from the adapter. The values are copied out of C memory before
// anything runs, and a string answer is malloc'd for the adapter to free.
//
//export PBHook
func PBHook(id C.int32_t, args *C.PBValue, count C.size_t, result *C.PBValue) C.int {
	values := make([]content.Value, int(count))
	for i, a := range unsafe.Slice(args, int(count)) {
		values[i].I = int64(a.i)
		values[i].D = float64(a.d)
		if a.len > 0 {
			values[i].S = C.GoStringN(a.s, C.int(a.len))
		}
	}
	v, ok := hooks.Dispatch(content.Hook(id), values)
	if !ok {
		return 0
	}
	result.i = C.int64_t(v.I)
	result.d = C.double(v.D)
	if v.S != "" {
		result.s = C.CString(v.S)
		result.len = C.size_t(len(v.S))
	}
	return 1
}

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
		"hooks":       hooks.Counts(),
	})
	return C.CString(string(b))
}

func main() {}
