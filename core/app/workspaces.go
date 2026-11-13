package app

import (
	"encoding/json"
	"maps"

	"projectb/commands"
	"projectb/engine"
	"projectb/settings"
	"projectb/shell"
	"projectb/timeline"
)

// workspaceList is the list in the Tab menu's New Tab in Workspace: every workspace, by
// name. Choosing one opens a new tab in it, in the window in front.
func (a *App) workspaceList() commands.List {
	return commands.List{
		ID: "workspaces",
		Items: func() []commands.ListItem {
			var items []commands.ListItem
			for _, w := range a.Workspaces.List() {
				items = append(items, commands.ListItem{Key: w.ID, Title: w.Name})
			}
			return items
		},
		Choose: func(id string) {
			a.Shell.Interaction(a.Tabs.Front(), "tab.new", func() { a.Tabs.CreateIn(a.Tabs.Front(), id) })
		},
	}
}

// workspaceShellPart gives a window's shell the workspaces in its snapshot, for the dot on
// each tab in one.
func (a *App) workspaceShellPart() shell.Part {
	return shell.Part{
		Snapshot: func(into map[string]any) { into["workspaces"] = a.Workspaces.List() },
		Handle:   func(engine.WindowID, string, []byte) bool { return false },
	}
}

// workspaceCommand is what the settings page's Workspaces section sends.
type workspaceCommand struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Color string `json:"color"`
	By    int    `json:"by"`
}

// workspaceSettingsPart is the settings page's Workspaces section: the list, each with the
// number of tabs open in it, New workspace with a name and a colour, editing one's name and
// colour, and moving one up or down the list. Every copy has it, a copy on a folder named
// outright too, since the list is the profile folder's. The pages hear the list again
// through its OnChanged, as the shells do, and the counts again when a tab opens or closes
// and they differ.
func (a *App) workspaceSettingsPart() settings.Part {
	counts := a.workspaceTabs()
	a.Tabs.OnListChanged(func() {
		if now := a.workspaceTabs(); !maps.Equal(now, counts) {
			counts = now
			a.Settings.Send(map[string]any{"t": "workspaceTabs", "workspaceTabs": counts})
		}
	})
	return settings.Part{
		Snapshot: func(into map[string]any) {
			into["workspaces"] = a.Workspaces.List()
			into["workspaceTabs"] = counts
		},
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			var c workspaceCommand
			json.Unmarshal(msg, &c)
			var err error
			switch t {
			case "createWorkspace":
				_, err = a.Workspaces.Create(c.Text, c.Color)
			case "renameWorkspace":
				err = a.Workspaces.SetName(c.ID, c.Text)
			// The page's dialog sends a workspace's name and colour together. The name goes
			// first, so a name refused changes nothing.
			case "editWorkspace":
				if err = a.Workspaces.SetName(c.ID, c.Text); err == nil {
					err = a.Workspaces.SetColor(c.ID, c.Color)
				}
			case "moveWorkspace":
				err = a.Workspaces.Move(c.ID, c.By)
			default:
				return false
			}
			if err != nil {
				timeline.Warnf("workspaces: %v", err)
			}
			return true
		},
	}
}

// workspaceTabs counts the tabs open in each workspace, by its id, in every window. A
// workspace with none isn't in it, nor is the default.
func (a *App) workspaceTabs() map[string]int {
	counts := map[string]int{}
	for _, t := range a.Tabs.All() {
		if t.Workspace != "" {
			counts[t.Workspace]++
		}
	}
	return counts
}
