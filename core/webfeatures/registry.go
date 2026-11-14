// Package webfeatures holds the web-features registry: one row per web feature whose
// browser side is Chrome's rather than content's, with what a page gets of it on each
// platform and why. Go embeds registry.json, GN reads it for the features the engine
// turns off (engine/web_features.gni), and the probes in tools/ read it for what each
// call should give. docs/web-features.md says how to read and change it.
package webfeatures

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

//go:embed registry.json
var registry []byte

// The statuses: what a page gets on a platform.
const (
	Works   = "works"   // does what Chrome does
	Off     = "off"     // turned off by the engine, so the API isn't there
	Quiet   = "quiet"   // rejects, answers empty or denied, or never answers
	Absent  = "absent"  // isn't there, or throws at once, without our turning it off
	Partly  = "partly"  // some of it works
	Kills   = "kills"   // content kills the page's renderer
	Unknown = "unknown" // not run, and the code doesn't settle it
)

// Platforms are the platforms each row gives a status for.
var Platforms = []string{"mac", "android"}

var statuses = []string{Works, Off, Quiet, Absent, Partly, Kills, Unknown}

// What a row may say it needs to work as in Chrome.
var needs = []string{"stay-off", "binder", "delegate", "go-manager", "prompt", "build"}

// Feature is one row of registry.json.
type Feature struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// API is what a page calls, or does.
	API string `json:"api"`
	// Interface is the Mojo interface Blink asks the browser for, when there is one.
	Interface *Interface `json:"interface,omitempty"`
	// Delegate is the content method that decides it, as Class::Method.
	Delegate string `json:"delegate,omitempty"`
	// Permissions are the blink::PermissionType values that decide it.
	Permissions []string `json:"permissions,omitempty"`
	// Status is what a page gets, by platform.
	Status map[string]string `json:"status"`
	// Why says, in a line a person can read, why it is so.
	Why   string   `json:"why"`
	Needs []string `json:"needs,omitempty"`
	// Owner is our file and function that answers for it.
	Owner string `json:"owner,omitempty"`
	// The Blink runtime features and the base features the engine turns off for it.
	// A row has them exactly when it is off.
	DisableBlinkFeatures []string `json:"disable_blink_features,omitempty"`
	DisableFeatures      []string `json:"disable_features,omitempty"`
	// Probe is what the probes call in a page.
	Probe []Probe `json:"probe,omitempty"`
	// Test names the checks that prove the status: probe:<name>, basics:<number> or
	// check:<a check's name>.
	Test []string `json:"test,omitempty"`
}

// Interface is a Mojo interface and the scopes Blink asks for it in.
type Interface struct {
	Name  string   `json:"name"`
	Scope []string `json:"scope"`
}

// Probe is one call the probes make.
type Probe struct {
	// Name is the call's name in the probes' output.
	Name string `json:"name"`
	// Call is a JavaScript expression. Throwing at once means the API isn't there;
	// otherwise its value, or the promise it gives, is the answer.
	Call string `json:"call"`
	// SeesNothing says the call has no answer a page can see, as adding a link has none.
	// The probes only check that the renderer lives.
	SeesNothing bool `json:"sees_nothing,omitempty"`
}

// All is every row, in the file's order.
var All = mustParse(registry)

// Parse reads rows in registry.json's form, refusing a key it doesn't know and a row
// that doesn't hold together.
func Parse(b []byte) ([]Feature, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var rows []Feature
	if err := d.Decode(&rows); err != nil {
		return nil, err
	}
	ids, probes := map[string]bool{}, map[string]bool{}
	for _, f := range rows {
		if ids[f.ID] {
			return nil, fmt.Errorf("%q is used twice", f.ID)
		}
		ids[f.ID] = true
		for _, p := range f.Probe {
			if probes[p.Name] {
				return nil, fmt.Errorf("%s: probe %q is used twice", f.ID, p.Name)
			}
			probes[p.Name] = true
		}
		if err := f.check(); err != nil {
			return nil, fmt.Errorf("%s: %w", f.ID, err)
		}
	}
	return rows, nil
}

