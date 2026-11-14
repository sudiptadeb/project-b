package mockengine

import (
	"strings"

	"projectb/engine"
)

// FindMatches is how many times a mock page holds any text searched for, except text
// with "nothing" in it, which it never holds.
const FindMatches = 3

// finds is what the mock keeps of searches in pages: the active match in each tab, and
// the tabs whose search was stopped, in order.
type finds struct {
	active  map[engine.TabID]int
	stopped []engine.TabID
}

func (e *Engine) Find(tab engine.TabID, request int, text string, forward, next bool) {
	matches := FindMatches
	if strings.Contains(text, "nothing") || text == "" {
		matches = 0
	}
	e.mu.Lock()
	if e.finds.active == nil {
		e.finds.active = map[engine.TabID]int{}
	}
	active := 1
	if next && matches > 0 {
		step := 1
		if !forward {
			step = -1
		}
		active = (e.finds.active[tab]-1+step+matches)%matches + 1
	}
	if matches == 0 {
		active = 0
	}
	e.finds.active[tab] = active
	e.mu.Unlock()
	e.Post(func() { e.listener.FindReply(tab, request, matches, active, true) })
}

func (e *Engine) StopFinding(tab engine.TabID) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.finds.stopped = append(e.finds.stopped, tab)
}

// FindsStopped are the tabs whose search the app stopped, in order.
func (e *Engine) FindsStopped() []engine.TabID {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.TabID(nil), e.finds.stopped...)
}
