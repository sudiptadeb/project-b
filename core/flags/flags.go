// Package flags owns the flags: switches the app reads once, as it starts. Each is saved in
// flags.json in the profile and becomes a command-line switch for the engine before
// Chromium starts, so a change takes effect on the next launch. The settings page's Flags
// section shows them and offers a relaunch while what is saved differs from what the app
// started with.
package flags

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// Kinds of flag.
const (
	KindSwitch = "switch" // on or off: "on" or ""
	KindText   = "text"   // a value, or "" for off
)

// Flag is one flag as the program defines it.
type Flag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Build names the build flag the flag needs, or "" when every build has what it turns on.
	Build string `json:"build,omitempty"`
	// valid says whether a text value can be used; toSwitch is the switch for a value.
	valid    func(string) bool
	toSwitch func(string) string
}

// Flag ids.
const (
	Extensions      = "extensions"
	RemoteDebugging = "remoteDebugging"
	Features        = "features"
)

var featureList = regexp.MustCompile(`^[A-Za-z0-9_.:<>,]+$`)

// All is every flag, in the order the settings page lists them.
var All = []Flag{
	{
		ID: Extensions, Name: "Extensions", Kind: KindSwitch, Build: "extensions",
		toSwitch: func(string) string { return "--pb-extensions" },
	},
	{
		ID: RemoteDebugging, Name: "Remote debugging port", Kind: KindText,
		valid: func(v string) bool {
			p, err := strconv.Atoi(v)
			return err == nil && p >= 1024 && p <= 65535
		},
		toSwitch: func(v string) string { return "--remote-debugging-port=" + v },
	},
	{
		ID: Features, Name: "Chromium features", Kind: KindText,
		valid:    featureList.MatchString,
		toSwitch: func(v string) string { return "--enable-features=" + v },
	},
}

// Builds is every build flag, by id and name: what tools/build.sh can leave out.
var Builds = []struct{ ID, Name string }{
	{"extensions", "Extensions"},
}

// builds is the build flags this build was made with. A tagged file adds its own.
var builds = map[string]bool{}

func byID(id string) (Flag, bool) {
	for _, f := range All {
		if f.ID == id {
			return f, true
		}
	}
	return Flag{}, false
}

// usable says whether a flag can take a value: the build has it, and the value is one.
func usable(f Flag, v string) bool {
	if f.Build != "" && !builds[f.Build] {
		return false
	}
	switch f.Kind {
	case KindSwitch:
		return v == "" || v == "on"
	default:
		return v == "" || f.valid(v)
	}
}

// Manager holds the flags as saved and as the app started with them.
type Manager struct {
	path     string
	saved    map[string]string
	launched map[string]string
	onChange []func()
}

// New reads the flags saved under dir. What the file holds for a flag that isn't one, or a
// value that can't be used, is dropped.
func New(dir string) *Manager {
	m := &Manager{path: filepath.Join(dir, "flags.json"), saved: map[string]string{}}
	var file map[string]string
	if b, err := os.ReadFile(m.path); err == nil {
		if err := json.Unmarshal(b, &file); err != nil {
			log.Printf("flags: %v", err)
		}
	}
	for id, v := range file {
		if f, ok := byID(id); ok && v != "" && usable(f, v) {
			m.saved[id] = v
		}
	}
	m.launched = clone(m.saved)
	return m
}

func clone(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Switches are the command-line switches the flags the app started with ask for.
func (m *Manager) Switches() []string {
	var out []string
	for _, f := range All {
		if v := m.launched[f.ID]; v != "" {
			out = append(out, f.toSwitch(v))
		}
	}
	return out
}

// On says whether a flag was on when the app started.
func (m *Manager) On(id string) bool { return m.launched[id] != "" }

// OnChanged subscribes to a flag being saved with a new value.
func (m *Manager) OnChanged(f func()) { m.onChange = append(m.onChange, f) }

// Set saves a flag's value for the next launch: "on" or "" for a switch, the value or ""
// for text. A flag that isn't one, or a value it can't take, is ignored.
func (m *Manager) Set(id, v string) {
	f, ok := byID(id)
	if !ok || !usable(f, v) || m.saved[id] == v {
		return
	}
	if v == "" {
		delete(m.saved, id)
	} else {
		m.saved[id] = v
	}
	m.save()
	for _, f := range m.onChange {
		f()
	}
}

// FlagView is one flag as the settings page shows it.
type FlagView struct {
	Flag
	Value string `json:"value"`
	// Built is false when the build lacks what the flag turns on.
	Built bool `json:"built"`
}

// BuildView is one build flag as the settings page shows it.
type BuildView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	On   bool   `json:"on"`
}

// View is the Flags section: each flag with its saved value, whether a relaunch is needed
// for them to take effect, and the build flags this build was made with.
type View struct {
	Flags    []FlagView  `json:"flags"`
	Relaunch bool        `json:"relaunch"`
	Builds   []BuildView `json:"builds"`
}

// View returns what the settings page shows.
func (m *Manager) View() View {
	v := View{Flags: []FlagView{}, Builds: []BuildView{}}
	for _, b := range Builds {
		v.Builds = append(v.Builds, BuildView{b.ID, b.Name, builds[b.ID]})
	}
	for _, f := range All {
		v.Flags = append(v.Flags, FlagView{Flag: f, Value: m.saved[f.ID], Built: f.Build == "" || builds[f.Build]})
		if m.saved[f.ID] != m.launched[f.ID] {
			v.Relaunch = true
		}
	}
	return v
}

func (m *Manager) save() {
	b, _ := json.MarshalIndent(m.saved, "", "  ")
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		log.Printf("flags: %v", err)
		return
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		log.Printf("flags: %v", err)
		return
	}
	if err := os.Rename(tmp, m.path); err != nil {
		log.Printf("flags: %v", err)
	}
}
