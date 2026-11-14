// Package shells is the list of shells the app is built with: each one's id, the folder
// it builds from, shell/src/apps/shell/<id>/, and the page it loads as, the name the user
// sees, and its own actions. Which one a profile loads is its shell pref (docs/shells.md).
package shells

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

// Shell is one shell of the registry.
type Shell struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Example is a shell that is built and can be loaded, but is offered nowhere: the one
	// to copy to build another.
	Example bool `json:"example,omitempty"`
	// Phone is a shell made for a phone: a phone offers and loads only these, and a desktop
	// offers none of them, though it loads one named outright, as the mock does for a
	// phone-sized test.
	Phone bool `json:"phone,omitempty"`
	// Actions are the shell's own, such as the terminal's splits. Go registers each with
	// the others, enabled only while this shell is the profile's; running one sends the
	// shell action with its id. An id starts with the shell's and a dot.
	Actions []Action `json:"actions,omitempty"`
}

// Action is one of a shell's own actions: its id, which never changes once released, and
// the title its menu item shows.
type Action struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// OnPhone is whether this app runs on a phone. Android is a phone: the app has no tablet
// layout.
var OnPhone = runtime.GOOS == "android"

//go:embed registry.json
var registry []byte

// An id is lower-case letters, digits and dashes.
var id = regexp.MustCompile(`^[a-z0-9-]+$`)

// all is the registry, in its order. It is built in, so a bad one is a bug in the build.
var all = func() []Shell {
	var out []Shell
	if err := json.Unmarshal(registry, &out); err != nil {
		panic(fmt.Sprintf("shells: registry.json: %v", err))
	}
	for _, s := range out {
		if !id.MatchString(s.ID) || s.Name == "" {
			panic(fmt.Sprintf("shells: registry.json: %+v", s))
		}
		for _, a := range s.Actions {
			if !strings.HasPrefix(a.ID, s.ID+".") || a.Title == "" {
				panic(fmt.Sprintf("shells: registry.json: %s: %+v", s.ID, a))
			}
		}
	}
	return out
}()

// All is every shell, in the registry's order.
func All() []Shell { return append([]Shell(nil), all...) }

// Listed is the shells the user can choose: every one made for this kind of device, but
// the examples.
func Listed() []Shell {
	var out []Shell
	for _, s := range all {
		if !s.Example && s.Phone == OnPhone {
			out = append(out, s)
		}
	}
	return out
}

// Default is the shell a profile loads until it chooses one: the first the user can
// choose on this kind of device.
func Default() string { return Listed()[0].ID }

// Fits says whether a profile whose pref names id loads it. A desktop loads any shell
// named outright, an example or a phone's; a phone only a phone's, so the desktop's that
// the profile templates name loads Default there.
func Fits(id string) bool {
	for _, s := range all {
		if s.ID == id {
			return s.Phone || !OnPhone
		}
	}
	return false
}

// Has says whether id is a shell's.
func Has(id string) bool {
	for _, s := range all {
		if s.ID == id {
			return true
		}
	}
	return false
}
