// Package profiles owns the list of profiles and the choice of the one a copy of the app
// starts on. The list is profiles.json in the root, beside the profile folders, and every
// copy reads it and changes it: each change holds a lock on the list for as long as it
// reads and writes, so two copies never lose each other's changes.
package profiles

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"projectb/prefs"
)

// First is the folder of the profile a first start makes.
const First = "default"

// Profile is one entry of the list. Its folder is the root's folder named by its id,
// which Go makes and never changes.
type Profile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Color is a copy of the profile's frame colour, so a profile that isn't running
	// can be shown in it.
	Color string `json:"color"`
	// Template is the name of the template it was made from, kept only to show it.
	Template string    `json:"template"`
	LastUsed time.Time `json:"lastUsed,omitzero"`
}

// file is profiles.json.
type file struct {
	Profiles []Profile `json:"profiles"`
}

// Manager is the list in one root.
type Manager struct {
	root string
}

// New returns the list in root, ~/.project-b unless the copy was started with another.
func New(root string) *Manager { return &Manager{root: root} }

// Root is the folder that holds the profile folders and the list.
func (m *Manager) Root() string { return m.root }

// Dir is a listed profile's folder.
func (m *Manager) Dir(id string) string { return filepath.Join(m.root, id) }

func (m *Manager) path() string { return filepath.Join(m.root, "profiles.json") }

// List reads the list as it is now. Before the first start there is none.
func (m *Manager) List() ([]Profile, error) {
	b, err := os.ReadFile(m.path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s: %v", m.path(), err)
	}
	return f.Profiles, nil
}

// Get is the listed profile id.
func (m *Manager) Get(id string) (Profile, bool, error) {
	list, err := m.List()
	for _, p := range list {
		if p.ID == id {
			return p, true, err
		}
	}
	return Profile{}, false, err
}

// change applies one change to the list, holding its lock from the read to the write.
func (m *Manager) change(f func(list []Profile) []Profile) error {
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(m.root, "profiles.lock"), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	list, err := m.List()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(file{Profiles: f(list)}, "", "  ")
	if err != nil {
		return err
	}
	// A new file renamed over the old, so no reader sees half of one.
	tmp := m.path() + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path())
}

// update changes the listed profile id.
func (m *Manager) update(id string, f func(p *Profile)) error {
	return m.change(func(list []Profile) []Profile {
		for i := range list {
			if list[i].ID == id {
				f(&list[i])
			}
		}
		return list
	})
}

// MarkUsed records that a copy started on the listed profile id now.
func (m *Manager) MarkUsed(id string) error {
	return m.update(id, func(p *Profile) { p.LastUsed = time.Now() })
}

// SetColor keeps the list's copy of the listed profile id's frame colour.
func (m *Manager) SetColor(id, color string) error {
	return m.update(id, func(p *Profile) { p.Color = color })
}

// SetName renames the listed profile id. Its folder keeps its id.
func (m *Manager) SetName(id, name string) error {
	return m.update(id, func(p *Profile) { p.Name = name })
}

// SetTemplate keeps the name of the template the listed profile id was reset to, in the
// place of the one it was made from.
func (m *Manager) SetTemplate(id, template string) error {
	return m.update(id, func(p *Profile) { p.Template = template })
}

// Choice is the profile a copy starts on: its folder, and its id in the list, which is ""
// for a folder named outright. Pick says the copy has no profile yet: it shows the
// picker.
type Choice struct {
	Dir  string
	ID   string
	Pick bool
}

// Named is the folder --user-data-dir names: it is the profile as it is, and never in the
// list.
func Named(dir string) Choice { return Choice{Dir: dir} }

// Choose chooses the profile a copy starts on when no folder is named: the listed profile
// id when it isn't "", else none when the copy was started to pick, else the profile used
// last. A first start, with no list, makes the list holding the first profile and chooses
// it. A list where no profile was used yet leaves the choice to the picker.
func (m *Manager) Choose(id string, pick bool) (Choice, error) {
	if id != "" {
		_, ok, err := m.Get(id)
		if err != nil {
			return Choice{}, err
		}
		if !ok {
			return Choice{}, fmt.Errorf("no profile %q in %s", id, m.path())
		}
		return Choice{Dir: m.Dir(id), ID: id}, nil
	}
	if pick {
		return Choice{Pick: true}, nil
	}
	list, err := m.List()
	if err != nil {
		return Choice{}, err
	}
	if list == nil {
		if err := m.makeFirst(); err != nil {
			return Choice{}, err
		}
		return Choice{Dir: m.Dir(First), ID: First}, nil
	}
	var last Profile
	for _, p := range list {
		if p.LastUsed.After(last.LastUsed) {
			last = p
		}
	}
	if last.ID == "" {
		return Choice{Pick: true}, nil
	}
	return Choice{Dir: m.Dir(last.ID), ID: last.ID}, nil
}

// Create makes a profile called name from the template called template: a folder of its
// own, with an id Go makes, holding the template's values as its prefs, and an entry in
// the list.
func (m *Manager) Create(name, template string) (Profile, error) {
	t, ok := TemplateNamed(template)
	if !ok {
		return Profile{}, fmt.Errorf("no profile template %q", template)
	}
	var made Profile
	var err error
	if e := m.change(func(list []Profile) []Profile {
		id := newID(list)
		if err = t.writePrefs(m.Dir(id)); err != nil {
			return list
		}
		made = Profile{ID: id, Name: name, Color: prefs.New(m.Dir(id)).FrameColor(), Template: t.Name}
		return append(list, made)
	}); e != nil {
		return Profile{}, e
	}
	return made, err
}

// newID is an id no listed profile has: eight hex digits, never a folder's name the
// user chose, so a rename never moves files.
func newID(list []Profile) string {
	for {
		b := make([]byte, 4)
		rand.Read(b)
		id := hex.EncodeToString(b)
		if !slices.ContainsFunc(list, func(p Profile) bool { return p.ID == id }) {
			return id
		}
	}
}

// makeFirst makes the list holding the first profile, from the Personal template. Its
// folder may already hold a profile, kept as it is, from before there was a list.
func (m *Manager) makeFirst() error {
	var err error
	if e := m.change(func(list []Profile) []Profile {
		if list != nil {
			return list
		}
		dir := m.Dir(First)
		if _, statErr := os.Stat(filepath.Join(dir, "prefs.json")); errors.Is(statErr, os.ErrNotExist) {
			t, _ := TemplateNamed(Personal)
			err = t.writePrefs(dir)
		}
		return []Profile{{ID: First, Name: Personal, Color: prefs.New(dir).FrameColor(), Template: Personal}}
	}); e != nil {
		return e
	}
	return err
}
