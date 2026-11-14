// Command lib is the Go core as a c-archive, linked statically into the main binary.
// It implements engine.Engine on the C function table and hands every event from the
// adapter to the content package. Nothing else in core/ touches cgo, so everything
// else still builds and tests with no C++.
package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../../engine/abi
#include <stdlib.h>
#include "pb_abi.h"

static inline int64_t call_create_tab(const PBEngineCalls* c, const char* workspace) { return c->create_tab(workspace); }
static inline void call_set_first_workspace(const PBEngineCalls* c, const char* workspace) { c->set_first_workspace(workspace); }
static inline void call_erase_workspace(const PBEngineCalls* c, const char* workspace) { c->erase_workspace(workspace); }
static inline void call_close_tab(const PBEngineCalls* c, int64_t tab) { c->close_tab(tab); }
static inline void call_open_profile(const PBEngineCalls* c, const char* dir) { c->open_profile(dir); }
static inline void call_set_window_page(const PBEngineCalls* c, int64_t window, const char* page) { c->set_window_page(window, page); }
static inline void call_create_window(const PBEngineCalls* c, int64_t window, int64_t near, const char* page, const PBWindowLayout* layout, const PBRegion* regions, size_t count, const PBVisibleTab* tabs, size_t tab_count) { c->create_window(window, near, page, layout, regions, count, tabs, tab_count); }
static inline void call_set_window_bounds(const PBEngineCalls* c, int64_t window, double x, double y, double w, double h) { c->set_window_bounds(window, x, y, w, h); }
static inline void call_set_window_mode(const PBEngineCalls* c, int64_t window, int32_t mode) { c->set_window_mode(window, mode); }
static inline void call_request_window(const PBEngineCalls* c, int64_t window, int32_t request) { c->request_window(window, request); }
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
static inline void call_write_clipboard(const PBEngineCalls* c, const char* text, size_t length) { c->write_clipboard(text, length); }
static inline void call_open_settings(const PBEngineCalls* c, int64_t window) { c->open_settings(window); }
static inline void call_close_settings(const PBEngineCalls* c, int64_t window) { c->close_settings(window); }
static inline void call_send_to_settings(const PBEngineCalls* c, int64_t window, const char* json, size_t length) { c->send_to_settings(window, json, length); }
static inline void call_set_menu(const PBEngineCalls* c, const char* json, size_t length) { c->set_menu(json, length); }
static inline void call_app_command(const PBEngineCalls* c, int32_t command) { c->app_command(command); }
static inline void call_set_window_layout(const PBEngineCalls* c, int64_t window, const PBWindowLayout* layout, const PBRegion* regions, size_t count, const PBVisibleTab* tabs, size_t tab_count) { c->set_window_layout(window, layout, regions, count, tabs, tab_count); }
static inline void call_focus_tab(const PBEngineCalls* c, int64_t tab) { c->focus_tab(tab); }
static inline void call_focus_shell(const PBEngineCalls* c, int64_t window) { c->focus_shell(window); }
static inline void call_send_to_shell(const PBEngineCalls* c, int64_t window, const char* json, size_t length) { c->send_to_shell(window, json, length); }
static inline void call_set_shell_back(const PBEngineCalls* c, int64_t window, int on) { c->set_shell_back(window, on); }
static inline void call_set_hook_subscribed(const PBEngineCalls* c, int32_t hook, int on) { c->set_hook_subscribed(hook, on); }
static inline void call_post_wakeup(const PBEngineCalls* c) { c->post_wakeup(); }
static inline void native_nil_deref(void) { *(volatile int*)0 = 1; }
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"projectb/app"
	"projectb/content"
	"projectb/engine"
	"projectb/flags"
	"projectb/instance"
	"projectb/profiles"
	"projectb/relaunch"
	"projectb/timeline"
)

// Where releases are published. The project and the URL are the ones in amarit.json at the
// repo root, which tools/release.sh publishes with. updateConfig, beside this file for each
// platform, describes this build to the updater.
const (
	releaseProject  = "project-b"
	releaseManifest = "https://github.com/sudiptadeb/project-b/releases/latest/download/releases.json"
)

// nativeEngine is the real engine: every method calls the C++ adapter.
type nativeEngine struct {
	calls *C.PBEngineCalls

	mu     sync.Mutex
	posted []func()
}

