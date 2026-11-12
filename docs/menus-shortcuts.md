# Menus, shortcuts and actions

A design note, 2026-11-10, from a discussion with the owner, and built for macOS the same
day. [What was built](#built-2026-11-10) says where the build differs from the design and
what is left. Chrome's full list of menus and keys, read from its source, is in
[chrome-shortcuts.md](chrome-shortcuts.md).

## Before

What there was before this work:

- **Go reserved a fixed list of keys:** Cmd+L, Cmd+T, Cmd+W, Cmd+1 to 9, Ctrl+Tab and
  Ctrl+Shift+Tab. `shortcutFor` in `core/content/wire.go` matches them in content's
  `PreHandleKeyboardEvent`, before the page sees the key. `App.Shortcut` in
  `core/app/app.go` is a switch that calls one manager for each. `engine.Shortcut` in
  `core/engine/engine.go` is the enum between them.
- **The Mac menu bar was written by hand in Objective-C:** an app menu with Quit, and Edit
  with Undo, Cut, Copy, Paste and Select All (`InstallMainMenu` in
  `engine/browser/mac_app.mm`). There is no Redo.
- **A key the page didn't use** came back through `HandleKeyboardEvent` in
  `page_tab.cc` and `shell_page.cc`, and `Window::HandleUnhandledKey` hands it to the menu
  bar (`window_mac.mm`).
- **The shell had no shortcuts of its own.** It handled keys only inside the address bar
  and the settings panel, which it drew itself.
- **Windows and Linux had nothing.**

Two bugs from the VM tests ([vm-testing.md](vm-testing.md)) belong here:

- **Ctrl+Cmd+F does nothing.** On macOS that key is the View or Window menu's Enter Full
  Screen item, which AppKit adds to one of those menus itself. We have neither menu.
- **Shortcuts can wait behind the page.** Right after clicking a suggestion, Cmd+T or Cmd+L
  arrived up to 2.4 s late while the main thread sat idle. The likely cause, not yet
  proven, is that our keys reach Go only after joining the page's input queue. Chrome on
  Mac takes its reserved keys at the window, before the page.

## The design

One new Go manager, `commands`, owns three things: the actions, the menu tree, and the key
bindings. Every key press, menu click and shell button runs an action by its id. That
gives one path, and `engine.Shortcut` and the switch in `app.go` go away.

### Actions

- Every action has a stable id such as `tab.new`, `nav.back`, `view.fullscreen` or
  `edit.copy`. **An id never changes once it is released.** A rename is a new action
  plus a removed one.
- An action has a title, the function that runs it, whether it is enabled, and for some
  whether it is checked. Back is disabled with no history; Full Screen is checked in full
  screen.
- Managers don't know about `commands`. `app.go` registers each action with the manager
  that does the work, as it wires hooks today.
- Actions the shell performs, such as focusing the address bar or opening settings, are
  registered the same way. Running one sends the shell a message.

### Two files, two template folders

The menu tree and the key bindings each have one current file in the profile and one
template folder in the app:

| | Current file | Templates |
|---|---|---|
| Key bindings | `keymap.json` | `templates/keymap/`: `project-b.json`, `chrome.json`, `firefox.json`, `safari.json`, `edge.json` |
| Menu tree | `menu.json` | `templates/menu/`: `project-b.json` |

- **The default template for both is `project-b`.**
- **The current file and the templates have the same shape.** Nothing is a diff and
  nothing is laid over anything else, so copying `chrome.json` over `keymap.json` by hand
  is the same as picking Chrome in the panel.
- **The two files don't depend on each other.** The tree only names action ids; the keymap
  only binds them.
- Templates ship inside the app, and an update replaces them.

### Following and owned

A current file is in one of two states.

- **Following.** Until the first edit, the file only names its template:

  ```json
  { "follows": "project-b" }
  ```

  An update replaces the template, so a following file gets the new version by itself.
- **Owned.** The first edit copies the whole template into the file with that edit
  applied. From then on the file is the user's. A macOS remap counts as an edit too.
- **Reset** puts the file back to following: `{ "follows": "chrome" }`. Picking a template
  in the panel is a reset to it, so switching templates starts clean.

### The keymap file

```json
{
  "template": "chrome",
  "actions": {
    "tab.new": {
      "mac": [
        { "keys": "Cmd+T", "reserve": "always" },
        { "keys": "Cmd+Option+N", "reserve": "always", "fromOS": true }
      ],
      "windows": [{ "keys": "Ctrl+T", "reserve": "always" }],
      "linux":   [{ "keys": "Ctrl+T", "reserve": "always" }]
    },
    "tab.select1": {
      "hidden": true,
      "mac": [{ "keys": "Cmd+1", "reserve": "windowed" }]
    }
  }
}
```

- **`template`:** in an owned file, the template it was copied from. Updates compare
  against it, and resetting one action copies that action back from it.
- **`actions`:** a flat list keyed by action id.
- **`hidden`:** the action has no item in the menu, but its keys still work. Left out
  means shown.
- **`mac`, `windows`, `linux`:** each is a list of bindings. An action can have several
  keys on one platform; Chrome's next tab has five on Mac. A platform left out means no
  keys there.
- **`keys`:** one combination. Modifiers are spelled out and always in the same order:
  Ctrl, Option (Alt on Windows and Linux), Shift, Cmd.
- **`reserve`:** who wins the key, below.
- **`fromOS`:** written only when true. The binding came from macOS App Shortcuts, below.

The panel edits only the current platform's lists and leaves the others as they are, so
one file works on a Mac and on Windows.

### Who wins a key

Each binding has one of three settings, and the user chooses it:

| Value | Panel label | Meaning |
|---|---|---|
| `always` | Always | The browser wins, in full screen too, and even if the page asked to capture keys. |
| `windowed` | Outside full screen | The browser wins in a normal window; in full screen the page gets the key. Chrome's rule, and what a remote desktop page needs. |
| `page` | Page first | The page sees the key first; the action runs only if the page didn't use it. |

So no page can take a key the user reserved. Chrome reserves twelve actions outside full
screen (new tab, new window, close tab or window, reopen closed tab, the four tab
switches, quit) and in full screen only Quit and Full Screen. The `project-b` and `chrome`
templates start from that.

### The menu file

The tree belongs to the template. Users can't change it for now; they change only an
action's keys and whether it is hidden.

```json
{
  "template": "project-b",
  "menus": [
    {
      "title": "File",
      "items": [
        { "action": "tab.new" },
        { "action": "window.new" },
        { "separator": true },
        { "action": "tab.close" }
      ]
    },
    {
      "title": "Tab",
      "items": [
        { "title": "Select Tab", "items": [{ "action": "tab.select1" }] }
      ]
    }
  ]
}
```

- A submenu is a title and its items. A submenu shows when any item in it shows.
- An action item's title comes from the action, not from the file, so it stays the same
  for macOS remaps and for Voice Control, which both match by title.
- Every action has a place in the tree, including keys-only ones like Tab 1 to 8, so the
  user can show any of them.
- On Mac the engine builds the menu bar from the tree. On Windows and Linux the shell draws
  the same tree as one menu, with the top-level menus as its submenus.
- A menu editor (moving items, user separators and submenus) is for later. When it comes,
  `menu.json` becomes editable with the same following and owned rule, and more tree
  templates can ship.

### Updates

- **A following file** gets the new template with the update. Nothing else happens.
- **An owned keymap** is compared with the new version of its template. It's a flat list,
  so it merges: an action that is new in the template is added with its default keys,
  unless one of its keys is already bound in the file. Those keys are left off and the
  action goes in the notice.
- **An owned menu file** merges only its locked parts. A locked menu that is new in the
  template is added at the template's place; a locked item that is new, or missing after
  a hand edit, is added to its locked menu at the template's place. Everything else the
  template has and the file doesn't goes in the notice.
- **The notice** shows at startup and stays in settings, listing what is missing or
  clashed, with "add" for each, "add all" and "reset".
- Nothing else in an owned file changes. A default key that changed in the template doesn't
  reach an owned keymap. An action the app removed stays in the file, does nothing, and is
  shown as no longer available. The same rule covers a file from a newer version of the
  app.

### Locked keys and items

Some keys and menu items exist so the user can always get back. They are not the user's
to change.

- **Locked keys come from their own file,** `templates/locked.json`, shipped in the app,
  in the keymap's shape. They always apply, whatever `keymap.json` says, and they win a
  clash with a user binding. The panel shows them with a lock and no edit controls.
- **Proposed locked keys:**
  - Quit, Cmd+Q on Mac.
  - Settings, Cmd+, on Mac, so reset is always one key away.
  - F6 and Shift+F6, between the shell and the page, so the keyboard can't get stuck in a
    page.
  - Holding Escape to leave full screen, which is a behaviour rather than a binding, but
    belongs on the same list.
- **The tree marks locked menus and items** with `"locked": true` in the tree template.
  A locked menu can't be removed or renamed: the app menu, Edit (macOS adds its own items
  there, and copy and paste on Mac only work through it), Window (macOS adds the window
  list) and Help. A locked item can't be hidden or deleted: Quit and Settings, both in the
  app menu.
