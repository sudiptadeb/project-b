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
// it and hands every crossing to Dispatch.
type Hooks struct {
	subscribe func(Hook)
	counts    map[Hook]int

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
func (h *Hooks) Dispatch(id Hook, args []Value) (Value, bool) {
	h.counts[id]++
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

// Counts returns how often each hook has crossed, by name. For stats.
func (h *Hooks) Counts() map[string]int {
	out := map[string]int{}
	for id, n := range h.counts {
		name, ok := Names[id]
		if !ok {
			name = eventNames[id]
		}
		out[name] = n
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
