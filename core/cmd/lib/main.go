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
static inline void call_request_window(const PBEngineCalls* c, int32_t request) { c->request_window(request); }
static inline void call_load_url(const PBEngineCalls* c, int64_t tab, const char* url) { c->load_url(tab, url); }
static inline void call_go_back(const PBEngineCalls* c, int64_t tab) { c->go_back(tab); }
static inline void call_go_forward(const PBEngineCalls* c, int64_t tab) { c->go_forward(tab); }
static inline void call_reload(const PBEngineCalls* c, int64_t tab) { c->reload(tab); }
static inline void call_reload_bypassing_cache(const PBEngineCalls* c, int64_t tab) { c->reload_bypassing_cache(tab); }
static inline void call_stop(const PBEngineCalls* c, int64_t tab) { c->stop(tab); }
static inline void call_set_zoom(const PBEngineCalls* c, int64_t tab, double factor) { c->set_zoom(tab, factor); }
static inline double call_get_zoom(const PBEngineCalls* c, int64_t tab) { return c->get_zoom(tab); }
static inline int64_t call_duplicate_tab(const PBEngineCalls* c, int64_t tab) { return c->duplicate_tab(tab); }
static inline void call_edit(const PBEngineCalls* c, int32_t command) { c->edit(command); }
static inline void call_open_settings(const PBEngineCalls* c) { c->open_settings(); }
static inline void call_close_settings(const PBEngineCalls* c) { c->close_settings(); }
static inline void call_send_to_settings(const PBEngineCalls* c, const char* json, size_t length) { c->send_to_settings(json, length); }
static inline void call_set_menu(const PBEngineCalls* c, const char* json, size_t length) { c->set_menu(json, length); }
static inline void call_app_command(const PBEngineCalls* c, int32_t command) { c->app_command(command); }
static inline void call_set_window_layout(const PBEngineCalls* c, const PBWindowLayout* layout, const PBRegion* regions, size_t count) { c->set_window_layout(layout, regions, count); }
static inline void call_focus_page(const PBEngineCalls* c) { c->focus_page(); }
static inline void call_focus_shell(const PBEngineCalls* c) { c->focus_shell(); }
static inline void call_send_to_shell(const PBEngineCalls* c, const char* json, size_t length) { c->send_to_shell(json, length); }
static inline void call_set_hook_subscribed(const PBEngineCalls* c, int32_t hook, int on) { c->set_hook_subscribed(hook, on); }
static inline void call_post_wakeup(const PBEngineCalls* c) { c->post_wakeup(); }
static inline void native_nil_deref(void) { *(volatile int*)0 = 1; }
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

func (e *nativeEngine) RequestWindow(r engine.WindowRequest) {
	C.call_request_window(e.calls, C.int32_t(r))
}

func (e *nativeEngine) LoadURL(tab engine.TabID, url string) {
	c := C.CString(url)
	defer C.free(unsafe.Pointer(c))
	C.call_load_url(e.calls, C.int64_t(tab), c)
}

func (e *nativeEngine) GoBack(tab engine.TabID) { C.call_go_back(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) GoForward(tab engine.TabID) { C.call_go_forward(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) Reload(tab engine.TabID) { C.call_reload(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) ReloadBypassingCache(tab engine.TabID) {
	C.call_reload_bypassing_cache(e.calls, C.int64_t(tab))
}

func (e *nativeEngine) Stop(tab engine.TabID) { C.call_stop(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) Zoom(tab engine.TabID, factor float64) {
	C.call_set_zoom(e.calls, C.int64_t(tab), C.double(factor))
}

func (e *nativeEngine) ZoomOf(tab engine.TabID) float64 {
	return float64(C.call_get_zoom(e.calls, C.int64_t(tab)))
}

func (e *nativeEngine) DuplicateTab(tab engine.TabID) engine.TabID {
	return engine.TabID(C.call_duplicate_tab(e.calls, C.int64_t(tab)))
}

func (e *nativeEngine) Edit(c engine.EditCommand) { C.call_edit(e.calls, C.int32_t(c)) }

func (e *nativeEngine) OpenSettings() { C.call_open_settings(e.calls) }

func (e *nativeEngine) CloseSettings() { C.call_close_settings(e.calls) }

func (e *nativeEngine) SendToSettings(msg []byte) {
	if len(msg) == 0 {
		return
	}
	C.call_send_to_settings(e.calls, (*C.char)(unsafe.Pointer(&msg[0])), C.size_t(len(msg)))
}

func (e *nativeEngine) SetMenu(m engine.Menu) {
	b, err := json.Marshal(m)
	if err != nil {
		log.Printf("menu: %v", err)
		return
	}
	C.call_set_menu(e.calls, (*C.char)(unsafe.Pointer(&b[0])), C.size_t(len(b)))
}

func (e *nativeEngine) AppCommand(c engine.AppCommand) { C.call_app_command(e.calls, C.int32_t(c)) }

func (e *nativeEngine) SetWindowLayout(l engine.WindowLayout) {
	rgb, err := strconv.ParseInt(strings.TrimPrefix(l.FrameColor, "#"), 16, 32)
	if err != nil {
		log.Printf("window colour %q is not #rrggbb", l.FrameColor)
		return
	}
	regions := make([]C.PBRegion, len(l.Regions))
	for i, g := range l.Regions {
		regions[i] = C.PBRegion{x: C.double(g.Rect.X), y: C.double(g.Rect.Y), w: C.double(g.Rect.W), h: C.double(g.Rect.H), area: C.int32_t(g.Area)}
	}
	var first *C.PBRegion
	if len(regions) > 0 {
		first = &regions[0]
	}
	layout := C.PBWindowLayout{
		view_w: C.double(l.ViewW), view_h: C.double(l.ViewH),
		page_x: C.double(l.Page.X), page_y: C.double(l.Page.Y), page_w: C.double(l.Page.W), page_h: C.double(l.Page.H),
		frame_rgb: C.int32_t(rgb), frame_width: C.double(l.FrameWidth),
		theme: C.int32_t(l.Theme),
	}
	C.call_set_window_layout(e.calls, &layout, first, C.size_t(len(regions)))
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

// PBCrash crashes on purpose, as pb_abi.h describes, for the crash log's tests.
//
//export PBCrash
func PBCrash(kind C.int32_t) {
	var p *int
	switch kind {
	case 0:
		*p = 1
	case 1:
		panic("PBCrash on the calling thread")
	case 2:
		go func() { *p = 1 }()
		select {}
	case 3:
		C.native_nil_deref()
	}
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
