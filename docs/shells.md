# Shells: choosing one, building one, testing one

A design note, 2026-11-13, for the fifth step of [profiles.md](profiles.md), with the
owner's decisions. A shell that passes its tests on the mock engine is expected to work in
the real app; "What the real app adds" lists the little the mock can't show.

## Decided

- The shell is only the representation of Go's data and the surface the user acts on.
- There are several shells to choose from. The choice is per profile and switches live.
- A shell may keep data only it understands, such as how it splits a window into panes.
  It is kept per window. Changing shell loses it, and the next shell starts from Go's
  data alone.
- A concept moves into Go only when every shell is expected to have it.
- Building a shell is friendly to its developer: it is written and tested on the mock
  engine, with no Chromium build.

## Where a shell lives

- **A shell is one folder,** `shell/src/apps/shell/<id>/`, with an `index.html` and a
  `main.js`, and **one line in `core/shells/registry.json`:** its id and the name the user
  sees, and `"example": true` for a shell that is built but offered nowhere. The id is
  lower-case letters, digits and dashes.
- Each shell builds into `shell/<id>/` beside the other pages, and loads at
  `chrome://shell/<id>/` in the real app and at `/shell/<id>/` on the mock. Nothing in the
  engine names a shell: it serves `chrome://shell/<id>/` from that folder.
- Every shell speaks on the one channel, `shell`. Go doesn't know which shell is on the
  other end.
- The shells: `standard`, the one there has always been; `terminal`, panes that split, as
  a terminal's do; `fox`, laid out as Firefox is.

## Choosing a shell

- **Which shell a profile loads is a pref,** `shell`, whose value is a shell's id. A
  template can set it, and a reset to a template sets it too.
- The user chooses in View, Shell (a list read from the registry, with a tick on the one
  in use) and in the Profile section of settings.
- **Switching is a reload.** When the pref changes, every window's shell page loads the
  other shell. It says `ready` and gets the snapshot, as a shell does after a crash. The
  tabs keep running: their views stay where they were until the new shell reports its
  layout. Go drops what the old shell claimed of the window, and its data.

## The kit

`shell/src/kit/` is what every shell is built from. A shell uses it and keeps no copy of
its own.

| From | What |
|---|---|
| `@/kit/shell.js` | `connect(handlers)`: says `ready`, keeps `state` as Go sends it, reports the layout and says `shown` once the snapshot is drawn. `state`: the reactive copy of everything Go sends. `send(msg)`. `activeTab`. `saveData(value)`. `focusPage()`. `tabLabel(tab)`, `workspaceOf(tab)`, `controlSpace`. |
| `@/kit/regions.js` | The directives a shell marks itself with. `v-shell`: this element is the shell's, so presses on it reach the shell. `v-drag`: this element moves the window wherever it has no control: an empty space, or a whole bar of controls, whose gaps then drag; its controls, and anything in it marked `v-shell`, stay the shell's. `v-page="tab.id"`: this tab's page shows here; an element with no tab shows none. |
| `@/kit/controls.js` | `controls`: the one selector for what a control is, something the user presses or types in (buttons, links with an address, fields, selects, text areas, `role="tab"` and `role="button"`, anything with a `tabindex`). The kit finds a drag bar's controls by it, and the check reads it for rule 6. |
| `@/theme.js`, `@/tones.js`, `@/design.css` | The colours, made from the frame colour, and the shared controls ([design-system.md](design-system.md)). |
| `@extensions` | `ExtensionsToolbar`: the extensions' buttons, their menu and the install prompt. |

The kit measures the marked elements and sends the layout by itself, once a frame and only
when it changed. It watches each `v-drag` element's subtree, so a control that comes or goes
in a bar, such as a tab added to a strip, is measured with nothing from the shell. A shell
never sends `layout`.

