package app

import (
	"encoding/json"
	"net/url"
	"strings"

	"projectb/engine"
	"projectb/shell"
	"projectb/tabs"
)

// A right-click in a page (docs/daily-driver.md): Go works out what can run there and sends
// it to the window's shell, which draws the menu and says which action was chosen. Go runs
// it. A window has one menu at most, and Go drops it when its tab closes, navigates or stops
// being the one shown.

// contextMenu is what a shell is sent: what the press was on, and what can run there.
type contextMenu struct {
	ID  int          `json:"id"`
	Tab engine.TabID `json:"tab"`
	engine.ContextMenu
	Actions []menuAction `json:"actions"`
	// url is the page's address when the menu opened, to drop it once the tab navigates.
	url string
}

// menuAction is one thing a menu can run: its id, which never changes once released, the
// title a shell shows, the group it belongs to, and a submenu's own actions.
type menuAction struct {
	ID      string       `json:"id"`
	Title   string       `json:"title"`
	Group   string       `json:"group"`
	Enabled bool         `json:"enabled"`
	Actions []menuAction `json:"actions,omitempty"`
}

// quoteLimit is how much of a selection a title quotes.
const quoteLimit = 24

// contextMenus keeps each window's open menu.
type contextMenus struct {
	a    *App
	e    engine.Engine
	last int
	open map[engine.WindowID]*contextMenu
}

func newContextMenus(a *App, e engine.Engine) *contextMenus {
	c := &contextMenus{a: a, e: e, open: map[engine.WindowID]*contextMenu{}}
	a.Tabs.OnChanged(func(t tabs.Tab) {
		if m := c.open[a.Tabs.WindowOf(t.ID)]; m != nil && m.Tab == t.ID && t.URL != m.url {
			c.drop(a.Tabs.WindowOf(t.ID))
		}
	})
	a.Tabs.OnListChanged(func() {
		for w, m := range c.open {
			if a.Tabs.WindowOf(m.Tab) != w || a.Tabs.ActiveIn(w) != m.Tab {
				c.drop(w)
			}
		}
	})
	return c
}

// shellPart takes the shell's answers.
func (c *contextMenus) shellPart() shell.Part {
	return shell.Part{
		Snapshot: func(map[string]any) {},
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			var r struct {
				ID   int    `json:"id"`
				Item string `json:"item"`
			}
			switch t {
			case "contextMenuChoose":
				json.Unmarshal(msg, &r)
				if m := c.open[w]; m != nil && m.ID == r.ID {
					c.drop(w)
					c.run(m, r.Item)
				}
			case "contextMenuClose":
				json.Unmarshal(msg, &r)
				if m := c.open[w]; m != nil && m.ID == r.ID {
					c.drop(w)
				}
			default:
				return false
			}
			return true
		},
	}
}

// show opens a menu for a right-click in tab's page, in place of any its window had.
func (c *contextMenus) show(tab engine.TabID, cm engine.ContextMenu) {
	t, ok := c.a.Tabs.Get(tab)
	w := c.a.Tabs.WindowOf(tab)
	if !ok || w == 0 {
		return
	}
	c.last++
	m := &contextMenu{ID: c.last, Tab: tab, ContextMenu: cm, Actions: c.actions(t, cm), url: t.URL}
	c.open[w] = m
	c.a.Shell.SendTo(w, map[string]any{"t": "contextMenu", "contextMenu": m})
}

func (c *contextMenus) drop(w engine.WindowID) {
	if c.open[w] == nil {
		return
	}
	delete(c.open, w)
	c.a.Shell.SendTo(w, map[string]any{"t": "contextMenu", "contextMenu": nil})
}

