package content

import (
	"encoding/json"
	"errors"
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

func (r *recorder) note(s string) { r.got = append(r.got, s) }
func (r *recorder) ShellMessage(w engine.WindowID, msg []byte) {
	r.note(fmt.Sprintf("shell %d %s", w, msg))
}
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
func (r *recorder) ContextMenu(tab engine.TabID, m engine.ContextMenu) {
	r.note(fmt.Sprintf("menu %d %s %s %s", tab, m.Link, m.Src, m.Media))
}
func (r *recorder) ScrollDirectionChanged(tab engine.TabID, down bool) {
	r.note(fmt.Sprintf("scroll %d %v", tab, down))
}

func (r *recorder) LoadProgressChanged(tab engine.TabID, progress float64) {
	r.note(fmt.Sprintf("progress %.2f", progress))
}
func (r *recorder) PageCrashed(tab engine.TabID)                    { r.note("crashed") }
func (r *recorder) FaviconChanged(tab engine.TabID, dataURL string) { r.note("favicon " + dataURL) }
func (r *recorder) TabOpened(opener, tab engine.TabID, url, workspace string, foreground, newWindow bool) {
	r.note(fmt.Sprint("opened ", url, " in ", workspace, " new window ", newWindow))
}
func (r *recorder) CloseRequested(tab engine.TabID)    { r.note(fmt.Sprintf("close %d", tab)) }
func (r *recorder) ActivateRequested(tab engine.TabID) { r.note(fmt.Sprintf("activate %d", tab)) }
func (r *recorder) Key(k engine.Key, phase engine.KeyPhase, in engine.Surface) bool {
	r.note(fmt.Sprintf("key %d %d %d %d", k.Code, k.Mods, phase, in))
	return k.Mods&engine.ModCmd != 0
}
func (r *recorder) ModsChanged(m engine.Mods)                { r.note(fmt.Sprint("mods ", m)) }
func (r *recorder) MenuChosen(action string)                 { r.note("menu " + action) }
func (r *recorder) MenuList(name string) engine.Menu         { r.note("menu list " + name); return nil }
func (r *recorder) OSBindings(byTitle map[string]engine.Key) { r.note(fmt.Sprint("os ", byTitle)) }
func (r *recorder) SettingsMessage(w engine.WindowID, msg []byte) {
	r.note(fmt.Sprintf("settings %d %s", w, msg))
}
func (r *recorder) WindowChanged(w engine.WindowID, s engine.WindowState) {
	r.note(fmt.Sprintf("window %d %gx%g r=%g full=%v", w, s.W, s.H, s.CornerRadius, s.FullScreen))
}
func (r *recorder) TabFocused(tab engine.TabID)     { r.note(fmt.Sprint("focused ", tab)) }
func (r *recorder) WindowClosed(w engine.WindowID)  { r.note(fmt.Sprintf("window %d closed", w)) }
func (r *recorder) WindowFocused(w engine.WindowID) { r.note(fmt.Sprintf("window %d focused", w)) }
func (r *recorder) ShellCrashed(w engine.WindowID)  { r.note(fmt.Sprintf("shell %d crashed", w)) }
func (r *recorder) ProcessCrashed(p engine.ProcessCrash) {
	r.note(fmt.Sprintf("%s %s %d %d %s", p.Type, p.Status, p.ExitCode, p.PID, p.URL))
}

func (r *recorder) DevToolsCommand(method string, params []byte) (any, error) {
	r.note("devtools " + method + " " + string(params))
	switch method {
	case "ProjectB.echo":
		return json.RawMessage(params), nil
	case "ProjectB.fail":
		return nil, errors.New("it failed")
	}
	return nil, engine.ErrUnknownCommand
}

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

// A page's scroll direction reaches the listener as down or not; no direction, and the
// shell's own page, don't.
func TestScrollDirectionReachesTheListener(t *testing.T) {
	h, r, _ := wired(t)
	h.Dispatch(ObserverDidChangeVerticalScrollDirection, pack(WebContents{ID: 2}, int64(VerticalScrollDirectionDown)))
	h.Dispatch(ObserverDidChangeVerticalScrollDirection, pack(WebContents{ID: 2}, int64(VerticalScrollDirectionUp)))
	h.Dispatch(ObserverDidChangeVerticalScrollDirection, pack(WebContents{ID: 2}, int64(VerticalScrollDirectionNull)))
	h.Dispatch(ObserverDidChangeVerticalScrollDirection, pack(WebContents{ID: 0}, int64(VerticalScrollDirectionDown)))
	if !reflect.DeepEqual(r.got, []string{"scroll 2 true", "scroll 2 false"}) {
		t.Fatalf("got %v", r.got)
	}
}

// A long press on a link or an image in a tab is the shell's to show, and content is
// told it was handled; one on anything else, and in the shell's own page, stays content's.
func TestALongPressOnALinkReachesTheListener(t *testing.T) {
	h, r, _ := wired(t)
	ask := func(tab int64, p ContextMenuParams) bool {
		handled, _ := h.Delegate.answerHandleContextMenu(WebContents{ID: tab}, RenderFrameHost{}, p)
		return handled
	}
	if !ask(2, ContextMenuParams{LinkURL: "https://a.test/"}) || !ask(2, ContextMenuParams{SrcURL: "https://a.test/i.png", MediaType: 1}) ||
		ask(2, ContextMenuParams{SelectionText: "words"}) || ask(0, ContextMenuParams{LinkURL: "https://a.test/"}) {
		t.Fatalf("answers wrong, got %v", r.got)
	}
	if !reflect.DeepEqual(r.got, []string{"menu 2 https://a.test/  ", "menu 2  https://a.test/i.png image"}) {
		t.Fatalf("got %q", r.got)
	}
}

func TestEngineEventsNeedNoSubscription(t *testing.T) {
	h, r, _ := wired(t)
	h.Dispatch(eventTabOpened, []Value{{I: 0}, {I: 2}, {S: "https://p.test/"}, {S: "a1b2c3d4"}, {I: 1}, {I: 1}})
	h.Dispatch(eventWindowChanged, []Value{{I: 2}, {D: 1100}, {D: 700}, {D: 1}, {D: 2}, {D: 3}, {D: 4}, {D: 20}, {I: 1}})
	h.Dispatch(eventShellCrashed, []Value{{I: 2}})
	h.Dispatch(eventShellMessage, []Value{{I: 2}, {S: `{"t":"ready"}`}})
	h.Dispatch(eventSettingsMsg, []Value{{I: 2}, {S: `{"t":"close"}`}})
	h.Dispatch(eventWindowFocused, []Value{{I: 2}})
	h.Dispatch(eventWindowClosed, []Value{{I: 2}})
	h.Dispatch(eventProcessCrashed, []Value{{S: "renderer"}, {I: int64(TerminationStatusProcessCrashed)}, {I: 11}, {I: 42}, {S: "https://p.test/"}})
	if !reflect.DeepEqual(r.got, []string{"opened https://p.test/ in a1b2c3d4 new window true", "window 2 1100x700 r=20 full=true", "shell 2 crashed", `shell 2 {"t":"ready"}`, `settings 2 {"t":"close"}`, "window 2 focused", "window 2 closed", "renderer crashed 11 42 https://p.test/"}) {
		t.Fatalf("got %v", r.got)
	}
	if h.Counts()["engine.TabOpened"] != 1 {
		t.Fatalf("counts %v", h.Counts())
	}
}

func TestDevToolsCommandsAnswerWithTheirResultOrError(t *testing.T) {
	h, _, _ := wired(t)
	for _, c := range []struct{ method, params, answer string }{
		{"ProjectB.echo", `{"a":1}`, `{"result":{"a":1}}`},
		{"ProjectB.fail", `{}`, `{"error":{"code":-32000,"message":"it failed"}}`},
	} {
		v, ok := h.Dispatch(eventDevTools, pack(c.method, c.params))
		if !ok || v.S != c.answer {
			t.Errorf("%s: %v %q, want %q", c.method, ok, v.S, c.answer)
		}
	}
	if _, ok := h.Dispatch(eventDevTools, pack("ProjectB.nothing", `{}`)); ok {
		t.Error("an unknown command was answered, not left to content")
	}
}
