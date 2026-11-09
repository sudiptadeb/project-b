// Package suggest answers what the address bar should suggest. History is its only
// source today, and it stays separate so history doesn't learn about address bars.
package suggest

import "projectb/history"

const maxItems = 6

// Item is one suggestion.
type Item struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// Manager answers suggestion queries.
type Manager struct {
	hist *history.Manager
}

// New returns a manager reading from hist.
func New(hist *history.Manager) *Manager { return &Manager{hist: hist} }

// Query returns suggestions for what the user has typed so far.
func (m *Manager) Query(text string) []Item {
	entries := m.hist.Search(text, maxItems)
	items := make([]Item, len(entries))
	for i, e := range entries {
		items[i] = Item{URL: e.URL, Title: e.Title}
	}
	return items
}
