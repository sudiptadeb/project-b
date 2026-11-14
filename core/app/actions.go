package app

import (
	"fmt"

	"projectb/commands"
	"projectb/engine"
	"projectb/prefs"
	"projectb/relaunch"
	"projectb/shells"
)

// registerActions gives every action its title and the one manager call it makes. Which
// keys run them and where they sit in the menu is the commands manager's business.
func (a *App) registerActions(e engine.Engine) {
	add := func(id, title string, run func()) *commands.Action {
		act := &commands.Action{ID: id, Title: title, Run: run}
		a.actions = append(a.actions, act)
		return act
	}
	active := func() bool { return a.Tabs.Active().ID != 0 }

	a.actions = append(a.actions, copyActions(e, a.quit, func(r engine.WindowRequest) { a.Window.Request(a.Tabs.Front(), r) })...)
	add("app.settings", "Settings…", func() { a.Settings.Open(a.Tabs.Front()) })
	add("app.checkUpdate", "Check for Updates…", func() {
		a.Settings.Open(a.Tabs.Front())
		a.Update.CheckNow()
	})
	add("profile.picker", "Profiles…", func() { relaunch.Copy("--root="+a.profiles.Root(), "--picker") })

	add("tab.new", "New Tab", func() { a.Shell.Interaction(a.Tabs.Front(), "tab.new", func() { a.Tabs.Create(a.Tabs.Front()) }) })
	add("address.focus", "Open Location…", func() { a.Shell.FocusAddressBar(a.Tabs.Front()) })
	add("window.new", "New Window", func() { a.NewWindow("") })
	add("window.close", "Close Window", func() { a.Window.Request(a.Tabs.Front(), engine.WindowClose) })
	// The settings page sits over the tabs like a tab of its own, so closing the tab
	// closes it first.
	add("tab.close", "Close Tab", func() {
		if a.Settings.IsOpen(a.Tabs.Front()) {
			a.Settings.Close(a.Tabs.Front())
			return
		}
		a.Shell.Interaction(a.Tabs.Front(), "tab.close", a.Tabs.CloseActive)
	})

	side := add("view.tabsSide", "Tabs on Side", func() { a.Shell.SwitchTabLayout(prefs.TabsVertical) })
	side.Checked = func() bool { return a.Prefs.TabLayout() == prefs.TabsVertical }
	top := add("view.tabsTop", "Tabs on Top", func() { a.Shell.SwitchTabLayout(prefs.TabsHorizontal) })
	top.Checked = func() bool { return a.Prefs.TabLayout() == prefs.TabsHorizontal }
	// Each shell's own actions, as the registry lists them. One runs only while its shell
	// is the profile's, and asks the shell in front to run it; with another shell on, its
	// keys go on to the page.
	for _, s := range shells.All() {
		for _, sa := range s.Actions {
			act := add(sa.ID, sa.Title, func() { a.Shell.Action(a.Tabs.Front(), sa.ID) })
			act.Enabled = func() bool { return a.Prefs.Shell() == s.ID }
		}
	}

	stop := add("nav.stop", "Stop", a.Tabs.Stop)
	stop.Enabled = func() bool { return a.Tabs.Active().Loading }
	add("nav.reload", "Reload This Page", a.Tabs.Reload).Enabled = active
	add("nav.reloadHard", "Force Reload This Page", a.Tabs.ReloadBypassingCache).Enabled = active
	full := add("view.fullScreen", "Full Screen", func() { a.Window.Request(a.Tabs.Front(), engine.WindowFullScreen) })
	full.Checked = func() bool { return a.Window.FullScreen(a.Tabs.Front()) }
	add("view.zoomReset", "Actual Size", a.Tabs.ZoomReset).Enabled = active
	add("view.zoomIn", "Zoom In", a.Tabs.ZoomIn).Enabled = active
	add("view.zoomOut", "Zoom Out", a.Tabs.ZoomOut).Enabled = active
	source := add("dev.viewSource", "View Source", func() { a.Tabs.Open("view-source:" + a.Tabs.Active().URL) })
	source.Enabled = func() bool { return a.Tabs.Active().URL != "" }

	// F6 moves between the shell and the active tab's page, whichever had the keyboard.
	nextPane := func() {
		if a.keyIn == engine.SurfaceShell {
			e.FocusTab(a.Tabs.ActiveID())
		} else {
			a.Shell.FocusAddressBar(a.Tabs.Front())
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
	add("tab.moveToNewWindow", "Move Tab to New Window", func() { a.NewWindowWith(a.Tabs.ActiveID()) }).Enabled = active
	for i := range 8 {
		add(fmt.Sprintf("tab.select%d", i+1), fmt.Sprintf("Tab %d", i+1), func() { a.Tabs.ActivateAt(i) })
	}
	add("tab.selectLast", "Last Tab", a.Tabs.ActivateLast)

	for _, act := range a.actions {
		a.Commands.Register(*act)
	}
}

// copyActions are the actions every copy has, whether it is on a profile or picking one:
// the app's own, text editing, and what the window in front's buttons do, which request
// asks of it.
func copyActions(e engine.Engine, quit func(), request func(engine.WindowRequest)) []*commands.Action {
	var out []*commands.Action
	add := func(id, title string, run func()) {
		out = append(out, &commands.Action{ID: id, Title: title, Run: run})
	}
	add("app.about", "About Project B", func() { e.AppCommand(engine.AppAbout) })
	add("app.hide", "Hide Project B", func() { e.AppCommand(engine.AppHide) })
	add("app.hideOthers", "Hide Others", func() { e.AppCommand(engine.AppHideOthers) })
	add("app.showAll", "Show All", func() { e.AppCommand(engine.AppShowAll) })
	add("app.quit", "Quit Project B", quit)
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
	add("window.minimize", "Minimize", func() { request(engine.WindowMinimise) })
	add("window.zoom", "Zoom", func() { request(engine.WindowZoom) })
	add("window.bringAllToFront", "Bring All to Front", func() { e.AppCommand(engine.AppBringAllToFront) })
	return out
}