func mustParse(b []byte) []Feature {
	rows, err := Parse(b)
	if err != nil {
		panic("registry.json: " + err.Error())
	}
	return rows
}

var (
	idPattern      = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	featurePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	basicsPattern  = regexp.MustCompile(`^basics:[1-9][0-9]*$`)
)

func (f *Feature) check() error {
	if !idPattern.MatchString(f.ID) {
		return fmt.Errorf("an id is lower case words joined by -")
	}
	if f.Name == "" || f.API == "" || f.Why == "" {
		return fmt.Errorf("name, api and why are required")
	}
	if len(f.Status) != len(Platforms) {
		return fmt.Errorf("status gives one for each of %v", Platforms)
	}
	off := 0
	for _, p := range Platforms {
		s, ok := f.Status[p]
		if !ok || !slices.Contains(statuses, s) {
			return fmt.Errorf("status for %s is one of %v", p, statuses)
		}
		if s == Off {
			off++
		}
	}
	switched := len(f.DisableBlinkFeatures) + len(f.DisableFeatures)
	if switched > 0 && off != len(Platforms) {
		return fmt.Errorf("the features it turns off are off on every platform, so its status is off on each")
	}
	if switched == 0 && off > 0 {
		return fmt.Errorf("off says which features turn it off")
	}
	for _, name := range append(append([]string{}, f.DisableBlinkFeatures...), f.DisableFeatures...) {
		if !featurePattern.MatchString(name) {
			return fmt.Errorf("%q isn't a feature's name", name)
		}
	}
	if f.Interface != nil {
		if f.Interface.Name == "" || len(f.Interface.Scope) == 0 {
			return fmt.Errorf("an interface has a name and its scopes")
		}
		for _, s := range f.Interface.Scope {
			if !slices.Contains([]string{"frame", "service_worker", "dedicated_worker", "shared_worker"}, s) {
				return fmt.Errorf("scope %q isn't frame, service_worker, dedicated_worker or shared_worker", s)
			}
		}
	}
	if f.Delegate != "" && !strings.Contains(f.Delegate, "::") {
		return fmt.Errorf("a delegate is Class::Method")
	}
	for _, n := range f.Needs {
		if !slices.Contains(needs, n) {
			return fmt.Errorf("needs %q isn't one of %v", n, needs)
		}
	}
	for _, p := range f.Probe {
		if p.Name == "" || p.Call == "" {
			return fmt.Errorf("a probe has a name and a call")
		}
	}
	for _, t := range f.Test {
		switch {
		case strings.HasPrefix(t, "probe:"):
			if !f.hasProbe(strings.TrimPrefix(t, "probe:")) {
				return fmt.Errorf("%s names no probe of this row", t)
			}
		case strings.HasPrefix(t, "basics:"):
			if !basicsPattern.MatchString(t) {
				return fmt.Errorf("%s isn't basics:<number>", t)
			}
		case strings.HasPrefix(t, "check:"):
			if strings.TrimPrefix(t, "check:") == "" {
				return fmt.Errorf("check: names a check")
			}
		default:
			return fmt.Errorf("a test is probe:, basics: or check:, not %q", t)
		}
	}
	return nil
}

func (f *Feature) hasProbe(name string) bool {
	for _, p := range f.Probe {
		if p.Name == name {
			return true
		}
	}
	return false
}

// Platform is the platform this runs on, as the rows name it.
func Platform() string {
	if runtime.GOOS == "android" {
		return "android"
	}
	return "mac"
}

// Row is what the settings page shows of a feature: what a page gets of it on this
// platform, and why.
type Row struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	API    string `json:"api"`
	Status string `json:"status"`
	Why    string `json:"why"`
}

// View is every row for this platform, in the registry's order.
func View() []Row {
	rows := make([]Row, 0, len(All))
	for _, f := range All {
		rows = append(rows, Row{ID: f.ID, Name: f.Name, API: f.API, Status: f.Status[Platform()], Why: f.Why})
	}
	return rows
}
