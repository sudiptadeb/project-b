// Package downloads keeps a profile's downloads (docs/daily-driver.md): where each goes,
// what each is now, and the finished ones in downloads.json in the profile folder, the
// last 100, so the list is there after a restart. The engine keeps no list; it asks
// where a download goes and reports each change, and acts on one only while it runs.
package downloads

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"projectb/engine"
	"projectb/timeline"
)

// kept is how many finished downloads the list keeps.
const kept = 100

// Item is one download as the shells see it. ID is Go's, the same across restarts.
type Item struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Path     string `json:"path"`
	State    string `json:"state"` // progress, done, cancelled or failed
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
	Paused   bool   `json:"paused"`

	// engine is the engine's number for it while this run has it.
	engine engine.DownloadID
}

var states = map[engine.DownloadState]string{
	engine.DownloadProgress:  "progress",
	engine.DownloadDone:      "done",
	engine.DownloadCancelled: "cancelled",
	engine.DownloadFailed:    "failed",
}

// file is downloads.json.
type file struct {
	Version   int    `json:"version"`
	Next      int    `json:"next"`
	Downloads []Item `json:"downloads"`
}

// Manager keeps one profile's downloads, newest first.
type Manager struct {
	e         engine.Engine
	file      string
	dir       string
	list      []*Item
	next      int
	byEngine  map[engine.DownloadID]*Item
	onChanged []func()
}

// New returns a manager for the profile folder profile, saving files into dir.
func New(e engine.Engine, profile, dir string) *Manager {
	return &Manager{e: e, file: filepath.Join(profile, "downloads.json"), dir: dir, next: 1, byEngine: map[engine.DownloadID]*Item{}}
}

// Load reads the list kept from the last run.
func (m *Manager) Load() {
	b, err := os.ReadFile(m.file)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			timeline.Errorf("downloads: %v", err)
		}
		return
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		timeline.Errorf("downloads: %s: %v", m.file, err)
		return
	}
	for i := range f.Downloads {
		m.list = append(m.list, &f.Downloads[i])
	}
	m.next = max(f.Next, 1)
}

// OnChanged subscribes to any change to the list.
func (m *Manager) OnChanged(f func()) { m.onChanged = append(m.onChanged, f) }

func (m *Manager) changed() {
	for _, f := range m.onChanged {
		f()
	}
}

// List is the list now, newest first.
func (m *Manager) List() []Item {
	out := make([]Item, len(m.list))
	for i, it := range m.list {
		out[i] = *it
	}
	return out
}

// Target is where a download goes: the folder, under the name the engine suggests,
// numbered when that is taken, through the Save panel when it asked for one.
func (m *Manager) Target(r engine.DownloadRequest) engine.DownloadTarget {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		timeline.Errorf("downloads: %v", err)
		return engine.DownloadTarget{}
	}
	return engine.DownloadTarget{Path: m.free(cleanName(r.Name)), Prompt: r.SaveAs}
}

// cleanName is a file name with no folder in it.
func cleanName(name string) string {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == ".." || name == "/" {
		return "download"
	}
	return name
}

// free is a path in the folder for name that no file and no running download has:
// name, then name (1), name (2) and so on, before its extension.
func (m *Manager) free(name string) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 0; ; n++ {
		try := name
		if n > 0 {
			try = fmt.Sprintf("%s (%d)%s", stem, n, ext)
		}
		p := filepath.Join(m.dir, try)
		if _, err := os.Stat(p); err == nil {
			continue
		}
		if m.running(p) {
			continue
		}
		return p
	}
}

func (m *Manager) running(path string) bool {
	for _, it := range m.list {
		if it.State == "progress" && it.Path == path {
			return true
		}
	}
	return false
}

// Updated takes what the engine says a download is now.
func (m *Manager) Updated(d engine.Download) {
	it := m.byEngine[d.ID]
	if it == nil {
		it = &Item{ID: m.next, URL: d.URL, engine: d.ID}
		m.next++
		m.byEngine[d.ID] = it
		m.list = append([]*Item{it}, m.list...)
	}
	it.Path, it.Name = d.Path, filepath.Base(d.Path)
	it.State, it.Received, it.Total, it.Paused = states[d.State], d.Received, d.Total, d.Paused
	if it.State != "progress" {
		delete(m.byEngine, d.ID)
		m.save()
	}
	m.changed()
}

// Act does what a shell asks to one download: open or reveal its file, cancel, pause or
// resume it, or remove it from the list, which leaves the file where it is.
func (m *Manager) Act(id int, action string) {
	var it *Item
	for _, x := range m.list {
		if x.ID == id {
			it = x
		}
	}
	if it == nil {
		return
	}
	live := it.State == "progress"
	switch action {
	case "open":
		if it.State == "done" {
			m.e.OpenFile(it.Path)
		}
	case "reveal":
		if it.State == "done" {
			m.e.RevealFile(it.Path)
		}
	case "cancel":
		if live {
			m.e.DownloadCommand(it.engine, engine.DownloadCancel)
		}
	case "pause":
		if live {
			m.e.DownloadCommand(it.engine, engine.DownloadPause)
		}
	case "resume":
		if live {
			m.e.DownloadCommand(it.engine, engine.DownloadResume)
		}
	case "remove":
		if live {
			m.e.DownloadCommand(it.engine, engine.DownloadCancel)
			m.e.DownloadCommand(it.engine, engine.DownloadRemove)
			delete(m.byEngine, it.engine)
		}
		m.drop(func(x *Item) bool { return x == it })
	}
}

// Clear removes every finished download from the list.
func (m *Manager) Clear() {
	m.drop(func(x *Item) bool { return x.State != "progress" })
}

func (m *Manager) drop(gone func(*Item) bool) {
	kept := m.list[:0]
	for _, x := range m.list {
		if !gone(x) {
			kept = append(kept, x)
		}
	}
	m.list = kept
	m.save()
	m.changed()
}

// save keeps the finished downloads, the newest kept, in a new file renamed over the old.
func (m *Manager) save() {
	f := file{Version: 1, Next: m.next, Downloads: []Item{}}
	for _, it := range m.list {
		if it.State != "progress" && len(f.Downloads) < kept {
			f.Downloads = append(f.Downloads, *it)
		}
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err == nil {
		tmp := m.file + ".tmp"
		if err = os.WriteFile(tmp, append(b, '\n'), 0o600); err == nil {
			err = os.Rename(tmp, m.file)
		}
	}
	if err != nil {
		timeline.Errorf("downloads: %v", err)
	}
}
