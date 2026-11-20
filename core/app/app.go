// Package app builds the components and wires them together. All hook subscriptions
// live in Start, so this one function answers "who listens to what".
package app

import (
	"encoding/json"
	"path/filepath"

	"github.com/sudiptadeb/project-b/browserdata"

	"projectb/bookmarks"
	"projectb/commands"
	"projectb/crash"
	"projectb/downloads"
	"projectb/engine"
	"projectb/flags"
	"projectb/history"
	"projectb/keys"
	"projectb/passkeys"
	"projectb/passwords"
	"projectb/pins"
	"projectb/policy"
	"projectb/prefs"
	"projectb/profiles"
	"projectb/relaunch"
	"projectb/session"
	"projectb/settings"
	"projectb/shell"
	"projectb/shelldata"
	"projectb/shells"
	"projectb/sites"
	"projectb/sparks"
	"projectb/store"
	"projectb/suggest"
	"projectb/sync"
	"projectb/system"
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
	Profiles *profiles.Manager
	Profile  string
	// Made says this start made the profile: its first, which opens the welcome.
	Made bool
	// SkipWelcome is --skip-welcome: a start that would open the welcome notes it finished
	// instead, as its Skip setup does, so a test starts past it.
	SkipWelcome bool
	// Platform is the platform the keys and the pages are written for, "mac", "windows" or
	// "linux"; this one's when empty. The mock's -platform sets it for tests.
	Platform string
	// BrowserHome is the home folder whose browsers Import finds: the user's when "", a
	// test's own folder of browsers in the mock.
	BrowserHome string
	// Keychain reads a Chromium browser's passwords' key, its "<name> Safe Storage": the
	// login keychain's when nil, a test's own in the mock.
	Keychain func(service, account string) ([]byte, error)
	// KeychainItems lists the login keychain's "<name> Safe Storage" items, for a browser
	// known only by the folder the person chose: the login keychain's when nil, a test's
	// own in the mock.
	KeychainItems func() ([]browserdata.KeychainItem, error)
	// URLs are the addresses the command line names, which the first window opens as its
	// tabs at the start, in place of the blank one; ChromeDriver starts the app so.
	URLs         []string
	BlockedHosts []string
	Update       update.Config
	// CrashReports is the folder macOS writes its crash reports to.
	CrashReports string
	// DownloadsDir is where downloads go: the user's Downloads folder in the real app, and
	// a Downloads folder in the profile when it is "".
	DownloadsDir string
	// System is the OS's door (core/system): files opened, the default browser, the Dock,
	// and what a crash report says of the device. The real one in the app, mocksystem's in
	// the mock.
	System system.System
	// DeviceKey is the device key that wraps the profile's key (docs/sync.md): the keychain's
	// for a listed profile, nil for a folder named outright, which keeps a key of its own.
	DeviceKey keys.Device
}

// menuRead is what the menu reads of the front window's active tab: which tab, whether it
// loads (Stop), has a page (View Source), and can go back and forward.
type menuRead struct {
	tab                          engine.TabID
	loading, page, back, forward bool
}

func readOf(t tabs.Tab) menuRead {
	return menuRead{t.ID, t.Loading, t.URL != "", t.CanGoBack, t.CanGoForward}
}

// refreshMenu sends the menu again if it changed, and keeps what it read of the active
// tab, so a tab's change that leaves that alone needn't build it.
func (a *App) refreshMenu() {
	a.menuRead = readOf(a.Tabs.Active())
	a.Commands.RefreshMenu()
}

