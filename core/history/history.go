// Package history owns browsing history: a map in memory on the main thread, and a
// worker that appends to a file. The worker holds no state and only gets copies.
package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"projectb/timeline"
	"sort"
	"strings"
	"time"
)

// Entry is one visited URL.
type Entry struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Visits    int    `json:"visits"`
	LastVisit int64  `json:"lastVisit"`
}

// Manager owns the history.
type Manager struct {
	path    string
	post    func(func())
	entries map[string]*Entry
	loaded  bool
	writes  chan Entry
	done    chan struct{}
}

// New returns a manager storing under dir and starts its writer. post runs a function
// back on the main thread.
func New(dir string, post func(func())) *Manager {
	m := &Manager{
		path:    filepath.Join(dir, "history.jsonl"),
		post:    post,
		entries: map[string]*Entry{},
		writes:  make(chan Entry, 1024),
		done:    make(chan struct{}),
	}
	go m.writer()
	return m
}

// Load reads saved history on a worker and calls ready on the main thread when it's in.
// Until then, Search just sees what's been visited this run.
func (m *Manager) Load(ready func()) {
	go func() {
		saved := readAll(m.path)
		m.post(func() {
			for url, e := range saved {
				if cur, ok := m.entries[url]; ok {
					cur.Visits += e.Visits
					if cur.Title == "" {
						cur.Title = e.Title
					}
				} else {
					m.entries[url] = e
				}
			}
			m.loaded = true
			if ready != nil {
				ready()
			}
		})
	}()
}

// AddVisit records a visit. Only http and https pages count.
func (m *Manager) AddVisit(url, title string) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return
	}
	e, ok := m.entries[url]
	if !ok {
		e = &Entry{URL: url}
		m.entries[url] = e
	}
	e.Visits++
	e.LastVisit = time.Now().Unix()
	if title != "" {
		e.Title = title
	}
	m.save(e)
}

// UpdateTitle sets the title of a URL already visited. Titles usually arrive after
// the visit.
func (m *Manager) UpdateTitle(url, title string) {
	e, ok := m.entries[url]
	if !ok || e.Title == title {
		return
	}
	e.Title = title
	m.save(e)
}

// Search returns the best matches for text, best first.
func (m *Manager) Search(text string, limit int) []Entry {
	q := strings.ToLower(strings.TrimSpace(text))
	if q == "" {
		return nil
	}
	type scored struct {
		e     *Entry
		score int
	}
	var hits []scored
	for _, e := range m.entries {
		if s := score(e, q); s > 0 {
			hits = append(hits, scored{e, s})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		if hits[i].e.LastVisit != hits[j].e.LastVisit {
			return hits[i].e.LastVisit > hits[j].e.LastVisit
		}
		return hits[i].e.URL < hits[j].e.URL
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]Entry, len(hits))
	for i, h := range hits {
		out[i] = *h.e
	}
	return out
}

// Close waits for pending writes.
func (m *Manager) Close() {
	close(m.writes)
	<-m.done
}

func score(e *Entry, q string) int {
	bare := bareURL(e.URL)
	base := 0
	switch {
	case strings.HasPrefix(bare, q):
		base = 1000
	case strings.Contains(strings.ToLower(e.URL), q):
		base = 100
	case strings.Contains(strings.ToLower(e.Title), q):
		base = 50
	default:
		return 0
	}
	return base + min(e.Visits, 50)*2
}

func bareURL(u string) string {
	u = strings.ToLower(u)
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.TrimPrefix(u, "www.")
}

func (m *Manager) save(e *Entry) {
	select {
	case m.writes <- *e:
	default:
		timeline.Warnf("history: write queue full, dropped a visit")
	}
}

func (m *Manager) writer() {
	defer close(m.done)
	var w *os.File
	for e := range m.writes {
		if w == nil {
			if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
				timeline.Errorf("history: %v", err)
				continue
			}
			f, err := os.OpenFile(m.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if err != nil {
				timeline.Errorf("history: %v", err)
				continue
			}
			w = f
			defer w.Close()
		}
		line, _ := json.Marshal(e)
		if _, err := w.Write(append(line, '\n')); err != nil {
			timeline.Errorf("history: %v", err)
		}
	}
}

// readAll reads the append-only file. The last line for a URL wins.
func readAll(path string) map[string]*Entry {
	out := map[string]*Entry{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.URL != "" {
			out[e.URL] = &e
		}
	}
	return out
}
