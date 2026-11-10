// Package app builds the components and wires them together. All hook subscriptions
// live in Start, so this one function answers "who listens to what".
package app

import (
	"projectb/crash"
	"projectb/engine"
	"projectb/history"
	"projectb/policy"
	"projectb/prefs"
	"projectb/shell"
	"projectb/suggest"
	"projectb/tabs"
	"projectb/update"
	"projectb/window"
)

// Config is what the app needs to start.
type Config struct {
	ProfileDir   string
	BlockedHosts []string
	Update       update.Config
}

// App holds the managers and is the engine's listener: each function only forwards.
type App struct {
	Crash   *crash.Manager
	Tabs    *tabs.Manager
	History *history.Manager
	Suggest *suggest.Manager
	Policy  *policy.Manager
	Window  *window.Manager
	Prefs   *prefs.Manager
	Update  *update.Manager
	Shell   *shell.Manager
}

// Start builds everything, on the main thread. The caller then gives the returned App
// to the engine as its listener.
func Start(e engine.Engine, cfg Config) *App {
	a := &App{}
	a.Crash = crash.New(cfg.ProfileDir)
	a.Policy = policy.New(cfg.BlockedHosts)
	a.History = history.New(cfg.ProfileDir, e.Post)
	a.Suggest = suggest.New(a.History)
	a.Tabs = tabs.New(e)
	a.Prefs = prefs.New(cfg.ProfileDir)
	a.Window = window.New(e, a.Prefs.FrameColor())
	closeWindow := func() { a.Window.Request(engine.WindowClose) }
	a.Update = update.New(cfg.Update, e.Post, closeWindow)
	a.Update.SetAuto(a.Prefs.AutoUpdate())
	a.Shell = shell.New(e, a.Tabs, a.Suggest, a.Window, a.Prefs, a.Update)

	a.Tabs.AddGate(a.Policy.IsAllowedURL)
	a.Tabs.OnNavigationFinished(func(t tabs.Tab) { a.History.AddVisit(t.URL, t.Title) })
	a.Tabs.OnTitleChanged(func(t tabs.Tab) { a.History.UpdateTitle(t.URL, t.Title) })
	a.Tabs.OnChanged(a.Shell.TabChanged)
	a.Tabs.OnListChanged(a.Shell.TabsChanged)
	a.Tabs.OnEmpty(closeWindow)
	a.Window.OnControlsChanged(a.Shell.WindowControlsChanged)
	a.Window.OnColorChanged(a.Shell.WindowColorChanged)
	a.Prefs.OnFrameColorChanged(a.Window.SetColor)
	a.Prefs.OnTabLayoutChanged(a.Shell.TabLayoutChanged)
	a.Prefs.OnAutoUpdateChanged(a.Update.SetAuto)
	a.Prefs.OnAutoUpdateChanged(a.Shell.AutoUpdateChanged)
	a.Update.OnChanged(a.Shell.UpdateChanged)

	a.History.Load(nil)
	a.Tabs.Create()
	a.Update.Start()
	return a
}

// Stop flushes what's pending. Call it on the way out.
func (a *App) Stop() {
	a.Update.Stop()
	a.History.Close()
}

func (a *App) ShellMessage(msg []byte) { a.Shell.Handle(msg) }

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

func (a *App) CloseRequested(tab engine.TabID) { a.Tabs.Close(tab) }

func (a *App) ActivateRequested(tab engine.TabID) { a.Tabs.Activate(tab) }

// Shortcut maps each reserved key combination to the one function that does it.
func (a *App) Shortcut(s engine.Shortcut) {
	switch {
	case s == engine.ShortcutFocusAddressBar:
		a.Shell.FocusAddressBar()
	case s == engine.ShortcutNewTab:
		a.Tabs.Create()
	case s == engine.ShortcutCloseTab:
		a.Tabs.CloseActive()
	case s == engine.ShortcutNextTab:
		a.Tabs.ActivateNext(false)
	case s == engine.ShortcutPreviousTab:
		a.Tabs.ActivateNext(true)
	case s == engine.ShortcutTab1+8:
		a.Tabs.ActivateLast()
	case s >= engine.ShortcutTab1:
		a.Tabs.ActivateAt(int(s - engine.ShortcutTab1))
	}
}

func (a *App) WindowChanged(s engine.WindowState) { a.Window.HandleWindowChanged(s) }

func (a *App) ShellCrashed() { a.Window.HandleShellCrashed() }
