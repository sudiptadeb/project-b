package app

import (
	"encoding/json"

	"projectb/engine"
)

// DevToolsCommand answers our own DevTools commands, which a client of the debugging
// port sends as it sends content's. They let a test see and do what only Go knows:
//
//	ProjectB.getTabs              {tabs: [the tabs as the shell has them], active: id}
//	ProjectB.isAllowedURL {url}   {allowed: whether policy lets a tab load it}
//	ProjectB.panic                panics in Go, as a bug would
//
// The port is only open when the app is started with --remote-debugging-port, and a
// client of it can already drive the whole browser.
func (a *App) DevToolsCommand(method string, params []byte) (any, error) {
	switch method {
	case "ProjectB.getTabs":
		return map[string]any{"tabs": a.Tabs.All(), "active": a.Tabs.ActiveID()}, nil
	case "ProjectB.isAllowedURL":
		var p struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return map[string]any{"allowed": a.Policy.IsAllowedURL(p.URL)}, nil
	case "ProjectB.panic":
		panic("ProjectB.panic, asked for over DevTools")
	}
	return nil, engine.ErrUnknownCommand
}
