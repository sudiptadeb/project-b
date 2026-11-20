//go:build !android

package main

import (
	"projectb/keys"
	"projectb/profiles"
)

// deviceKeyFor is none on Linux for now: every profile's key is wrapped by its folder's own
// folder.key, as for a folder named outright. That is for development and tests only. Step 4's
// Secret portal holds the device key instead, before any Flatpak reaches a user
// (docs/research/linux.md, step 2's plan).
func deviceKeyFor(profiles.Choice) keys.Device { return nil }
