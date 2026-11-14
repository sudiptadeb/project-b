// Package commands owns what the user can do and how: every action by a stable id, the
// menu tree that shows them, and the keys that run them. A key, a menu item and a shell
// button all run an action the same way, through Run.
//
// The keys live in keymap.json and the tree in menu.json, both in the profile. Each
// follows a built-in template until the user's first edit copies the template in; from
// then on the file is the user's. Locked keys come from a file of their own and always
// apply. See docs/menus-shortcuts.md.
package commands

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"

	"projectb/engine"
	"projectb/timeline"
)

// Action is one thing the user can do.
type Action struct {
	ID    string
	Title string
	Run   func()
	// Enabled says whether it can run now; nil means always. Checked, when set, makes
	// its menu item show a check.
	Enabled func() bool
	Checked func() bool
	// Release, when set, makes the action one that lasts while its key's modifiers are
	// held, as the OS's app switcher does: it runs when they are let go. Run from a menu
	// or a button, where nothing is held, it runs at once.
	Release func()
	// Shell names the shell whose own action this is, "" for the app's: Keyboard settings
	// lists it under that shell, and only while the shell is on (Enabled).
	Shell string
}

// List is a part of a menu that changes while the app runs, such as the profiles. Its
// items are read when the menu opens, so they are as they are then. Each item is a key
// the list knows; choosing it runs Choose with the key.
type List struct {
	ID     string
	Items  func() []ListItem
	Choose func(key string)
}

// ListItem is one item of a List.
type ListItem struct {
	Key     string
	Title   string
	Checked bool
}

// Missing is an action an update brought that the user's own keymap doesn't have yet,
// with the keys it couldn't take because the keymap already uses them.
type Missing struct {
	ID    string   `json:"id"`
	Taken []string `json:"taken"`
}

// match is what a key press means: the action and the binding it matched.
type match struct {
	id      string
	reserve string
}

// held is an action waiting for the modifiers of the key that ran it to be let go.
type held struct {
	mods    engine.Mods
	release func()
}

// Manager owns the actions, the tree and the keymap.
type Manager struct {
	eng        engine.Engine
	platform   string
	keymapFile string
	menuFile   string
	fullScreen func() bool

	actions map[string]*Action
	lists   map[string]*List
	tree    []Node
	locked  Keymap
	file    Keymap // keymap.json as it is on disk
	keys    Keymap // what applies: the file, or the template it follows
	index   map[engine.Key]match

	missing   []Missing
	problems  []string
	recording bool
	holding   *held
	sent      string // the menu last sent, as JSON
	onChanged []func()
}

// New returns a manager for the files in dir. fullScreen says whether the window is in
// full screen now, for keys reserved only outside it.
func New(eng engine.Engine, dir string, fullScreen func() bool) *Manager {
	return &Manager{
		eng:        eng,
		platform:   platformOf(runtime.GOOS),
		keymapFile: filepath.Join(dir, "keymap.json"),
		menuFile:   filepath.Join(dir, "menu.json"),
		fullScreen: fullScreen,
		actions:    map[string]*Action{},
		lists:      map[string]*List{},
		locked:     lockedKeys(),
	}
}

// NewFromTemplates returns a manager for a copy with no profile, the picker's: it follows
// the templates and keeps no file.
func NewFromTemplates(eng engine.Engine, fullScreen func() bool) *Manager {
	m := New(eng, "", fullScreen)
	m.keymapFile, m.menuFile = "", ""
	return m
}

func platformOf(goos string) string {
	switch goos {
	case "darwin":
		return Mac
	case "windows":
		return Windows
	}
	return Linux
}

// Platform is the keymap's name for this platform.
func (m *Manager) Platform() string { return m.platform }

// Register adds an action. Call it for every action before Load.
func (m *Manager) Register(a Action) { m.actions[a.ID] = &a }

// RegisterList adds a list, which a menu template places by its id. Call it before Load.
func (m *Manager) RegisterList(l List) { m.lists[l.ID] = &l }

// ListItems is a list's items as they are now, as menu items. Choosing one runs its
// action, "<list>:<key>". A list nothing registered has none.
func (m *Manager) ListItems(id string) engine.Menu {
	l, ok := m.lists[id]
	if !ok {
		return nil
	}
	var out engine.Menu
	for _, it := range l.Items() {
		out = append(out, engine.MenuItem{Title: it.Title, Action: id + ":" + it.Key, Enabled: true, Checked: it.Checked})
	}
	return out
}

// OnChanged subscribes to the keymap or the actions' state changing.
func (m *Manager) OnChanged(f func()) { m.onChanged = append(m.onChanged, f) }

// Load reads both files, merges what an update added into an owned keymap, and puts
// the menu on screen.
func (m *Manager) Load() {
	m.loadMenu()
	m.loadKeymap()
	m.apply()
}

