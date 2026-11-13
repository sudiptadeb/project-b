package app

import (
	"projectb/commands"
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
