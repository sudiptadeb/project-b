package policy

import "testing"

func TestIsAllowedURL(t *testing.T) {
	m := New([]string{"blocked.test"})
	for raw, want := range map[string]bool{
		"https://example.com/":        true,
		"https://blocked.test/":       false,
		"https://www.blocked.test/x":  false,
		"chrome://shell/":             false,
		"CHROME://SHELL/index.html":   false,
		"chrome://gpu/":               true,
		"chrome://process-internals/": true,
		"https://shell/":              true,
	} {
		if got := m.IsAllowedURL(raw); got != want {
			t.Errorf("IsAllowedURL(%q) = %v, want %v", raw, got, want)
		}
	}
}
