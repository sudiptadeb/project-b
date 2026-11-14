// Package app builds the components and wires them together. All hook subscriptions
// live in Start, so this one function answers "who listens to what".
package app

import (
	"projectb/commands"
	"projectb/crash"
	"projectb/engine"
	"projectb/flags"
	"projectb/history"
	"projectb/policy"
	"projectb/prefs"
	"projectb/profiles"
	"projectb/relaunch"
	"projectb/settings"
	"projectb/shell"
	"projectb/shelldata"
	"projectb/shells"
	"projectb/suggest"
	"projectb/tabs"
	"projectb/timeline"
	"projectb/update"
	"projectb/window"
	"projectb/workspaces"
)

// Config is what the app needs to start.
type Config struct {
	ProfileDir string
	// Profiles is the list of profiles, and Profile this copy's: its id is "" for a folder
	// named outright, which isn't listed.
	Profiles     *profiles.Manager
	Profile      string
	BlockedHosts []string
	Update       update.Config
	// CrashReports is the folder macOS writes its crash reports to.
	CrashReports string
}

// App holds the managers and is the engine's listener: each function only forwards.
type App struct {
	Crash    *crash.Manager
	Tabs     *tabs.Manager
	History  *history.Manager
	Suggest  *suggest.Manager
	Policy   *policy.Manager
	Window   *window.Manager
	Prefs    *prefs.Manager
	Flags    *flags.Manager
	Update   *update.Manager
	Shell    *shell.Manager
	Commands *commands.Manager
	Settings *settings.Manager
	// Workspaces is the profile's list of workspaces, beside its prefs.
	Workspaces *workspaces.Manager
	// ShellData is what each shell saves for the whole profile, beside its prefs.
	ShellData *shelldata.Manager

	actions []*commands.Action
	// keyIn is which page had the keyboard at the last key press.
	keyIn engine.Surface
	// lastWindow is the id of the window made last; Go numbers them from 1.
	lastWindow engine.WindowID
	quit       func()
	profiles   *profiles.Manager
}

// starters are what a build tag adds to Start. A tagged file appends one when it loads,
// so nothing here names it; each builds its managers and wires them to the rest.
var starters []func(a *App, e engine.Engine, cfg Config)