func (e *nativeEngine) CreateTab(workspace string) engine.TabID {
	c := C.CString(workspace)
	defer C.free(unsafe.Pointer(c))
	return engine.TabID(C.call_create_tab(e.calls, c))
}

func (e *nativeEngine) SetFirstWorkspace(workspace string) {
	c := C.CString(workspace)
	defer C.free(unsafe.Pointer(c))
	C.call_set_first_workspace(e.calls, c)
}

func (e *nativeEngine) EraseWorkspace(workspace string) {
	c := C.CString(workspace)
	defer C.free(unsafe.Pointer(c))
	C.call_erase_workspace(e.calls, c)
}

func (e *nativeEngine) CloseTab(tab engine.TabID) { C.call_close_tab(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) RequestWindow(w engine.WindowID, r engine.WindowRequest) {
	C.call_request_window(e.calls, C.int64_t(w), C.int32_t(r))
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

func (e *nativeEngine) WriteClipboard(text string) {
	cs := C.CString(text)
	defer C.free(unsafe.Pointer(cs))
	C.call_write_clipboard(e.calls, cs, C.size_t(len(text)))
}

func (e *nativeEngine) OpenSettings(w engine.WindowID) { C.call_open_settings(e.calls, C.int64_t(w)) }

func (e *nativeEngine) CloseSettings(w engine.WindowID) { C.call_close_settings(e.calls, C.int64_t(w)) }

func (e *nativeEngine) SendToSettings(w engine.WindowID, msg []byte) {
	if len(msg) == 0 {
		return
	}
	C.call_send_to_settings(e.calls, C.int64_t(w), (*C.char)(unsafe.Pointer(&msg[0])), C.size_t(len(msg)))
}

func (e *nativeEngine) SetMenu(m engine.Menu) {
	b, err := json.Marshal(m)
	if err != nil {
		timeline.Errorf("menu: %v", err)
		return
	}
	C.call_set_menu(e.calls, (*C.char)(unsafe.Pointer(&b[0])), C.size_t(len(b)))
}

func (e *nativeEngine) AppCommand(c engine.AppCommand) { C.call_app_command(e.calls, C.int32_t(c)) }

func (e *nativeEngine) SetWindowBounds(w engine.WindowID, r engine.Rect) {
	C.call_set_window_bounds(e.calls, C.int64_t(w), C.double(r.X), C.double(r.Y), C.double(r.W), C.double(r.H))
}

func (e *nativeEngine) SetWindowMode(w engine.WindowID, m engine.WindowMode) {
	C.call_set_window_mode(e.calls, C.int64_t(w), C.int32_t(m))
}

func (e *nativeEngine) CreateWindow(w, near engine.WindowID, page engine.Page, l engine.WindowLayout) {
	p := C.CString(string(page))
	defer C.free(unsafe.Pointer(p))
	e.withLayout(l, func(layout *C.PBWindowLayout, regions *C.PBRegion, count C.size_t, tabs *C.PBVisibleTab, tabCount C.size_t) {
		C.call_create_window(e.calls, C.int64_t(w), C.int64_t(near), p, layout, regions, count, tabs, tabCount)
	})
}

func (e *nativeEngine) SetWindowPage(w engine.WindowID, page engine.Page) {
	p := C.CString(string(page))
	defer C.free(unsafe.Pointer(p))
	C.call_set_window_page(e.calls, C.int64_t(w), p)
}

// OpenProfile makes the profile's context in the engine, with Chromium's files in its
// folder dir. It is called once, as the copy becomes that profile's.
func (e *nativeEngine) OpenProfile(dir string) {
	c := C.CString(dir)
	defer C.free(unsafe.Pointer(c))
	C.call_open_profile(e.calls, c)
}

func (e *nativeEngine) SetWindowLayout(w engine.WindowID, l engine.WindowLayout) {
	e.withLayout(l, func(layout *C.PBWindowLayout, regions *C.PBRegion, count C.size_t, tabs *C.PBVisibleTab, tabCount C.size_t) {
		C.call_set_window_layout(e.calls, C.int64_t(w), layout, regions, count, tabs, tabCount)
	})
}

// withLayout hands call a layout as the C side has it.
func (e *nativeEngine) withLayout(l engine.WindowLayout, call func(*C.PBWindowLayout, *C.PBRegion, C.size_t, *C.PBVisibleTab, C.size_t)) {
	rgb, err := strconv.ParseInt(strings.TrimPrefix(l.FrameColor, "#"), 16, 32)
	if err != nil {
		timeline.Errorf("window colour %q is not #rrggbb", l.FrameColor)
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
	tabs := make([]C.PBVisibleTab, len(l.Tabs))
	for i, v := range l.Tabs {
		tabs[i] = C.PBVisibleTab{tab: C.int64_t(v.Tab), x: C.double(v.Rect.X), y: C.double(v.Rect.Y), w: C.double(v.Rect.W), h: C.double(v.Rect.H)}
	}
	var firstTab *C.PBVisibleTab
	if len(tabs) > 0 {
		firstTab = &tabs[0]
	}
	layout := C.PBWindowLayout{
		view_w: C.double(l.ViewW), view_h: C.double(l.ViewH),
		frame_rgb: C.int32_t(rgb), frame_width: C.double(l.FrameWidth),
		theme: C.int32_t(l.Theme),
	}
	call(&layout, first, C.size_t(len(regions)), firstTab, C.size_t(len(tabs)))
}

func (e *nativeEngine) FocusTab(tab engine.TabID) { C.call_focus_tab(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) FocusShell(w engine.WindowID) { C.call_focus_shell(e.calls, C.int64_t(w)) }

func (e *nativeEngine) SetShellBack(w engine.WindowID, on bool) {
	n := 0
	if on {
		n = 1
	}
	C.call_set_shell_back(e.calls, C.int64_t(w), C.int(n))
}

func (e *nativeEngine) SendToShell(w engine.WindowID, msg []byte) {
	if len(msg) == 0 {
		return
	}
	C.call_send_to_shell(e.calls, C.int64_t(w), (*C.char)(unsafe.Pointer(&msg[0])), C.size_t(len(msg)))
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
	native  *nativeEngine
	hooks   *content.Hooks
	list    *profiles.Manager
	blocked []string
	// The profile this copy runs on, the folder it owns, and its managers. A copy started
	// to pick has none of them.
	profile profiles.Choice
	owner   *instance.Owner
	theApp  *app.App
)

// PBClaimProfile is the first call, before anything is written to the profile. It chooses
// the profile from the command line's --root, --profile and --user-data-dir, each "" when
// not given, and --picker, and returns the folder this copy keeps its files in, once this
// copy owns it: the profile's, or the root when the copy picks its profile later. It
// returns nil when another copy owns that folder and has been handed urls, one per line,
// so this copy quits.
//
//export PBClaimProfile
func PBClaimProfile(root, id, userDataDir *C.char, picker C.int, urls *C.char) *C.char {
	var handed []string
	if s := C.GoString(urls); s != "" {
		handed = strings.Split(s, "\n")
	}
	list = profiles.New(rootOf(C.GoString(root)))
	var err error
	if dir := C.GoString(userDataDir); dir != "" {
		profile = profiles.Named(dir)
	} else if profile, err = list.Choose(C.GoString(id), picker != 0); err != nil {
		fmt.Fprintf(os.Stderr, "the profile: %v\n", err)
		os.Exit(1)
	}
	// A picker copy owns the root, so a copy relaunching for an update finds it, and a
	// second picker copy hands over to it.
	dir := profile.Dir
	if profile.Pick {
		dir = list.Root()
	}
	o, err := instance.Claim(dir, handed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "the profile folder: %v\n", err)
		os.Exit(1)
	}
	if o == nil {
		return nil
	}
	owner = o
	return C.CString(dir)
}

// rootOf is the folder that holds the profiles: the one --root names, else ~/.project-b.
func rootOf(root string) string {
	if root != "" {
		return root
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".project-b")
}

// PBStart starts what a copy has before and apart from its profile, the engine and the
// list, then the profile's managers, or the picker when the copy has no profile yet.
//
//export PBStart
func PBStart(calls *C.PBEngineCalls, blockedHosts, hookTable *C.char) {
	timeline.Mark("go.starting")
	if table := C.GoString(hookTable); table != content.TableHash {
		timeline.Errorf("the library's hook table is %s, Go's is %s: regenerate with tools/gen_hooks.py and rebuild", table, content.TableHash)
		timeline.Close()
		os.Exit(1)
	}
	// The table is C memory that lives as long as the process.
	native = &nativeEngine{calls: calls}
	hooks = content.New(func(h content.Hook) { C.call_set_hook_subscribed(native.calls, C.int32_t(h), 1) })
	if s := C.GoString(blockedHosts); s != "" {
		blocked = strings.Split(s, ",")
	}
	if profile.Pick {
		picker := app.StartPicker(native, list, func(id string) {
			relaunch.Copy("--root="+list.Root(), "--profile="+id)
		})
		content.WirePicker(hooks, picker)
		owner.Serve(func([]string) { native.Post(picker.Handover) }, quit)
	} else {
		startProfile(profile)
	}
	timeline.Mark("go.started")
}

// startProfile makes this copy the profile c's, whose folder it owns: the profile's
// context in the engine, then its managers.
func startProfile(c profiles.Choice) {
	native.OpenProfile(c.Dir)
	if c.ID != "" {
		if err := list.MarkUsed(c.ID); err != nil {
			timeline.Errorf("profiles: %v", err)
		}
	}
	home, _ := os.UserHomeDir()
	theApp = app.Start(native, app.Config{
		ProfileDir:   c.Dir,
		Profiles:     list,
		Profile:      c.ID,
		BlockedHosts: blocked,
		Update:       updateConfig(c),
		CrashReports: filepath.Join(home, "Library/Logs/DiagnosticReports"),
	})
	content.Wire(hooks, theApp)
	owner.Serve(func(urls []string) { native.Post(func() { theApp.Handover(urls) }) }, quit)
}

// quit ends this copy, as another copy relaunching for an update asks.
func quit() { native.Post(func() { native.AppCommand(engine.AppQuit) }) }

//export PBStop
func PBStop() {
	// A picker copy has no profile to stop.
	if theApp != nil {
		theApp.Stop()
	}
	owner.Close()
	timeline.Close()
}

// The timeline's kinds as they cross, PBTimelineKind in pb_abi.h.
var timelineKinds = []timeline.Kind{timeline.KindMark, timeline.KindBegin, timeline.KindEnd, timeline.KindInfo, timeline.KindWarn, timeline.KindError}

// PBTimelineStart starts the timeline: its file goes in the folder this copy keeps its
// files in, and time zero is processAge nanoseconds ago.
//
//export PBTimelineStart
func PBTimelineStart(profileDir *C.char, processAge C.int64_t) {
	timeline.Start(C.GoString(profileDir), time.Duration(processAge), printer())
}

// PBTimelineNow is the time on the timeline, in nanoseconds since process start. The
// adapter reads it once, to place its own clock on ours.
//
//export PBTimelineNow
func PBTimelineNow() C.int64_t { return C.int64_t(timeline.NowNanos()) }

// PBTimelineRecord records one entry from the adapter, on any thread. t is its time, or
// -1 for now.
//
//export PBTimelineRecord
func PBTimelineRecord(kind C.int32_t, t C.int64_t, name *C.char, nameLen C.size_t, text *C.char, textLen C.size_t) {
	timeline.Record(timelineKinds[kind], timeline.SrcEngine, C.GoStringN(name, C.int(nameLen)), C.GoStringN(text, C.int(textLen)), int64(t))
}

// PBTimelinePageOrigin says when one of our pages' own clock started: see
// timeline.PageOrigin.
//
//export PBTimelinePageOrigin
func PBTimelinePageOrigin(page *C.char, t C.int64_t) {
	timeline.PageOrigin(C.GoString(page), int64(t))
}

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

// PBFree frees a string Go returned, for the engine, whose own free may be another
// allocator's.
//
//export PBFree
func PBFree(p unsafe.Pointer) { C.free(p) }

// PBVersion is what --version prints first: the release this build is, or "development
// build" for one that isn't a release. The caller frees the string.
//
//export PBVersion
func PBVersion() *C.char {
	v := appVersion()
	if v == "" {
		v = "development build"
	}
	return C.CString("Project B " + v)
}

// PBLaunchSwitches is the switches the flags saved in the profile ask for, one per line,
// for the adapter to add to the command line before content starts. The caller frees it.
//
//export PBLaunchSwitches
func PBLaunchSwitches(profileDir *C.char) *C.char {
	return C.CString(strings.Join(flags.New(C.GoString(profileDir)).Switches(), "\n"))
}

func main() {}
