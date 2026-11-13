// Package shells is the list of shells the app is built with: each one's id, the folder
// it builds from, shell/src/apps/shell/<id>/, and the page it loads as, and the name the
// user sees. Which one a profile loads is its shell pref (docs/shells.md).
package shells

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
)

// Shell is one shell of the registry.
type Shell struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Example is a shell that is built and can be loaded, but is offered nowhere: the one
	// to copy to build another.
	Example bool `json:"example,omitempty"`
}

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
	}
	return out
}()

// All is every shell, in the registry's order.
func All() []Shell { return append([]Shell(nil), all...) }

// Listed is the shells the user can choose: every one but the examples.
func Listed() []Shell {
	var out []Shell
	for _, s := range all {
		if !s.Example {
			out = append(out, s)
		}
	}
	return out
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
