# Several windows, and the tabs each one shows

A design note, 2026-11-13, for the second step of [profiles.md](profiles.md), with the
owner's decisions. Built for macOS; Android keeps its one window. The survey of the code
behind it is `project-b-research/windows-survey.md`.

## Today

- The engine has one window, one shell page and one settings page, and its close is the
  app's quit.
- Go says which tab shows with `ShowTab`, and the window's layout holds one page rectangle.
  The engine puts that tab's view there and hides every other tab.
- The shell is "page 0" to the hooks, so a key or a message can't say which window it came
  from.
- Go's tabs have one list and one active tab. Commands, menus and the extension API read it.

## What changes

**A window has an id.** Go makes the ids, 1 for the first. Every call about a window and
every event from one carries it: the layout, a window request, a message to or from the
shell, settings, a window's change of size.

**A window is a shell page, a settings page over it when open, and the tabs it shows.** Go
keeps the windows, and in each an ordered list of tabs. A tab keeps its id for its life,
whichever window it is in.

**The layout lists the visible tabs.** The shell reports, with its regions, a list of
rectangles, each with a tab. That list takes the place of the one page rectangle and of
`ShowTab`. The engine places each listed tab's view at its rectangle and tells a tab it is
shown or hidden as it enters or leaves the list. The shells we have list one tab.

**Focus stays its own call.** `FocusPage` becomes `FocusTab(tab)` and `FocusShell` takes a
window. When a click moves the keyboard into a visible page, the engine says which tab.

**Go's one active tab becomes three things:** each window's visible tabs, each window's
focused tab, and the window in front. A command acts on the focused tab of the window in
front.

## Behaviour, as decided

- A new window (Cmd+N) starts with one new tab, and opens a little down and right of the
  window in front.
- Closing a window's last tab closes the window.
- Closing a profile's last window quits its process.
- Quit closes every window at once, with no question.
- A link from another app, or from a second start of the app, opens as a new tab in the
  window used last.
- Settings is per window, over the window whose button opened it.
- A window a page opens (`window.open`) stays a tab, as today.
- Ctrl+Tab stays within a window.
- A tab in a minimised window is told it is hidden.
- A shell's own data, when shells have any, is kept per window.

## Order of building

Each commit builds and leaves a working app.

1. A window id on every window call and event, with one window.
2. The list of visible tabs in the layout, with one entry; `ShowTab` goes.
3. A second window: making one, closing one, the window in front, Cmd+N, commands and
   menus following it, quit apart from close, the handover into the window used last.
4. What macOS expects of an app with several windows: the Window menu's list and Cmd+`.
5. Moving a tab to another window.
6. Extensions' `chrome.windows` and DevTools' window calls with real windows.
