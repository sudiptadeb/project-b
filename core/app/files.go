package app

import (
	"projectb/engine"
)

// FileChooser lets a page open the file panel while its address is one policy allows,
// in the folder files were last chosen from in this run (docs/daily-driver.md).
func (a *App) FileChooser(r engine.FileChooserRequest) engine.FileChooserAnswer {
	t, ok := a.Tabs.Get(r.Tab)
	if !ok || !a.Policy.IsAllowedURL(t.URL) {
		return engine.FileChooserAnswer{}
	}
	return engine.FileChooserAnswer{Allow: true, Folder: a.lastFolder}
}

// FilesChosen keeps the folder files came from, for the next panel.
func (a *App) FilesChosen(_ engine.TabID, _ int, folder string) { a.lastFolder = folder }
