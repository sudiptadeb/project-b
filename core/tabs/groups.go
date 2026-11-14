package tabs

import (
	"regexp"
	"slices"
	"strconv"

	"projectb/engine"
)

// A tab group: tabs of one window and one workspace kept together under a name and a
// colour, which every shell draws its own way (docs/groups.md). A tab is in one group at
// most. A group that drops below two tabs ends; one made with a single tab stays until a
// tab leaves it. Groups live in memory with their tabs.

// GroupColors are the colours a group takes, the workspaces' seven: a new group takes the
// first its window's other groups haven't.
var GroupColors = []string{"#3470d0", "#d03434", "#4f9e9e", "#e8a317", "#8a4fd0", "#2e9d5b", "#34404f"}

var groupColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Group is a copy of one group. Tabs are its tabs' ids in the group's own order.
type Group struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Color     string         `json:"color"`
	Collapsed bool           `json:"collapsed"`
	Tabs      []engine.TabID `json:"tabs"`
}

// GroupsIn returns copies of window w's groups, in the order they were made.
func (m *Manager) GroupsIn(w engine.WindowID) []Group {
	out := []Group{}
	if win := m.windows[w]; win != nil {
		for _, g := range win.groups {
			c := *g
			c.Tabs = slices.Clone(g.Tabs)
			out = append(out, c)
		}
	}
	return out
}

// group finds a group by id, and the window it is in.
func (m *Manager) group(id string) (*Group, *window) {
	for _, win := range m.windows {
		for _, g := range win.groups {
			if g.ID == id {
				return g, win
			}
		}
	}
	return nil, nil
}

// fits says whether tab id can be in a group whose window is w and whose tabs are in
// workspace.
func (m *Manager) fits(id engine.TabID, w engine.WindowID, workspace string) bool {
	t := m.tabs[id]
	return t != nil && m.windowOf[id] == w && t.Workspace == workspace
}

// CreateGroup makes a group of the tabs ids: the first's window and workspace, and those
// of the rest that share them, each leaving any group it was in. An empty color takes
// the next free one. Returns the group's id, or "" when the first is no tab.
func (m *Manager) CreateGroup(ids []engine.TabID, name, color string) string {
	id := m.newGroup(ids, name, color)
	if id != "" {
		m.listChanged()
	}
	return id
}

func (m *Manager) newGroup(ids []engine.TabID, name, color string) string {
	if len(ids) == 0 || m.tabs[ids[0]] == nil {
		return ""
	}
	w := m.windowOf[ids[0]]
	workspace := m.tabs[ids[0]].Workspace
	win := m.windows[w]
	if !groupColor.MatchString(color) {
		color = GroupColors[len(win.groups)%len(GroupColors)]
		for _, c := range GroupColors {
			if !slices.ContainsFunc(win.groups, func(g *Group) bool { return g.Color == c }) {
				color = c
				break
			}
		}
	}
	m.groupSeq++
	g := &Group{ID: "g" + strconv.Itoa(m.groupSeq), Name: name, Color: color}
	for _, t := range ids {
		if m.fits(t, w, workspace) && !slices.Contains(g.Tabs, t) {
			m.leaveGroup(t)
			g.Tabs = append(g.Tabs, t)
			m.tabs[t].Group = g.ID
		}
	}
	win.groups = append(win.groups, g)
	return g.ID
}

// AddToGroup puts tab id at the end of a group, out of any other, when it is in the
// group's window and workspace.
func (m *Manager) AddToGroup(id engine.TabID, group string) {
	if m.join(id, group, 0) {
		m.listChanged()
	}
}

// join puts tab id in group, after the tab after or at the end.
func (m *Manager) join(id engine.TabID, group string, after engine.TabID) bool {
	g, _ := m.group(group)
	if g == nil || len(g.Tabs) == 0 || slices.Contains(g.Tabs, id) {
		return false
	}
	first := g.Tabs[0]
	if !m.fits(id, m.windowOf[first], m.tabs[first].Workspace) {
		return false
	}
	m.leaveGroup(id)
	if i := slices.Index(g.Tabs, after); i >= 0 {
		g.Tabs = slices.Insert(g.Tabs, i+1, id)
	} else {
		g.Tabs = append(g.Tabs, id)
	}
	m.tabs[id].Group = g.ID
	return true
}

