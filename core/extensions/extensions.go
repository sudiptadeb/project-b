//go:build extensions

// Package extensions owns what the browser decides about extensions: which store it
// installs from, which buttons are pinned to the toolbar, and what each button shows.
// The engine runs the extensions and remembers which are installed; this package asks it.
package extensions

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"slices"

	"projectb/engine"
)

// saved is what this package keeps in the profile, in extensions.json.
type saved struct {
	StoreURL string   `json:"storeUrl"`
	Pinned   []string `json:"pinned"`
}

// Button is one extension's toolbar button as a tab shows it.
type Button struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Title          string `json:"title"`
	Icon           string `json:"icon"`
	Badge          string `json:"badge"`
	BadgeColor     string `json:"badgeColor"`
	BadgeTextColor string `json:"badgeTextColor"`
	Enabled        bool   `json:"enabled"`
	Pinned         bool   `json:"pinned"`
}

// Item is one extension the store offers.
type Item struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Size        int64  `json:"size"`
	Crx         string `json:"crx"`
	Icon        string `json:"icon"`
	// Phase is "" or, while an install runs, "downloading" or "installing".
	Phase string `json:"phase"`
}

// View is everything the extension manager in settings shows.
type View struct {
	StoreURL     string                `json:"storeUrl"`
	Installed    []engine.Extension    `json:"installed"`
	Pinned       []string              `json:"pinned"`
	Catalogue    []Item                `json:"catalogue"`
	CatalogueErr string                `json:"catalogueError"`
	Prompt       *engine.InstallPrompt `json:"prompt"`
	// Error is why the last install or load failed, until the next one starts.
	Error string `json:"error"`
}

// Manager owns the store setting, the pins, the buttons and the installs under way.
type Manager struct {
	eng  engine.Extensions
	post func(func())
	dir  string
	path string

	saved   saved
	list    []engine.Extension
	actions map[string]*action

	catalogue    []Item
	catalogueErr string
	// phases is the install under way for a store item, by its id; downloads is which
	// item a downloaded package belongs to, by the package's path.
	phases    map[string]string
	downloads map[string]string
	prompt    *engine.InstallPrompt
	failure   string
	choosing  engine.PathKind
	// popup is the extension whose popup is open, or "".
	popup string

	activeTab func() engine.TabID
	onChanged []func()
	onClicked func(id string)
}

// New returns a manager with what the profile in dir saved. post runs a function on the
// main thread; activeTab says which tab is showing.
func New(eng engine.Extensions, dir string, post func(func()), activeTab func() engine.TabID) *Manager {
	m := &Manager{
		eng: eng, post: post, dir: dir, path: filepath.Join(dir, "extensions.json"),
		actions: map[string]*action{}, phases: map[string]string{}, downloads: map[string]string{},
		activeTab: activeTab,
	}
	if b, err := os.ReadFile(m.path); err == nil {
		if err := json.Unmarshal(b, &m.saved); err != nil {
			log.Printf("extensions: %s: %v", m.path, err)
		}
	}
	m.list = eng.Extensions()
	return m
}

// OnChanged subscribes to any change in what the shell or the settings page shows.
func (m *Manager) OnChanged(f func()) { m.onChanged = append(m.onChanged, f) }

// OnClicked subscribes to a click on a button that has no popup to open.
func (m *Manager) OnClicked(f func(id string)) { m.onClicked = f }

func (m *Manager) changed() {
	for _, f := range m.onChanged {
		f()
	}
}

func (m *Manager) save() {
	b, _ := json.Marshal(m.saved)
	if err := os.WriteFile(m.path, append(b, '\n'), 0o600); err != nil {
		log.Printf("extensions: %v", err)
	}
}

// Installed returns what the engine has installed.
func (m *Manager) Installed() []engine.Extension { return m.list }

