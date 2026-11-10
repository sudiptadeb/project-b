package commands

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Reserve values: who wins a key.
const (
	ReserveAlways   = "always"   // The browser, always.
	ReserveWindowed = "windowed" // The browser outside full screen, the page in it.
	ReservePage     = "page"     // The page first; the action only if it didn't use the key.
)

// DefaultTemplate is what both files follow until the user edits them.
const DefaultTemplate = "project-b"

// Binding is one key combination for an action on one platform.
type Binding struct {
	Keys    string `json:"keys"`
	Reserve string `json:"reserve"`
	// FromOS says the OS gave this key (macOS App Shortcuts). Go writes these bindings
	// from what the OS reports; they are never edited here.
	FromOS bool `json:"fromOS,omitempty"`
}

// ActionKeys is everything a keymap says about one action.
type ActionKeys struct {
	Hidden  bool      `json:"hidden,omitempty"`
	Mac     []Binding `json:"mac,omitempty"`
	Windows []Binding `json:"windows,omitempty"`
	Linux   []Binding `json:"linux,omitempty"`
}

// On returns the bindings for one platform.
func (a *ActionKeys) On(platform string) []Binding {
	switch platform {
	case Mac:
		return a.Mac
	case Windows:
		return a.Windows
	}
	return a.Linux
}

// SetOn replaces the bindings for one platform.
func (a *ActionKeys) SetOn(platform string, b []Binding) {
	switch platform {
	case Mac:
		a.Mac = b
	case Windows:
		a.Windows = b
	default:
		a.Linux = b
	}
}

func (a *ActionKeys) clone() *ActionKeys {
	c := *a
	c.Mac = append([]Binding(nil), a.Mac...)
	c.Windows = append([]Binding(nil), a.Windows...)
	c.Linux = append([]Binding(nil), a.Linux...)
	return &c
}

// Keymap is keymap.json, and every keymap template: one shape for both. A file that
// follows a template says only which; one the user owns holds every action.
type Keymap struct {
	Follows  string                 `json:"follows,omitempty"`
	Template string                 `json:"template,omitempty"`
	Actions  map[string]*ActionKeys `json:"actions,omitempty"`
}

func (k Keymap) clone() Keymap {
	c := Keymap{Follows: k.Follows, Template: k.Template, Actions: map[string]*ActionKeys{}}
	for id, a := range k.Actions {
		c.Actions[id] = a.clone()
	}
	return c
}

// Node is one entry of a menu tree: a submenu (Title and Items), an action, or a
// separator. Locked parts can't be hidden or removed, and live only inside locked menus.
type Node struct {
	Title     string `json:"title,omitempty"`
	Action    string `json:"action,omitempty"`
	Separator bool   `json:"separator,omitempty"`
	Locked    bool   `json:"locked,omitempty"`
	// Role names a menu the OS knows: "app", "window", "services".
	Role string `json:"role,omitempty"`
	// Only limits the entry to one platform.
	Only  string `json:"only,omitempty"`
	Items []Node `json:"items,omitempty"`
}

// MenuFile is menu.json and every menu template. Until there is a menu editor the
// current file only ever follows a template.
type MenuFile struct {
	Follows  string `json:"follows,omitempty"`
	Template string `json:"template,omitempty"`
	Menus    []Node `json:"menus,omitempty"`
}

//go:embed templates
var templateFS embed.FS

// keymapTemplates reads every keymap template, by name.
func keymapTemplates() map[string]Keymap {
	out := map[string]Keymap{}
	entries, _ := templateFS.ReadDir("templates/keymap")
	for _, e := range entries {
		var k Keymap
		mustDecode(path.Join("templates/keymap", e.Name()), &k)
		out[strings.TrimSuffix(e.Name(), ".json")] = k
	}
	return out
}

func menuTemplate(name string) (MenuFile, bool) {
	var m MenuFile
	b, err := templateFS.ReadFile("templates/menu/" + name + ".json")
	if err != nil {
		return m, false
	}
	if err := json.Unmarshal(b, &m); err != nil {
		panic(fmt.Sprintf("menu template %s: %v", name, err))
	}
	return m, true
}

func lockedKeys() Keymap {
	var k Keymap
	mustDecode("templates/locked.json", &k)
	return k
}

// The templates are built in, so a bad one is a bug in the build.
func mustDecode(name string, v any) {
	b, err := templateFS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		panic(fmt.Sprintf("%s: %v", name, err))
	}
}

// TemplateNames lists the keymap templates, the default first.
func TemplateNames() []string {
	var names []string
	for n := range keymapTemplates() {
		if n != DefaultTemplate {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return append([]string{DefaultTemplate}, names...)
}

// readJSON reads a current file. A missing file is no error: ok is false. A file that
// doesn't parse is moved aside, so a reset can write a good one in its place.
func readJSON(file string, v any) (ok bool, err error) {
	b, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, v); err != nil {
		bad := file + ".bad"
		if renameErr := os.Rename(file, bad); renameErr != nil {
			return false, fmt.Errorf("%s doesn't parse (%v) and couldn't be moved aside: %v", filepath.Base(file), err, renameErr)
		}
		return false, fmt.Errorf("%s didn't parse (%v); it is kept as %s", filepath.Base(file), err, filepath.Base(bad))
	}
	return true, nil
}

func writeJSON(file string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}
