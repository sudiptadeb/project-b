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
	"projectb/relaunch"
	"projectb/settings"
	"projectb/shell"
	"projectb/suggest"
	"projectb/tabs"
	"projectb/timeline"
	"projectb/update"
	"projectb/window"
)

// Config is what the app needs to start.
type Config struct {
	ProfileDir   string
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

	actions []*commands.Action
	// keyIn is which page had the keyboard at the last key press.
	keyIn engine.Surface
	// lastWindow is the id of the window made last; Go numbers them from 1.
	lastWindow engine.WindowID
	quit       func()
}

// starters are what a build tag adds to Start. A tagged file appends one when it loads,
// so nothing here names it; each builds its managers and wires them to the rest.
var starters []func(a *App, e engine.Engine, cfg Config)

// Start builds everything, on the main thread. The caller then gives the returned App
// to the engine as its listener.
func Start(e engine.Engine, cfg Config) *App {
	a := &App{}
	a.Crash = crash.New(cfg.ProfileDir, cfg.CrashReports)
	a.Policy = policy.New(cfg.BlockedHosts)
	a.History = history.New(cfg.ProfileDir, e.Post)
	a.Suggest = suggest.New(a.History)
	a.Tabs = tabs.New(e)
	a.Prefs = prefs.New(cfg.ProfileDir)
	a.Flags = flags.New(cfg.ProfileDir)
	a.Window = window.New(e, a.Prefs.FrameColor(), themeOf(a.Prefs.Theme()))
	first := a.openWindow()
	a.quit = func() { e.AppCommand(engine.AppQuit) }
	a.Update = update.New(cfg.Update, e.Post, a.quit)
	a.Update.SetAuto(a.Prefs.AutoUpdate())
	a.Commands = commands.New(e, cfg.ProfileDir, func() bool { return a.Window.FullScreen(a.Tabs.Front()) })
	a.Shell = shell.New(e, a.Tabs, a.Suggest, a.Window, a.Prefs, a.Update, a.Crash, a.Commands.Run, a.Commands.ShownKeys, a.Commands.Platform())
	a.Settings = settings.New(e, a.Window, a.Prefs, a.Flags, a.Update, a.Commands)
	a.Settings.OnRelaunch(func() { relaunch.Self(); a.quit() })
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
	a.Prefs.OnThemeChanged(func(t string) { a.Window.SetTheme(themeOf(t)) })
	a.Prefs.OnThemeChanged(a.Shell.ThemeChanged)
	a.Prefs.OnThemeChanged(a.Settings.ThemeChanged)
	a.Prefs.OnTabLayoutChanged(a.Shell.TabLayoutChanged)
	a.Prefs.OnTabLayoutChanged(a.Settings.TabLayoutChanged)
	a.Prefs.OnAutoUpdateChanged(a.Update.SetAuto)
	a.Prefs.OnAutoUpdateChanged(a.Settings.AutoUpdateChanged)
	a.Update.OnChanged(a.Shell.UpdateChanged)
	a.Update.OnChanged(a.Settings.UpdateChanged)
	a.Commands.OnChanged(a.Settings.KeyboardChanged)
	a.Flags.OnChanged(a.Settings.FlagsChanged)
	a.Commands.OnChanged(a.Shell.KeysChanged)
	// What the menu shows enabled or checked follows the window in front: its active tab
	// and full screen, and the layout.
	a.Tabs.OnChanged(func(tabs.Tab) { a.Commands.RefreshMenu() })
	a.Tabs.OnListChanged(a.Commands.RefreshMenu)
	a.Prefs.OnTabLayoutChanged(func(string) { a.Commands.RefreshMenu() })
	a.Window.OnControlsChanged(func(engine.WindowID, engine.Rect, float64) { a.Commands.RefreshMenu() })
	a.Commands.Load()
	for _, start := range starters {
		start(a, e, cfg)
	}

	a.History.Load(nil)
	a.Tabs.CreateIn(first)
	a.Update.Start()
	return a
}

// openWindow makes a window with no tabs, a little down and right of the window in
// front, and puts it in front.
func (a *App) openWindow() engine.WindowID {
	near := a.Tabs.Front()
	a.lastWindow++
	w := a.lastWindow
	a.Tabs.AddWindow(w)
	a.Window.Open(w, near)
	return w
}

// NewWindow opens a window with one new tab.
func (a *App) NewWindow() {
	a.Tabs.CreateIn(a.openWindow())
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

func (a *App) LoadProgressChanged(tab engine.TabID, progress float64) {
	a.Tabs.HandleLoadProgressChanged(tab, progress)
}

func (a *App) PageCrashed(tab engine.TabID) { a.Tabs.HandlePageCrashed(tab) }

func (a *App) FaviconChanged(tab engine.TabID, dataURL string) {
	a.Tabs.HandleFaviconChanged(tab, dataURL)
}

func (a *App) TabOpened(opener, tab engine.TabID, url string, foreground bool) {
	a.Tabs.HandleTabOpened(opener, tab, url, foreground)
}

func (a *App) TabFocused(tab engine.TabID) { a.Tabs.HandleTabFocused(tab) }

func (a *App) CloseRequested(tab engine.TabID) { a.Tabs.Close(tab) }

func (a *App) ActivateRequested(tab engine.TabID) { a.Tabs.Activate(tab) }

// Handover is the request of a copy started on this profile while this one runs: the
// window used last comes to the front, and each URL opens in a new tab there.
func (a *App) Handover(urls []string) {
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