// Get returns one installed extension.
func (m *Manager) Get(id string) (engine.Extension, bool) {
	for _, e := range m.list {
		if e.ID == id {
			return e, true
		}
	}
	return engine.Extension{}, false
}

// View returns what the extension manager in settings shows.
func (m *Manager) View() View {
	items := make([]Item, len(m.catalogue))
	for i, it := range m.catalogue {
		it.Phase = m.phases[it.ID]
		items[i] = it
	}
	// Lists go to the page as lists, never null.
	return View{
		StoreURL: m.saved.StoreURL, Installed: append([]engine.Extension{}, m.list...),
		Pinned:    append([]string{}, m.saved.Pinned...),
		Catalogue: items, CatalogueErr: m.catalogueErr, Prompt: m.prompt, Error: m.failure,
	}
}

// HandleChanged takes the engine's list again, and forgets the pins and button state of
// extensions that are gone.
func (m *Manager) HandleChanged() {
	m.list = m.eng.Extensions()
	has := func(id string) bool { _, ok := m.Get(id); return ok }
	if kept := slices.DeleteFunc(slices.Clone(m.saved.Pinned), func(id string) bool { return !has(id) }); len(kept) != len(m.saved.Pinned) {
		m.saved.Pinned = kept
		m.save()
	}
	for id := range m.actions {
		if !has(id) {
			delete(m.actions, id)
		}
	}
	m.changed()
}

// Remove uninstalls an extension.
func (m *Manager) Remove(id string) { m.eng.UninstallExtension(id) }

// SetEnabled turns an extension on or off.
func (m *Manager) SetEnabled(id string, on bool) { m.eng.SetExtensionEnabled(id, on) }

// Pinned says whether an extension's button is on the toolbar.
func (m *Manager) Pinned(id string) bool { return slices.Contains(m.saved.Pinned, id) }

// Pin puts an extension's button on the toolbar, after the ones already there, or takes
// it off.
func (m *Manager) Pin(id string, on bool) {
	if _, ok := m.Get(id); !ok || on == m.Pinned(id) {
		return
	}
	if on {
		m.saved.Pinned = append(m.saved.Pinned, id)
	} else {
		m.saved.Pinned = slices.DeleteFunc(m.saved.Pinned, func(p string) bool { return p == id })
	}
	m.save()
	m.changed()
}

// LoadUnpacked asks the user for a folder and loads the extension in it.
func (m *Manager) LoadUnpacked() {
	m.choosing = engine.PathFolder
	m.eng.ChoosePath(engine.PathFolder)
}

// InstallPackage asks the user for a package file and installs it.
func (m *Manager) InstallPackage() {
	m.choosing = engine.PathPackage
	m.eng.ChoosePath(engine.PathPackage)
}

// HandlePathChosen takes the folder or package the user picked.
func (m *Manager) HandlePathChosen(path string) {
	kind := m.choosing
	m.choosing = 0
	if path == "" {
		return
	}
	m.failure = ""
	if kind == engine.PathFolder {
		m.eng.LoadUnpackedExtension(path)
	} else {
		m.eng.InstallExtension(path)
	}
	m.changed()
}

// HandlePrompt shows the engine's question in the settings page.
func (m *Manager) HandlePrompt(p engine.InstallPrompt) {
	m.prompt = &p
	m.changed()
}

// AnswerPrompt gives the engine the user's answer.
func (m *Manager) AnswerPrompt(accept bool) {
	if m.prompt == nil {
		return
	}
	id := m.prompt.ID
	m.prompt = nil
	m.eng.AnswerInstallPrompt(id, accept)
	m.changed()
}

// HandleInstallDone clears the install's progress, removes the package this manager
// downloaded for it, and keeps the reason when it failed.
func (m *Manager) HandleInstallDone(r engine.InstallResult) {
	if item, ok := m.downloads[r.Path]; ok {
		delete(m.downloads, r.Path)
		delete(m.phases, item)
		os.Remove(r.Path)
	}
	m.failure = r.Error
	m.changed()
}