Go's window manager orders the regions, and the first that holds a point decides: the
controls in the drag bars (the shell's), then the drag bars, then the rest of the shell's,
and the pages under everything. In full screen nothing drags, so a bar that is also `v-shell`
is the shell's there.

## The messages

One JSON object each way, with its kind in `t`.

**Go to the shell**

| `t` | Carries | When |
|---|---|---|
| `snapshot` | Everything below at once, and `data` | In answer to `ready` |
| `tabs` | `tabs`, `active`: this window's tabs in order, and the active one's id | Tabs open, close, move, or another becomes active |
| `tab` | `tab`: one tab's new state | Its title, address, loading, favicon or history changed |
| `theme` | `theme`: `system`, `light` or `dark` | The pref changed |
| `windowColor` | `windowColor`: `#rrggbb` | The frame colour changed |
| `controls` | `controls`, `cornerRadius` | The native window buttons moved, or full screen |
| `keys` | `keys`: the key each action shows, by action id | The keymap changed |
| `workspaces` | `workspaces`: id, name and colour of each | The list changed |
| `update` | `update`: what the updater is doing | It changed |
| `crashLog` | `crashLog`, `crashReport` | The notice of the last run's crash was closed |
| `tabLayout` | `tabLayout` | The standard shell's tab position changed |
| `focusAddressBar` | | The user asked for the address bar |
| `suggestions` | `id`, `items` | In answer to `suggest` |
| `paint` | `span` | Go wants a timeline span ended once this is drawn |

A tab is `{id, url, title, loading, progress, blocked, crashed, error, favicon, canGoBack,
canGoForward, workspace}`. The snapshot also has `frameWidth`, `running` (the version) and
`platform`.

**The shell to Go**

| `t` | Carries | Does |
|---|---|---|
| `ready` | | Asks for the snapshot. Nothing reaches a shell before it |
| `shown` | | The snapshot is drawn and the layout sent |
| `layout` | `shell`, `controls`, `drag`, `tabs`, `viewW`, `viewH` | The regions: the shell's, the controls inside the drag bars, and the drag bars; and the tabs shown with their rectangles. The kit sends it |
| `newTab` | | Opens a tab in this window and makes it active |
| `closeTab` | `id` | Closes a tab |
| `activateTab` | `id` | Makes a tab the active one |
| `navigate` | `text` | Takes the active tab to an address or a search |
| `back`, `forward`, `reload` | | Act on the active tab |
| `focusTab` | `id` | Gives a tab's page the keyboard |
| `focusShell` | | Gives the shell the keyboard |
| `suggest` | `id`, `text` | Asks for address suggestions |
| `run` | `text`: an action's id | Runs an action, as its menu item would |
| `window` | `text`: `close`, `minimise`, `zoom` or `fullScreen` | Asks the window |
| `data` | `data`: any JSON | Replaces this window's copy of the shell's own data |
| `relaunch`, `dismissCrash` | | The update bubble's and the crash notice's buttons |
| `timeline` | | The page's timeline entries. The kit sends them |

The active tab is the one the address bar, back, forward and reload act on. A shell that
shows several tabs makes the one the user works in active: it sends `activateTab` when the
user picks one in the shell, and Go makes a tab active by itself when a press in its page
gives it the keyboard.

## A shell's own data

`saveData(value)` sends the whole value; Go keeps the latest for the window and never
reads it. It comes back as `state.data` in the snapshot, so a shell that was reloaded lays
itself out as it was. A new window's shell gets none. It is dropped when the shell
changes, the profile is reset to a template, or the window closes. It is in memory for
now, and will be saved with the windows and tabs when sessions are.

A shell's data names tabs by id. A tab in the data that Go no longer lists is gone, and a
tab Go lists that the data doesn't have is new: Go's list is the truth.

## What every shell does

The check holds every shell to these, knowing nothing else about it.

1. It says `ready`, draws the snapshot, and says `shown`.
2. **The active tab's page shows,** after every change: a new tab, another made active, a
   close, a tab a page opened, a resize. It asks for a page's keyboard (`focusTab`) only once
   the layout it sent shows that page: the window can't give the keyboard to a page it
   hides, and no key reaches Go until something takes it.
3. The pages it shows lie inside the window's frame, don't overlap, and are all tabs of
   this window.
4. **Each tab of the window is an element with `role="tab"` and `data-tab="<id>"`,** the
   active one with `aria-selected="true"`. A click on it makes that tab active.
5. **`focusAddressBar` puts the keyboard in a text field** where an address and Enter take
   the active tab there.
6. **Every control it draws is the shell's,** a control as `@/kit/controls.js` says: it
   lies in a region it marked `v-shell`, or in a `v-drag` bar, so a press reaches it and not
   the page beneath, nor the window's drag.
7. A press in the middle of a page it shows reaches that page.
8. Some space is marked `v-drag`, and no page lies under the native window buttons.
9. Reloaded, it comes back showing the same pages: what it needs to do so it saved.
10. Switched to and away from, it starts from Go's data alone.
11. It logs no errors.

Every shell also mounts `ExtensionsToolbar`: without it the extensions' buttons and the
install prompt have nowhere to show.

## Building one

1. Make `shell/src/apps/shell/<id>/` with `index.html` and `main.js`, and add the line to
   `core/shells/registry.json`.
2. In `main.js`: import `@/design.css` and mount the app with the three directives. The
   app calls `connect()` as it is made.
3. Draw `state.tabs`; mark what is the shell's with `v-shell`, the top bar with `v-drag`
   (its gaps move the window, its controls stay the shell's), and each place a page shows
   with `v-page`.
4. Run it on the mock (below) and make the check pass.

The smallest shell is the example, `shell/src/apps/shell/example/`: the one to copy. It is
built with the others and offered nowhere (`"example": true` in the registry); `go run
./cmd/dev -use example` loads it. Beside an `index.html` and a `main.js` of a few lines,
its `App.vue` is the whole of it:

```vue
<script setup>
import { ref } from 'vue';
import { connect, controlSpace, send, state, tabLabel } from '@/kit/shell.js';
import { ExtensionsToolbar } from '@extensions';
import { useTheme } from '@/theme.js';

useTheme(state);
const address = ref(null);
connect({ focusAddressBar: () => address.value.focus() });
</script>

<template>
  <header v-shell v-drag :style="{ paddingLeft: controlSpace.left }">
    <button
      v-for="tab in state.tabs" :key="tab.id"
      role="tab" :data-tab="tab.id" :aria-selected="tab.id === state.active"
      @click="send({ t: 'activateTab', id: tab.id })"
    >{{ tabLabel(tab) }}</button>
    <button aria-label="New Tab" @click="send({ t: 'newTab' })">+</button>
    <input ref="address" aria-label="Address"
      @keydown.enter="send({ t: 'navigate', text: $event.target.value })">
    <component :is="ExtensionsToolbar" v-if="ExtensionsToolbar" />
  </header>
  <main v-page="state.active" :style="{ margin: `0 ${state.frameWidth}px ${state.frameWidth}px` }"></main>
</template>

<style>
#app { display: flex; flex-direction: column; }
header { display: flex; align-items: center; gap: 4px; height: 44px; padding-right: 8px; background: var(--bar); color: var(--text); }
input { flex: 1; }
main { flex: 1; }
</style>
```

## Testing one

**While writing it.** `go run ./cmd/dev -use <id>` in `core/` and `npm run dev` in
`shell/`, then open `/mock/window` on vite's address. The page is the OS window: the shell
in a frame, and under it a mock page for each tab the layout shows, where the layout puts
it. A mock page has buttons that do what a real page can: navigate, change its title, open
a tab, crash. Edits to the shell reload in place.

**Seeing its regions.** Turn on Show window regions in settings, Flags, and start
`go run ./cmd/dev` again: the mock window then outlines every rectangle the window manager
tracks, as the real window does ([windows.md](windows.md#seeing-the-regions)): the shell's
areas red, the controls in a drag bar among them, the drag areas blue and dashed, each
shown tab's page green with its id, the window buttons purple. A gap that should drag and
doesn't, or a control that isn't outlined red, shows at a glance. As in the real app the
flag is read at launch, from the profile's `flags.json`.

**The check.** `node shell/e2e/shell-contract.cjs <id>` runs "What every shell does"
against one shell, in a real browser on the mock engine; with no id it runs every shell in
the registry. It ends with one line per rule, `ok` or `FAIL`.

**The shell's own tests.** `shell/e2e/shells/<id>.cjs`, for what only this shell does: its
panes, its menus, its keys. `shell/e2e/lib.cjs` starts the mock on a port of its own and a
profile of its own, so tests run side by side, and has the helpers the other tests use:
`check`, `until`, `settle`, `regions`, `fire`, and the window with its frames
(`openWindow`). A test starts the mock with `-use=<id>`, and ends with `done()`.

`tools/test.sh mock` runs the check and every shell's own tests with the rest.

**What the mock tells a test.** `/mock/regions?window=<n>` answers with the last layout:
`regions` (the shell's, a drag bar's controls among them) and `drag`, `visible` (each shown tab and its rectangle), `shell` (the id of the
shell the window loads), `focused` (the tab last given the keyboard), `unshownFocus` (the
tabs a shell asked to give the keyboard while no window showed them), Go's `tabs` and
`active`, and what it had before; with `x` and `y`, `hit` says what a press there does.
`/mock/fire` does what a page or the user can: `kind=menu` with an action's id runs it,
`kind=focusTab` is a press in a tab's page, and each button of the mock page is a kind,
acting on the tab `tab` names. `go run ./cmd/dev -addr=127.0.0.1:0` takes a free port and
prints the address it got.

## What the real app adds

The mock can't show these, so the real app's own tests cover them once for every shell in
the registry (`tools/drive_first_app.py`): the app starts on the shell and marks
`app.ready`; switching to it live leaves the tabs loaded; the page shows through the
shell's unpainted part; a press in each shown page gives that page the keyboard.

## The terminal shell

`terminal`: a minimal shell laid out as a terminal emulator with split panes. Monospace,
dense and square.

- **Go's tabs are panes, in groups.** One group shows at a time and fills the window. A
  group is a tree: a split is a row or a column of parts with their shares, a leaf is one
  tab. The strip on top has a chip per group, with a segment per pane, each the tab's
  `role="tab"` element; the chips follow Go's order. Each pane has a title bar with split
  right, split down and close; a press on it makes the pane active. The prompt line at the
  foot is the address bar, with back, forward and reload, and the update when one is ready.
- **The active tab is the focused pane,** and the group shown is the one that holds it.
  Closing the active pane makes the pane that takes its room active first, so its group
  stays shown.
- **A new tab is a new group,** unless a pane's split asked for it: then it goes beside
  that pane. Dividers drag to resize, no pane smaller than 80px.
- **Its own data** is `{groups}`, each group's tree, naming tabs by id, and nothing Go
  has. It is fitted to Go's list on the snapshot and on every `tabs`: a tab that is gone
  leaves its tree, its share going to the part before it; a split left with one part
  becomes that part.
- **It leaves out** keys and menu items to split and close (a shell's own commands have no
  message yet), dimming the pages of panes that aren't active, dragging panes between
  groups, broadcasting input, and the notice of the last run's crash.

## The fox shell

`fox` puts the tabs in a strip on top, a navigation bar under it with a wide centred
address field, and a menu button at the far right, in our tones.

- **The tab strip** is the title bar. Tabs share it between a minimum and a maximum width;
  past the minimum it scrolls sideways, with arrows at its ends, New Tab fixed at its end,
  and the active tab always scrolled into view. "List all tabs" at its right opens a menu
  of every tab. A tab in a workspace has a line in the workspace's colour along its top.
- **The navigation bar** has back, forward, and reload, which is Stop (`run nav.stop`)
  while the page loads; the address field, centred, with the suggestions attached under it
  and the active tab's workspace at its right end; the extensions; and the menu button,
  whose commands each run an action Go has (`run`): New Tab, New Window, Zoom In, Zoom Out,
  Actual Size, Full Screen, Settings.
- The crash notice and the update notice are bars under the navigation bar.
- **Its own data:** none. The strip's scroll position follows the active tab, so it never
  calls `saveData`.
- **Left out:** dragging tabs to reorder or tear off, pinned tabs, a tab's context menu,
  a bookmarks bar, a sidebar, customising the toolbar, and print and find in the menu,
  which have no action yet.

## Not in this step

Keys and menu items for a shell's own commands, such as a split. Saving a shell's data
across a restart. A shell from outside the app. A shell per window. Dropping the tab
position pref, which only the standard shell reads.

## Order of building

Each commit builds, leaves a working app, and carries its tests.

1. The standard shell moves to `apps/shell/standard`, what every shell needs moves into
   the kit, and the engine and the mock load a shell by its id. No change the user sees.
2. The mock shows a page for every tab a layout lists.
3. The `shell` pref, the registry, View, Shell and the settings choice, the live switch.
4. A shell's own data.
5. The check, and the tests' shared helpers.
6. The terminal shell. 7. The fox shell.