// RemoveFromGroup takes tab id out of its group.
func (m *Manager) RemoveFromGroup(id engine.TabID) {
	if t := m.tabs[id]; t != nil && t.Group != "" {
		m.leaveGroup(id)
		m.listChanged()
	}
}

// leaveGroup takes tab id out of its group, which ends if it drops below two tabs.
func (m *Manager) leaveGroup(id engine.TabID) {
	t := m.tabs[id]
	if t == nil || t.Group == "" {
		return
	}
	g, win := m.group(t.Group)
	t.Group = ""
	if g == nil {
		return
	}
	g.Tabs = slices.DeleteFunc(g.Tabs, func(x engine.TabID) bool { return x == id })
	if len(g.Tabs) < 2 {
		m.endGroup(g, win)
	}
}

func (m *Manager) endGroup(g *Group, win *window) {
	for _, t := range g.Tabs {
		if m.tabs[t] != nil {
			m.tabs[t].Group = ""
		}
	}
	win.groups = slices.DeleteFunc(win.groups, func(x *Group) bool { return x == g })
}

// EditGroup changes what of a group's name, colour and whether it is collapsed is given:
// nil leaves it. A colour that isn't "#rrggbb" is ignored.
func (m *Manager) EditGroup(group string, name, color *string, collapsed *bool) {
	g, _ := m.group(group)
	if g == nil {
		return
	}
	if name != nil {
		g.Name = *name
	}
	if color != nil && groupColor.MatchString(*color) {
		g.Color = *color
	}
	if collapsed != nil {
		g.Collapsed = *collapsed
	}
	m.listChanged()
}

// Ungroup ends a group, leaving its tabs where they are.
func (m *Manager) Ungroup(group string) {
	if g, win := m.group(group); g != nil {
		m.endGroup(g, win)
		m.listChanged()
	}
}

// CloseGroup closes a group's tabs.
func (m *Manager) CloseGroup(group string) {
	if g, _ := m.group(group); g != nil {
		for _, id := range slices.Clone(g.Tabs) {
			m.Close(id)
		}
	}
}

// CreateWith opens a new tab beside tab with in its group, starting one of the two when
// it is in none, as a new tab in a group does: at the end of with's window, in with's
// workspace; active when foreground, or behind the active one, as a link opened in a new
// tab is. Returns 0 when with is no tab.
func (m *Manager) CreateWith(with engine.TabID, foreground bool) engine.TabID {
	t := m.tabs[with]
	if t == nil {
		return 0
	}
	w := m.windowOf[with]
	id := m.create(w, t.Workspace)
	if t.Group == "" {
		m.newGroup([]engine.TabID{with, id}, "", "")
	} else {
		g, _ := m.group(t.Group)
		m.join(id, t.Group, g.Tabs[len(g.Tabs)-1])
	}
	m.shown(id, foreground)
	return id
}

// Reorder moves tab id to index in its window's order, the place it has after the move,
// clamped to the window. Its group follows where it lands, as dragging a tab does in
// Chrome: between two tabs of one group it joins that group; out of its own group's run,
// with neither neighbour in it, it leaves. The active tab stays as it was.
func (m *Manager) Reorder(id engine.TabID, index int) {
	win := m.windows[m.windowOf[id]]
	if win == nil {
		return
	}
	from := slices.Index(win.order, id)
	index = max(0, min(index, len(win.order)-1))
	if index == from {
		return
	}
	win.order = slices.Insert(slices.Delete(win.order, from, from+1), index, id)
	var left, right string
	if index > 0 {
		left = m.tabs[win.order[index-1]].Group
	}
	if index < len(win.order)-1 {
		right = m.tabs[win.order[index+1]].Group
	}
	own := m.tabs[id].Group
	switch {
	case left != "" && left == right && left != own:
		m.join(id, left, 0)
	case own != "" && left != own && right != own:
		m.leaveGroup(id)
	}
	if g, _ := m.group(m.tabs[id].Group); g != nil {
		// A group's tabs keep the window's order, so a shell drawing the group sees the
		// move.
		slices.SortFunc(g.Tabs, func(a, b engine.TabID) int {
			return slices.Index(win.order, a) - slices.Index(win.order, b)
		})
	}
	m.listChanged()
}
