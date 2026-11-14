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
- **A shell is held to the messages on channel `shell` and to the rules in "What every
  shell does", and to nothing else.** The SDK is a convenience the browser gives a shell's
  author on top of them: a shell may use it, or talk to Go itself.
- How a shell shows the extensions is the shell's own, and so is the band around the
  window.

## Where a shell lives

- **A shell is one folder,** `shell/src/apps/shell/<id>/`, with an `index.html` and a
  `main.js`, and **one line in `core/shells/registry.json`:** its id and the name the user
  sees, `"example": true` for a shell that is built but offered nowhere, and its own
  actions, if it has any ([below](#a-shells-own-actions)). The id is lower-case letters,
  digits and dashes.
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

## The SDK

`shell/src/sdk/` is what the browser gives a shell's author. Its core has no framework in
it: plain functions and plain objects. A Vue shell and a Svelte shell each use a thin
adapter over it; a shell with no framework uses the core as it is. A shell keeps no copy
of it.

| From | What |
|---|---|
| `@/sdk/shell.js` | `state`: everything Go sends, as a plain object; a field is replaced whole when it changes, never changed in place. `onChange(fn)`: calls `fn(fields)` after each change, with the fields that changed. `connect(handlers, afterDraw)`: says `ready`, keeps `state`, calls the shell's `handlers[kind](message)` after each message, and once the snapshot is drawn reports the layout and says `shown`. `send(msg)`. `saveData(value)`. `focusPage()`. `setFrame(width)`. The helpers, each given the state it reads: `activeTab(state)`, `workspaceOf(state, tab)`, `controlSpace(state)`, `keyLabel(state, action)`, and `tabLabel(tab)`. It puts the theme's colours on the root. |
| `@/sdk/regions.js` | The markers a shell marks its elements with, each with `mark(el)` and `unmark(el)`. `shell`: this element is the shell's, so presses on it reach the shell. `drag`: this element moves the window wherever it has no control: an empty space, or a whole bar of controls, whose gaps then drag; its controls, and anything in it marked `shell`, stay the shell's. `page`: `mark(el, tab)` and `update(el, tab)`, this tab's page shows here; 0 shows none. |
| `@/sdk/extensions.js` | What a shell sends about extensions: `pressExtension(id, el)`, `pinExtension(id, on)`, `manageExtensions()`, `answerExtensionPrompt(on)`. |
| `@/sdk/controls.js` | `controls`: the one selector for what a control is, something the user presses or types in (buttons, links with an address, fields, selects, text areas, `role="tab"` and `role="button"`, anything with a `tabindex`). The SDK finds a drag bar's controls by it, and the check reads it for rule 6. |
| `@/sdk/vue.js` | The Vue adapter: `state`, a reactive copy that follows the SDK's; `activeTab` and `controlSpace` as computeds; `workspaceOf(tab)` and `keyLabel(action)`; `connect(handlers)`, which waits for Vue's `nextTick`; the directives `v-shell`, `v-drag` and `v-page="tab.id"`; and `send`, `saveData`, `focusPage`, `setFrame`, `tabLabel`. |
| `@/sdk/svelte.svelte.js` | The Svelte 5 adapter: `state`, a `$state` copy that follows the SDK's; `connect(handlers)`, which waits for Svelte's `tick`; the actions `use:shell`, `use:drag` and `use:page={tab.id}`; and the SDK's helpers, given that `state`. |
| `@/theme.js`, `@/tones.js`, `@/design.css` | The colours, made from the frame colour, and the shared controls ([design-system.md](design-system.md)). |
| `@extensions` | `ExtensionsToolbar`: the Vue shells' extensions buttons, their menu and the install prompt, drawn from `state.extensions`. `extensionsBuilt`: whether the build has extensions. |

The SDK measures the marked elements and sends the layout by itself, once a frame and only
when it changed. It watches each `drag` element's subtree, so a control that comes or goes
in a bar, such as a tab added to a strip, is measured with nothing from the shell. A shell
on the SDK never sends `layout`. It does not measure on a scroll: a strip that scrolls is
marked `shell`, with `drag` spaces beside it, as the fox shell's is.

Go's window manager orders the regions, and the first that holds a point decides: the
controls in the drag bars (the shell's), then the drag bars, then the rest of the shell's,
and the pages under everything. In full screen nothing drags, so a bar that is also `shell`
is the shell's there.

## The messages

One JSON object each way, with its kind in `t`. A shell sends one with
`chrome.send('shell', [JSON.stringify(msg)])`, which the engine gives the page, and Go
calls `window.__shellReceive(json)` with each of its own, a JSON string. Nothing reaches a
shell before it has set `__shellReceive` and said `ready`. On the mock, `/mock/webui.js`
gives the page the same two.

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
| `action` | `id`: one of the shell's own actions | Its key was pressed, or its menu item chosen, while this shell is on |
| `suggestions` | `id`, `items` | In answer to `suggest` |
| `paint` | `span` | Go wants a timeline span ended once this is drawn |
| `extensions` | `extensions`: the buttons, the open popup and the prompt | While the Extensions flag is on: an extension changed, or the active tab did |

A tab is `{id, url, title, loading, progress, blocked, crashed, error, favicon, canGoBack,
canGoForward, workspace}`. The snapshot also has `running` (the version), `platform`
(`mac`, as the keymap names it) and `data`, and `extensions` only while their flag is on.

- `controls` is `{x, y, w, h}`, where the native window buttons are, zero size when there
  are none; `cornerRadius` is the radius the OS rounds the corners with, 0 in full screen.
- `update` is `{phase, version, error, done, total}`: `phase` is `""`, `downloading`,
  `ready` or `failed`.
- `keys` is the key each action shows, by action id, written as the keymap writes it
  (`Cmd+T`); `@/keys.js` and `keyLabel` show it as the menus do.
- `workspaces` is each workspace's `{id, name, color}`; the default one isn't among them.
- `suggestions` has the `id` the shell asked with and `items`, each `{url, title}`.
- `extensions` is `{buttons, popup, prompt}`. A button is `{id, name, title, icon, badge,
  badgeColor, badgeTextColor, enabled, pinned}`, in the order the menu lists them; `popup`
  is the id of the extension whose popup is open, or `""`; `prompt` is an install or a
  removal waiting for an answer, `{id, name, version, icon, permissions, remove}`, or
  `null`.

**The shell to Go**

| `t` | Carries | Does |
|---|---|---|
| `ready` | | Asks for the snapshot. Nothing reaches a shell before it |
| `shown` | | The snapshot is drawn and the layout sent |
| `layout` | `shell`, `controls`, `drag`, `tabs`, `viewW`, `viewH`, `frame` | The regions, the pages shown, and the band. Below. The SDK sends it |
| `newTab` | `workspace`, optional: a workspace's id, `""` for the default | Opens a tab in this window and makes it active, in the workspace named, or without one in the active tab's |
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
| `setTabLayout` | `text`: `vertical` or `horizontal` | Sets the tab position pref, which only the standard shell reads |
| `extensionClick` | `extension`, `rect` | A press on an extension's button at `rect`: Go opens its popup under it, closes it when it is the one open, or tells the extension it was clicked |
| `extensionPin` | `extension`, `on` | Puts the extension's button on the toolbar, or takes it off |
| `extensionsManage` | | Opens settings at its Extensions section |
| `extensionClosePopup` | | Closes the extension popup that is open |
| `extensionAnswerPrompt` | `on` | Answers the install or removal prompt |
| `timeline` | `kind`, `name`, `text`, `at` | An entry on the app's timeline ([logging.md](logging.md)): `kind` is `mark`, `begin`, `end`, `info`, `warn` or `error`, and `at` the page's `performance.now()`. The SDK sends them |

**The layout.** Each rectangle is `{x, y, w, h}` in CSS pixels of the shell's page, which
covers the window. `shell` is the shell's own areas; `drag`, the bars and spaces that move
the window; `controls`, the controls inside those bars, which stay the shell's; `tabs`,
`{tab, rect}` for each tab whose page shows, in the order the shell shows them; `viewW` and
`viewH`, the page's size it measured against; `frame`, the width of the band the window
draws around its edge in the frame colour, over the shell and the pages, 0 for none. A
window has no band before its shell asks for one, and keeps it while a shell that crashed
or was switched loads again.

The active tab is the one the address bar, back, forward and reload act on. A shell that
shows several tabs makes the one the user works in active: it sends `activateTab` when the
user picks one in the shell, and Go makes a tab active by itself when a press in its page
gives it the keyboard.

### A shell without the SDK

The SDK does these for a shell on it. A shell that talks to Go itself does them itself.

1. It sets `window.__shellReceive`, sends `ready`, and keeps what each message says: the
   snapshot has it all, a message of state sets the fields it carries, and `tab`
   replaces that one tab in `tabs`.
2. It draws the snapshot, sends `layout`, then sends `shown`.
3. It sends `layout` again whenever anything in it changes: a resize, a tab shown in
   another place, a control that comes or goes in a drag bar, something that moves in an
   animation. The SDK sends it at most once a frame, and only when it changed.
4. It sends `focusTab` only after a layout that shows that tab, and `focusShell` before it
   puts the keyboard in a field of its own while a page has it.
5. While `extensions.popup` names an open popup, a press anywhere in the shell sends
   `extensionClosePopup`, except a press on that popup's own button, which goes to Go as
   `extensionClick` and closes it there.
6. It paints its own colours from `windowColor` and `theme`, or with `tones()` from
   `@/tones.js`, as the SDK does.
7. For the timeline only: it answers `paint` with a `timeline` entry of kind `end` and that
   span once it has drawn. Without it the span stays open.

## A shell's own data

`saveData(value)` sends the whole value; Go keeps the latest for the window and never
reads it. It comes back as `state.data` in the snapshot, so a shell that was reloaded lays
itself out as it was. A new window's shell gets none. It is dropped when the shell
changes, the profile is reset to a template, or the window closes. It is in memory for
now, and will be saved with the windows and tabs when sessions are.

A shell's data names tabs by id. A tab in the data that Go no longer lists is gone, and a
tab Go lists that the data doesn't have is new: Go's list is the truth.

## A shell's own actions

A shell may have actions of its own, with keys, such as the terminal's splits. Each is an
ordinary action in the keymap ([menus-shortcuts.md](menus-shortcuts.md#a-shells-own-actions)):

- **The shell declares them** in its entry in `core/shells/registry.json`, each an id and
  the title its menu item shows: `"actions": [{"id": "terminal.splitRight", "title":
  "Split Right"}]`. The id starts with the shell's id and a dot, and never changes once
  released.
- Go registers them, and each is enabled only while its shell is the profile's. Their keys
  are in the keymap templates and their place in the menu tree, as any action's.
- **Running one sends the shell `action`** with its id. The shell handles it as it would
  its own control: with the SDK, `connect({ action: (m) => ... })`. With another shell on,
  the action is disabled: its key goes on to the page and no shell hears it.

## What every shell does

The check holds every shell to these, knowing nothing else about it.

1. It says `ready`, draws the snapshot, and says `shown`.
2. **The active tab's page shows,** after every change: a new tab, another made active, a
   close, a tab a page opened, a resize. It asks for a page's keyboard (`focusTab`) only once
   the layout it sent shows that page: the window can't give the keyboard to a page it
   hides, and no key reaches Go until something takes it.
3. The pages it shows lie inside the band it asked for, don't overlap, and are all tabs of
   this window.
4. **Each tab of the window is an element with `role="tab"` and `data-tab="<id>"`,** the
   active one with `aria-selected="true"`. A click on it makes that tab active.
5. **`focusAddressBar` puts the keyboard in the field named Address,** where an address
   and Enter take the active tab there.
6. **Every control it draws is the shell's,** a control as `@/sdk/controls.js` says: it
   lies in a region it marked `shell`, or in a `drag` bar, so a press reaches it and not
   the page beneath, nor the window's drag.
7. A press in the middle of a page it shows reaches that page.
8. Some space is marked `drag`, and no page lies under the native window buttons.
9. Reloaded, it comes back showing the same pages: what it needs to do so it saved.
10. Switched to and away from, it starts from Go's data alone.
11. **Settings is one press away, or two:** a control named Settings, on the shell or in a
    menu one of its controls opens, opens the settings page.
12. **The basic controls are somewhere in the shell, found by role and name, and each does
    its job.** The check finds each one only as a screen reader would, by its role and
    its accessible name, never by a class or where it sits, and presses it or types into
    it to see Go do what it says.
13. It logs no errors.

**The names a shell gives its basic controls.** Each is a button, or a menu item in a menu
one of the shell's controls opens; a menu item's name may end with its key.

- **Back**, **Forward**, and **Reload**, which may be **Stop** while a page loads.
- **New Tab**.
- A way to close a tab: **Close Tab**, for the active one, or **Close** and the tab's
  label as `tabLabel` gives it, such as Close Mock example.com.
- **Address**: a text box or a combo box, where an address and Enter take the active tab
  there.
- **Settings**.
- **Extensions**, while their flag is on, whose menu has **Manage extensions**.

**The extensions.** Every shell shows the extensions' buttons and the install prompt, from
`state.extensions`: the Vue shells with `ExtensionsToolbar`, `minimal-svelte` with its
own. What a shell draws carries the hooks the real app's extensions tests
(`tools/drive_extensions.py`) find it by:

- the Extensions button, `aria-label="Extensions"`;
- its menu, `.pbx-menu`, which Escape closes, with Manage extensions as `.pbx-manage` and
  each extension's pin as a button labelled `Pin <name>` or `Unpin <name>`;
- each pinned button, `.pbx-action`, labelled with the extension's title or name, with
  `data-extension="<id>"`, `.pbx-open` while its popup is open, its icon an `img` and its
  badge `.pbx-badge`;
- the prompt, `.pbx-prompt`, its yes button `.primary` and its no button another.

A shell that draws them itself draws them only when `extensionsBuilt` from `@extensions`
says the build has them, so a build without extensions has none of it.

## Building one

1. Make `shell/src/apps/shell/<id>/` with `index.html` and `main.js`, and add the line to
   `core/shells/registry.json`.
2. In `main.js`: import `@/design.css` and mount the app: a Vue app with the three
   directives from `@/sdk/vue.js`, a Svelte one with `mount`. The app calls `connect()` as
   it is made.
3. Draw `state.tabs` and the basic controls with their names; mark what is the shell's
   with `shell`, the top bar with `drag` (its gaps move the window, its controls stay the
   shell's), and each place a page shows with `page`. Ask for a band with `setFrame` if
   the shell wants one.
4. Run it on the mock (below) and make the check pass.

The smallest shell is the example, `shell/src/apps/shell/example/`: the one to copy. It is
built with the others and offered nowhere (`"example": true` in the registry); `go run
./cmd/dev -use example` loads it. Beside an `index.html` and a `main.js` of a few lines,
its `App.vue` is the whole of it:

```vue
<script setup>
import { ref } from 'vue';
import { connect, controlSpace, send, setFrame, state, tabLabel } from '@/sdk/vue.js';
import { ExtensionsToolbar } from '@extensions';

setFrame(4);
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
    <button @click="send({ t: 'closeTab', id: state.active })">Close Tab</button>
    <button @click="send({ t: 'back' })">Back</button>
    <button @click="send({ t: 'forward' })">Forward</button>
    <button @click="send({ t: 'reload' })">Reload</button>
    <input ref="address" aria-label="Address"
      @keydown.enter="send({ t: 'navigate', text: $event.target.value })">
    <component :is="ExtensionsToolbar" v-if="ExtensionsToolbar" :extensions="state.extensions" />
    <button @click="send({ t: 'run', text: 'app.settings' })">Settings</button>
  </header>
  <main v-page="state.active"></main>
</template>

<style>
#app { display: flex; flex-direction: column; }
header { display: flex; align-items: center; gap: 4px; height: 44px; padding-right: 8px; background: var(--bar); color: var(--text); }
input { flex: 1; }
main { flex: 1; margin: 0 var(--frame-width) var(--frame-width); }
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
against one shell, in a real browser on the mock engine, with the Extensions flag on; with
no id it runs every shell in the registry. It ends with one line per rule, `ok` or `FAIL`.

**The shell's own tests.** `shell/e2e/shells/<id>.cjs`, for what only this shell does: its
panes, its menus, its keys. `shell/e2e/lib.cjs` starts the mock, built with extensions, on
a port of its own and a profile of its own, so tests run side by side, and has the helpers the other tests use:
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
  `role="tab"` element; the chips follow Go's order. New Tab follows the chips, and at the
  strip's far end are the extensions and a gear that opens settings. Each pane has a title
  bar with split right, split down and close; a press on it makes the pane active. The
  prompt line at the foot is the address bar, with back, forward and reload, and the update
  when one is ready. Cmd+D splits the active pane right and Shift+Cmd+D down, its own
  actions `terminal.splitRight` and `terminal.splitDown`; the split buttons' titles show
  the keys.
- **The active tab is the focused pane,** and the group shown is the one that holds it.
  Closing the active pane makes the pane that takes its room active first, so its group
  stays shown.
- **A new tab is a new group,** unless a pane's split asked for it: then it goes beside
  that pane. Dividers drag to resize, no pane smaller than 80px.
- **Its own data** is `{groups}`, each group's tree, naming tabs by id, and nothing Go
  has. It is fitted to Go's list on the snapshot and on every `tabs`: a tab that is gone
  leaves its tree, its share going to the part before it; a split left with one part
  becomes that part.
- **It leaves out** a key to close a pane, dimming the pages of panes that aren't active,
  dragging panes between groups, broadcasting input, and the notice of the last run's
  crash.

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

Keys and menu items for a shell's own commands, such as a split, which came later
([A shell's own actions](#a-shells-own-actions)). Saving a shell's data across a restart. A
shell from outside the app. A shell per window. Dropping the tab position pref, which only
the standard shell reads.

## Order of building

Each commit builds, leaves a working app, and carries its tests.

1. The standard shell moves to `apps/shell/standard`, what every shell needs moves into
   the kit, and the engine and the mock load a shell by its id. No change the user sees.
2. The mock shows a page for every tab a layout lists.
3. The `shell` pref, the registry, View, Shell and the settings choice, the live switch.
4. A shell's own data.
5. The check, and the tests' shared helpers.
6. The terminal shell. 7. The fox shell.

## Proposed: the shell API without Vue

The owner asked that the shell's API not depend on Vue, and that a shell in Svelte show
where a shell's functional part ends and its presentation begins. The branch `shell-api`
builds it: the SDK above with no framework in it, a Vue and a Svelte adapter over it, and
`minimal-svelte`, a bare shell in Svelte that keeps every rule. This is the line it draws,
and what is left to decide.

### The functional part: the SDK

What every shell needs, whatever it looks like:

- Talking to Go: the channel, `ready` and `shown`, and sending what the user does.
- Go's state as it arrives, and hearing what changed.
- The shell's own data.
- Giving a page the keyboard only once the layout shows it.
- Measuring the layout: the shell's areas, the drag bars and their controls, where the
  pages show, and the band.
- What a control is.
- Key labels, and the theme's colours on the root.
- The extensions' state, what goes back about them, and closing an open popup on a press
  elsewhere in the shell.
- The timeline's entries.

### The presentation: each shell's own

- What it draws and where: the tabs, the bars, menus, notices, and the extensions'
  buttons and prompt.
- Which elements it marks `shell`, `drag` and `page`.
- The band around the window, from none to any width. The window draws it, over the
  pages, so the shell keeps its pages inside it.
- Where the keyboard goes when a tab becomes active: the address, or the page.
- The shape of its own data, such as the terminal's panes.
- Its look: the design system's classes, or none.

### The SDK's names

- `state`: Go's state, a plain object; a field is replaced whole when it changes.
- `onChange(fn)`: calls `fn(fields)` after each change, with what changed.
- `connect(handlers, afterDraw)`: says ready, keeps the state, calls the shell's handler
  for each message, and says shown once the snapshot is drawn.
- `send(msg)`: one message to Go.
- `saveData(value)`: the shell's own data for this window.
- `focusPage()`: gives the active tab's page the keyboard, once the view has drawn and the
  layout shows it.
- `setFrame(width)`: the band the window draws, 0 for none.
- `shell`, `drag` and `page`: the markers, each with `mark(el)` and `unmark(el)`, and
  `page` with `update(el, tab)`.
- `activeTab(state)`, `workspaceOf(state, tab)`, `tabLabel(tab)`, `keyLabel(state,
  action)`, `controlSpace(state)`: helpers that read the state they are given.
- `pressExtension(id, el)`, `pinExtension(id, on)`, `manageExtensions()`,
  `answerExtensionPrompt(on)`: what goes back about extensions.
- `controls`: the selector for what a control is.

"Once the view has drawn" is the one place a framework comes in. `connect` takes
`afterDraw(fn)`: Vue's `nextTick`, Svelte's `tick`. Without one, the SDK takes it that the
view draws as it hears of a change, as a shell with no framework does in its `onChange`.
The helpers take the state they read so that a framework's copy can be given to them, and
the framework then knows what was read.

### The adapters

- **Vue**, `@/sdk/vue.js`, gives the names the Vue shells already used: `state` as a
  reactive copy that follows the SDK's, `activeTab` and `controlSpace` as computeds,
  `workspaceOf(tab)`, `connect` with `nextTick`, and the three directives. The four Vue
  shells changed only where the SDK took a job from them: the import path; no
  `useTheme`, as the SDK paints the colours; `setFrame(4)` in place of reading the band
  from Go; and the extensions' toolbar given `state.extensions`. The example gained the
  basic controls it lacked.
- **Svelte 5**, `@/sdk/svelte.svelte.js`: `state` as a `$state` copy, `connect` with
  `tick`, the actions `use:shell`, `use:drag` and `use:page={id}`, and the SDK's helpers,
  given that `state`.
- **No framework**: the shell imports `@/sdk/shell.js` and `@/sdk/regions.js`, marks
  elements as it makes them, and draws again what `onChange` names.

### What a shell's author writes

In Vue, as the example above. In Svelte, as `minimal-svelte` does:

```svelte
<script>
  import { activeTab, connect, drag, page, send, shell, state, tabLabel } from '@/sdk/svelte.svelte.js';

  let address;
  connect({ focusAddressBar: () => address.focus() });
</script>

<header use:shell use:drag>
  {#each state.tabs as tab (tab.id)}
    <button role="tab" data-tab={tab.id} aria-selected={tab.id === state.active}
      onclick={() => send({ t: 'activateTab', id: tab.id })}>{tabLabel(tab)}</button>
  {/each}
  <input bind:this={address} aria-label="Address" value={activeTab(state).url}
    onkeydown={(e) => e.key === 'Enter' && send({ t: 'navigate', text: address.value })} />
</header>
<main use:page={state.active}></main>
```

With no framework, nothing is built for it yet. It would read:

```js
import { connect, onChange, send, state, tabLabel } from '@/sdk/shell.js';
import { drag, page, shell } from '@/sdk/regions.js';

const header = document.querySelector('header');
const main = document.querySelector('main');
shell.mark(header);
drag.mark(header);
page.mark(main, 0);

onChange((fields) => {
  if ('tabs' in fields || 'active' in fields) drawTabs();
  page.update(main, state.active);
});
connect({ focusAddressBar: () => address.focus() });
```

where `drawTabs` makes a button for each of `state.tabs`, and keeps the ones it made by
tab id.

### The extensions

Drawing them is the shell's. The SDK keeps `state.extensions`, sends what goes back, and
closes an open popup on a press elsewhere in the shell; each pinned button carries
`data-extension="<id>"` so its own press isn't one. The Vue shells keep `ExtensionsToolbar`,
which now takes `extensions` as a prop and sends through the SDK, and looks and works as
it did. `minimal-svelte` draws plain buttons and a plain prompt with the same hooks.
`extensionsBuilt` from `@extensions` lets a shell's own buttons leave a build without
extensions, and nothing of `@extensions` but that one constant reaches a Svelte shell, so
no Vue does.

What it costs: `minimal-svelte` is 51 KB, 20 KB gzipped, of which its extensions part is
under 5 KB, and 47 KB without extensions. The example, the same shell in Vue, is 81 KB,
32 KB gzipped. The SDK alone is 13 KB, 5 KB gzipped.

### The band

The shell asks for it with `setFrame(width)`, and the SDK sends it in the layout as
`frame`. Go keeps it per window and the engine draws it from that width, as it already
took the width with every layout, so the engine didn't change. The four Vue shells ask for
4px and look as they did; `minimal-svelte` asks for none and its page reaches the window's
edge. Rule 3 checks the pages lie inside the band the shell asked for. The snapshot no
longer has `frameWidth`; the settings page had a copy it never read, which went too.

### The two minimal shells

`minimal-svelte` and `minimal-js` are to be, in the owner's words, a tab bar, a toolbar, a
vertical list too, the simplest button placements, a simple layout, no rounding, with
thought for many tabs. The SDK makes that easy: a shell draws `state.tabs` however it lays
them out, marks its bars, and the SDK measures whatever moves. A strip that scrolls is
marked `shell`, with `drag` spaces beside it, as the fox shell's is; a list that wraps or
scrolls is the same. The spike's `minimal-svelte` is not that design: it only proves the
API, with every control plain.

### What the spike changed

1. The kit became the SDK, with no framework in it, and the Vue shells use an adapter over
   it.
2. The extensions' state and the messages about them moved into the SDK, and the Vue
   toolbar draws from them.
3. Svelte joined the build, with its adapter. `svelte` 5 and `@sveltejs/vite-plugin-svelte`
   6 are new dependencies of `shell/`, so `npm install` there comes before the next build:
   the engine's build and the Android release both build from `shell/node_modules`.
4. `minimal-svelte`, registered with `"example": true`, so it is offered nowhere.
5. The band became the shell's.
6. Rule 12, the basic controls, and the example's.

### Left for the owner

- **How the browser provides the SDK.** Bundled into each shell at build time, as it is
  now, or served once at `chrome://shell/sdk/sdk.js` for any shell to import, so a shell
  with no framework needs no build step. Serving needs no engine change: the engine
  already serves `chrome://shell/<id>/<id>.js`, so `sdk` would be an id no shell may take.
  It takes a library build of the SDK in `build.js`; every built shell, and the adapters,
  importing that file instead of a copy, since two copies would keep two states; and the
  SDK's names becoming a contract a shell from outside relies on. Recommendation: bundle
  it, as now, and serve it once a shell from outside the app is wanted, which is not in
  this step.
- **Svelte's `$state` and the SDK's `state`.** A Svelte component that imports `state`
  and also uses the `$state` rune gets a warning about the name. The spike's components
  don't need both: the extensions part takes what it needs as a prop. Recommendation: keep
  the one name in every adapter, and where a component needs both, import it under
  another name.
- **The extensions' hooks.** They are classes today, `.pbx-*`, beside a few labels. Roles
  and names would say the same more plainly, as the basic controls do: a menu named
  Extensions menu with its items, a pinned button pressed (`aria-pressed`) while its
  popup is open, a dialog named for its question. The real app's extensions tests and
  the check of a build without extensions would then find them as a screen reader does.
  Recommendation: move them to roles and names with the extensions session, after this.
- **`extensionsBuilt`.** One line in each of `src/extensions` and `src/extensions-off`,
  the only way a shell's own extensions buttons leave a build without extensions without
  Vue coming with them. Recommendation: keep it, named as the extensions session likes.
- **A tab that becomes active.** The standard, fox and terminal shells, and
  `minimal-svelte`, each repeat one rule: a tab with nothing in it wants the address, one
  with a page wants its page. It names the address field, which is presentation, so the
  SDK leaves it to the shell. Recommendation: leave it there.
- **The standard shell's tab position.** `state.tabLayout` and `setTabLayout` are only the
  standard shell's. Recommendation: they go with that pref, as "Not in this step" says.
- **`minimal-js`'s drawing.** Recommendation: no library. `document.createElement` with a
  small helper that makes an element with its attributes and children, and the tab list
  drawn again by tab id. lit-html, about 3 KB gzipped, is the smallest library that would
  do the drawing again for it.