func (m *Manager) loadMenu() {
	var f MenuFile
	ok, err := readJSON(m.menuFile, &f)
	if err != nil {
		m.problem(err.Error())
	}
	name := DefaultTemplate
	if ok && f.Follows != "" {
		name = f.Follows
	}
	t, found := menuTemplate(name)
	if !found {
		m.problem("menu.json follows " + name + ", which isn't a menu template; using " + DefaultTemplate)
		t, _ = menuTemplate(DefaultTemplate)
	}
	m.tree = t.Menus
}

func (m *Manager) loadKeymap() {
	var f Keymap
	ok, err := readJSON(m.keymapFile, &f)
	if err != nil {
		m.problem(err.Error())
	}
	templates := keymapTemplates()
	if !ok || f.Follows != "" || f.Actions == nil {
		name := f.Follows
		if _, found := templates[name]; !found {
			if name != "" {
				m.problem("keymap.json follows " + name + ", which isn't a template; using " + DefaultTemplate)
			}
			name = DefaultTemplate
		}
		m.file = Keymap{Follows: name}
		m.keys = templates[name].clone()
		m.keys.Template = name
		return
	}
	m.file = f
	m.keys = f
	m.mergeNew(templates[f.Template])
}

// mergeNew adds to an owned keymap each action its template has and it doesn't. A key
// the keymap already uses elsewhere is left off, and the action is reported.
func (m *Manager) mergeNew(t Keymap) {
	if t.Actions == nil {
		return
	}
	used := map[string]map[engine.Key]bool{}
	for _, p := range []string{Mac, Windows, Linux} {
		used[p] = map[engine.Key]bool{}
		for _, a := range m.file.Actions {
			for _, b := range a.On(p) {
				if k, err := ParseKeys(b.Keys); err == nil {
					used[p][k] = true
				}
			}
		}
	}
	changed := false
	for id, a := range t.Actions {
		if _, ok := m.file.Actions[id]; ok {
			continue
		}
		add := a.clone()
		var taken []string
		for _, p := range []string{Mac, Windows, Linux} {
			var keep []Binding
			for _, b := range add.On(p) {
				k, err := ParseKeys(b.Keys)
				if err == nil && used[p][k] {
					if p == m.platform {
						taken = append(taken, b.Keys)
					}
					continue
				}
				keep = append(keep, b)
			}
			add.SetOn(p, keep)
		}
		m.file.Actions[id] = add
		changed = true
		if len(taken) > 0 {
			m.missing = append(m.missing, Missing{ID: id, Taken: taken})
		}
	}
	if changed {
		m.save()
	}
}

func (m *Manager) problem(s string) {
	timeline.Errorf("commands: %s", s)
	m.problems = append(m.problems, s)
}

// apply rebuilds the key index and the menu from the current state, and tells
// subscribers.
func (m *Manager) apply() {
	m.index = map[engine.Key]match{}
	add := func(keymap Keymap, ids []string) {
		for _, id := range ids {
			a, ok := keymap.Actions[id]
			if !ok {
				continue
			}
			for _, b := range a.On(m.platform) {
				k, err := ParseKeys(b.Keys)
				if err != nil {
					timeline.Errorf("commands: %s: %v", id, err)
					continue
				}
				if _, taken := m.index[k]; !taken {
					m.index[k] = match{id: id, reserve: b.Reserve}
				}
			}
		}
	}
	// Locked keys win; then the tree's order decides between two actions on one key.
	order := m.order()
	add(m.locked, order)
	add(m.keys, order)
	m.RefreshMenu()
	for _, f := range m.onChanged {
		f()
	}
}

// order is every action id in the tree, in menu order, for this platform.
func (m *Manager) order() []string {
	var ids []string
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			if n.Only != "" && n.Only != m.platform {
				continue
			}
			if n.Action != "" {
				ids = append(ids, n.Action)
			}
			walk(n.Items)
		}
	}
	walk(m.tree)
	return ids
}

// Run runs an action by id, if it exists and can run now, as a menu item or a button
// does: with nothing held.
func (m *Manager) Run(id string) {
	if list, key, ok := strings.Cut(id, ":"); ok && m.lists[list] != nil {
		m.lists[list].Choose(key)
		return
	}
	a, ok := m.actions[id]
	if !ok {
		timeline.Warnf("commands: no action %q", id)
		return
	}
	if a.Enabled != nil && !a.Enabled() {
		return
	}
	m.run(a, 0)
}

// run runs an action whose key had these modifiers. An action that lasts while they are
// held ends when they are let go, Shift aside: as with the OS's app switcher, Shift only
// turns the direction, so Ctrl+Tab and Ctrl+Shift+Tab are one hold on Ctrl. Any other
// action ends it first.
func (m *Manager) run(a *Action, mods engine.Mods) {
	watch := mods &^ engine.ModShift
	if watch == 0 {
		watch = mods
	}
	if m.holding != nil && (a.Release == nil || m.holding.mods != watch) {
		m.letGo()
	}
	a.Run()
	if a.Release == nil {
		return
	}
	if mods == 0 {
		a.Release()
		return
	}
	m.holding = &held{mods: watch, release: a.Release}
}

