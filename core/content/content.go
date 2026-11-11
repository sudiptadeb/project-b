// Package content is where content's hooks arrive in Go. Nothing outside it sees a
// raw hook: Wire turns the few the app needs into engine.Listener calls, and a new
// use of a hook is a Go change here, with no C or C++ touched.
//
// Every carried hook has a number and a flat list of values (engine/abi/pb_abi.h).
// The generated files (*_gen.go, from tools/gen_hooks.py) know each hook's layout
// and offer typed functions: On for a hook that informs, Answer for one that wants a
// value back. Subscribing tells the adapter to let that hook cross; an unsubscribed
// hook never does.
package content

import (
	"fmt"
	"runtime/debug"
)

// Hook is a hook's number, shared with the adapter.
type Hook int32

// Value is one value as it crosses: an integer, a float or a string, by layout.
type Value struct {
	I int64
	D float64
	S string
}

// Optional is a std::optional that crossed: Set says whether Value means anything.
type Optional[T any] struct {
	Set   bool
	Value T
}

// Hooks is every hook the adapter can raise. There is one per process; cmd/lib makes
// it and hands every crossing to Dispatch, and every function posted to the main thread
// to Run.
//
// A panic in Go is caught at that boundary once: OnPanic is told, the call returns as if
// Go had not answered, and the browser carries on. A navigation gate that panics blocks
// the navigation rather than let it through. A second panic in the same run is not
// caught: the app crashes, and its crash log has the stack.
type Hooks struct {
	subscribe func(Hook)
	counts    map[Hook]int
	panics    int

	// OnPanic is told about the panic that was caught: where it happened, its value and
	// the stack where it was raised.
	OnPanic func(where string, value any, stack []byte)

	Observer ObserverHooks
	Delegate DelegateHooks
	Browser  BrowserHooks
	Throttle ThrottleHooks
	Events   Events
}

// New returns a Hooks whose subscriptions reach the adapter through subscribe.
func New(subscribe func(Hook)) *Hooks {
	h := &Hooks{subscribe: subscribe, counts: map[Hook]int{}}
	h.Observer.hooks = h
	h.Delegate.hooks = h
	h.Browser.hooks = h
	h.Throttle.hooks = h
	return h
}

// Dispatch takes one crossing and returns the answer, if a handler gave one.
func (h *Hooks) Dispatch(id Hook, args []Value) (v Value, ok bool) {
	h.counts[id]++
	defer func() {
		if p := recover(); p != nil {
			h.panicked(h.name(id), p)
			v, ok = Value{}, false
			if id >= firstThrottle {
				v, ok = Value{I: int64(NavigationThrottleThrottleActionBlockRequest)}, true
			}
		}
	}()
	r := &reader{v: args}
	switch {
	case id < FirstHook:
		return h.Events.dispatch(id, r)
	case id < firstDelegate:
		return h.Observer.dispatch(id, r)
	case id < firstBrowser:
		return h.Delegate.dispatch(id, r)
	case id < firstThrottle:
		return h.Browser.dispatch(id, r)
	}
	return h.Throttle.dispatch(id, r)
}

// Run runs a function posted to the main thread, with the same catch for a panic as
// Dispatch.
func (h *Hooks) Run(fn func()) {
	defer func() {
		if p := recover(); p != nil {
			h.panicked("a posted function", p)
		}
	}()
	fn()
}

// panicked is called from the deferred function that recovered p, so the stack still
// holds the frames that raised it.
func (h *Hooks) panicked(where string, p any) {
	h.panics++
	if h.panics > 1 {
		panic(fmt.Sprintf("a second panic in Go, in %s: %v", where, p))
	}
	if h.OnPanic != nil {
		h.OnPanic(where, p, debug.Stack())
	}
}

func (h *Hooks) name(id Hook) string {
	if name, ok := Names[id]; ok {
		return name
	}
	return eventNames[id]
}

// Counts returns how often each hook has crossed, by name. For stats.
func (h *Hooks) Counts() map[string]int {
	out := map[string]int{}
	for id, n := range h.counts {
		out[h.name(id)] = n
	}
	return out
}

// reader walks a hook's values in layout order.
type reader struct {
	v []Value
	i int
}

func (r *reader) next() Value {
	if r.i >= len(r.v) {
		return Value{}
	}
	v := r.v[r.i]
	r.i++
	return v
}

func (r *reader) int() int64     { return r.next().I }
func (r *reader) bool() bool     { return r.next().I != 0 }
func (r *reader) float() float64 { return r.next().D }
func (r *reader) str() string    { return r.next().S }

func readOptional[T any](r *reader, read func() T) Optional[T] {
	set := r.bool()
	return Optional[T]{Set: set, Value: read()}
}

func readVector[T any](r *reader, read func() T) []T {
	n := int(r.int())
	out := make([]T, n)
	for i := range out {
		out[i] = read()
	}
	return out
}

func boolValue(b bool) Value {
	if b {
		return Value{I: 1}
	}
	return Value{}
}
