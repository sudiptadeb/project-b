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

// List returns a copy of the workspaces, in the user's order, which starts as the order
// they were made: none is an empty list, never nil, so it goes to a page as [].
func (m *Manager) List() []Workspace { return append([]Workspace{}, m.list...) }

// OnChanged subscribes to a workspace being made, renamed, recoloured or moved.
func (m *Manager) OnChanged(f func()) { m.onChanged = append(m.onChanged, f) }

// Create makes a workspace called name, in color, with an id Go makes: eight hex digits.
// Names are unique in the list, whatever their case.
func (m *Manager) Create(name, c string) (Workspace, error) {
	name = strings.TrimSpace(name)
	if err := m.checkName("", name); err != nil {
		return Workspace{}, err
	}
	if !color.MatchString(c) {
		return Workspace{}, errors.New("a workspace's colour is #rrggbb")
	}
	w := Workspace{ID: m.newID(), Name: name, Color: c}
	m.list = append(m.list, w)
	m.changed()
	return w, nil
}

// SetName renames a workspace. Its id stays, and so do its tabs and storage. The new name
// may differ from its own only in case, but not match another's.
func (m *Manager) SetName(id, name string) error {
	name = strings.TrimSpace(name)
	i, err := m.index(id)
	if err != nil {
		return err
	}
	if err := m.checkName(id, name); err != nil {
		return err
	}
	if m.list[i].Name == name {
		return nil
	}
	m.list[i].Name = name
	m.changed()
	return nil
}

// SetColor changes a workspace's colour, the dot on its tabs.
func (m *Manager) SetColor(id, c string) error {
	i, err := m.index(id)
	if err != nil {
		return err
	}
	if !color.MatchString(c) {
		return errors.New("a workspace's colour is #rrggbb")
	}
	if m.list[i].Color == c {
		return nil
	}
	m.list[i].Color = c
	m.changed()
	return nil
}

// Move moves a workspace one place up the list (by -1) or down it (by +1). The list's
// order is the order the Tab menu and settings show.
func (m *Manager) Move(id string, by int) error {
	i, err := m.index(id)
	if err != nil {
		return err
	}
	if by != -1 && by != 1 {
		return errors.New("a workspace moves one place at a time")
	}
	j := i + by
	if j < 0 || j >= len(m.list) {
		return errors.New("the workspace is already at that end")
	}
	m.list[i], m.list[j] = m.list[j], m.list[i]
	m.changed()
	return nil
}

func (m *Manager) index(id string) (int, error) {
	i := slices.IndexFunc(m.list, func(w Workspace) bool { return w.ID == id })
	if i < 0 {
		return -1, errors.New("no workspace " + id)
	}
	return i, nil
}

// checkName refuses an empty name, and one another workspace than id has, in any case.
func (m *Manager) checkName(id, name string) error {
	if name == "" {
		return errors.New("a workspace needs a name")
	}
	if slices.ContainsFunc(m.list, func(w Workspace) bool { return w.ID != id && strings.EqualFold(w.Name, name) }) {
		return errors.New("there is already a workspace called " + name)
	}
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