// Start builds everything, on the main thread. The caller then gives the returned App
// to the engine as its listener.
func Start(e engine.Engine, cfg Config) *App {
	a := &App{profiles: cfg.Profiles}
	a.Crash = crash.New(cfg.ProfileDir, cfg.CrashReports)
	a.Policy = policy.New(cfg.BlockedHosts)
	a.History = history.New(cfg.ProfileDir, e.Post)
	a.Suggest = suggest.New(a.History)
	a.Workspaces = workspaces.New(cfg.ProfileDir)
	a.ShellData = shelldata.New(cfg.ProfileDir)
	a.Tabs = tabs.New(e, a.Workspaces.First)
	a.Prefs = prefs.New(cfg.ProfileDir)
	a.Flags = flags.New(cfg.ProfileDir)
	a.Window = window.New(e, engine.ShellPage(a.Prefs.Shell()), a.Prefs.FrameColor(), themeOf(a.Prefs.Theme()), a.Tabs.Front)
	a.Tabs.OnFrontChanged(a.Window.HandleFrontChanged)
	a.Tabs.OnMoved(a.Window.HandleTabMoved)
	first := a.openWindow()
	a.quit = func() { e.AppCommand(engine.AppQuit) }
	a.Update = update.New(cfg.Update, e.Post, a.quit)
	a.Commands = commands.New(e, cfg.ProfileDir, func() bool { return a.Window.FullScreen(a.Tabs.Front()) })
	a.Commands.RegisterList(a.profileList(cfg))
	a.Commands.RegisterList(a.workspaceList())
	a.Commands.RegisterList(a.shellList())
	a.Shell = shell.New(e, a.Tabs, a.Suggest, a.Window, a.Prefs, a.Update, a.Crash, a.Commands.Run, a.Commands.ShownKeys, a.Commands.Platform())
	a.Shell.AddPart(a.workspaceShellPart(e))
	a.Shell.AddPart(a.shellDataPart())
	// Every action a shell can run by name, for a shell that lists them, as Glide's
	// Command Bar does.
	a.Shell.AddPart(shell.Part{
		Snapshot: func(into map[string]any) { into["actions"] = a.Commands.Titles() },
		Handle:   func(engine.WindowID, string, []byte) bool { return false },
	})
	a.Settings = settings.New(e, a.Window, a.Prefs, a.Flags, a.Update, a.Commands)
	a.Settings.OnRelaunch(func() { relaunch.Self(); a.quit() })
	a.Settings.AddPart(a.workspaceSettingsPart(e))
	a.registerActions(e)

	a.Tabs.AddGate(a.Policy.IsAllowedURL)
	a.Tabs.OnNavigationFinished(func(t tabs.Tab) { a.History.AddVisit(t.URL, t.Title) })
	a.Tabs.OnTitleChanged(func(t tabs.Tab) { a.History.UpdateTitle(t.URL, t.Title) })
	a.Tabs.OnChanged(a.Shell.TabChanged)
	a.Tabs.OnListChanged(a.Shell.TabsChanged)
	// Closing a window's last tab closes the window.
	a.Tabs.OnEmpty(func(w engine.WindowID) { a.Window.Request(w, engine.WindowClose) })
	a.Window.OnControlsChanged(a.Shell.WindowControlsChanged)
	a.Window.OnControlsChanged(a.Settings.WindowControlsChanged)
	a.Window.OnColorChanged(a.Shell.WindowColorChanged)
	a.Window.OnColorChanged(a.Settings.WindowColorChanged)
	a.Prefs.OnFrameColorChanged(a.Window.SetColor)
	// The profile's colour is its frame colour: on its Dock icon, and in the list.
	e.SetDockColor(a.Prefs.FrameColor())
	a.Prefs.OnFrameColorChanged(e.SetDockColor)
	if cfg.Profile != "" {
		a.Prefs.OnFrameColorChanged(func(c string) {
			if err := cfg.Profiles.SetColor(cfg.Profile, c); err != nil {
				timeline.Errorf("profiles: %v", err)
			}
		})
		a.Settings.AddPart(a.profileSettingsPart(cfg.Profiles, cfg.Profile))
	}
	// A change to the workspaces reaches every shell at once, for the dots, and every
	// settings page; the Tab menu reads the list as it opens. The engine hears which is
	// first, at start and whenever that changes, for the tabs it makes on its own.
	firstWorkspace := a.Workspaces.First()
	e.SetFirstWorkspace(firstWorkspace)
	a.Workspaces.OnChanged(func() {
		m := map[string]any{"t": "workspaces", "workspaces": a.Workspaces.List()}
		a.Shell.Send(m)
		a.Settings.Send(m)
		if now := a.Workspaces.First(); now != firstWorkspace {
			firstWorkspace = now
			e.SetFirstWorkspace(now)
		}
	})
	a.Prefs.OnThemeChanged(func(t string) { a.Window.SetTheme(themeOf(t)) })
	a.Prefs.OnThemeChanged(a.Shell.ThemeChanged)
	a.Prefs.OnThemeChanged(a.Settings.ThemeChanged)
	// Another shell loads in every window at once. The old one's regions go, as after a
	// crash, and the tabs stay where they were until the new one reports.
	a.Prefs.OnShellChanged(func(id string) {
		a.Shell.ShellChanged()
		a.Window.SetPage(engine.ShellPage(id))
	})
	a.Prefs.OnTabLayoutChanged(a.Shell.TabLayoutChanged)
	a.Prefs.OnTabLayoutChanged(a.Settings.TabLayoutChanged)
	a.Prefs.OnAddressBarPositionChanged(a.Shell.AddressBarPositionChanged)
	a.Prefs.OnAddressBarPositionChanged(a.Settings.AddressBarPositionChanged)
	a.Update.OnAutoChanged(a.Settings.AutoUpdateChanged)
	a.Update.OnChanged(a.Shell.UpdateChanged)
	a.Update.OnChanged(a.Settings.UpdateChanged)
	a.Commands.OnChanged(a.Settings.KeyboardChanged)
	a.Flags.OnChanged(a.Settings.FlagsChanged)
	a.Commands.OnChanged(a.Shell.KeysChanged)
	// What the menu shows enabled or checked follows the window in front: its active tab
	// and full screen, the layout, and the shell.
	a.Tabs.OnChanged(func(tabs.Tab) { a.Commands.RefreshMenu() })
	a.Tabs.OnListChanged(a.Commands.RefreshMenu)
	a.Prefs.OnTabLayoutChanged(func(string) { a.Commands.RefreshMenu() })
	a.Prefs.OnShellChanged(func(string) { a.Commands.RefreshMenu() })
	// Keyboard settings lists only the shell in use's own actions.
	a.Prefs.OnShellChanged(func(string) { a.Settings.KeyboardChanged() })
	a.Window.OnControlsChanged(func(engine.WindowID, engine.Rect, float64) { a.Commands.RefreshMenu() })
	a.Commands.Load()
	for _, start := range starters {
		start(a, e, cfg)
	}

	a.History.Load(nil)
	a.Tabs.Create(first)
	a.Update.Start()
	return a
}

