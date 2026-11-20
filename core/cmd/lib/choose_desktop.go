//go:build darwin || (linux && !android)

package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../../engine/abi
#include <stdlib.h>
#include "pb_abi.h"

static inline void call_choose_folder(const PBEngineCalls* c, int64_t window, const char* start) { c->choose_folder(window, start); }
*/
import "C"

import (
	"unsafe"

	"projectb/engine"
)

// ChooseFolder asks for a folder in Chromium's file dialog on the window, opening at start;
// PB_EVENT_FOLDER_CHOSEN says which.
func (e *nativeEngine) ChooseFolder(w engine.WindowID, start string) {
	cs := C.CString(start)
	defer C.free(unsafe.Pointer(cs))
	C.call_choose_folder(e.calls, C.int64_t(w), cs)
}
