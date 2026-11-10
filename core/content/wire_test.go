package content

import (
	"fmt"
	"reflect"
	"testing"

	"projectb/engine"
)

// pack lays Go values out the way the adapter does, so a test can fake a crossing.
// The generated structs declare their fields in the adapter's order.
func pack(values ...any) []Value {
	var out []Value
	for _, v := range values {
		out = append(out, packOne(reflect.ValueOf(v))...)
	}
	return out
}

func packOne(v reflect.Value) []Value {
	switch v.Kind() {
	case reflect.Bool:
		return []Value{boolValue(v.Bool())}
	case reflect.Int, reflect.Int32, reflect.Int64:
		return []Value{{I: v.Int()}}
	case reflect.Float64:
		return []Value{{D: v.Float()}}
	case reflect.String:
		return []Value{{S: v.String()}}
	case reflect.Slice:
		out := []Value{{I: int64(v.Len())}}
		for i := 0; i < v.Len(); i++ {
			out = append(out, packOne(v.Index(i))...)
		}
		return out
	case reflect.Struct:
		var out []Value
		for i := 0; i < v.NumField(); i++ {
			out = append(out, packOne(v.Field(i))...)
		}
		return out
	}
	panic("cannot pack " + v.Kind().String())
}

// recorder is an engine.Listener that writes down what it hears.
type recorder struct {
	got   []string
	allow bool
}

func (r *recorder) note(s string)           { r.got = append(r.got, s) }
func (r *recorder) ShellMessage(msg []byte) { r.note("shell " + string(msg)) }
func (r *recorder) NavigationStarting(tab engine.TabID, url string) bool {
	r.note("starting " + url)
	return r.allow
}
func (r *recorder) NavigationFinished(tab engine.TabID, url string) { r.note("finished " + url) }
func (r *recorder) NavigationFailed(tab engine.TabID, url, reason string) {
	r.note("failed " + url + " " + reason)
}
func (r *recorder) HistoryChanged(tab engine.TabID, back, fwd bool) {
	if back {
		r.note("canGoBack")
	}
}
func (r *recorder) TitleChanged(tab engine.TabID, title string) { r.note("title " + title) }
func (r *recorder) LoadingChanged(tab engine.TabID, loading bool) {
	if loading {
		r.note("loading")
	} else {
		r.note("loaded")
	}
}
func (r *recorder) LoadProgressChanged(tab engine.TabID, progress float64) {
	r.note(fmt.Sprintf("progress %.2f", progress))
}
func (r *recorder) PageCrashed(tab engine.TabID)                    { r.note("crashed") }
func (r *recorder) FaviconChanged(tab engine.TabID, dataURL string) { r.note("favicon " + dataURL) }
func (r *recorder) TabOpened(opener, tab engine.TabID, url string, foreground bool) {
	r.note("opened " + url)
}
func (r *recorder) CloseRequested(tab engine.TabID)    { r.note(fmt.Sprintf("close %d", tab)) }
func (r *recorder) ActivateRequested(tab engine.TabID) { r.note(fmt.Sprintf("activate %d", tab)) }
func (r *recorder) Key(k engine.Key, phase engine.KeyPhase, in engine.Surface) bool {
	r.note(fmt.Sprintf("key %d %d %d %d", k.Code, k.Mods, phase, in))
	return k.Mods&engine.ModCmd != 0
}
func (r *recorder) ModsChanged(m engine.Mods)                { r.note(fmt.Sprint("mods ", m)) }
func (r *recorder) MenuChosen(action string)                 { r.note("menu " + action) }
func (r *recorder) OSBindings(byTitle map[string]engine.Key) { r.note(fmt.Sprint("os ", byTitle)) }
func (r *recorder) SettingsMessage(msg []byte)               { r.note("settings " + string(msg)) }
func (r *recorder) WindowChanged(s engine.WindowState) {
	r.note(fmt.Sprintf("window %gx%g r=%g full=%v", s.W, s.H, s.CornerRadius, s.FullScreen))
}
func (r *recorder) ShellCrashed() { r.note("shell crashed") }