// profileList is the Profiles menu's list: every profile, with a tick on this one.
// Choosing one starts a copy of the app on it, which hands over to the copy already
// running on it, if any.
func (a *App) profileList(cfg Config) commands.List {
	return commands.List{
		ID: "profiles",
		Items: func() []commands.ListItem {
			list, err := cfg.Profiles.List()
			if err != nil {
				timeline.Errorf("profiles: %v", err)
			}
			var items []commands.ListItem
			for _, p := range list {
				items = append(items, commands.ListItem{Key: p.ID, Title: p.Name, Checked: p.ID == cfg.Profile})
			}
			return items
		},
		Choose: func(id string) { relaunch.Copy("--root="+cfg.Profiles.Root(), "--profile="+id) },
	}
}

// shellList is View, Shell: every shell the user can choose, with a tick on the one in use.
// Choosing one makes it the profile's, and every window loads it.
func (a *App) shellList() commands.List {
	return commands.List{
		ID: "shells",
		Items: func() []commands.ListItem {
			var items []commands.ListItem
			for _, s := range shells.Listed() {
				items = append(items, commands.ListItem{Key: s.ID, Title: s.Name, Checked: s.ID == a.Prefs.Shell()})
			}
			return items
		},
		Choose: a.Prefs.SetShell,
	}
}

// openWindow makes a window with no tabs, a little down and right of the window in
// front, and puts it in front.
func (a *App) openWindow() engine.WindowID {
	near := a.Tabs.Front()
	a.lastWindow++
	w := a.lastWindow
	a.Window.Open(w, near)
	a.Tabs.AddWindow(w)
	return w
}

// NewWindow opens a window with one tab, in the first workspace, and returns both: a new
// tab, or one loading u when it isn't "".
func (a *App) NewWindow(u string) (engine.WindowID, engine.TabID) {
	w := a.openWindow()
	if u == "" {
		return w, a.Tabs.Create(w)
	}
	// The new window is in front, where OpenIn opens.
	return w, a.Tabs.OpenIn(u, true)
}

// NewWindowWith opens a window holding a tab moved out of its window, with no tab of its
// own, and returns it. A tab that is its window's only one has a window already: it stays,
// and that window is the one returned.
func (a *App) NewWindowWith(tab engine.TabID) engine.WindowID {
	if w := a.Tabs.WindowOf(tab); len(a.Tabs.AllIn(w)) == 1 {
		return w
	}
	w := a.openWindow()
	a.Tabs.Move(tab, w, 0)
	return w
}

// Stop flushes what's pending and says the run ended cleanly. Call it on the way out.
func (a *App) Stop() {
	a.Update.Stop()
	a.History.Close()
	a.Crash.Stop()
}

func (a *App) ShellMessage(w engine.WindowID, msg []byte) { a.Shell.Handle(w, msg) }

func (a *App) NavigationStarting(tab engine.TabID, url string) bool {
	return a.Tabs.HandleNavigationStarting(tab, url)
}

func (a *App) NavigationFinished(tab engine.TabID, url string) {
	a.Tabs.HandleNavigationFinished(tab, url)
}

func (a *App) NavigationFailed(tab engine.TabID, url, reason string) {
	a.Tabs.HandleNavigationFailed(tab, url, reason)
}

func (a *App) HistoryChanged(tab engine.TabID, canGoBack, canGoForward bool) {
	a.Tabs.HandleHistoryChanged(tab, canGoBack, canGoForward)
}

