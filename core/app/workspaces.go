package app

import (
	"encoding/json"

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
}

// workspaceSettingsPart is the settings page's Workspaces section: the list, New Workspace
// with a name and a colour, and rename. Every copy has it, a copy on a folder named
// outright too, since the list is the profile folder's. The pages hear the list again
// through its OnChanged, as the shells do.
func (a *App) workspaceSettingsPart() settings.Part {
	return settings.Part{
		Snapshot: func(into map[string]any) { into["workspaces"] = a.Workspaces.List() },
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			var c workspaceCommand
			json.Unmarshal(msg, &c)
			var err error
			switch t {
			case "createWorkspace":
				_, err = a.Workspaces.Create(c.Text, c.Color)
			case "renameWorkspace":
				err = a.Workspaces.SetName(c.ID, c.Text)
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
