package app

import (
	"encoding/json"

	"projectb/engine"
	"projectb/shell"
	"projectb/timeline"
)

// shellDataPart gives the shell what it saved for the whole profile, profileData, in its
// snapshot, and keeps what it sends: profileData with its data, any JSON. What one window's
// shell saves reaches the shell in every window at once, so two windows agree. Each shell
// has its own, so a switch to another shell and back finds it as it was.
func (a *App) shellDataPart() shell.Part {
	return shell.Part{
		Snapshot: func(into map[string]any) { into["profileData"] = a.ShellData.Get(a.Prefs.Shell()) },
		Handle: func(_ engine.WindowID, t string, msg []byte) bool {
			if t != "profileData" {
				return false
			}
			var c struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(msg, &c); err != nil || c.Data == nil {
				timeline.Warnf("shell: bad profileData")
				return true
			}
			if err := a.ShellData.Set(a.Prefs.Shell(), c.Data); err != nil {
				timeline.Errorf("shelldata: %v", err)
				return true
			}
			a.Shell.Send(map[string]any{"t": "profileData", "profileData": c.Data})
			return true
		},
	}
}
