package app

import (
	"encoding/json"

	"projectb/engine"
	"projectb/shell"
)

// downloadsShellPart gives every shell the profile's downloads, in its snapshot and on
// each change, and takes what a shell asks of one (docs/daily-driver.md).
func (a *App) downloadsShellPart() shell.Part {
	a.Downloads.OnChanged(func() { a.Shell.Send(map[string]any{"t": "downloads", "downloads": a.Downloads.List()}) })
	return shell.Part{
		Snapshot: func(into map[string]any) { into["downloads"] = a.Downloads.List() },
		Handle: func(_ engine.WindowID, t string, msg []byte) bool {
			switch t {
			case "downloadAction":
				var r struct {
					ID     int    `json:"id"`
					Action string `json:"action"`
				}
				json.Unmarshal(msg, &r)
				a.Downloads.Act(r.ID, r.Action)
			case "clearDownloads":
				a.Downloads.Clear()
			default:
				return false
			}
			return true
		},
	}
}
