// Package workspaces owns the profile's workspaces. A workspace is a set of tabs whose
// cookies and site storage are their own: each tab is in one for its life, and the engine
// keeps each one's storage apart. A profile always has at least one. A new profile starts
// with Main, whose storage is the profile's own, the one the engine makes a tab in for "",
// so Main's id is "". Every workspace made later has storage of its own. None is special
// to the user: each can be renamed, recoloured, moved and deleted, but the last one can't
// be deleted. The list is one small file in the profile, workspaces.json beside
// prefs.json, read when the app starts and written when it changes: each workspace's id,
// which Go makes and never changes, its name and its colour, in the user's order.
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
	Version    int         `json:"version"`
	Workspaces []Workspace `json:"workspaces"`
}

// version 2 lists Main. A file from before has no version.
const version = 2

// mainWorkspace is the workspace a new profile starts with, in the first of the colours
// the settings page offers.
var mainWorkspace = Workspace{ID: "", Name: "Main", Color: "#3470d0"}

// A colour is "#rrggbb", as a colour input gives it.
var color = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Manager owns the list.
type Manager struct {
	path      string
	list      []Workspace
	onChanged []func()
}

// New returns a manager for the list in the profile folder dir, read here.
func New(dir string) *Manager {
	m := &Manager{path: filepath.Join(dir, "workspaces.json")}
	var f file
	if b, err := os.ReadFile(m.path); err == nil {
		if err := json.Unmarshal(b, &f); err != nil {
			timeline.Errorf("workspaces: %s: %v", m.path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		timeline.Errorf("workspaces: %v", err)
	}
	m.list = f.Workspaces
	// The one upgrade step. A new profile has no file, and one from before version 2 has
	// a list without Main: either gets Main at the top, so the tabs and logins in the
	// profile's own storage are in a listed workspace. The file says version 2 from the
	// next change on, so a Main deleted after that never comes back.
	if f.Version < version {
		m.list = slices.Insert(m.list, 0, mainWorkspace)
	}
	return m
}

// List returns a copy of the workspaces, in the user's order: Main first in a new profile,
// then the order they were made.
func (m *Manager) List() []Workspace { return slices.Clone(m.list) }

// First is the id of the first workspace in the list, where a tab opens when nothing
// picks a workspace for it.
func (m *Manager) First() string { return m.list[0].ID }

// OnChanged subscribes to a workspace being made, renamed, recoloured, moved or deleted.
func (m *Manager) OnChanged(f func()) { m.onChanged = append(m.onChanged, f) }

// Create makes a workspace called name, in color, with an id Go makes: eight hex digits.
// Names are unique in the list, whatever their case.
func (m *Manager) Create(name, c string) (Workspace, error) {
	name = strings.TrimSpace(name)
	if err := m.checkName(-1, name); err != nil {
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
	if err := m.checkName(i, name); err != nil {
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

// Delete takes a workspace out of the list. The last one stays, as a profile always has
// one. Closing its tabs and erasing its storage are the caller's.
func (m *Manager) Delete(id string) error {
	i, err := m.index(id)
	if err != nil {
		return err
	}
	if len(m.list) == 1 {
		return errors.New("a profile keeps one workspace at least")
	}
	m.list = slices.Delete(m.list, i, i+1)
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

// checkName refuses an empty name, and one a workspace other than the one at index i has,
// in any case. A new workspace is at none, -1.
func (m *Manager) checkName(i int, name string) error {
	if name == "" {
		return errors.New("a workspace needs a name")
	}
	for j, w := range m.list {
		if j != i && strings.EqualFold(w.Name, name) {
			return errors.New("there is already a workspace called " + name)
		}
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
	b, _ := json.MarshalIndent(file{Version: version, Workspaces: m.list}, "", "  ")
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
