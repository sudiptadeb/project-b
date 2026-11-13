package app

import (
	"encoding/json"

	"projectb/engine"
	"projectb/profiles"
	"projectb/settings"
	"projectb/timeline"
)

// profileCommand is what the settings page's Profile section sends.
type profileCommand struct {
	Text string `json:"text"`
}

// profileSettingsPart is the settings page's Profile section, in a copy on a listed
// profile id: its name in the list, the template it was made from or last reset to, and
// a reset to a template. A folder named outright has no entry in the list, so no section.
func (a *App) profileSettingsPart(list *profiles.Manager, id string) settings.Part {
	view := func() map[string]any {
		p, _, err := list.Get(id)
		if err != nil {
			timeline.Errorf("profiles: %v", err)
		}
		templates := []string{}
		for _, t := range profiles.Templates() {
			templates = append(templates, t.Name)
		}
		return map[string]any{"name": p.Name, "template": p.Template, "templates": templates}
	}
	return settings.Part{
		Snapshot: func(into map[string]any) { into["profile"] = view() },
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			var c profileCommand
			json.Unmarshal(msg, &c)
			switch t {
			case "setProfileName":
				if err := list.SetName(id, c.Text); err != nil {
					timeline.Errorf("profiles: %v", err)
				}
			case "resetProfile":
				a.resetProfile(list, id, c.Text)
			default:
				return false
			}
			a.Settings.Send(map[string]any{"t": "profile", "profile": view()})
			return true
		},
	}
}

// resetProfile writes the template called name's values over the profile's prefs, through
// the prefs manager, so the shell in every window, the Dock and the list follow as they do
// a change by hand. History, cookies, logins and extensions stay as they are.
func (a *App) resetProfile(list *profiles.Manager, id, name string) {
	t, ok := profiles.TemplateNamed(name)
	if !ok {
		timeline.Warnf("profiles: no template %q", name)
		return
	}
	if err := a.Prefs.Apply(t.Prefs); err != nil {
		timeline.Errorf("profiles: template %s: %v", t.Name, err)
		return
	}
	if err := list.SetTemplate(id, t.Name); err != nil {
		timeline.Errorf("profiles: %v", err)
	}
}
