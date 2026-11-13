package profiles

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"projectb/commands"
)

// Personal is the template a first start makes its profile from.
const Personal = "Personal"

// Template is a file of values a profile is made from: its name, and the prefs it writes
// as the profile's own. Nothing else, and no code asks which one a profile came from.
type Template struct {
	Name  string          `json:"name"`
	Prefs json.RawMessage `json:"prefs"`
}

// Templates reads every template, Personal first. They are built in, so a bad one is a
// bug in the build.
func Templates() []Template {
	entries, err := fs.ReadDir(commands.ProfileTemplates, ".")
	if err != nil {
		panic(err)
	}
	var out []Template
	for _, e := range entries {
		b, err := fs.ReadFile(commands.ProfileTemplates, e.Name())
		if err != nil {
			panic(err)
		}
		var t Template
		if err := json.Unmarshal(b, &t); err != nil {
			panic(fmt.Sprintf("profile template %s: %v", e.Name(), err))
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name == Personal || out[j].Name != Personal && out[i].Name < out[j].Name
	})
	return out
}

// TemplateNamed is the template called name.
func TemplateNamed(name string) (Template, bool) {
	for _, t := range Templates() {
		if t.Name == name {
			return t, true
		}
	}
	return Template{}, false
}

// writePrefs writes the template's values as the prefs of the profile in dir.
func (t Template) writePrefs(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "prefs.json"), append(append([]byte(nil), t.Prefs...), '\n'), 0o600)
}
