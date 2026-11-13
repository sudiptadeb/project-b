// Package workspaces owns the profile's workspaces. A workspace is a set of tabs whose
// cookies and site storage are their own, apart from the rest of the profile's: each tab is
// in one for its life, and the engine keeps its storage apart. The default workspace is the
// profile's own storage; it always exists and isn't in the list. The list is one small
// file in the profile, workspaces.json beside prefs.json, read when the app starts and
// written when it changes: each workspace's id, which Go makes and never changes, its name
// and its colour.
package workspaces

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"projectb/timeline"
)

// Workspace is one workspace, as the file holds it.
type Workspace struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"` // "#rrggbb", the dot on its tabs.
}

// file is workspaces.json.
type file struct {
	Workspaces []Workspace `json:"workspaces"`
}

// A colour is "#rrggbb", as a colour input gives it.
var color = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Manager owns the list.
type Manager struct {
	path      string
	list      []Workspace
	onChanged []func()
}

// New returns a manager for the list in the profile folder dir, read here: none when there
// is no file yet.
func New(dir string) *Manager {
	m := &Manager{path: filepath.Join(dir, "workspaces.json")}
	b, err := os.ReadFile(m.path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			timeline.Errorf("workspaces: %v", err)
		}
		return m
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		timeline.Errorf("workspaces: %s: %v", m.path, err)
		return m
	}
	m.list = f.Workspaces
	return m
}

// List returns a copy of the workspaces, in the order they were made.
func (m *Manager) List() []Workspace { return slices.Clone(m.list) }

// OnChanged subscribes to a workspace being made or renamed.
func (m *Manager) OnChanged(f func()) { m.onChanged = append(m.onChanged, f) }

// Create makes a workspace called name, in color, with an id Go makes: eight hex digits.
func (m *Manager) Create(name, c string) (Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Workspace{}, errors.New("a workspace needs a name")
	}
	if !color.MatchString(c) {
		return Workspace{}, errors.New("a workspace's colour is #rrggbb")
	}
	w := Workspace{ID: m.newID(), Name: name, Color: c}
	m.list = append(m.list, w)
	m.changed()
	return w, nil
}

// SetName renames a workspace. Its id stays, and so do its tabs and storage.
func (m *Manager) SetName(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("a workspace needs a name")
	}
	i := slices.IndexFunc(m.list, func(w Workspace) bool { return w.ID == id })
	if i < 0 {
		return errors.New("no workspace " + id)
	}
	m.list[i].Name = name
	m.changed()
	return nil
}

func (m *Manager) newID() string {
	for {
		b := make([]byte, 4)
		rand.Read(b)
		id := hex.EncodeToString(b)
		if !slices.ContainsFunc(m.list, func(w Workspace) bool { return w.ID == id }) {
			return id
		}
	}
}

func (m *Manager) changed() {
	m.save()
	for _, f := range m.onChanged {
		f()
	}
}

func (m *Manager) save() {
	b, _ := json.MarshalIndent(file{Workspaces: m.list}, "", "  ")
	// A new file renamed over the old, so no reader and no crash sees half of one.
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		timeline.Errorf("workspaces: %v", err)
		return
	}
	if err := os.Rename(tmp, m.path); err != nil {
		timeline.Errorf("workspaces: %v", err)
	}
}
