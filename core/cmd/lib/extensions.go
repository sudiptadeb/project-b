//go:build extensions

package main

/*
#include <stdlib.h>
#include "pb_abi.h"
#include "pb_extensions_abi.h"

static inline void ext_install(const PBEngineCalls* c, const char* path) { c->extensions()->install(path); }
static inline void ext_answer_install_prompt(const PBEngineCalls* c, int64_t prompt, int accept) { c->extensions()->answer_install_prompt(prompt, accept); }
static inline void ext_load_unpacked(const PBEngineCalls* c, const char* dir) { c->extensions()->load_unpacked(dir); }
static inline void ext_uninstall(const PBEngineCalls* c, const char* id) { c->extensions()->uninstall(id); }
static inline void ext_set_enabled(const PBEngineCalls* c, const char* id, int on) { c->extensions()->set_enabled(id, on); }
static inline char* ext_list(const PBEngineCalls* c) { return c->extensions()->list(); }
static inline void ext_dispatch_event(const PBEngineCalls* c, const char* id, const char* event, const char* args) { c->extensions()->dispatch_event(id, event, args); }
static inline void ext_open_popup(const PBEngineCalls* c, int64_t window, const char* id, const char* url, double x, double y, double w, double h) { c->extensions()->open_popup(window, id, url, x, y, w, h); }
static inline void ext_close_popup(const PBEngineCalls* c) { c->extensions()->close_popup(); }
static inline void ext_choose_path(const PBEngineCalls* c, int64_t window, int32_t kind) { c->extensions()->choose_path(window, kind); }
*/
import "C"

import (
	"encoding/json"
	"unsafe"

	"projectb/content"
	"projectb/engine"
	"projectb/timeline"
)

// The real engine's extension calls: each goes to the C++ adapter.

func cstr(s string) (*C.char, func()) {
	c := C.CString(s)
	return c, func() { C.free(unsafe.Pointer(c)) }
}

// The adapter's extension events reach the listener as soon as the app names it, which
// it does while it starts.
func (e *nativeEngine) SetExtensionsListener(l engine.ExtensionsListener) {
	content.WireExtensions(hooks, l)
}

func (e *nativeEngine) InstallExtension(path string) {
	c, free := cstr(path)
	defer free()
	C.ext_install(e.calls, c)
}

func (e *nativeEngine) AnswerInstallPrompt(prompt int64, accept bool) {
	on := C.int(0)
	if accept {
		on = 1
	}
	C.ext_answer_install_prompt(e.calls, C.int64_t(prompt), on)
}

func (e *nativeEngine) LoadUnpackedExtension(dir string) {
	c, free := cstr(dir)
	defer free()
	C.ext_load_unpacked(e.calls, c)
}

func (e *nativeEngine) UninstallExtension(id string) {
	c, free := cstr(id)
	defer free()
	C.ext_uninstall(e.calls, c)
}

func (e *nativeEngine) SetExtensionEnabled(id string, on bool) {
	c, free := cstr(id)
	defer free()
	v := C.int(0)
	if on {
		v = 1
	}
	C.ext_set_enabled(e.calls, c, v)
}

func (e *nativeEngine) Extensions() []engine.Extension {
	c := C.ext_list(e.calls)
	defer C.free(unsafe.Pointer(c))
	var list []engine.Extension
	if err := json.Unmarshal([]byte(C.GoString(c)), &list); err != nil {
		timeline.Warnf("extensions: bad list from the engine: %v", err)
	}
	return list
}

func (e *nativeEngine) DispatchExtensionEvent(id, event, args string) {
	cid, free1 := cstr(id)
	defer free1()
	cevent, free2 := cstr(event)
	defer free2()
	cargs, free3 := cstr(args)
	defer free3()
	C.ext_dispatch_event(e.calls, cid, cevent, cargs)
}

func (e *nativeEngine) OpenExtensionPopup(w engine.WindowID, id, url string, anchor engine.Rect) {
	cid, free1 := cstr(id)
	defer free1()
	curl, free2 := cstr(url)
	defer free2()
	C.ext_open_popup(e.calls, C.int64_t(w), cid, curl, C.double(anchor.X), C.double(anchor.Y), C.double(anchor.W), C.double(anchor.H))
}

func (e *nativeEngine) CloseExtensionPopup() { C.ext_close_popup(e.calls) }

func (e *nativeEngine) ChoosePath(w engine.WindowID, kind engine.PathKind) {
	C.ext_choose_path(e.calls, C.int64_t(w), C.int32_t(kind))
}
