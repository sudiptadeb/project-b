package main

/*
#include <stdlib.h>
#include "pb_abi.h"

static inline void call_install_update(const PBEngineCalls* c, const char* path) { c->install_update(path); }
*/
import "C"

import (
	"path/filepath"
	"unsafe"

	"projectb/update"
)

// version is the release this APK is, from VERSION, set at build time by
// engine/android/build_go.py. The APK's own versionName is the same.
var version string

// updateConfig describes this build to the updater. The APK is the release: a newer one is
// downloaded into the profile and handed to Android's package installer, which refuses one
// signed with another key and asks the user.
func updateConfig(profileDir string) update.Config {
	return update.Config{
		Project:  releaseProject,
		Version:  version,
		Releases: releaseManifest,
		StateDir: filepath.Join(profileDir, "updates"),
		Install: func(apk string) {
			c := C.CString(apk)
			defer C.free(unsafe.Pointer(c))
			C.call_install_update(native.calls, c)
		},
	}
}