func (m *Manager) letGo() {
	h := m.holding
	m.holding = nil
	h.release()
}

// ModsChanged takes the modifiers held now. Letting go of those an action is held by
// ends it.
func (m *Manager) ModsChanged(mods engine.Mods) {
	if m.holding != nil && mods&m.holding.mods != m.holding.mods {
		m.letGo()
	}
}

// Key decides about one key press. Before the page, only a reserved binding takes it;
// after the page didn't use it, any binding does.
func (m *Manager) Key(k engine.Key, phase engine.KeyPhase) bool {
	if m.recording {
		return false
	}
	hit, ok := m.index[k]
	if !ok {
		return false
	}
	if phase == engine.KeyBeforePage {
		reserved := hit.reserve == ReserveAlways ||
			hit.reserve == ReserveWindowed && !m.fullScreen()
		if !reserved {
			return false
		}
	}
	a, ok := m.actions[hit.id]
	if !ok || a.Enabled != nil && !a.Enabled() {
		return false
	}
	m.run(a, k.Mods)
	return true
}

// SetRecording stops every key from running anything while the settings page records
// a new binding, so the keys reach it.
func (m *Manager) SetRecording(on bool) { m.recording = on }

// RefreshMenu sends the menu again if an action's enabled or checked state changed it.
func (m *Manager) RefreshMenu() {
	menu := m.menu(m.tree)
	b, _ := json.Marshal(menu)
	if string(b) == m.sent {
		return
	}
	m.sent = string(b)
	m.eng.SetMenu(menu)
}

func (m *Manager) menu(nodes []Node) engine.Menu {
	var out engine.Menu
	for _, n := range nodes {
		if n.Only != "" && n.Only != m.platform {
			continue
		}
		switch {
		case n.Separator:
			out = append(out, engine.MenuItem{Separator: true})
		case n.List != "":
			if _, ok := m.lists[n.List]; ok {
				out = append(out, engine.MenuItem{List: n.List})
			}
		case n.Action != "":
			a, ok := m.actions[n.Action]
			if !ok || m.hidden(n) {
				continue
			}
			item := engine.MenuItem{
				Title:   a.Title,
				Action:  a.ID,
				Key:     m.shownKey(a.ID),
				Enabled: a.Enabled == nil || a.Enabled(),
				Checked: a.Checked != nil && a.Checked(),
			}
			out = append(out, item)
		default:
			items := m.menu(n.Items)
			if len(items) == 0 && n.Role == "" && !n.Locked {
				continue
			}
			out = append(out, engine.MenuItem{Title: n.Title, Role: n.Role, Items: items})
		}
	}
	return out
}

func (m *Manager) hidden(n Node) bool {
	if n.Locked {
		return false
	}
	a, ok := m.keys.Actions[n.Action]
	return ok && a.Hidden
}

// shownKey is the key a menu item shows: the OS's, which the OS forces on the item
// anyway, else a locked one, else the first.
func (m *Manager) shownKey(id string) *engine.Key {
	var all []Binding
	if a, ok := m.keys.Actions[id]; ok {
		for _, b := range a.On(m.platform) {
			if b.FromOS {
				all = append([]Binding{b}, all...)
			} else {
				all = append(all, b)
			}
		}
	}
	if a, ok := m.locked.Actions[id]; ok {
		all = append(a.On(m.platform), all...)
	}
	for _, b := range all {
		if k, err := ParseKeys(b.Keys); err == nil {
			return &k
		}
	}
	return nil
}

// OSBindings takes the keys the OS gives our menu items, by title, and keeps them in the
// keymap as bindings marked fromOS. A change makes the keymap the user's, like any edit.
func (m *Manager) OSBindings(byTitle map[string]engine.Key) {
	want := map[string]engine.Key{}
	for title, k := range byTitle {
		// Newer macOS names an item by its menu path, "->File->New Tab".
		if i := strings.LastIndex(title, "->"); i >= 0 {
			title = title[i+2:]
		}
		for _, a := range m.actions {
			if a.Title == title {
				want[a.ID] = k
			}
		}
	}
	same := true
	for id, a := range m.keys.Actions {
		for _, b := range a.On(m.platform) {
			if !b.FromOS {
				continue
			}
			k, err := ParseKeys(b.Keys)
			if w, ok := want[id]; !ok || err != nil || w != k {
				same = false
			}
		}
	}
	for id, k := range want {
		a, ok := m.keys.Actions[id]
		found := false
		if ok {
			for _, b := range a.On(m.platform) {
				if b.FromOS && b.Keys == FormatKeys(m.platform, k) {
					found = true
				}
			}
		}
		if !found {
			same = false
		}
	}
	if same {
		return
	}
	m.edit(func(keys Keymap) {
		for _, a := range keys.Actions {
			var keep []Binding
			for _, b := range a.On(m.platform) {
				if !b.FromOS {
					keep = append(keep, b)
				}
			}
			a.SetOn(m.platform, keep)
		}
		for id, k := range want {
			a := keys.action(id)
			reserve := ReservePage
			if bs := a.On(m.platform); len(bs) > 0 {
				reserve = bs[0].Reserve
			}
			a.SetOn(m.platform, append(a.On(m.platform), Binding{Keys: FormatKeys(m.platform, k), Reserve: reserve, FromOS: true}))
		}
	})
}

