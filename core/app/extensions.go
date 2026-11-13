//go:build extensions

package app

import (
	"encoding/json"

	"projectb/engine"
	"projectb/extensions"
	"projectb/extensions/chromeapi"
	"projectb/flags"
	"projectb/settings"
	"projectb/shell"
)

func init() { starters = append(starters, startExtensions) }

// extensionManagers is each app's extension manager, for the tests.
var extensionManagers = map[*App]*extensions.Manager{}

// Extensions returns the app's extension manager.
func (a *App) Extensions() *extensions.Manager { return extensionManagers[a] }

// startExtensions builds the extension manager and Chrome's API on Go's tabs, and wires
// them: the engine's extension events in, the shell's buttons and the settings page's
// extension manager out. Only while the Extensions flag is on: off, the build carries
// the code and nothing of it runs.
func startExtensions(a *App, e engine.Engine, cfg Config) {
	if !a.Flags.IsOn(flags.Extensions) {
		return
	}
	eng := e.(engine.Extensions)
	m := extensions.New(eng, cfg.ProfileDir, a.Tabs.ActiveID)
	api := chromeapi.New(eng, a.Tabs, a.Window, a, m)
	extensionManagers[a] = m
	eng.SetExtensionsListener(&extensionEvents{m, api})

	m.OnClicked(api.Clicked)
	a.Tabs.OnListChanged(api.TabsChanged)
	a.Tabs.OnChanged(api.TabChanged)
	a.Window.OnEvent(api.WindowEvent)

	// The shell draws the buttons; they differ by tab, so they go again when the active
	// tab changes.
	buttons := func() map[string]any {
		return map[string]any{"buttons": m.Buttons(), "popup": m.OpenPopup(), "prompt": m.Prompt()}
	}
	sendButtons := func() { a.Shell.Send(map[string]any{"t": "extensions", "extensions": buttons()}) }
	m.OnChanged(sendButtons)
	a.Tabs.OnListChanged(sendButtons)
	a.Shell.AddPart(extensionsShellPart(a, m, buttons))

	m.OnChanged(func() { a.Settings.Send(map[string]any{"t": "extensions", "extensions": m.View()}) })
	a.Settings.AddPart(extensionsSettingsPart(a, m))
}

// extensionEvents is the engine's extension listener: each function only forwards.
type extensionEvents struct {
	m   *extensions.Manager
	api *chromeapi.API
}

func (x *extensionEvents) ExtensionsChanged()                            { x.m.HandleChanged() }
func (x *extensionEvents) ExtensionInstallPrompt(p engine.InstallPrompt) { x.m.HandlePrompt(p) }
func (x *extensionEvents) ExtensionInstallDone(r engine.InstallResult)   { x.m.HandleInstallDone(r) }
func (x *extensionEvents) ExtensionCall(c engine.ExtensionCall) (string, string) {
	return x.api.Call(c)
}
func (x *extensionEvents) ExtensionPopupClosed()  { x.m.HandlePopupClosed() }
func (x *extensionEvents) PathChosen(path string) { x.m.HandlePathChosen(path) }
func (x *extensionEvents) ExtensionBlockedCount(id string, tab engine.TabID, count int) {
	x.m.HandleBlockedCount(id, tab, count)
}

// webStore is where "Chrome Web Store" in the extension manager goes. An extension is
// installed from the store's own page.
const webStore = "https://chromewebstore.google.com/"

// extensionCommand is what the shell and the settings page send about extensions.
type extensionCommand struct {
	ID   string      `json:"extension"`
	Text string      `json:"text"`
	On   bool        `json:"on"`
	Rect engine.Rect `json:"rect"`
}

// The shell's part: the buttons and the install prompt in its snapshot, and a press on a
// button, a pin, the prompt's answer, and the menu's way to the extension manager.
func extensionsShellPart(a *App, m *extensions.Manager, buttons func() map[string]any) shell.Part {
	return shell.Part{
		Snapshot: func(into map[string]any) { into["extensions"] = buttons() },
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			var c extensionCommand
			json.Unmarshal(msg, &c)
			switch t {
			case "extensionClick":
				m.Click(w, c.ID, c.Rect)
			case "extensionAnswerPrompt":
				m.AnswerPrompt(c.On)
			case "extensionClosePopup":
				m.ClosePopup()
			case "extensionPin":
				m.Pin(c.ID, c.On)
			case "extensionsManage":
				a.Settings.OpenSection(w, "extensions")
			default:
				return false
			}
			return true
		},
	}
}

// The settings page's part: the extension manager's view in its snapshot, and its commands.
func extensionsSettingsPart(a *App, m *extensions.Manager) settings.Part {
	return settings.Part{
		Snapshot: func(into map[string]any) { into["extensions"] = m.View() },
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			var c extensionCommand
			json.Unmarshal(msg, &c)
			switch t {
			case "extensionOpenStore":
				// The store is a website: it opens in a tab, which the settings page
				// would cover.
				a.Settings.Close(w)
				a.Tabs.Open(webStore)
			case "extensionAnswerPrompt":
				m.AnswerPrompt(c.On)
			case "extensionLoadUnpacked":
				m.LoadUnpacked(w)
			case "extensionInstallPackage":
				m.InstallPackage(w)
			case "extensionDismissError":
				m.DismissError()
			case "extensionRemove":
				m.Remove(c.ID)
			case "extensionSetEnabled":
				m.SetEnabled(c.ID, c.On)
			case "extensionPin":
				m.Pin(c.ID, c.On)
			default:
				return false
			}
			return true
		},
	}
}