- **A locked item lives only inside a locked menu,** all the way up. A new locked item
  goes into an existing locked menu, or into a new locked menu made for it. When the menu
  editor comes, a locked item can be moved, but only between locked menus. Users may put
  their own items inside a locked menu.
- **So locked parts always have a known place,** even in an owned tree, and updates can
  merge them (below).

### A bad file

A user's change may stop things from working, and that's theirs to choose, as long as it
can't break the app and a reset always fixes it.

- A current file that doesn't parse is ignored: the app follows its template, and the
  notice says so and offers reset. The bad file is kept beside it, not deleted.
- An unknown action id, an unknown key name or an unknown `reserve` value is skipped on its
  own; the rest of the file still applies.
- Locked keys and locked items mean Settings, and with it reset, is always reachable by
  key and by menu.

### macOS App Shortcuts

macOS lets people rebind any app's menu items in System Settings, Keyboard, Keyboard
Shortcuts, App Shortcuts. A test app on this Mac (2026-11-10) showed how it works:

- macOS knows only our menu titles. It stores title-to-key pairs in our app's own defaults
  under `NSUserKeyEquivalents`.
- It applies them when a menu item is created at launch. An item made with Cmd+T came out
  as Cmd+Option+N, and setting our own key on it afterwards had no effect: macOS keeps its
  own.
