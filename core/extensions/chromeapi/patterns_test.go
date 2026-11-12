//go:build extensions

package chromeapi

import "testing"

func TestMatchPatterns(t *testing.T) {
	for _, c := range []struct {
		pattern, url string
		want         bool
	}{
		{"<all_urls>", "https://a.example/x", true},
		{"<all_urls>", "chrome-extension://abc/p.html", false},
		{"*://*/*", "http://a.example:8080/x?y", true},
		{"*://*/*", "file:///tmp/a", false},
		{"https://*.example.com/*", "https://example.com/", true},
		{"https://*.example.com/*", "https://a.b.example.com/p", true},
		{"https://*.example.com/*", "https://badexample.com/", false},
		{"https://*.example.com/*", "http://example.com/", false},
		{"*://Example.com/a*", "https://example.com/abc", true},
		{"*://example.com/a*", "https://example.com/b", false},
		{"*://example.com/*?q=1", "https://example.com/x?q=1", true},
		{"*://example.com:8080/*", "https://example.com:8080/", true},
		{"*://example.com:8080/*", "https://example.com/", false},
		{"*://example.com:*/*", "https://example.com:9/", true},
		{"file:///tmp/*", "file:///tmp/a.txt", true},
		{"http://127.0.0.1/*", "http://127.0.0.1:5555/", true},
		{"no pattern", "https://example.com/", false},
	} {
		if got := matchURL(c.pattern, c.url); got != c.want {
			t.Errorf("matchURL(%q, %q) = %v", c.pattern, c.url, got)
		}
	}
	for _, c := range []struct {
		pattern, title string
		want           bool
	}{
		{"*News*", "Daily News - Home", true},
		{"*news*", "Daily News", false},
		{"Inbox (?)", "Inbox (3)", true},
		{"Inbox (?)", "Inbox (12)", false},
		{`a\*b`, "a*b", true},
		{`a\*b`, "axb", false},
		{"", "", true},
		{"*", "", true},
	} {
		if got := matchTitle(c.pattern, c.title); got != c.want {
			t.Errorf("matchTitle(%q, %q) = %v", c.pattern, c.title, got)
		}
	}
}
