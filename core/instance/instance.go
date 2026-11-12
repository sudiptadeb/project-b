// Package instance keeps one copy of the app per profile folder. The first copy to start
// on a folder owns it for as long as it runs. A copy started on a folder another one owns
// hands its request over to the owner and quits, before it writes anything there.
package instance

// request is what a second copy hands over, as one line of JSON: the URLs it was
// started with, possibly none.
type request struct {
	URLs []string `json:"urls"`
}