- A change while the app runs doesn't touch existing items, nor new ones, until a relaunch.
- Watching our own defaults with key-value observing tells us straight away when another
  process changes them.

So Go reads the remaps at launch and on each change, matches each title to an action, and
writes it into `keymap.json` as a binding with `fromOS: true`. It's just another binding
in the list, next to the user's own, and both work. Go owns these bindings: it rewrites
them from macOS, so editing one by hand does nothing. The panel shows them read-only with
a pointer to System Settings, but the user can choose their `reserve`. A remap makes the
keymap owned, like any edit. A menu item shows one key, and on Mac that is the macOS one.

Not tested yet: remaps set for All Applications, which macOS stores elsewhere, and whether
a hidden menu item's key still fires. If it does, every action can be a real menu item,
hidden or not, and macOS remaps work for all of them.

### How a key is handled

- The engine hands Go every key press before the page sees it, and Go is the one place that
  decides. A binding set to `always`, or to `windowed` outside full screen, runs at once
  and the page never sees the key.
- Anything else goes to the page. If the page doesn't use it, the engine hands it back and
  Go runs the action if it is bound.
- On Mac the engine is to catch keys at the window, before they join the page's input
  queue, which is expected to fix the late keys after clicking a suggestion. Not built
  yet; see below.
- The native menu only shows keys; Go does the matching. A menu click runs the action by
  id.
- A page's request to capture keys (the Keyboard Lock API, content's `RequestKeyboardLock`)
  is to be answered from Go, and never get an `always` key. Today no page can lock the
  keyboard: our delegate doesn't grant it.
- Text fields keep their editing keys. The shell never matches shortcuts itself.

### The settings page

Settings is an app of its own at `chrome://settings/`, in a WebContents of its own that
the gear (or Settings…, Cmd+,) puts over the whole window and takes away again, so the
shell never carries it. Each of our pages is a separate app, built the way pweb builds its
pages: `shell/src/apps/<app>/` has the app's `index.html` and code, what apps share sits in
`shell/src/` (imported as `@/`), and `VITE_APP=<app> vite build` builds one app into
`dist/<app>/` as `index.html`, `<app>.js` and `<app>.css`; `npm run build` builds them all.
The app's own build doesn't use `dist`: ninja runs `engine/build_shell.py`, which builds every
app into the build folder's `shell/<app>/` whenever a file the shell reads has changed.
The engine serves each app at `chrome://<app>/` from its own folder, and gives the bridge
to Go only to the WebContents the app belongs in. It has two sections, General (what the shell's panel had: tab
layout, frame colour, updates) and Keyboard:

