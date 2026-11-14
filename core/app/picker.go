package app

import (
	"encoding/json"
	"slices"

	"projectb/commands"
	"projectb/engine"
	"projectb/prefs"
	"projectb/profiles"
	"projectb/timeline"
	"projectb/tones"
	"projectb/window"
)

// pickerWindow is the picker's one window.
const pickerWindow engine.WindowID = 1

// Picker is a copy of the app that hasn't chosen its profile yet: one window holding the
// picker page, in our pages' context, which lists the profiles and makes new ones. It is
// the engine's listener until the choice. It has no tabs, so it hears of none.
type Picker struct {
	e        engine.Engine
	list     *profiles.Manager
	window   *window.Manager
	commands *commands.Manager
	quit     func()
	// open starts a copy of the app on a listed profile.
	open func(id string)
	// Whether app.ready is marked: the page first showed the profiles.
	ready bool
}

// pickerCommand is what the picker page sends us.
type pickerCommand struct {
	T        string `json:"t"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Template string `json:"template"`
}

// StartPicker opens the picker window, on the main thread. Choosing a profile starts a copy
// of the app on it with open, which hands over to the copy already running there, if any,
// and this copy quits. Closing the picker window quits too.
func StartPicker(e engine.Engine, list *profiles.Manager, open func(id string)) *Picker {
	p := &Picker{e: e, list: list, open: open, quit: func() { e.AppCommand(engine.AppQuit) }}
	// The picker's window is no profile's, so it has the app's own colour.
	p.window = window.New(e, engine.PagePicker, prefs.Defaults().FrameColor, engine.ThemeSystem, func() engine.WindowID { return pickerWindow })
	p.commands = commands.NewFromTemplates(e, func() bool { return p.window.FullScreen(pickerWindow) })
	for _, a := range copyActions(e, p.quit, func(r engine.WindowRequest) { p.window.Request(pickerWindow, r) }) {
		p.commands.Register(*a)
	}
	// Profiles… shows the picker, which is already here.
	p.commands.Register(commands.Action{ID: "profile.picker", Title: "Profiles…", Run: func() { p.window.Request(pickerWindow, engine.WindowFront) }})
	p.commands.RegisterList(commands.List{
		ID:     "profiles",
		Items:  func() []commands.ListItem { return pickerItems(p.profiles()) },
		Choose: p.choose,
	})
	p.commands.Load()
	p.window.Open(pickerWindow, 0)
	return p
}

func pickerItems(list []profiles.Profile) []commands.ListItem {
	var items []commands.ListItem
	for _, pr := range list {
		items = append(items, commands.ListItem{Key: pr.ID, Title: pr.Name})
	}
	return items
}

func (p *Picker) profiles() []profiles.Profile {
	list, err := p.list.List()
	if err != nil {
		timeline.Errorf("profiles: %v", err)
	}
	return list
}

// Message is a message from the picker page.
func (p *Picker) Message(w engine.WindowID, msg []byte) {
	var c pickerCommand
	if err := json.Unmarshal(msg, &c); err != nil {
		timeline.Warnf("picker: a bad message: %v", err)
		return
	}
	switch c.T {
	case "ready":
		p.sendState()
	case "shown":
		// The page shows the profiles, so the copy can be used. Tests wait for this line.
		if !p.ready {
			p.ready = true
			timeline.Mark("app.ready")
		}
	case "timeline":
		timeline.FromPage("picker", msg)
	case "layout":
		var l window.ShellLayout
		if err := json.Unmarshal(msg, &l); err != nil {
			timeline.Warnf("picker: a bad layout: %v", err)
			return
		}
		p.window.HandleShellLayout(w, l)
	case "open":
		p.choose(c.ID)
	case "create":
		made, err := p.list.Create(c.Name, c.Template)
		if err != nil {
			timeline.Errorf("profiles: %v", err)
			return
		}
		p.choose(made.ID)
	}
}

// sendState gives the page the profiles and the templates.
func (p *Picker) sendState() {
	type shown struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	list := []shown{}
	for _, pr := range p.profiles() {
		list = append(list, shown{pr.ID, pr.Name, pr.Color})
	}
	var templates []string
	for _, t := range profiles.Templates() {
		templates = append(templates, t.Name)
	}
	b, _ := json.Marshal(map[string]any{"t": "state", "profiles": list, "templates": templates, "color": p.window.Color(), "tones": tones.Of(p.window.Color())})
	p.e.SendToShell(pickerWindow, b)
}

// choose opens the listed profile id in a copy of its own, and quits.
func (p *Picker) choose(id string) {
	if !slices.ContainsFunc(p.profiles(), func(pr profiles.Profile) bool { return pr.ID == id }) {
		timeline.Warnf("picker: no profile %q", id)
		return
	}
	p.open(id)
	p.quit()
}

// WindowClosed is the picker window gone: the copy has nothing left to show.
func (p *Picker) WindowClosed(w engine.WindowID) {
	p.window.Close(w)
	p.quit()
}

func (p *Picker) WindowChanged(w engine.WindowID, s engine.WindowState) {
	p.window.HandleWindowChanged(w, s)
}

func (p *Picker) WindowFocused(w engine.WindowID) { p.commands.RefreshMenu() }

func (p *Picker) ShellCrashed(w engine.WindowID) {
	p.window.HandleShellCrashed(w)
}

func (p *Picker) Key(k engine.Key, phase engine.KeyPhase, in engine.Surface) bool {
	timeline.End("input.key")
	return p.commands.Key(k, phase)
}

func (p *Picker) ModsChanged(m engine.Mods) { p.commands.ModsChanged(m) }

func (p *Picker) MenuChosen(action string) { p.commands.Run(action) }

func (p *Picker) MenuList(name string) engine.Menu { return p.commands.ListItems(name) }

func (p *Picker) OSBindings(byTitle map[string]engine.Key) { p.commands.OSBindings(byTitle) }

// A picker copy has no tabs, no shell and no settings page, so it hears nothing of them.

func (p *Picker) ShellMessage(engine.WindowID, []byte)                             {}
func (p *Picker) SettingsMessage(engine.WindowID, []byte)                          {}
func (p *Picker) NavigationStarting(engine.TabID, string) bool                     { return false }
func (p *Picker) NavigationFinished(engine.TabID, string)                          {}
func (p *Picker) NavigationFailed(engine.TabID, string, string)                    {}
func (p *Picker) HistoryChanged(engine.TabID, bool, bool)                          {}
func (p *Picker) TitleChanged(engine.TabID, string)                                {}
func (p *Picker) LoadingChanged(engine.TabID, bool)                                {}
func (p *Picker) LoadProgressChanged(engine.TabID, float64)                        {}
func (p *Picker) ScrollDirectionChanged(engine.TabID, bool)                        {}
func (p *Picker) ContextMenu(engine.TabID, engine.ContextMenu)                     {}
func (p *Picker) PageCrashed(engine.TabID)                                         {}
func (p *Picker) FaviconChanged(engine.TabID, string)                              {}
func (p *Picker) TabOpened(engine.TabID, engine.TabID, string, string, bool, bool) {}
func (p *Picker) TabFocused(engine.TabID)                                          {}
func (p *Picker) CloseRequested(engine.TabID)                                      {}
func (p *Picker) ActivateRequested(engine.TabID)                                   {}
func (p *Picker) ProcessCrashed(engine.ProcessCrash)                               {}
func (p *Picker) DevToolsCommand(string, []byte) (any, error)                      { return nil, engine.ErrUnknownCommand }

// Handover is the request of a copy started to pick while this one runs: the picker comes
// to the front.
func (p *Picker) Handover() { p.window.Request(pickerWindow, engine.WindowFront) }