// actions is what can run for a press on cm in tab t's page, by group.
func (c *contextMenus) actions(t tabs.Tab, cm engine.ContextMenu) []menuAction {
	var list []menuAction
	add := func(group, id, title string, enabled bool) {
		list = append(list, menuAction{ID: id, Title: title, Group: group, Enabled: enabled})
	}
	if cm.Link != "" {
		if opens(cm.Link) {
			add("link", "link.newTab", "Open Link in New Tab", true)
			var others []menuAction
			for _, ws := range c.a.Workspaces.List() {
				if ws.ID != t.Workspace {
					others = append(others, menuAction{ID: "link.workspace." + ws.ID, Title: ws.Name, Group: "link", Enabled: true})
				}
			}
			if len(others) > 0 {
				list = append(list, menuAction{ID: "link.workspace", Title: "Open Link in Workspace", Group: "link", Enabled: true, Actions: others})
			}
			add("link", "link.newWindow", "Open Link in New Window", true)
		}
		add("link", "link.copy", "Copy Link", true)
	}
	if cm.Src != "" && cm.Media == "image" {
		if opens(cm.Src) {
			add("image", "image.newTab", "Open Image in New Tab", true)
		}
		add("image", "image.copyAddress", "Copy Image Address", true)
	}
	selected := strings.TrimSpace(cm.Selection) != ""
	if cm.Editable {
		add("edit", "edit.undo", "Undo", true)
		add("edit", "edit.redo", "Redo", true)
		add("edit", "edit.cut", "Cut", selected)
		add("edit", "edit.copy", "Copy", selected)
		add("edit", "edit.paste", "Paste", true)
		add("edit", "edit.selectAll", "Select All", true)
	} else if selected {
		add("selection", "selection.copy", "Copy", true)
	}
	if selected {
		add("selection", "selection.search", "Search for “"+quote(cm.Selection)+"”", true)
	}
	if cm.Link == "" && cm.Src == "" && !selected && !cm.Editable {
		add("page", "page.back", "Back", t.CanGoBack)
		add("page", "page.forward", "Forward", t.CanGoForward)
		add("page", "page.reload", "Reload", true)
	}
	return list
}

// run does what action id of menu m says.
func (c *contextMenus) run(m *contextMenu, id string) {
	t, ok := c.a.Tabs.Get(m.Tab)
	if !ok || !m.has(id) {
		return
	}
	edit := func(cmd engine.EditCommand) {
		// The menu may have taken the keyboard: the page gets it back first.
		c.e.FocusTab(m.Tab)
		c.e.Edit(cmd)
	}
	switch {
	case id == "link.newTab":
		c.a.Tabs.OpenFrom(m.Tab, m.Link, t.Workspace, false)
	case id == "link.newWindow":
		c.a.NewWindowWith(c.a.Tabs.OpenFrom(m.Tab, m.Link, t.Workspace, false))
	case strings.HasPrefix(id, "link.workspace."):
		c.a.Tabs.OpenFrom(0, m.Link, strings.TrimPrefix(id, "link.workspace."), false)
	case id == "link.copy":
		c.e.WriteClipboard(m.Link)
	case id == "image.newTab":
		c.a.Tabs.OpenFrom(m.Tab, m.Src, t.Workspace, false)
	case id == "image.copyAddress":
		c.e.WriteClipboard(m.Src)
	case id == "selection.copy":
		c.e.WriteClipboard(m.Selection)
	case id == "selection.search":
		c.a.Tabs.OpenFrom(m.Tab, tabs.SearchURL(m.Selection), t.Workspace, true)
	case id == "edit.undo":
		edit(engine.EditUndo)
	case id == "edit.redo":
		edit(engine.EditRedo)
	case id == "edit.cut":
		edit(engine.EditCut)
	case id == "edit.copy":
		edit(engine.EditCopy)
	case id == "edit.paste":
		edit(engine.EditPaste)
	case id == "edit.selectAll":
		edit(engine.EditSelectAll)
	case id == "page.back":
		c.e.GoBack(m.Tab)
	case id == "page.forward":
		c.e.GoForward(m.Tab)
	case id == "page.reload":
		c.a.Tabs.ReloadTab(m.Tab, false)
	}
}

// has says whether the menu offered action id, enabled.
func (m *contextMenu) has(id string) bool {
	var find func([]menuAction) bool
	find = func(list []menuAction) bool {
		for _, a := range list {
			if a.ID == id && a.Enabled && a.Actions == nil || find(a.Actions) {
				return true
			}
		}
		return false
	}
	return find(m.Actions)
}

// opens says whether a tab can load u: a web page, a file, or data, not script or mail.
func opens(u string) bool {
	p, err := url.Parse(u)
	if err != nil {
		return false
	}
	switch p.Scheme {
	case "http", "https", "file", "data", "blob":
		return true
	}
	return false
}

// quote is the start of a selection, on one line, for a title.
func quote(s string) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) > quoteLimit {
		return strings.TrimSpace(string(r[:quoteLimit])) + "…"
	}
	return string(r)
}
