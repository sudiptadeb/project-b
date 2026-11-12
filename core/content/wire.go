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
	h.Events.OnWindowChanged = l.WindowChanged
	h.Events.OnShellCrashed = l.ShellCrashed
	h.Events.OnWindowClosed = l.WindowClosed
	h.Events.OnWindowFocused = l.WindowFocused
	h.Events.OnSettingsMessage = l.SettingsMessage
	h.Events.OnMenuChosen = l.MenuChosen
	h.Events.OnOSBindings = l.OSBindings
	h.Events.OnModsChanged = l.ModsChanged
	h.Events.OnProcessCrashed = l.ProcessCrashed
	h.Events.OnDevTools = l.DevToolsCommand

	// The gate: every top-level navigation in a tab asks Go before it starts and on
	// each redirect. The shell's and the settings page's own navigations (ids 0 and -1) have no gate.
	gate := func(nav NavigationHandle) (NavigationThrottleThrottleAction, bool) {
		if !nav.IsInPrimaryMainFrame || nav.TabID <= 0 {
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
		if !nav.IsInPrimaryMainFrame || c.ID <= 0 {
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
		if c.ID > 0 {
			l.LoadingChanged(engine.TabID(c.ID), true)
		}
	})
	h.Observer.OnDidStopLoading(func(c WebContents) {
		if c.ID > 0 {
			l.LoadingChanged(engine.TabID(c.ID), false)
		}
	})
	h.Observer.OnLoadProgressChanged(func(c WebContents, progress float64) {
		if c.ID > 0 {
			l.LoadProgressChanged(engine.TabID(c.ID), progress)
		}
	})
	h.Observer.OnTitleWasSet(func(c WebContents, entry NavigationEntry) {
		if c.ID > 0 {
			l.TitleChanged(engine.TabID(c.ID), c.Title)
		}
	})
	h.Observer.OnWebContentsFocused(func(c WebContents, _ RenderWidgetHost) {
		if c.ID > 0 {
			l.TabFocused(engine.TabID(c.ID))
		}
	})
	h.Observer.OnPrimaryMainFrameRenderProcessGone(func(c WebContents, status TerminationStatus) {
		if c.ID > 0 {
			l.PageCrashed(engine.TabID(c.ID))
		}
	})

	// Content asks the tab's delegate to close or show it. The shell and the settings page
	// are no tabs.
	h.Delegate.OnCloseContents(func(_, source WebContents) {
		if source.ID > 0 {
			l.CloseRequested(engine.TabID(source.ID))
		}
	})
	h.Delegate.OnActivateContents(func(_, contents WebContents) {
		if contents.ID > 0 {
			l.ActivateRequested(engine.TabID(contents.ID))
		}
	})

	// Every key press in any of our pages goes to the app before the page sees it, and
	// again if the page didn't use it.
	h.Delegate.AnswerPreHandleKeyboardEvent(func(c, source WebContents, event NativeWebKeyboardEvent) (KeyboardEventProcessingResult, bool) {
		if k, ok := keyOf(event); ok && l.Key(k, engine.KeyBeforePage, surfaceOf(source)) {
			return KeyboardEventProcessingResultHandled, true
		}
		return 0, false
	})
	h.Delegate.AnswerHandleKeyboardEvent(func(c, source WebContents, event NativeWebKeyboardEvent) (bool, bool) {
		if k, ok := keyOf(event); ok && l.Key(k, engine.KeyUnhandled, surfaceOf(source)) {
			return true, true
		}
		return false, false
	})
}

// settingsID is the id the adapter gives the settings page; the shell's is 0 and tabs'
// are positive.
const settingsID = -1

func surfaceOf(c WebContents) engine.Surface {
	switch c.ID {
	case 0:
		return engine.SurfaceShell
	case settingsID:
		return engine.SurfaceSettings
	}
	return engine.SurfacePage
}

// keyOf is the key a key-down event is. Other events (key up, typed characters) are no
// key press.
func keyOf(e NativeWebKeyboardEvent) (engine.Key, bool) {
	if e.Type != WebInputEventTypeRawKeyDown && e.Type != WebInputEventTypeKeyDown {
		return engine.Key{}, false
	}
	k := engine.Key{Code: int(e.WindowsKeyCode)}
	for _, m := range []struct {
		web WebInputEventModifiers
		mod engine.Mods
	}{
		{WebInputEventModifiersControlKey, engine.ModCtrl},
		{WebInputEventModifiersAltKey, engine.ModAlt},
		{WebInputEventModifiersShiftKey, engine.ModShift},
		{WebInputEventModifiersMetaKey, engine.ModCmd},
	} {
		if e.Modifiers&m.web != 0 {
			k.Mods |= m.mod
		}
	}
	return k, true
}
