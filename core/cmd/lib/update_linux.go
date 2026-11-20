//go:build !android

package main

import (
	"path/filepath"

	"projectb/profiles"
	"projectb/update"
)

// version is the release this build is, from VERSION, set at build time by
// engine/build_go.py.
var version string

// appVersion is the release this build is.
func appVersion() string { return version }

// updateConfig describes this build to the updater: no installer, as Flatpak installs updates
// (docs/research/linux.md, 5; amarit's Flatpak source comes in step 5). Its folder still holds
// the automatic-updates setting.
func updateConfig(c profiles.Choice) update.Config {
	return update.Config{StateDir: filepath.Join(c.Dir, "updates"), Version: version}
}
