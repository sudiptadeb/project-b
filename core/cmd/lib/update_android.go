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

	"projectb/profiles"
	"projectb/update"
)

// version is the release this APK is, from VERSION, set at build time by
// engine/android/build_go.py. The APK's own versionName is the same.
var version string

// appVersion is the release this APK is.
func appVersion() string { return version }

// updateConfig describes this build to the updater. The APK is the release: a newer one is
// downloaded into the profile and handed to Android's package installer, which refuses one
// signed with another key and asks the user.
func updateConfig(c profiles.Choice) update.Config {
	return update.Config{
		Project:  releaseProject,
		Version:  version,
		Releases: releaseManifest,
		StateDir: filepath.Join(c.Dir, "updates"),
		Install: func(apk string) {
			// The installer ends this process to replace the app, which isn't a crash:
			// the run counts as stopped cleanly from here. A user who cancels goes on
			// without crash.log, so a crash later in that run goes unreported.
			if theApp != nil {
				theApp.Crash.Stop()
			}
			c := C.CString(apk)
			defer C.free(unsafe.Pointer(c))
			C.call_install_update(native.calls, c)
		},
	}
}
