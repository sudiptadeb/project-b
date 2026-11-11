//go:build extensions

package chromeapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"projectb/engine"
	"projectb/extensions"
)

// details is the argument most chrome.action functions take.
type details struct {
	TabID engine.TabID `json:"tabId"`
}

// colour reads a badge colour, [r, g, b, a] or "#rrggbb", "#rgb" or "#rrggbbaa", into
// "#rrggbbaa".
func colour(_ call, raw json.RawMessage) (string, error) {
	var parts []int
	if json.Unmarshal(raw, &parts) == nil && len(parts) == 4 {
		return fmt.Sprintf("#%02x%02x%02x%02x", parts[0]&255, parts[1]&255, parts[2]&255, parts[3]&255), nil
	}
	var s string
	json.Unmarshal(raw, &s)
	if len(s) > 0 && s[0] == '#' {
		hex := s[1:]
		if len(hex) == 3 {
			hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
		}
		if len(hex) == 6 {
			hex += "ff"
		}
		if _, err := strconv.ParseUint(hex, 16, 32); err == nil && len(hex) == 8 {
			return "#" + hex, nil
		}
	}
	return "", errors.New("The color specification could not be parsed.")
}

// colourArray turns "#rrggbbaa" back into [r, g, b, a]. A colour never set is clear.
func colourArray(c string) [4]int {
	var out [4]int
	if len(c) == 9 {
		v, _ := strconv.ParseUint(c[1:], 16, 32)
		out = [4]int{int(v >> 24), int(v >> 16 & 255), int(v >> 8 & 255), int(v & 255)}
	}
	return out
}

func init() {
	// A setter takes {tabId?, <key>: value}; a null value takes back what was set.
	set := func(name, key string, p extensions.Prop, convert func(call, json.RawMessage) (string, error)) {
		functions[name] = func(a *API, c call) (any, error) {
			var d details
			var fields map[string]json.RawMessage
			c.arg(0, &d)
			c.arg(0, &fields)
			var value *string
			if raw := fields[key]; raw != nil && string(raw) != "null" {
				s, err := convert(c, raw)
				if err != nil {
					return nil, err
				}
				value = &s
			}
			a.ext.SetProp(c.Extension, d.TabID, p, value)
			return nil, nil
		}
	}
	text := func(_ call, raw json.RawMessage) (string, error) {
		var s string
		err := json.Unmarshal(raw, &s)
		return s, err
	}
	get := func(name string, p extensions.Prop, convert func(string) any) {
		functions[name] = func(a *API, c call) (any, error) {
			var d details
			c.arg(0, &d)
			return convert(a.ext.Prop(c.Extension, d.TabID, p)), nil
		}
	}
	same := func(s string) any { return s }

	set("action.setTitle", "title", extensions.Title, text)
	get("action.getTitle", extensions.Title, same)
	set("action.setBadgeText", "text", extensions.Badge, text)
	get("action.getBadgeText", extensions.Badge, same)
	set("action.setBadgeBackgroundColor", "color", extensions.BadgeColor, colour)
	get("action.getBadgeBackgroundColor", extensions.BadgeColor, func(s string) any { return colourArray(s) })
	set("action.setBadgeTextColor", "color", extensions.BadgeTextColor, colour)
	get("action.getBadgeTextColor", extensions.BadgeTextColor, func(s string) any { return colourArray(s) })
	// A popup is a page inside the extension, given by its path; "" means the button has none.
	set("action.setPopup", "popup", extensions.Popup, func(c call, raw json.RawMessage) (string, error) {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || s == "" {
			return "", err
		}
		return resolve(c.Extension, s), nil
	})
	get("action.getPopup", extensions.Popup, same)
	// The engine reads the icon's file and hands the picture over as a data: URL.
	set("action.setIcon", "icon", extensions.Icon, text)

	enable := func(on bool) func(a *API, c call) (any, error) {
		return func(a *API, c call) (any, error) {
			var tab engine.TabID
			c.arg(0, &tab)
			a.ext.SetButtonEnabled(c.Extension, tab, on)
			return nil, nil
		}
	}
	functions["action.enable"] = enable(true)
	functions["action.disable"] = enable(false)
	functions["action.isEnabled"] = func(a *API, c call) (any, error) {
		var tab engine.TabID
		c.arg(0, &tab)
		return a.ext.ButtonEnabled(c.Extension, tab), nil
	}
	functions["action.getUserSettings"] = func(a *API, c call) (any, error) {
		return map[string]bool{"isOnToolbar": a.ext.Pinned(c.Extension)}, nil
	}

	functions["commands.getAll"] = func(a *API, c call) (any, error) {
		type command struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Shortcut    string `json:"shortcut"`
		}
		out := []command{}
		for _, m := range a.caller(c).Commands {
			out = append(out, command{Name: m.Name, Description: m.Description})
		}
		return out, nil
	}
}

// Clicked tells an extension its button was pressed, with the tab that was showing.
func (a *API) Clicked(id string) {
	e, _ := a.ext.Get(id)
	a.dispatch(id, "action.onClicked", a.tabFor(e, a.tabs.Active()))
}
