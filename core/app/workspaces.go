package app

import (
	"projectb/commands"
	"projectb/engine"
	"projectb/shell"
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
