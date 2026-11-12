package app

import (
	"fmt"

	"projectb/commands"
	"projectb/engine"
	"projectb/prefs"
)

// registerActions gives every action its title and the one manager call it makes. Which
// keys run them and where they sit in the menu is the commands manager's business.
func (a *App) registerActions(e engine.Engine, closeWindow func()) {
	add := func(id, title string, run func()) *commands.Action {
		act := &commands.Action{ID: id, Title: title, Run: run}
		a.actions = append(a.actions, act)
		return act
	}
	active := func() bool { return a.Tabs.Active().ID != 0 }

	add("app.about", "About Project B", func() { e.AppCommand(engine.AppAbout) })
	add("app.settings", "Settings…", func() { a.Settings.Open(theWindow) })
	add("app.checkUpdate", "Check for Updates…", func() {
		a.Settings.Open(theWindow)
		a.Update.CheckNow()
	})
	add("app.hide", "Hide Project B", func() { e.AppCommand(engine.AppHide) })
	add("app.hideOthers", "Hide Others", func() { e.AppCommand(engine.AppHideOthers) })
	add("app.showAll", "Show All", func() { e.AppCommand(engine.AppShowAll) })
	add("app.quit", "Quit Project B", closeWindow)

	add("tab.new", "New Tab", func() { a.Shell.Interaction(theWindow, "tab.new", func() { a.Tabs.Create() }) })
	add("address.focus", "Open Location…", func() { a.Shell.FocusAddressBar(theWindow) })
	add("window.close", "Close Window", closeWindow)
	// The settings page sits over the tabs like a tab of its own, so closing the tab
	// closes it first.
	add("tab.close", "Close Tab", func() {
		if a.Settings.IsOpen(theWindow) {
			a.Settings.Close(theWindow)
			return
		}
		a.Shell.Interaction(theWindow, "tab.close", a.Tabs.CloseActive)
	})

	for _, c := range []struct {
		id, title string
		cmd       engine.EditCommand
	}{
		{"edit.undo", "Undo", engine.EditUndo},
		{"edit.redo", "Redo", engine.EditRedo},
		{"edit.cut", "Cut", engine.EditCut},
		{"edit.copy", "Copy", engine.EditCopy},
		{"edit.paste", "Paste", engine.EditPaste},
		{"edit.pasteAndMatchStyle", "Paste and Match Style", engine.EditPasteAndMatchStyle},
		{"edit.delete", "Delete", engine.EditDelete},
		{"edit.selectAll", "Select All", engine.EditSelectAll},
	} {
		add(c.id, c.title, func() { e.Edit(c.cmd) })
	}

	side := add("view.tabsSide", "Tabs on Side", func() { a.Shell.SwitchTabLayout(prefs.TabsVertical) })
	side.Checked = func() bool { return a.Prefs.TabLayout() == prefs.TabsVertical }
	top := add("view.tabsTop", "Tabs on Top", func() { a.Shell.SwitchTabLayout(prefs.TabsHorizontal) })
	top.Checked = func() bool { return a.Prefs.TabLayout() == prefs.TabsHorizontal }

	stop := add("nav.stop", "Stop", a.Tabs.Stop)
	stop.Enabled = func() bool { return a.Tabs.Active().Loading }
	add("nav.reload", "Reload This Page", a.Tabs.Reload).Enabled = active
	add("nav.reloadHard", "Force Reload This Page", a.Tabs.ReloadBypassingCache).Enabled = active
	full := add("view.fullScreen", "Full Screen", func() { a.Window.Request(theWindow, engine.WindowFullScreen) })
	full.Checked = func() bool { return a.Window.FullScreen(theWindow) }
	add("view.zoomReset", "Actual Size", a.Tabs.ZoomReset).Enabled = active
	add("view.zoomIn", "Zoom In", a.Tabs.ZoomIn).Enabled = active
	add("view.zoomOut", "Zoom Out", a.Tabs.ZoomOut).Enabled = active
	source := add("dev.viewSource", "View Source", func() { a.Tabs.Open("view-source:" + a.Tabs.Active().URL) })
	source.Enabled = func() bool { return a.Tabs.Active().URL != "" }

	// F6 moves between the shell and the page, whichever had the keyboard.
	nextPane := func() {
		if a.keyIn == engine.SurfaceShell {
			e.FocusPage()
		} else {
			a.Shell.FocusAddressBar(theWindow)
		}
	}
	add("focus.nextPane", "Focus Next Pane", nextPane)
	add("focus.previousPane", "Focus Previous Pane", nextPane)

	back := add("nav.back", "Back", a.Tabs.Back)
	back.Enabled = func() bool { return a.Tabs.Active().CanGoBack }
	forward := add("nav.forward", "Forward", a.Tabs.Forward)
	forward.Enabled = func() bool { return a.Tabs.Active().CanGoForward }

	add("tab.next", "Select Next Tab", func() { a.Tabs.ActivateNext(false) })
	add("tab.previous", "Select Previous Tab", func() { a.Tabs.ActivateNext(true) })
	// Held like the OS's app switcher: each press goes one tab further back in recent
	// order, and letting go of the modifier settles on the tab it reached.
	add("tab.recentNext", "Select Recent Tab", func() { a.Tabs.CycleRecent(false) }).Release = a.Tabs.EndCycle
	add("tab.recentPrevious", "Select Recent Tab Backward", func() { a.Tabs.CycleRecent(true) }).Release = a.Tabs.EndCycle
	add("tab.duplicate", "Duplicate Tab", a.Tabs.Duplicate).Enabled = active
	for i := range 8 {
		add(fmt.Sprintf("tab.select%d", i+1), fmt.Sprintf("Tab %d", i+1), func() { a.Tabs.ActivateAt(i) })
	}
	add("tab.selectLast", "Last Tab", a.Tabs.ActivateLast)

	add("window.minimize", "Minimize", func() { a.Window.Request(theWindow, engine.WindowMinimise) })
	add("window.zoom", "Zoom", func() { a.Window.Request(theWindow, engine.WindowZoom) })
	add("window.bringAllToFront", "Bring All to Front", func() { e.AppCommand(engine.AppBringAllToFront) })

	for _, act := range a.actions {
		a.Commands.Register(*act)
	}
}