func wired(t *testing.T) (*Hooks, *recorder, map[Hook]bool) {
	t.Helper()
	subscribed := map[Hook]bool{}
	h := New(func(id Hook) { subscribed[id] = true })
	r := &recorder{allow: true}
	Wire(h, r)
	return h, r, subscribed
}

func TestOnlyWiredHooksAreSubscribed(t *testing.T) {
	_, _, subscribed := wired(t)
	for _, want := range []Hook{ObserverDidFinishNavigation, ObserverDidStartLoading, ThrottleWillStartRequest, DelegatePreHandleKeyboardEvent} {
		if !subscribed[want] {
			t.Errorf("%s not subscribed", Names[want])
		}
	}
	if subscribed[ObserverDidChangeVisibleSecurityState] {
		t.Error("an unused hook was subscribed")
	}
}

func TestCommitBecomesFinishedAndHistory(t *testing.T) {
	h, r, _ := wired(t)
	tab := WebContents{ID: 3, URL: "https://a.test/", Title: "A", CanGoBack: true}
	nav := NavigationHandle{TabID: 3, URL: "https://a.test/", IsInPrimaryMainFrame: true, HasCommitted: true}
	h.Dispatch(ObserverDidFinishNavigation, pack(tab, nav))
	want := []string{"canGoBack", "finished https://a.test/", "title A"}
	if !reflect.DeepEqual(r.got, want) {
		t.Fatalf("got %v, want %v", r.got, want)
	}
}

func TestFailureCarriesTheErrorName(t *testing.T) {
	h, r, _ := wired(t)
	nav := NavigationHandle{TabID: 1, URL: "https://x.test/", IsInPrimaryMainFrame: true, NetError: -105, NetErrorName: "net::ERR_NAME_NOT_RESOLVED"}
	h.Dispatch(ObserverDidFinishNavigation, pack(WebContents{ID: 1}, nav))
	if len(r.got) != 1 || r.got[0] != "failed https://x.test/ net::ERR_NAME_NOT_RESOLVED" {
		t.Fatalf("got %v", r.got)
	}
	// Replaced by a newer navigation: not a failure.
	r.got = nil
	nav.NetError = netErrAborted
	h.Dispatch(ObserverDidFinishNavigation, pack(WebContents{ID: 1}, nav))
	if len(r.got) != 0 {
		t.Fatalf("got %v", r.got)
	}
}

func TestSubframesAndTheShellAreIgnored(t *testing.T) {
	h, r, _ := wired(t)
	h.Dispatch(ObserverDidFinishNavigation, pack(WebContents{ID: 1}, NavigationHandle{TabID: 1, HasCommitted: true}))
	h.Dispatch(ObserverDidFinishNavigation, pack(WebContents{ID: 0}, NavigationHandle{IsInPrimaryMainFrame: true, HasCommitted: true}))
	h.Dispatch(ObserverDidStartLoading, pack(WebContents{ID: 0}))
	if len(r.got) != 0 {
		t.Fatalf("got %v", r.got)
	}
}

func TestGateAnswersProceedOrBlock(t *testing.T) {
	h, r, _ := wired(t)
	nav := NavigationHandle{TabID: 2, URL: "https://blocked.test/", IsInPrimaryMainFrame: true}
	v, ok := h.Dispatch(ThrottleWillStartRequest, pack(nav))
	if !ok || v.I != int64(NavigationThrottleThrottleActionProceed) {
		t.Fatalf("allowed: %v %v", v, ok)
	}
	r.allow = false
	v, ok = h.Dispatch(ThrottleWillRedirectRequest, pack(nav))
	if !ok || v.I != int64(NavigationThrottleThrottleActionBlockRequest) {
		t.Fatalf("blocked: %v %v", v, ok)
	}
	// The shell's navigation is content's business.
	if _, ok := h.Dispatch(ThrottleWillStartRequest, pack(NavigationHandle{IsInPrimaryMainFrame: true})); ok {
		t.Fatal("the shell was gated")
	}
}

