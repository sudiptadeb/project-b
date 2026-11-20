//go:build !android

package instance

import (
	"os"
	"path/filepath"
)

// socketDir is a folder every copy of the app shares. Inside the Flatpak each copy has a /tmp
// of its own, so it is the app's folder in the user's runtime folder, which Flatpak shares
// between every copy of one app: $XDG_RUNTIME_DIR/app/io.github.sudiptadeb.project-b.
// Outside it, the runtime folder itself, or the temporary folder when there is none.
func socketDir() string {
	run := os.Getenv("XDG_RUNTIME_DIR")
	if run == "" {
		return os.TempDir()
	}
	if id := os.Getenv("FLATPAK_ID"); id != "" {
		return filepath.Join(run, "app", id)
	}
	return run
}
