// Package app builds the components and wires them together. All hook subscriptions
// live in Start, so this one function answers "who listens to what".
package app

import (
	"projectb/engine"
	"projectb/history"
	"projectb/policy"
	"projectb/shell"
	"projectb/suggest"
	"projectb/tabs"
)

// Config is what the app needs to start.
type Config struct {
	ProfileDir   string
	BlockedHosts []string
}

// App holds the managers and is the engine's listener: each function only forwards.
type App struct {
	Tabs    *tabs.Manager
	History *history.Manager
	Suggest *suggest.Manager
	Policy  *policy.Manager
	Shell   *shell.Manager
}

// Start builds everything, on the main thread. The caller then gives the returned App
// to the engine as its listener.
func Start(e engine.Engine, cfg Config) *App {
	a := &App{}
	a.Policy = policy.New(cfg.BlockedHosts)
	a.History = history.New(cfg.ProfileDir, e.Post)
	a.Suggest = suggest.New(a.History)
	a.Tabs = tabs.New(e)
	a.Shell = shell.New(e, a.Tabs, a.Suggest)

	a.Tabs.AddGate(a.Policy.IsAllowedURL)
	a.Tabs.OnNavigationFinished(func(t tabs.Tab) { a.History.AddVisit(t.URL, t.Title) })
	a.Tabs.OnTitleChanged(func(t tabs.Tab) { a.History.UpdateTitle(t.URL, t.Title) })
	a.Tabs.OnChanged(a.Shell.TabChanged)

	a.History.Load(nil)
	a.Tabs.Create()
	return a
}

// Stop flushes what's pending. Call it on the way out.
func (a *App) Stop() { a.History.Close() }

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

func (a *App) PageCrashed(tab engine.TabID) { a.Tabs.HandlePageCrashed(tab) }

func (a *App) FocusAddressBarRequested() { a.Shell.FocusAddressBar() }
