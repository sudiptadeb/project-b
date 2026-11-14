package app

import (
	"encoding/json"
	"net/url"

	"projectb/engine"
	"projectb/shell"
)

// A page's alert, confirm, prompt and leave-page question (docs/daily-driver.md): Go keeps
// each while its page waits and sends them all to every shell, which draws the one for its
// active tab and sends back the answer. The page stays held until then; the engine drops
// one whose tab navigated or closed.

// pageDialog is a dialog as the shells see it: what to show, and the labels of its buttons,
// so every shell words them alike. Cancel is "" for an alert, which has only OK.
type pageDialog struct {
	ID          engine.DialogID `json:"id"`
	Tab         engine.TabID    `json:"tab"`
	Kind        string          `json:"kind"`
	Title       string          `json:"title"`
	Message     string          `json:"message"`
	DefaultText string          `json:"defaultText"`
	OK          string          `json:"ok"`
	Cancel      string          `json:"cancel"`
	// A sign-in's: the site's own words, the host asking, for a shell to set apart, and
	// whether the password would go over a connection that isn't secure.
	Realm    string `json:"realm,omitempty"`
	Host     string `json:"host,omitempty"`
	Insecure bool   `json:"insecure,omitempty"`
}

func dialogOf(d engine.Dialog) pageDialog {
	p := pageDialog{ID: d.ID, Tab: d.Tab, Kind: d.Kind, Message: d.Message, DefaultText: d.DefaultText, OK: "OK", Cancel: "Cancel"}
	switch d.Kind {
	case "signin":
		// The realm is the site's own words, so it shows as theirs, under ours.
		p.Title, p.OK = "Sign in", "Sign in"
		host := d.Origin
		if u, err := url.Parse(d.Origin); err == nil && u.Host != "" {
			host = u.Host
		}
		p.Message = host + " asks for a username and password."
		if d.Reload {
			p.Message = "The proxy " + host + " asks for a username and password."
		}
		p.Realm, p.Host = d.Message, host
		// Over http the password would cross the network as it is typed.
		if u, err := url.Parse(d.Origin); err == nil && u.Scheme == "http" {
			p.Insecure = true
		}
	case "beforeunload":
		p.Title, p.Message, p.OK = "Leave site?", "Changes you made may not be saved.", "Leave"
		if d.Reload {
			p.Title, p.OK = "Reload site?", "Reload"
		}
	default:
		p.Title = "This page says"
		if u, err := url.Parse(d.Origin); err == nil && u.Host != "" {
			p.Title = u.Host + " says"
		}
		if d.Kind == "alert" {
			p.Cancel = ""
		}
	}
	return p
}

// pageDialogs keeps the dialogs pages wait on, oldest first.
type pageDialogs struct {
	a    *App
	e    engine.Engine
	list []pageDialog
}

func (p *pageDialogs) changed() {
	p.a.Shell.Send(map[string]any{"t": "dialogs", "dialogs": p.list})
}

func (p *pageDialogs) opened(d engine.Dialog) {
	p.list = append(p.list, dialogOf(d))
	p.changed()
}

func (p *pageDialogs) kindOf(id engine.DialogID) string {
	for _, d := range p.list {
		if d.ID == id {
			return d.Kind
		}
	}
	return ""
}

// drop takes dialog id off the list, saying whether it was there.
func (p *pageDialogs) drop(id engine.DialogID) bool {
	for i, d := range p.list {
		if d.ID == id {
			p.list = append(p.list[:i:i], p.list[i+1:]...)
			p.changed()
			return true
		}
	}
	return false
}

func (p *pageDialogs) shellPart() shell.Part {
	return shell.Part{
		Snapshot: func(into map[string]any) { into["dialogs"] = p.list },
		Handle: func(_ engine.WindowID, t string, msg []byte) bool {
			if t != "answerDialog" {
				return false
			}
			var r struct {
				ID       engine.DialogID `json:"id"`
				OK       bool            `json:"ok"`
				Text     string          `json:"text"`
				Password string          `json:"password"`
			}
			json.Unmarshal(msg, &r)
			kind := p.kindOf(r.ID)
			if !p.drop(r.ID) {
				return true
			}
			// A sign-in's text is the username.
			if kind == "signin" {
				p.e.AnswerSignIn(r.ID, r.OK, r.Text, r.Password)
			} else {
				p.e.AnswerDialog(r.ID, r.OK, r.Text)
			}
			return true
		},
	}
}

func (a *App) DialogOpened(d engine.Dialog)    { a.dialogs.opened(d) }
func (a *App) DialogClosed(id engine.DialogID) { a.dialogs.drop(id) }
