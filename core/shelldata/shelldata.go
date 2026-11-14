// Package shelldata keeps what a shell saves for the whole profile, such as Glide's
// Favorites and pinned tabs: one file per shell, shells/<id>.json in the profile folder,
// read when asked for and written when the shell saves. Go never reads what is in it.
// Unlike a shell's own data for a window (package shell), it outlives the window, a
// restart and a switch to another shell and back; a reset to a template keeps it, as it
// keeps the workspaces.
package shelldata

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"projectb/timeline"
)

// Manager keeps the files of one profile folder.
type Manager struct {
	dir string
}

// New returns a manager for the profile folder dir.
func New(dir string) *Manager { return &Manager{dir: filepath.Join(dir, "shells")} }

func (m *Manager) path(shell string) string { return filepath.Join(m.dir, shell+".json") }

// Get is what shell last saved, or null when it saved nothing.
func (m *Manager) Get(shell string) json.RawMessage {
	b, err := os.ReadFile(m.path(shell))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			timeline.Errorf("shelldata: %v", err)
		}
		return json.RawMessage("null")
	}
	if !json.Valid(b) {
		timeline.Errorf("shelldata: %s isn't JSON", m.path(shell))
		return json.RawMessage("null")
	}
	return json.RawMessage(bytes.TrimSpace(b))
}

// Set keeps data, any JSON, as shell's, in place of what it was.
func (m *Manager) Set(shell string, data json.RawMessage) error {
	if !json.Valid(data) {
		return errors.New("shelldata: not JSON")
	}
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return err
	}
	// A new file renamed over the old, so no reader and no crash sees half of one.
	tmp := m.path(shell) + ".tmp"
	if err := os.WriteFile(tmp, append(bytes.TrimSpace(data), '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, m.path(shell))
}
