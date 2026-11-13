// Package instance keeps one copy of the app per profile folder. The first copy to start
// on a folder owns it for as long as it runs. A copy started on a folder another one owns
// hands its request over to the owner and quits, before it writes anything there. A copy
// relaunching for an update asks the owner of each folder to quit.
package instance

// request is what another copy sends the owner, as one line of JSON: the URLs a second
// copy was started with, possibly none, or with Quit, that the owner should quit, as a
// copy relaunching for an update asks.
type request struct {
	URLs []string `json:"urls"`
	Quit bool     `json:"quit,omitempty"`
}

// reply is the owner's answer to Quit, before it quits: its process, so the asking copy
// can wait for it to end.
type reply struct {
	PID int `json:"pid"`
}