- Every action, grouped by menu, as one plain row: its name and its keys. Nothing on the
  row is a control; clicking it opens the action's editor under it (each key's priority and
  Remove, Add key, In menu, Reset).
- **Priority shows only where it differs from Page first** (owner's pick, 2026-11-10, "for
  now"; he isn't convinced yet): a tag after the keys, "Browser" or "Browser · not full
  screen", or "Locked · Browser". The other option mocked was a separate "Browser keeps
  these keys" list at the top, as Vivaldi has.
- Click an action, press the new keys. Add a second binding, remove one, choose its
  `reserve`.
- A template picker (a reset), reset one action, reset all.
- A conflict warning when two actions want the same keys.
- macOS bindings shown read-only, locked ones with a lock. Admin-locked bindings, later,
  the same way.
- The update notice, and any file that couldn't be read.

### Keys we can't fully control

- **Copy and paste differ by platform.** On Mac the page engine ignores Cmd+C, V, X, A and
  Z on purpose (`third_party/blink/renderer/core/editing/editing_behavior.cc`, lines 171
  to 182); they work only through our Edit menu, so they are fully ours to rebind. On
  Windows and Linux the page engine handles Ctrl+C and the rest itself as text editing.
  A second key for copy works there, through content's `WebContents::Copy`. Taking Ctrl+C
  away, or giving it to another action, works only with `always`, and then the page never
  sees that key. With `page`, the engine is expected to use the key for editing and never
  hand it back, so the action wouldn't run inside text; not tested yet. The panel marks
  these keys on Windows and Linux.
- **Keys the OS keeps never reach us:** Cmd+Tab, Cmd+Space, the screenshot keys, Mission
  Control and Cmd+` on Mac; Alt+Tab, Ctrl+Alt+Delete and the Windows key on Windows; the
  desktop's own keys on Linux. The panel warns that a binding on one won't work.
- **Accessibility tools take keys first:** VoiceOver's Control+Option, and screen readers'
  own keys on Windows.
- **While an input method is composing,** for Chinese or Japanese, keys go to it first, so
  shortcuts don't fire mid-word.
- **Text-editing keys inside fields on Mac,** such as Ctrl+A for the start of a line, come
  from macOS's text system. They stay out of our keymap.

### Admin policy, later

Stable ids make it easy for an admin to turn an action off or lock its keys. Edge has the
first half, only on Windows (`ConfigureKeyboardShortcuts`); Chrome and Firefox have
neither. Policy sits on top of the current file and is never written into it.

## The `project-b` menu tree

The default tree is `core/commands/templates/menu/project-b.json`. The keymap templates
`project-b.json` and `chrome.json` are the same for now. The tree has only actions that
work today; Chrome actions for features we don't have are added with their feature
(private windows, split view, tab groups, printing, bookmarks, the history and downloads
pages, profiles, extensions, spelling, the task manager, Cast, Translate, the Google
services). Keys are Chrome's Mac keys; `*` marks the ones reserved outside full screen.

- **Project B** (locked): About Project B; Settings…, Cmd+, (locked); Check for Updates…;
  Services; Hide Project B, Cmd+H; Hide Others, Option+Cmd+H; Show All; Quit Project B,
  Cmd+Q (locked).
- **File**: New Tab, Cmd+T*; Open Location…, Cmd+L; Close Window, Shift+Cmd+W*; Close
  Tab, Cmd+W*.
- **Edit** (locked): Undo, Cmd+Z; Redo, Shift+Cmd+Z; Cut, Copy, Paste; Paste and Match
  Style, Shift+Cmd+V and Option+Shift+Cmd+V; Delete; Select All, Cmd+A.
- **View**: Tabs on Side, Tabs on Top (checked); Stop, Cmd+.; Reload This Page, Cmd+R;
  Force Reload This Page, Shift+Cmd+R; Full Screen, Ctrl+Cmd+F, `always` (checked); Actual
  Size, Cmd+0; Zoom In, Cmd+= and Shift+Cmd+=; Zoom Out, Cmd+-; Developer: View Source,
  Option+Cmd+U; Keyboard (hidden): Focus Next Pane, F6 (locked) and Option+Cmd+Down;
  Focus Previous Pane, Shift+F6 (locked) and Option+Cmd+Up.
- **History**: Back, Cmd+[ and Cmd+Left; Forward, Cmd+] and Cmd+Right.
- **Tab**: Select Next Tab, Shift+Cmd+], Option+Cmd+Right and Ctrl+PageDown*; Select
  Previous Tab, the mirror keys*; Select Recent Tab, Ctrl+Tab*; Select Recent Tab
  Backward, Ctrl+Shift+Tab*; Duplicate Tab; Select Tab (hidden): Tab 1 to 8,
  Cmd+1 to 8, and Last Tab, Cmd+9.
- **Window** (locked): Minimize, Cmd+M; Zoom; Bring All to Front; macOS adds the window
  list.

Later, each with its feature: New Window, Reopen Closed Tab, Open File, Save Page, Email
Link, the find bar, Pin and Mute Tab, Search Tabs, Move Tab, Developer Tools, Home, Show
Full History, a Help menu.

Counted from Chrome's source, Chrome has 194 actions: 58 need only Go and the shell, 49 a
content call, 13 come from the OS, and 74 open features we don't have.

## From the guidelines, not yet discussed

A research pass over Apple's, Microsoft's and GNOME's guidelines, WCAG, and how VS Code,
JetBrains, ChromeOS, Firefox and Vivaldi do it raised these:

1. **Keyboard layouts.** Cmd+[ is awkward on German, and Chrome has special cases for
   Hebrew and Dvorak. A binding may need to say whether it means a character or a physical
   key, as VS Code does; Cmd+1 to 9 always mean the physical number keys. macOS 12 and
   later also moves shortcuts that can't be typed on the current layout, which an app with
   its own shortcut editor may turn off (`allowsAutomaticKeyEquivalentLocalization`).
2. **Single-key shortcuts** must be possible to turn off, and fire only outside text fields
   (WCAG 2.1.4).
3. **Panel warnings** when a binding takes a standard system shortcut, uses Option+letter
   on Mac (which breaks accented characters), or a key the OS keeps for itself.
4. **Grey out, don't hide,** an item that can't be used now. Apple's rule.
5. **Windows access keys,** the underlined Alt letters, translated per language.
6. **Per-site priority,** as in Vivaldi and Arc: let one site win the keys. Later.

## What touches content

Nothing changes in Chromium itself. The `commands` manager, both files, the templates,
updates, conflicts and the panel are Go and shell. The engine adapter gets mechanics only:
building the Mac menu bar from the tree, catching keys at the window and handing them to
Go, answering keyboard lock, and watching macOS's remaps. Each "wire" item above is one
engine call into content (`WebContents::Copy`, `Find`, `Stop`, `SavePage`, `HostZoomMap`
and so on).

## Theme (2026-11-10)

Settings has a Theme row: System, Light or Dark. Go's prefs keep it (`theme` in
`prefs.json`). The shell and the settings page take their tones from the frame colour in
either theme (`shell/src/tones.js`, applied by `shell/src/theme.js`); the frame keeps the
colour the user picked. The window's own buttons and title bar follow it
(`NSWindow.appearance`). Web pages keep following the system, since NSApp's appearance,
which content reads, is left alone.

## Decided (2026-11-10)

1. **One Go `commands` manager** holds the actions, the menu tree and the keys. Every key,
   menu item and shell button runs an action by its id.
2. **Two files with one shape each:** `keymap.json` and `menu.json`, each following a
   template until the first edit, then owned. Same shape for the file and its templates,
   so a template file copied over the current one works. A separate `keymap.json` so it
   can be imported and exported later, once there is a file manager.
3. **The default template is `project-b`.** For now `chrome` is the same.
4. **Picking a template resets** to it. Reset always fixes whatever a user's change did.
5. **Updates:** a following file gets the new template. An owned keymap is a flat list, so
   new actions merge in, without keys the user already uses; those go in a notice at
   startup and in settings. An owned menu tree only gets the notice.
6. **The user chooses who wins each key:** Always, Outside full screen, Page first. No page
   can take a key the user reserved.
7. **Several keys per action,** and macOS App Shortcuts are just more bindings in the same
   list, marked `fromOS`, written by Go. Both apply.
8. **Every action has a place in the tree;** `hidden` is on actions only, and a submenu
   shows when any of its items does. The user can show keys-only actions such as Tab 1
   in the menu, which also helps accessibility tools.
9. **Locked keys** come from their own file; **locked menu items** (Quit, Settings) live
   only inside locked menus, so updates can always place them.
10. **Users can't change the tree for now.** Moving items, user separators and submenus
    come with a menu editor, later.
11. **Settings is a self-contained page in a WebContents of its own,** placed over the
    whole window by the gear, so it can't bloat the shell. It is `chrome://settings/`:
    each page is a different app, built the way pweb builds its pages.
12. **Ctrl+Tab and Ctrl+Shift+Tab cycle through recent tabs,** as Cmd+Tab and
    Cmd+Shift+Tab cycle through apps on a Mac. Next and previous by position keep
    Chrome's other keys.

## Built (2026-11-10)

Mac only, on branch `menus-shortcuts`.

- **Go:** `core/commands` (the manager, the templates in `templates/`, the keymap and
  menu files, the merge, macOS bindings, the view the settings page shows) and
  `core/settings` (the settings page's contract, as `core/shell` is the shell's).
  `core/app/actions.go` registers every action with the one manager call it makes. The
  engine contract lost `Shortcut` and gained `Key`, `MenuChosen`, `OSBindings` and
  `SettingsMessage` on the listener, and the content calls each action needs.
- **Engine:** the menu bar is built from Go's menu (`SetMainMenu` in `mac_app.mm`), each
  item telling Go its action. Keys reach Go twice through content's own hooks:
  `PreHandleKeyboardEvent` before the page and `HandleKeyboardEvent` after it, for the
  shell, the tabs and the settings page alike; a key Go doesn't use goes to the menu bar.
  macOS App Shortcuts are read at start and on every change (key-value observing on our
  defaults, All Applications included) and sent to Go. `settings_page.cc` is the settings
  page; `Window::SetOverlayView` puts it over everything and gives it every press.
- **Shell:** `shell/src/apps/settings/` is the settings app and `shell/src/apps/shell/`
  the shell, each built on its own (`npm run build` builds both). The shell's panel is gone; the gear runs
  `app.settings`, and the sidebar's New Tab hint shows whatever key New Tab has now.

Where the build differs from the design, or decided something the design left open:

- **Keys aren't caught at the window yet.** They go through content's hooks as before, so
  the late keys after clicking a suggestion ([vm-testing.md](vm-testing.md), bug 5) may
  still happen; catching them at the window is its own change, to test in the VM.
- **`menu.json` can only follow a template** until the menu editor exists.
- **The update notice lasts for the run;** an action added from it is in the file, so it
  isn't shown again.
- **Closing a tab while settings is open closes settings,** as closing a settings tab
  would in another browser. While settings is open it keeps the keyboard: a focus request
  from the shell or Go waits until it closes.
- **F6 moves between the shell and the page,** whichever had the keyboard last.
- **Recent tabs, held like the app switcher.** The tabs manager keeps every tab in the
  order it was last shown. Each Ctrl+Tab goes one further back, past the oldest round to
  the newest, showing each tab as it goes; Ctrl+Shift+Tab goes the other way. The order
  stays as it was until Ctrl is let go, and the tab it stopped on becomes the most recent.
  Shift only turns the direction, so letting go of Shift doesn't end it, and any other
  action does. An action holds like this when it has `Release` (`core/commands`); run from
  the menu, where nothing is held, it settles at once. Content drops a modifier's release
  after a key the browser handled, so the app reports modifier changes itself
  (`PBApplication sendEvent:`, `ModsChanged`). There is no switcher overlay.
- **Full Screen keeps one title,** checked while in full screen, so a macOS remap of it
  keeps working.
- **No Help menu yet,** as there's nothing in it.
- **Holding Escape to leave a page's full screen** isn't built: pages can't go full screen
  yet.

Tests: `core/commands` (keys, who wins a key, the first edit, the update merge, a bad
file, macOS bindings), `core/app` (the menu from the tree, a key added in settings, the
settings page's prefs), `shell/e2e/settings.cjs` (26 checks on the mock) and
`tools/drive_first_app.py settings` (the real app: the menu bar, Cmd+, opening settings
over the window, recording a key, Cmd+W closing settings, Ctrl+Tab through recent tabs,
and the key surviving a restart). Copy and paste through Go and Ctrl+Cmd+F need a person's
desktop session, so they are in the `screen` group: `tools/check_clipboard.py` and
`tools/check_full_screen.py`. The
real-app check lets go of Ctrl by calling what `sendEvent:` calls, since the driver
doesn't go through AppKit's dispatch.