// App holds the managers and is the engine's listener: each function only forwards.
type App struct {
	// menuRead is what the menu last read of the front window's active tab.
	menuRead menuRead
	// siteData is what each workspace's storage held when the engine last counted it, by
	// its id (site_data_settings.go). Main thread.
	siteData map[string][]engine.SiteData

	// choosingFolder is the window whose folder panel Save to's Change… opened, 0 for
	// none (download_folder_settings.go). Main thread.
	choosingFolder engine.WindowID

	// System is the OS's door, Config's.
	System   system.System
	Crash    *crash.Manager
	Store    *store.Store
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
	// Pins is the profile's pinned tabs, beside its workspaces.
	Pins *pins.Manager
	// ShellData is what each shell saves for the whole profile, beside its prefs.
	ShellData *shelldata.Manager
	// Downloads is the profile's downloads.
	Downloads *downloads.Manager
	// Sites is each site's settings, what it may use among them, and the prompts open for the rest.
	Sites *sites.Manager
	// Sparks are the person's scripts for pages (docs/site-features.md).
	Sparks *sparks.Manager
	// Session is the profile's windows and tabs as the last run left them, and as they are.
	Session *session.Manager
	// Bookmarks is the profile's bookmarks.
	Bookmarks *bookmarks.Manager
	// Keys is the profile's key, and the secrets it seals.
	Keys *keys.Keys
	// Passkeys are the profile's own passkeys (docs/passkeys.md).
	Passkeys *passkeys.Manager
	// Passwords are the profile's saved logins (docs/passwords.md).
	Passwords *passwords.Manager
	// passkeysChanged counts the passkeys again for Settings, after sync wrote some.
	passkeysChanged func()
	// Sync syncs the bookmarks and history (docs/sync.md).
	Sync *sync.Manager
	// closed is the tabs and windows the person closed, newest first; quitting says the
	// app is ending, so what closes now isn't.
	closed   []session.Closed
	quitting bool
	// dir is the profile's folder.
	dir string

	actions []*commands.Action
	// menus are the windows' open right-click menus.
	menus *contextMenus
	// lastFolder is where files were last chosen from for a page.
	lastFolder string
	// dialogs are the pages' dialogs waiting for an answer.
	dialogs *pageDialogs
	// finds are the tabs' open searches.
	finds *pageFinds
	// fulls are the windows whose page fills the screen.
	fulls *pageFulls
	// thumbs asks for pictures of the tabs' pages, for a phone shell.
	thumbs *pageThumbs
	// notified is the page each notification shown on the system came from, by its id.
	notified map[string]string
	// startedIn is the window in front that each new download began in, by the engine's
	// number for it, until its first report says the list has it.
	startedIn map[engine.DownloadID]engine.WindowID
	// pages are the History and Bookmarks pages shown in tabs.
	pages *ourPages
	// welcome is the first start's welcome page (welcome.go).
	welcome *welcome
	imports *importer
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
	e.OpenProfile(cfg.ProfileDir)
	a := &App{passkeysChanged: func() {}, System: cfg.System, profiles: cfg.Profiles, startedIn: map[engine.DownloadID]engine.WindowID{}, dir: cfg.ProfileDir}
	a.Crash = crash.New(cfg.ProfileDir, cfg.CrashReports, e.ExitReason)
	a.Crash.System = func() []crash.Section { return cfg.System.Report(cfg.Update.Version) }
	a.Session = session.New(cfg.ProfileDir)
	a.Policy = policy.New(cfg.BlockedHosts, e.OurPage)
	e.SetBlockRules(a.Policy.BlockRules())
	st, err := store.Open(cfg.ProfileDir)
	if err != nil {
		// History and bookmarks have nowhere else to be.
		panic(err)
	}
	a.Store = st
	// The bookmarks first: the main thread waits for them, not behind history's first read
	// of history.jsonl.
	a.Bookmarks = bookmarks.New(cfg.ProfileDir, st)
	a.History = history.New(cfg.ProfileDir, st, e.Post)
	// Sync, after the tables it syncs: off the main thread, and at once when it is on.
	a.Keys = keys.New(cfg.ProfileDir, cfg.Profile, st, cfg.DeviceKey)
	a.History.OnClearAll(sync.ClearedAll)
	// The passkeys before sync, which marks their table's rows as it does the others'.
	a.Passkeys = passkeys.New(st, a.Keys, verifyPerson(cfg.System, e.Post))
	// And the saved logins, likewise.
	a.Passwords = passwords.New(st, a.Keys, checkForPassword(cfg.System, e.Post))
	a.Sync = sync.New(st, a.Keys, e.Post, append(history.Synced(), a.Bookmarks.Synced(e.Post), a.Passkeys.Synced(func() { a.passkeysChanged() }), a.Passwords.Synced())...)
	a.Suggest = suggest.New(a.History)
	a.Workspaces = workspaces.New(cfg.ProfileDir)
	a.Pins = pins.New(cfg.ProfileDir)
	a.ShellData = shelldata.New(cfg.ProfileDir)
	if cfg.DownloadsDir == "" {
		cfg.DownloadsDir = filepath.Join(cfg.ProfileDir, "Downloads")
	}
	a.Downloads = downloads.New(e, cfg.System, cfg.ProfileDir, cfg.DownloadsDir)
	a.Downloads.Load()
	a.Tabs = tabs.New(e, a.Workspaces, a.Pins)
	a.Prefs = prefs.New(cfg.ProfileDir)
	a.Flags = flags.New(cfg.ProfileDir)
	a.Window = window.New(e, shells.Page(a.Prefs.Shell()), a.Prefs.FrameColor(), themeOf(a.Prefs.Theme()), a.Tabs.Front)
	a.Tabs.OnFrontChanged(a.Window.HandleFrontChanged)
	a.Tabs.OnMoved(a.Window.HandleTabMoved)
	a.sayStart(cfg.ProfileDir)
	first := a.openWindow()
	// Quitting keeps the session as it was before: the windows that close as the app ends
	// aren't closed by the person.
	a.quit = func() {
		a.quitting = true
		e.AppCommand(engine.AppQuit)
	}
	a.Update = update.New(cfg.Update, e.Post, a.quit)
	// A release that didn't start three times is put back by the one before it, which this
	// start then is: it ends, and the previous release starts in its place.
	if a.Update.Settle() {
		a.quit()
	}
	a.Commands = commands.New(e, cfg.ProfileDir, func() bool { return a.Window.FullScreen(a.Tabs.Front()) })
	if cfg.Platform != "" {
		a.Commands.WriteFor(cfg.Platform)
	}
	a.Commands.RegisterList(a.profileList(cfg))
	a.Commands.RegisterList(a.workspaceList())
	a.Commands.RegisterList(a.shellList())
	a.Commands.RegisterList(a.closedList(e))
	a.Shell = shell.New(e, a.Tabs, a.Suggest, a.Window, a.Prefs, a.Update, a.Crash, a.Commands.Run, a.Commands.ShownKeys, a.Commands.Platform())
	a.Shell.AddPart(a.workspaceShellPart(e))
	a.Shell.AddPart(a.pinShellPart())
	a.Shell.AddPart(a.menuShellPart())
	a.Shell.AddPart(a.shellDataPart())
	a.menus = newContextMenus(a, e)
	a.Shell.AddPart(a.downloadsShellPart(e))
	a.dialogs = &pageDialogs{a: a, e: e, list: []pageDialog{}}
	a.dialogs.passkeys = &passkeyCards{p: a.dialogs, waiting: map[engine.DialogID]pageDialog{}, own: map[engine.DialogID]*ownPasskey{}}
	a.Shell.AddPart(a.dialogs.shellPart())
	a.finds = newPageFinds(a, e)
	a.Shell.AddPart(a.finds.shellPart())
	a.fulls = newPageFulls(a, e)
	a.Shell.OnBack(a.fulls.back)
	a.pages = newOurPages(a, e)
	a.thumbs = newPageThumbs(a, e)
	a.Shell.AddPart(a.thumbs.shellPart())
	a.Shell.AddPart(a.bookmarksShellPart(e))
	a.Shell.AddPart(a.menus.shellPart())
	// Every action a shell can run by name, for a shell that lists them, as Glide's
	// Command Bar does.
	a.Shell.AddPart(shell.Part{
		Snapshot: func(into map[string]any) { into["actions"] = a.Commands.Titles() },
		Handle:   func(engine.WindowID, string, []byte) bool { return false },
	})
	a.Settings = settings.New(e, a.Tabs, a.Window, a.Prefs, a.Flags, a.Update, a.Commands)
	a.Settings.OnRelaunch(func() { relaunch.Self(); a.quit() })
	a.Settings.AddPart(a.workspaceSettingsPart(e))
	a.startSites(e, cfg.ProfileDir)
	a.Settings.AddPart(a.startSettingsPart())
	a.welcome = newWelcome(a, e, cfg.ProfileDir)
	a.Shell.AddPart(a.welcome.shellPart())
	a.Settings.AddPart(settings.Part{
		Snapshot: func(map[string]any) {},
		Handle: func(tab engine.TabID, t string, _ []byte) bool {
			if t != "welcomeAgain" {
				return false
			}
			a.welcome.again()
			a.quit()
			return true
		},
	})
	a.imports = newImporter(a, e, cfg.BrowserHome, cfg.Keychain, cfg.KeychainItems, func(v any) {
		a.welcome.sendAll(v)
		a.Settings.Send(v)
	})
	// What comes over from another browser or a file is kept in the profile's own logins.
	a.imports.passwords = a.Passwords
	a.Settings.AddPart(settings.Part{
		Snapshot: a.imports.snapshot,
		Handle:   func(tab engine.TabID, t string, msg []byte) bool { return a.imports.handle(tab, t, msg) },
	})
	a.Settings.AddPart(a.bookmarkSettingsPart())
	a.Settings.AddPart(a.defaultBrowserSettingsPart())
	a.Settings.AddPart(a.clearDataSettingsPart(e))
	a.Settings.AddPart(a.siteDataSettingsPart(e))
	a.Settings.AddPart(a.passkeySettingsPart(e))
	a.Settings.AddPart(a.passwordSettingsPart(e))
	a.Settings.AddPart(a.keepCookiesSettingsPart())
	a.Settings.AddPart(a.downloadFolderSettingsPart(e, cfg.DownloadsDir))
	a.Settings.AddPart(a.syncSettingsPart())
	a.registerActions(e)

	a.Tabs.AddGate(a.Policy.IsAllowedURL)
	// History is the web's: our own tabs record nothing.
	a.Tabs.OnNavigationFinished(func(t tabs.Tab) {
		switch {
		case t.Page != "":
		case t.Typed:
			a.History.AddTypedVisit(t.URL, t.Title)
		default:
			a.History.AddVisit(t.URL, t.Title)
		}
	})
	a.Tabs.OnTitleChanged(func(t tabs.Tab) {
		if t.Page == "" {
			a.History.UpdateTitle(t.URL, t.Title)
		}
	})
	a.Tabs.OnChanged(a.Shell.TabChanged)
	a.Tabs.OnListChanged(a.Shell.TabsChanged)
	a.Tabs.OnClosed(a.Settings.TabClosed)
	a.Shell.OnMenuOpened(a.Settings.Prepare)
	// A window always has a tab: it closes with its last one (docs/windows.md).
	a.Tabs.OnEmpty(func(w engine.WindowID) { a.Window.Request(w, engine.WindowClose) })
	a.Window.OnControlsChanged(a.Shell.WindowControlsChanged)
	a.Window.OnColorChanged(a.Shell.WindowColorChanged)
	a.Window.OnColorChanged(a.Settings.WindowColorChanged)
	a.Prefs.OnFrameColorChanged(a.Window.SetColor)
	// The profile's colour is its frame colour: on its Dock icon, and in the list.
	a.System.SetDockColor(a.Prefs.FrameColor())
	a.Prefs.OnFrameColorChanged(a.System.SetDockColor)
	a.System.OnNotificationClicked(func(id string) { a.notificationClicked(e, id) })
	// The links and files the OS hands the app open as a second copy's do, beside the window
	// used last.
	a.System.OnOpenURLs(a.Handover)
	if cfg.Profile != "" {
		a.Prefs.OnFrameColorChanged(func(c string) {
			if err := cfg.Profiles.SetColor(cfg.Profile, c); err != nil {
				timeline.Errorf("profiles: %v", err)
			}
		})
		a.Settings.AddPart(a.profileSettingsPart(cfg.Profiles, cfg.Profile))
	}
	if shells.OnPhone {
		a.Settings.AddPart(a.phoneShellPart())
	}
	// A change to the workspaces reaches every shell at once, for the dots, and every
	// settings page; the Tab menu reads the list as it opens. The engine hears which is
	// first, at start and whenever that changes, for the tabs it makes on its own.
	firstWorkspace := a.Workspaces.First()
	e.SetFirstWorkspace(firstWorkspace)
	a.Workspaces.OnChanged(func() {
		m := map[string]any{"t": "workspaces", "workspaces": a.workspacesShown()}
		a.Shell.Send(m)
		a.Settings.Send(m)
		if now := a.Workspaces.First(); now != firstWorkspace {
			firstWorkspace = now
			e.SetFirstWorkspace(now)
		}
	})
	a.startQuitErasure(e)
	// A change to the pins reaches every shell at once.
	a.Pins.OnChanged(func() { a.Shell.Send(map[string]any{"t": "pins", "pins": a.Pins.List()}) })
	a.Prefs.OnThemeChanged(func(t string) { a.Window.SetTheme(themeOf(t)) })
	a.Prefs.OnThemeChanged(a.Shell.ThemeChanged)
	a.Prefs.OnChanged(a.Shell.PrefsChanged)
	a.Prefs.OnThemeChanged(a.Settings.ThemeChanged)
	// Typed words go to the profile's search engine, from the start and after a change.
	a.Tabs.SetSearch(a.Prefs.SearchEngine().For)
	a.Prefs.OnSearchEngineChanged(func(string) { a.Tabs.SetSearch(a.Prefs.SearchEngine().For) })
	a.Prefs.OnSearchEngineChanged(a.Shell.SearchEngineChanged)
	a.Prefs.OnSearchEngineChanged(a.Settings.SearchEngineChanged)
	a.Suggest.SetSources(a.suggestSources())
	a.Shell.AddPart(a.suggestShellPart(e))
	// Another shell loads in every window at once. The old one's regions go, as after a
	// crash, and the tabs stay where they were until the new one reports.
	a.Prefs.OnShellChanged(func(id string) {
		a.Shell.ShellChanged()
		a.Window.SetPage(shells.Page(id))
	})
	a.Prefs.OnTabLayoutChanged(a.Shell.TabLayoutChanged)
	a.Prefs.OnTabLayoutChanged(a.Settings.TabLayoutChanged)
	a.Prefs.OnAddressBarPositionChanged(a.Shell.AddressBarPositionChanged)
	a.Prefs.OnAddressBarPositionChanged(a.Settings.AddressBarPositionChanged)
	a.Update.OnAutoChanged(a.Settings.AutoUpdateChanged)
	a.Update.OnChannelChanged(a.Settings.ChannelChanged)
	a.Update.OnChanged(a.Shell.UpdateChanged)
	a.Update.OnChanged(a.Settings.UpdateChanged)
	a.Commands.OnChanged(a.Settings.KeyboardChanged)
	a.Flags.OnChanged(a.Settings.FlagsChanged)
	a.Commands.OnChanged(a.Shell.KeysChanged)
	// What the menu shows enabled or checked follows the window in front: its active tab
	// and full screen, the layout, and the shell. Of a tab's changes, only those to the
	// front window's active tab that Stop, View Source, Back and Forward read can change
	// it: building the menu was most of what Go did for each of a navigation's events.
	a.Tabs.OnChanged(func(t tabs.Tab) {
		if t.ID != a.Tabs.ActiveID() {
			return
		}
		if readOf(t) != a.menuRead {
			a.refreshMenu()
		}
	})
	a.Tabs.OnListChanged(a.refreshMenu)
	a.Prefs.OnTabLayoutChanged(func(string) { a.refreshMenu() })
	a.Prefs.OnShellChanged(func(string) { a.refreshMenu() })
	// Keyboard settings lists only the shell in use's own actions.
	a.Prefs.OnShellChanged(func(string) { a.Settings.KeyboardChanged() })
	a.Window.OnControlsChanged(func(engine.WindowID, engine.Rect, float64) { a.refreshMenu() })
	a.Commands.Load()
	for _, start := range starters {
		start(a, e, cfg)
	}

	// After a crash the last run's windows come back; addresses given at the start open
	// beside them.
	restored := a.restore(e, first)
	// The welcome opens over the window in front, after what the start opens; finishing it
	// makes the tab a start would have made.
	welcomed := a.welcome.due(cfg.Made)
	if welcomed && cfg.SkipWelcome {
		a.welcome.skip()
		welcomed = false
	}
	switch {
	case len(cfg.URLs) > 0:
		for _, u := range cfg.URLs {
			a.Tabs.OpenIn(u, true)
		}
	case !restored && !welcomed:
		a.Tabs.Create(first)
	}
	if welcomed {
		w := a.Tabs.Front()
		if w == 0 {
			w = first
		}
		a.welcome.openWhole(w)
	}
	a.keepSession(e)
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

// phoneShellPart is a phone's choice of shell, in General: a phone has no Profile
// section or View menu, the desktop's two places for it. The shells it offers are a
// phone's own (shells.Listed).
func (a *App) phoneShellPart() settings.Part {
	view := func() map[string]any {
		listed := []map[string]string{}
		for _, s := range shells.Listed() {
			listed = append(listed, map[string]string{"id": s.ID, "name": s.Name})
		}
		return map[string]any{"shell": a.Prefs.Shell(), "shells": listed}
	}
	a.Prefs.OnShellChanged(func(string) { a.Settings.Send(map[string]any{"t": "phoneShells", "phoneShells": view()}) })
	return settings.Part{
		Snapshot: func(into map[string]any) { into["phoneShells"] = view() },
		Handle: func(_ engine.TabID, t string, msg []byte) bool {
			if t != "setShell" {
				return false
			}
			var c struct {
				Text string `json:"text"`
			}
			json.Unmarshal(msg, &c)
			a.Prefs.SetShell(c.Text)
			return true
		},
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
	timeline.Infof("app: stopping; the windows and tabs are kept in %s", filepath.Join(a.dir, "session.json"))
	a.Update.Close()
	a.Sync.Stop()
	a.Store.Close()
	a.Session.Close()
	a.Prefs.Close()
	a.Crash.Stop()
}

func (a *App) ShellMessage(w engine.WindowID, msg []byte) { a.Shell.Handle(w, msg) }

func (a *App) NavigationStarting(tab engine.TabID, nav int64, url string) engine.Navigation {
	return a.Tabs.HandleNavigationStarting(tab, nav, url)
}

// AppLink is a page's link to one of the app's own, pressed: OpenAppLink takes it, in the
// tab's window. A tab no window holds is gone.
func (a *App) AppLink(tab engine.TabID, url string) {
	if w := a.Tabs.WindowOf(tab); w != 0 {
		a.OpenAppLink(w, url)
	}
}

func (a *App) NavigationFinished(tab engine.TabID, nav int64, url string) {
	a.Tabs.HandleNavigationFinished(tab, nav, url)
}

func (a *App) NavigationFailed(tab engine.TabID, nav int64, url string, why engine.Failure, code string) {
	a.Tabs.HandleNavigationFailed(tab, nav, url, why, code)
}

func (a *App) HistoryChanged(tab engine.TabID, canGoBack, canGoForward bool) {
	a.Tabs.HandleHistoryChanged(tab, canGoBack, canGoForward)
}

func (a *App) TitleChanged(tab engine.TabID, title string) { a.Tabs.HandleTitleChanged(tab, title) }

func (a *App) LoadingChanged(tab engine.TabID, loading bool) {
	a.Tabs.HandleLoadingChanged(tab, loading)
}

func (a *App) ContextMenu(tab engine.TabID, m engine.ContextMenu) {
	a.menus.show(tab, m)
}

func (a *App) ImageSaved(path string, ok bool) { a.menus.imageSaved(path, ok) }

func (a *App) ScrollDirectionChanged(tab engine.TabID, down bool) {
	a.Shell.ScrollDirectionChanged(tab, down)
}

func (a *App) LoadProgressChanged(tab engine.TabID, progress float64) {
	a.Tabs.HandleLoadProgressChanged(tab, progress)
}

func (a *App) PageCrashed(tab engine.TabID) { a.Tabs.HandlePageCrashed(tab) }

func (a *App) CertificateError(tab engine.TabID, e engine.CertificateError) bool {
	return a.Tabs.HandleCertificateError(tab, e)
}

func (a *App) ConnectionChanged(tab engine.TabID, c engine.Connection) {
	a.Tabs.HandleConnectionChanged(tab, c)
}

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

// Handover is the request of a copy started on this profile while this one runs, and the
// links and files the system hands the app: the window used last comes to the front, and each
// URL opens in a new tab there, but a project-b: link, which OpenAppLink takes.
func (a *App) Handover(urls []string) {
	timeline.Infof("a copy started on this profile handed over %d URLs", len(urls))
	a.Window.Request(a.Tabs.Front(), engine.WindowFront)
	for _, u := range urls {
		if isAppLink(u) {
			a.OpenAppLink(a.Tabs.Front(), u)
			continue
		}
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
	// Escape leaves a page's full screen before the page sees it.
	if phase == engine.KeyBeforePage && a.fulls.key(k) {
		return true
	}
	// The key press the window put on the timeline has reached us.
	timeline.End("input.key")
	return a.Commands.Key(k, phase)
}

func (a *App) ModsChanged(m engine.Mods) { a.Commands.ModsChanged(m) }

func (a *App) MenuChosen(action string) { a.Commands.Run(action) }

func (a *App) MenuList(name string) engine.Menu { return a.Commands.ListItems(name) }

func (a *App) OSBindings(byTitle map[string]engine.Key) { a.Commands.OSBindings(byTitle) }

// PageMessage hands a message from a page of ours to that page's manager.
func (a *App) PageMessage(tab engine.TabID, page engine.Page, msg []byte) {
	switch page {
	case "settings":
		a.Settings.Handle(tab, msg)
	case "history", "bookmarks", "downloads":
		a.pages.handle(tab, page, msg)
	case welcomePage:
		a.welcome.handle(tab, msg)
	default:
		timeline.Warnf("app: a message from %q, which isn't a page of ours", page)
	}
}

// PickerMessage is the picker's, which a copy with a profile never shows.
func (a *App) PickerMessage(engine.WindowID, []byte) {}

func (a *App) WindowChanged(w engine.WindowID, s engine.WindowState) {
	a.Window.HandleWindowChanged(w, s)
	a.fulls.windowChanged(w, s)
}

func (a *App) ShellCrashed(w engine.WindowID) {
	a.Window.HandleShellCrashed(w)
	a.Shell.HandleShellCrashed(w)
}

// WindowClosed forgets the window and closes its tabs. Closing the last window quits.
func (a *App) WindowClosed(w engine.WindowID) {
	a.windowClosing(w)
	a.Shell.WindowClosed(w)
	a.Window.Close(w)
	a.Tabs.RemoveWindow(w)
	if len(a.Tabs.Windows()) == 0 {
		a.quit()
		return
	}
	a.refreshMenu()
}

// WindowFocused puts the window in front: commands and the menu follow it.
func (a *App) WindowFocused(w engine.WindowID) {
	a.Tabs.Focus(w)
	a.refreshMenu()
}

func (a *App) ProcessCrashed(p engine.ProcessCrash) { a.Crash.ProcessCrashed(p) }

// DownloadTarget refuses a download from an address policy blocks; the list says where
// the rest go, through the Save panel when Settings' Ask where to save is on. One a tab of
// the window in front began is noted, so that window's shell hears it start.
func (a *App) DownloadTarget(r engine.DownloadRequest) engine.DownloadTarget {
	if !a.Policy.IsAllowedURL(r.URL) {
		return engine.DownloadTarget{}
	}
	t := a.Downloads.Target(r)
	t.Prompt = t.Prompt || t.Path != "" && a.Prefs.All().AskWhereToSave
	if w := a.Tabs.WindowOf(r.Tab); t.Path != "" && w != 0 && w == a.Tabs.Front() {
		a.startedIn[r.ID] = w
	}
	return t
}

// DownloadUpdated takes a download's news. The first news of one begun in the window in
// front tells that window's shell, after the list that has it, with downloadStarted, so a
// shell can show the download begin (Standard opens its Downloads panel).
func (a *App) DownloadUpdated(d engine.Download) {
	id := a.Downloads.Updated(d)
	if w, ok := a.startedIn[d.ID]; ok {
		delete(a.startedIn, d.ID)
		timeline.Mark("download.started")
		a.Shell.SendTo(w, map[string]any{"t": "downloadStarted", "id": id})
	}
}
