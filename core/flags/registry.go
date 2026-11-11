package flags

import (
	_ "embed"
)

// registry.json is the one place a flag is defined. Go embeds it, GN reads it
// (engine/flags.gni) and vite reads it (shell/vite.config.js). docs/flags.md says how to
// add one.
//
//go:embed registry.json
var registry []byte

// Flag ids, for the code that reads a flag.
const (
	Extensions      = "extensions"
	RemoteDebugging = "remoteDebugging"
	Features        = "features"
)

// All is every flag, in the order the settings page lists them.
var All = mustParse(registry)
