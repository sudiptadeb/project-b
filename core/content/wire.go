package content

import "projectb/engine"

// netErrAborted is net::ERR_ABORTED: a navigation replaced by a newer one, not a failure.
const netErrAborted = -3

// Wire subscribes to the hooks the app needs and turns each into the engine.Listener
// call it means. This is the only place raw hooks become our events; managers never
// see content's names.
func Wire(h *Hooks, l engine.Listener) {
	h.Events.OnShellMessage = l.ShellMessage
	h.Events.OnTabOpened = l.TabOpened
	h.Events.OnFaviconChanged = l.FaviconChanged
	h.Events.OnWindowControlsChanged = l.WindowControlsChanged

	// The gate: every top-level navigation in a tab asks Go before it starts and on
	// each redirect. The shell's own navigation (id 0) has no gate.
	gate := func(nav NavigationHandle) (NavigationThrottleThrottleAction, bool) {
		if !nav.IsInPrimaryMainFrame || nav.TabID == 0 {
			return 0, false
		}
		if l.NavigationStarting(engine.TabID(nav.TabID), nav.URL) {
			return NavigationThrottleThrottleActionProceed, true
		}
		return NavigationThrottleThrottleActionBlockRequest, true
	}
	h.Throttle.AnswerWillStartRequest(gate)
	h.Throttle.AnswerWillRedirectRequest(gate)

	h.Observer.OnDidFinishNavigation(func(c WebContents, nav NavigationHandle) {
		if !nav.IsInPrimaryMainFrame || c.ID == 0 {
			return
		}
		tab := engine.TabID(c.ID)
		l.HistoryChanged(tab, c.CanGoBack, c.CanGoForward)
		if nav.HasCommitted && !nav.IsErrorPage {
			l.NavigationFinished(tab, nav.URL)
			// A page coming back from history already has its title and won't announce
			// it again.
			if c.Title != "" {
				l.TitleChanged(tab, c.Title)
			}
			return
		}
		if nav.NetError != 0 && nav.NetError != netErrAborted {
			l.NavigationFailed(tab, nav.URL, nav.NetErrorName)
		}
	})
	h.Observer.OnDidStartLoading(func(c WebContents) {
		if c.ID != 0 {
			l.LoadingChanged(engine.TabID(c.ID), true)
		}
	})
	h.Observer.OnDidStopLoading(func(c WebContents) {
		if c.ID != 0 {
			l.LoadingChanged(engine.TabID(c.ID), false)
		}
	})
	h.Observer.OnTitleWasSet(func(c WebContents, entry NavigationEntry) {
		if c.ID != 0 {
			l.TitleChanged(engine.TabID(c.ID), c.Title)
		}
	})
	h.Observer.OnPrimaryMainFrameRenderProcessGone(func(c WebContents, status TerminationStatus) {
		if c.ID != 0 {
			l.PageCrashed(engine.TabID(c.ID))
		}
	})

	// Reserved keys are kept from the page and the shell alike, and handed to the app.
	h.Delegate.AnswerPreHandleKeyboardEvent(func(c, source WebContents, event NativeWebKeyboardEvent) (KeyboardEventProcessingResult, bool) {
		if s := shortcutFor(event); s != 0 {
			l.Shortcut(s)
			return KeyboardEventProcessingResultHandled, true
		}
		return 0, false
	})
}

// shortcutFor is the reserved key combination a key event is, or 0.
func shortcutFor(e NativeWebKeyboardEvent) engine.Shortcut {
	if e.Type != WebInputEventTypeRawKeyDown {
		return 0
	}
	mods := e.Modifiers & WebInputEventModifiersKeyModifiers
	key := e.WindowsKeyCode
	switch {
	case mods == WebInputEventModifiersMetaKey:
		switch {
		case key == KeyboardCodeL:
			return engine.ShortcutFocusAddressBar
		case key == KeyboardCodeT:
			return engine.ShortcutNewTab
		case key == KeyboardCodeW:
			return engine.ShortcutCloseTab
		case key >= KeyboardCode1 && key <= KeyboardCode9:
			return engine.ShortcutTab1 + engine.Shortcut(key-KeyboardCode1)
		}
	case key == KeyboardCodeTab && mods == WebInputEventModifiersControlKey:
		return engine.ShortcutNextTab
	case key == KeyboardCodeTab && mods == WebInputEventModifiersControlKey|WebInputEventModifiersShiftKey:
		return engine.ShortcutPreviousTab
	}
	return 0
}
