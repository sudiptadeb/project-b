// Package flags owns the flags. A flag is a named command-line switch: off until the user
// gives it a value, applied at launch, and its code may be left out of a build. Each is
// defined once, in registry.json. The values are saved in flags.json in the profile, and
// before Chromium starts the ones the app launched with become switches for the engine.
// The settings page's Flags section offers Relaunch beside each flag whose saved value
// isn't yet the one running.
package flags

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The types of value. A saved value is the text after the switch's "=", or "on" for a
// boolean; "" is off.
const (
	Boolean = "boolean" // --switch
	Number  = "number"  // a whole number, in min..max when given
	Enum    = "enum"    // one of choices
	String  = "string"  // text, matching pattern when given
	Numbers = "numbers" // numbers separated by commas
	Strings = "strings" // strings separated by commas
)

// Flag is one row of registry.json.
type Flag struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Switch is the command-line switch, without its dashes.
	Switch string `json:"switch"`
	Type   string `json:"type"`
	// Build says the flag's code can be left out of a build: WITHOUT=<id> tools/build.sh.
	Build bool `json:"build,omitempty"`
	// Min and Max bound a number, or each of numbers.
	Min *int `json:"min,omitempty"`
	Max *int `json:"max,omitempty"`
	// Choices are an enum's values.
	Choices []string `json:"choices,omitempty"`
	// Pattern is what a string, or each of strings, must match.
	Pattern string `json:"pattern,omitempty"`

	pattern *regexp.Regexp
}

// Parse reads rows in registry.json's form, refusing a key it doesn't know and a row that
// doesn't hold together.
func Parse(b []byte) ([]Flag, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var rows []Flag
	if err := d.Decode(&rows); err != nil {
		return nil, err
	}
	for i := range rows {
		if err := rows[i].check(); err != nil {
			return nil, fmt.Errorf("flag %q: %w", rows[i].ID, err)
		}
	}
	return rows, nil
}

func mustParse(b []byte) []Flag {
	rows, err := Parse(b)
	if err != nil {
		panic("registry.json: " + err.Error())
	}
	return rows
}

// check says what is wrong with a row, and compiles its pattern.
func (f *Flag) check() error {
	if f.ID == "" || f.Name == "" || f.Description == "" || f.Switch == "" {
		return fmt.Errorf("needs an id, a name, a description and a switch")
	}
	number := f.Type == Number || f.Type == Numbers
	text := f.Type == String || f.Type == Strings
	switch {
	case !slices.Contains([]string{Boolean, Number, Enum, String, Numbers, Strings}, f.Type):
		return fmt.Errorf("type %q", f.Type)
	case (f.Enum() != (len(f.Choices) > 0)):
		return fmt.Errorf("choices are for an enum, and an enum needs them")
	case !number && (f.Min != nil || f.Max != nil):
		return fmt.Errorf("min and max are for numbers")
	case !text && f.Pattern != "":
		return fmt.Errorf("a pattern is for strings")
	}
	if f.Pattern != "" {
		p, err := regexp.Compile(f.Pattern)
		if err != nil {
			return err
		}
		f.pattern = p
	}
	return nil
}

// Enum says whether the flag is an enum.
func (f Flag) Enum() bool { return f.Type == Enum }

// normal is a value as it is saved, or false when the flag can't take it. Spaces around
// a list's commas go.
func (f Flag) normal(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", true
	}
	switch f.Type {
	case Boolean:
		return v, v == "on"
	case Number:
		return v, f.number(v)
	case Enum:
		return v, slices.Contains(f.Choices, v)
	case String:
		return v, f.text(v)
	}
	items := strings.Split(v, ",")
	for i, item := range items {
		items[i] = strings.TrimSpace(item)
		if f.Type == Numbers && !f.number(items[i]) || f.Type == Strings && !f.text(items[i]) {
			return "", false
		}
	}
	return strings.Join(items, ","), true
}

func (f Flag) number(v string) bool {
	n, err := strconv.Atoi(v)
	return err == nil && (f.Min == nil || n >= *f.Min) && (f.Max == nil || n <= *f.Max)
}

func (f Flag) text(v string) bool {
	return v != "" && (f.pattern == nil || f.pattern.MatchString(v))
}

// toSwitch is the command-line switch for a value that is on.
func (f Flag) toSwitch(v string) string {
	if f.Type == Boolean {
		return "--" + f.Switch
	}
	return "--" + f.Switch + "=" + v
}

// builds is the build flags this build was made with, by id. A tagged file adds its own.
var builds = map[string]bool{}

func (f Flag) built() bool { return !f.Build || builds[f.ID] }

func byID(id string) (Flag, bool) {
	for _, f := range All {
		if f.ID == id {
			return f, true
		}
	}
	return Flag{}, false
}

// Manager holds the flags as saved and as the app launched with them.
type Manager struct {
	path     string
	saved    map[string]string
	launched map[string]string
	onChange []func()
}

// New reads the flags saved under dir. What the file holds for a flag that isn't one, that
// isn't in this build, or that the flag can't take, is dropped.
func New(dir string) *Manager {
	m := &Manager{path: filepath.Join(dir, "flags.json"), saved: map[string]string{}}
	var file map[string]string
	if b, err := os.ReadFile(m.path); err == nil {
		if err := json.Unmarshal(b, &file); err != nil {
			log.Printf("flags: %v", err)
		}
	}
	for id, v := range file {
		f, ok := byID(id)
		if !ok || !f.built() {
			continue
		}
		if v, ok := f.normal(v); ok && v != "" {
			m.saved[id] = v
		}
	}
	m.launched = make(map[string]string, len(m.saved))
	for k, v := range m.saved {
		m.launched[k] = v
	}
	return m
}

// Switches are the command-line switches of the flags the app launched with.
func (m *Manager) Switches() []string {
	var out []string
	for _, f := range All {
		if v := m.launched[f.ID]; v != "" {
			out = append(out, f.toSwitch(v))
		}
	}
	return out
}

// IsOn says whether a flag was on when the app launched.
func (m *Manager) IsOn(id string) bool { return m.launched[id] != "" }

// OnChanged subscribes to a flag being saved with a new value.
func (m *Manager) OnChanged(f func()) { m.onChange = append(m.onChange, f) }

// Set saves a flag's value for the next launch; "" turns it off. A flag that isn't one or
// isn't in this build, or a value it can't take, is ignored.
func (m *Manager) Set(id, v string) {
	f, ok := byID(id)
	if !ok || !f.built() {
		return
	}
	v, ok = f.normal(v)
	if !ok || v == m.saved[id] {
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
	// Built is false when the build left the flag's code out.
	Built bool `json:"built"`
	// Pending is true while the saved value waits for a relaunch.
	Pending bool `json:"pending"`
}

// View is every flag as the settings page shows it.
func (m *Manager) View() []FlagView {
	v := []FlagView{}
	for _, f := range All {
		v = append(v, FlagView{Flag: f, Value: m.saved[f.ID], Built: f.built(), Pending: m.saved[f.ID] != m.launched[f.ID]})
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