func (k Keymap) action(id string) *ActionKeys {
	a, ok := k.Actions[id]
	if !ok {
		a = &ActionKeys{}
		k.Actions[id] = a
	}
	return a
}

// edit applies one change. A keymap that follows a template becomes the user's first:
// the template is copied in, then changed.
func (m *Manager) edit(change func(Keymap)) {
	if m.file.Follows != "" {
		owned := m.keys.clone()
		owned.Follows = ""
		m.file = owned
	}
	change(m.file)
	m.keys = m.file
	m.save()
	m.apply()
}

func (m *Manager) save() {
	if m.keymapFile == "" {
		return
	}
	if err := writeJSON(m.keymapFile, m.file); err != nil {
		m.problem("keymap.json couldn't be saved: " + err.Error())
	}
}

// UseTemplate makes the keymap follow a template from now on: a reset to it.
func (m *Manager) UseTemplate(name string) {
	templates := keymapTemplates()
	t, ok := templates[name]
	if !ok {
		return
	}
	m.file = Keymap{Follows: name}
	m.keys = t.clone()
	m.keys.Template = name
	m.missing = nil
	m.save()
	m.apply()
}

// ResetAction puts one action's keys and visibility back as its template has them.
func (m *Manager) ResetAction(id string) {
	t := keymapTemplates()[m.keys.Template]
	m.edit(func(keys Keymap) {
		if a, ok := t.Actions[id]; ok {
			keys.Actions[id] = a.clone()
		} else {
			delete(keys.Actions, id)
		}
	})
	m.dropMissing(id)
}

// AddBinding gives an action another key on this platform. It reports a key it can't read.
func (m *Manager) AddBinding(id, keys, reserve string) error {
	k, err := ParseKeys(keys)
	if err != nil {
		return err
	}
	m.edit(func(km Keymap) {
		a := km.action(id)
		a.SetOn(m.platform, append(a.On(m.platform), Binding{Keys: FormatKeys(m.platform, k), Reserve: validReserve(reserve)}))
	})
	m.dropMissing(id)
	return nil
}

// RemoveBinding takes away an action's i'th key on this platform. A key from the OS
// can't be removed here.
func (m *Manager) RemoveBinding(id string, i int) {
	a, ok := m.keys.Actions[id]
	if !ok || i < 0 || i >= len(a.On(m.platform)) || a.On(m.platform)[i].FromOS {
		return
	}
	m.edit(func(km Keymap) {
		a := km.action(id)
		bs := a.On(m.platform)
		a.SetOn(m.platform, append(append([]Binding(nil), bs[:i]...), bs[i+1:]...))
	})
}

// SetReserve changes who wins an action's i'th key on this platform.
func (m *Manager) SetReserve(id string, i int, reserve string) {
	a, ok := m.keys.Actions[id]
	if !ok || i < 0 || i >= len(a.On(m.platform)) {
		return
	}
	m.edit(func(km Keymap) { km.action(id).On(m.platform)[i].Reserve = validReserve(reserve) })
}

// SetHidden shows or hides an action's menu item. A locked item is always shown.
func (m *Manager) SetHidden(id string, hidden bool) {
	if m.lockedItem(id) {
		return
	}
	m.edit(func(km Keymap) { km.action(id).Hidden = hidden })
}

func validReserve(r string) string {
	switch r {
	case ReserveAlways, ReserveWindowed:
		return r
	}
	return ReservePage
}

func (m *Manager) dropMissing(id string) {
	for i, x := range m.missing {
		if x.ID == id {
			m.missing = append(m.missing[:i], m.missing[i+1:]...)
			m.apply()
			return
		}
	}
}

// DismissMissing clears the update notice.
func (m *Manager) DismissMissing() {
	m.missing = nil
	m.apply()
}

func (m *Manager) lockedItem(id string) bool {
	var found bool
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			if n.Action == id && n.Locked {
				found = true
			}
			walk(n.Items)
		}
	}
	walk(m.tree)
	return found
}
