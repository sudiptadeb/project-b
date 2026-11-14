package commands

// View is what the settings page shows about keys: every action by menu, with its keys
// on this platform.
type View struct {
	Platform  string    `json:"platform"`
	Templates []string  `json:"templates"`
	Template  string    `json:"template"`
	Following bool      `json:"following"`
	Groups    []Group   `json:"groups"`
	Missing   []Missing `json:"missing"`
	Gone      []string  `json:"gone"`
	Problems  []string  `json:"problems"`
}

// Group is one menu's actions, its title the path to it ("View › Developer").
type Group struct {
	Title   string       `json:"title"`
	Actions []ActionView `json:"actions"`
}

// ActionView is one action as the settings page shows it.
type ActionView struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Hidden bool   `json:"hidden"`
	// Locked: its menu item can't be hidden.
	Locked bool          `json:"locked"`
	Keys   []BindingView `json:"keys"`
	// Changed: its keys on this platform, or whether it is hidden, aren't its template's,
	// so a reset or another template would lose something.
	Changed bool `json:"changed"`
}

// BindingView is one key. Index is its place in the keymap's list, -1 for a locked one.
type BindingView struct {
	Index   int    `json:"index"`
	Keys    string `json:"keys"`
	Reserve string `json:"reserve"`
	FromOS  bool   `json:"fromOS"`
	Locked  bool   `json:"locked"`
	// Clash names the action that also has this key and wins it, "" when none does, and
	// ClashID is its id, so the page can take the user there.
	Clash   string `json:"clash"`
	ClashID string `json:"clashID"`
}

// View returns what the settings page shows.
func (m *Manager) View() View {
	v := View{
		Platform:  m.platform,
		Templates: TemplateNames(),
		Template:  m.keys.Template,
		Following: m.file.Follows != "",
		Missing:   append([]Missing{}, m.missing...),
		Gone:      []string{},
		Problems:  append([]string{}, m.problems...),
	}
	// An owned keymap is compared with its template; a following one is the template.
	var template *Keymap
	if m.file.Follows == "" {
		t := keymapTemplates()[m.keys.Template]
		template = &t
	}
	inTree := map[string]bool{}
	var walk func(path string, nodes []Node, locked bool)
	walk = func(path string, nodes []Node, parentLocked bool) {
		// The group's index, not a pointer: a submenu walked in between adds groups, and
		// the slice may move.
		g := -1
		for _, n := range nodes {
			if n.Only != "" && n.Only != m.platform {
				continue
			}
			if n.Action == "" {
				if len(n.Items) > 0 {
					sub := n.Title
					if path != "" {
						sub = path + " › " + n.Title
					}
					walk(sub, n.Items, n.Locked)
				}
				continue
			}
			a, ok := m.actions[n.Action]
			if !ok {
				continue
			}
			inTree[a.ID] = true
			if g < 0 {
				v.Groups = append(v.Groups, Group{Title: path})
				g = len(v.Groups) - 1
			}
			v.Groups[g].Actions = append(v.Groups[g].Actions, m.actionView(a, n.Locked, template))
		}
	}
	walk("", m.tree, false)
	for id := range m.keys.Actions {
		if _, ok := m.actions[id]; !ok && !inTree[id] {
			v.Gone = append(v.Gone, id)
		}
	}
	return v
}

func (m *Manager) actionView(a *Action, locked bool, template *Keymap) ActionView {
	v := ActionView{ID: a.ID, Title: a.Title, Locked: locked, Keys: []BindingView{}}
	if template != nil {
		v.Changed = m.differs(m.keys.Actions[a.ID], template.Actions[a.ID])
	}
	if k, ok := m.keys.Actions[a.ID]; ok {
		v.Hidden = k.Hidden && !locked
	}
	add := func(b Binding, index int, isLocked bool) {
		bv := BindingView{Index: index, Keys: b.Keys, Reserve: b.Reserve, FromOS: b.FromOS, Locked: isLocked}
		if k, err := ParseKeys(b.Keys); err == nil {
			if hit, ok := m.index[k]; ok && hit.id != a.ID {
				bv.Clash = m.titleOf(hit.id)
				bv.ClashID = hit.id
			}
		}
		v.Keys = append(v.Keys, bv)
	}
	if k, ok := m.locked.Actions[a.ID]; ok {
		for _, b := range k.On(m.platform) {
			add(b, -1, true)
		}
	}
	if k, ok := m.keys.Actions[a.ID]; ok {
		for i, b := range k.On(m.platform) {
			add(b, i, false)
		}
	}
	return v
}

// differs says whether an action's keys on this platform, or its hidden mark, aren't its
// template's. A key from the OS is the OS's, not a change the user made here, so it
// doesn't count; nil is an action with no keys.
func (m *Manager) differs(mine, theirs *ActionKeys) bool {
	if mine == nil {
		mine = &ActionKeys{}
	}
	if theirs == nil {
		theirs = &ActionKeys{}
	}
	if mine.Hidden != theirs.Hidden {
		return true
	}
	var own []Binding
	for _, b := range mine.On(m.platform) {
		if !b.FromOS {
			own = append(own, b)
		}
	}
	want := theirs.On(m.platform)
	if len(own) != len(want) {
		return true
	}
	for i := range own {
		if own[i].Keys != want[i].Keys || own[i].Reserve != want[i].Reserve {
			return true
		}
	}
	return false
}

func (m *Manager) titleOf(id string) string {
	if a, ok := m.actions[id]; ok {
		return a.Title
	}
	return id
}

// ShownKeys is the key each action's menu item shows, by action id, for the shell's hints.
func (m *Manager) ShownKeys() map[string]string {
	out := map[string]string{}
	for id := range m.actions {
		if k := m.shownKey(id); k != nil {
			out[id] = FormatKeys(m.platform, *k)
		}
	}
	return out
}
