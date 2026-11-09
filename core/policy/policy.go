// Package policy decides what's allowed. The first app only has a host blocklist.
package policy

import (
	"net/url"
	"strings"
)

// Manager holds the current policy.
type Manager struct {
	blocked []string
}

// New returns a manager blocking these hosts and their subdomains.
func New(blocked []string) *Manager {
	m := &Manager{}
	for _, h := range blocked {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			m.blocked = append(m.blocked, h)
		}
	}
	return m
}

// IsAllowedURL is the one function every URL decision goes through. It's in memory
// and never waits.
func (m *Manager) IsAllowedURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, b := range m.blocked {
		if host == b || strings.HasSuffix(host, "."+b) {
			return false
		}
	}
	return true
}
