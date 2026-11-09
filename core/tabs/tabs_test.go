package tabs

import "testing"

func TestNormalizeInput(t *testing.T) {
	for in, want := range map[string]string{
		"":                      "",
		"  example.com ":        "https://example.com",
		"example.com:8080/a":    "https://example.com:8080/a",
		"https://example.com/a": "https://example.com/a",
		"http://example.com":    "http://example.com",
		"localhost:3000":        "http://localhost:3000",
		"about:blank":           "about:blank",
		"how do tabs work":      "https://duckduckgo.com/?q=how+do+tabs+work",
		"tabs":                  "https://duckduckgo.com/?q=tabs",
	} {
		if got := NormalizeInput(in); got != want {
			t.Errorf("NormalizeInput(%q) = %q, want %q", in, got, want)
		}
	}
}