func (a *App) TitleChanged(tab engine.TabID, title string) { a.Tabs.HandleTitleChanged(tab, title) }

func (a *App) LoadingChanged(tab engine.TabID, loading bool) {
	a.Tabs.HandleLoadingChanged(tab, loading)
}

func (a *App) ContextMenu(tab engine.TabID, m engine.ContextMenu) {
	a.Shell.ContextMenu(tab, m)
}

func (a *App) ScrollDirectionChanged(tab engine.TabID, down bool) {
	a.Shell.ScrollDirectionChanged(tab, down)
}

func (a *App) LoadProgressChanged(tab engine.TabID, progress float64) {
	a.Tabs.HandleLoadProgressChanged(tab, progress)
}

func (a *App) PageCrashed(tab engine.TabID) { a.Tabs.HandlePageCrashed(tab) }

func (a *App) FaviconChanged(tab engine.TabID, dataURL string) {
	a.Tabs.HandleFaviconChanged(tab, dataURL)
}

func (a *App) TabOpened(opener, tab engine.TabID, url, workspace string, foreground, newWindow bool) {
	// A tab nothing opened goes in the window in front, which a new one is.
	if newWindow {
		a.openWindow()
	}
	a.Tabs.HandleTabOpened(opener, tab, url, workspace, foreground)
}

func (a *App) TabFocused(tab engine.TabID) { a.Tabs.HandleTabFocused(tab) }

func (a *App) CloseRequested(tab engine.TabID) { a.Tabs.Close(tab) }

// ActivateRequested shows a tab in its window, and brings the window to the front.
func (a *App) ActivateRequested(tab engine.TabID) {
	a.Tabs.Activate(tab)
	a.Window.Request(a.Tabs.WindowOf(tab), engine.WindowFront)
}

// Handover is the request of a copy started on this profile while this one runs: the
// window used last comes to the front, and each URL opens in a new tab there.
func (a *App) Handover(urls []string) {
	timeline.Infof("a copy started on this profile handed over %d URLs", len(urls))
	a.Window.Request(a.Tabs.Front(), engine.WindowFront)
	for _, u := range urls {
		a.Tabs.Open(u)
	}
}

// themeOf is the window's theme for a theme pref.
func themeOf(t string) engine.Theme {
	switch t {
	case prefs.ThemeLight:
		return engine.ThemeLight
	case prefs.ThemeDark:
		return engine.ThemeDark
	}
	return engine.ThemeSystem
}

// Key hands a key press to the commands manager, which decides what it runs.
func (a *App) Key(k engine.Key, phase engine.KeyPhase, in engine.Surface) bool {
	a.keyIn = in
	// The key press the window put on the timeline has reached us.
	timeline.End("input.key")
	return a.Commands.Key(k, phase)
}

func (a *App) ModsChanged(m engine.Mods) { a.Commands.ModsChanged(m) }

func (a *App) MenuChosen(action string) { a.Commands.Run(action) }

func (a *App) MenuList(name string) engine.Menu { return a.Commands.ListItems(name) }

func (a *App) OSBindings(byTitle map[string]engine.Key) { a.Commands.OSBindings(byTitle) }

func (a *App) SettingsMessage(w engine.WindowID, msg []byte) { a.Settings.Handle(w, msg) }

func (a *App) WindowChanged(w engine.WindowID, s engine.WindowState) {
	a.Window.HandleWindowChanged(w, s)
}

func (a *App) ShellCrashed(w engine.WindowID) {
	a.Window.HandleShellCrashed(w)
	a.Shell.HandleShellCrashed(w)
}

// WindowClosed forgets the window and closes its tabs. Closing the last window quits.
func (a *App) WindowClosed(w engine.WindowID) {
	a.Settings.WindowClosed(w)
	a.Shell.WindowClosed(w)
	a.Window.Close(w)
	a.Tabs.RemoveWindow(w)
	if len(a.Tabs.Windows()) == 0 {
		a.quit()
		return
	}
	a.Commands.RefreshMenu()
}

// WindowFocused puts the window in front: commands and the menu follow it.
func (a *App) WindowFocused(w engine.WindowID) {
	a.Tabs.Focus(w)
	a.Commands.RefreshMenu()
}

func (a *App) ProcessCrashed(p engine.ProcessCrash) { a.Crash.ProcessCrashed(p) }