// The recorder takes any Cmd key: the app's answer is the hook's.
func TestKeysReachTheApp(t *testing.T) {
	h, r, _ := wired(t)
	key := func(id int64, mods WebInputEventModifiers, code KeyboardCode) []Value {
		return pack(WebContents{ID: id}, WebContents{ID: id}, NativeWebKeyboardEvent{Type: WebInputEventTypeRawKeyDown, Modifiers: mods, WindowsKeyCode: code})
	}
	v, ok := h.Dispatch(DelegatePreHandleKeyboardEvent, key(1, WebInputEventModifiersMetaKey|WebInputEventModifiersShiftKey, KeyboardCodeW))
	if !ok || v.I != int64(KeyboardEventProcessingResultHandled) {
		t.Fatalf("Cmd+Shift+W before the page: %v %v", v, ok)
	}
	if _, ok := h.Dispatch(DelegatePreHandleKeyboardEvent, key(1, 0, KeyboardCodeW)); ok {
		t.Fatal("a W the app didn't want was taken from the page")
	}
	if v, ok := h.Dispatch(DelegateHandleKeyboardEvent, key(-1, WebInputEventModifiersMetaKey, KeyboardCodeL)); !ok || v.I != 1 {
		t.Fatalf("Cmd+L after the settings page: %v %v", v, ok)
	}
	want := []string{
		fmt.Sprintf("key %d %d %d %d", KeyboardCodeW, engine.ModCmd|engine.ModShift, engine.KeyBeforePage, engine.SurfacePage),
		fmt.Sprintf("key %d %d %d %d", KeyboardCodeW, 0, engine.KeyBeforePage, engine.SurfacePage),
		fmt.Sprintf("key %d %d %d %d", KeyboardCodeL, engine.ModCmd, engine.KeyUnhandled, engine.SurfaceSettings),
	}
	if fmt.Sprint(r.got) != fmt.Sprint(want) {
		t.Fatalf("got %v, want %v", r.got, want)
	}
}

func TestCloseAndActivateReachTheListener(t *testing.T) {
	h, r, subscribed := wired(t)
	if !subscribed[DelegateCloseContents] || !subscribed[DelegateActivateContents] {
		t.Fatal("close or activate not subscribed")
	}
	h.Dispatch(DelegateCloseContents, pack(WebContents{ID: 4}, WebContents{ID: 4}))
	h.Dispatch(DelegateActivateContents, pack(WebContents{ID: 4}, WebContents{ID: 5}))
	h.Dispatch(DelegateCloseContents, pack(WebContents{ID: 0}, WebContents{ID: 0}))
	if !reflect.DeepEqual(r.got, []string{"close 4", "activate 5"}) {
		t.Fatalf("got %v", r.got)
	}
}

func TestLoadProgressReachesTheListener(t *testing.T) {
	h, r, _ := wired(t)
	h.Dispatch(ObserverLoadProgressChanged, pack(WebContents{ID: 2}, 0.4))
	h.Dispatch(ObserverLoadProgressChanged, pack(WebContents{ID: 0}, 0.9))
	if !reflect.DeepEqual(r.got, []string{"progress 0.40"}) {
		t.Fatalf("got %v", r.got)
	}
}

func TestEngineEventsNeedNoSubscription(t *testing.T) {
	h, r, _ := wired(t)
	h.Dispatch(eventTabOpened, []Value{{I: 1}, {I: 2}, {S: "https://p.test/"}, {I: 1}})
	h.Dispatch(eventWindowChanged, []Value{{D: 1100}, {D: 700}, {D: 1}, {D: 2}, {D: 3}, {D: 4}, {D: 20}, {I: 1}})
	h.Dispatch(eventShellCrashed, nil)
	if !reflect.DeepEqual(r.got, []string{"opened https://p.test/", "window 1100x700 r=20 full=true", "shell crashed"}) {
		t.Fatalf("got %v", r.got)
	}
	if h.Counts()["engine.TabOpened"] != 1 {
		t.Fatalf("counts %v", h.Counts())
	}
}
