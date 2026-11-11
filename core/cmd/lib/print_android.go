package main

/*
#cgo LDFLAGS: -llog
#include <android/log.h>
#include <stdlib.h>

static inline void log_line(const char* line) { __android_log_write(ANDROID_LOG_INFO, "ProjectB", line); }
*/
import "C"

import (
	"io"
	"unsafe"
)

// printer is where the timeline prints. Android sends stderr nowhere, so it is the
// system's log: adb logcat -s ProjectB.
func printer() io.Writer { return logcat{} }

type logcat struct{}

func (logcat) Write(p []byte) (int, error) {
	c := C.CString(string(p))
	C.log_line(c)
	C.free(unsafe.Pointer(c))
	return len(p), nil
}
