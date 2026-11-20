//go:build !android

package main

import (
	"os"

	"projectb/flags"
)

// contentSwitches are what Linux adds to content's command line, in dir, the folder this copy
// owns. First the display: content reads only --ozone-platform, not Chrome's
// --ozone-platform-hint, so Go picks it: Wayland when the session has a Wayland display, X11
// when it has an X one, else headless, as in a container with no desktop. Then, as on the Mac,
// --disable-in-process-stack-traces, since Go's runtime owns the crash signals (see
// switches_darwin.go), and the flags saved in the profile.
func contentSwitches(dir string) []string {
	return append([]string{"--ozone-platform=" + ozonePlatform(), "--disable-in-process-stack-traces"},
		flags.New(dir).Switches()...)
}

// ozonePlatform is the display content draws on: wayland, x11 or headless.
func ozonePlatform() string {
	switch {
	case os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland":
		return "wayland"
	case os.Getenv("DISPLAY") != "":
		return "x11"
	}
	return "headless"
}
