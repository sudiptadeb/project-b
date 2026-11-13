# Design system

How every piece of our own UI looks: the shell, the settings page, and any app added under
`shell/src/apps/`. Read this before drawing anything. Web pages are not ours and get none of it.

## Where it lives

| File | What it holds |
|---|---|
| `shell/src/tones.js` | Every colour, made from the one frame colour Go owns, light and dark |
| `shell/src/theme.js` | Puts those colours on the root as CSS variables and follows light or dark |
| `shell/src/design.css` | Type, the floating shadow, the focus ring, and the shared controls: `.icon-button`, `.text-button` with `.primary`, `.quiet` and `.danger`, `.field`, `select`, `.switch`, `.tag`, `.secondary`, `.field-error`, and their touch sizes |
| `shell/src/ui/` | The shared parts that need behaviour: `Dialog`, `RowMenu`, `Field`, `Segmented`, `Swatches`, and `escape.js`, which gives Escape to the innermost thing open |
| `shell/src/apps/settings/SettingsApp.vue` | The settings page's own layout: `.nav-item`, `.card`, `.row`, `.label`, `h2` |
| `shell/src/extensions/ExtensionsToolbar.vue` | The toolbar's floating menu (`.pbx-menu`) |

Each app's `main.js` imports `@/design.css` before mounting. A class two apps need goes into
`design.css`, never into one app's global style, and a part with behaviour two sections
need goes into `shell/src/ui/`, never copied. That is how the install prompt once drew
browser-default buttons: it used `.text-button`, which only the settings page defined, and the
shell shows the prompt over a web page, where that class did not exist.

## The rules

From the owner:

- **Flat first.** Hairline borders, no gradients, no shadows except on what floats.
- **A 4px grid** for sizes and spacing. Radii are 4, 6 and 8 only, plus 50% for round things.
- **Minimal and slim.** The density is real Ulaa's: not zoomed in, not cramped.
- **No colours of ours on pages.** Web pages keep Chromium's and the system's defaults.
- **No dummy controls.** Every control on screen works.
- **No explanatory prose.** No helper sentences where the design has no place for them.
- **The frame colour is the one thing in full accent.** Icons take a deeper ink.
- **Don't copy Chrome by reflex.** Chrome's behaviour is a reference, not its look.

## Colour

Never write a colour literal in a component. Use these variables, which `tones.js` sets:

| Variable | Use |
|---|---|
| `--frame` | The window frame and the empty strips of the shell. The colour the user picked |
| `--accent` | Primary buttons, a switch that is on, a pinned item, focus rings |
| `--on-accent` | Text or an icon on `--accent`: a primary button's words, an extension's badge |
| `--accent-soft` | Pressed or selected: the current settings section, an open popup's button |
| `--accent-tint` | Hover on any control, an open row's details |
| `--bar` | The toolbar and the settings page's background |
| `--surface` | Cards, menus, dialogs, text buttons |
| `--text` | Body text |
| `--muted` | Secondary text: section headings, versions, descriptions, quiet buttons |
| `--icon` | Icons |
| `--line` | Every hairline border and divider |
| `--hover`, `--press` | The toolbar's own hover and press |
| `--danger` | Errors, and a destructive answer or menu item |
| `--on-danger` | Text on `--danger`: a Delete button's words |

Text on `--accent` is `--on-accent`, never a fixed colour: the accent's lightness follows the
frame, so white reads on slate and not on amber, and the near-black ink the other way round.
`tones.js` picks whichever of the two contrasts more, and holds it to WCAG's 4.5 for text. Where
neither reaches 4.5 (the red preset in dark, a mid blue like Work's in light) it moves the accent
away from the label it picked, a little at a time, until it does; so the accent can differ
slightly from the frame. `shell/e2e/settings.cjs` checks every preset, light and dark.

Check light and dark with three frame colours: the default `#4f9e9e`, a dark one and a light
one. `tones.js` must keep every pair readable.

## Type

One family, Zoho Puvi with the system font behind it, 14px with a line height of 1.4, set on
the root by `design.css`.

| Size | Weight | Use |
|---|---|---|
| 16px | 600 | A page's title (settings' header) |
| 15px | 600 | A dialog's title |
| 14px | 400, 500 | Body; 500 for labels and buttons |
| 13px | 400, 500 | Menus, lists, details; section headings at 500 in `--muted` |
| 12px | 400 | Secondary lines: versions, descriptions |
| 9px | 600 | An extension's badge only |

No monospace in the UI. Code-like strings (a permission name, a host) are body text.

## Sizes and spacing

| Thing | Size |
|---|---|
| Icon button | 28px square, radius 6, icons 16px |
| Text button | 30px high, 12px side padding, radius 6 |
| Settings nav item | 34px high, radius 6 |
| Menu item | 32px high, radius 4 |
| Settings row | at least 48px high, padding 8px 16px |
| Gaps | 4px between toolbar buttons, 8px between buttons in a group, 12px between an icon and its text in a header |
| Touch | buttons grow to 40px; hover does nothing |

## Surfaces and elevation

- **Flat:** the bar, the settings page. No border, no shadow.
- **Card:** `--surface`, a `--line` hairline, radius 8, rows divided by hairlines.
- **Floating:** a menu, the address bar's suggestions, a dialog. `--surface`, a `--line`
  hairline, radius 8, and `var(--shadow-float)`. A modal dialog's backdrop is `--bar` at 70%.
- **Focus:** `var(--focus-ring)` on `:focus-visible`, never the browser's outline. Inside a menu
  the ring is inset, `inset 0 0 0 2px var(--accent-soft)`.

## Controls

Use these; never leave a `<button>`, `<select>`, `<input>` or radio to the browser's look.

| Need | Use | Not |
|---|---|---|
| An icon alone | `.icon-button` with an `aria-label` | |
| Words | `.text-button`; `.quiet` inside a row | |
| The one answer a dialog wants | `.text-button.primary` | a primary button on a row |
| A destructive answer (Delete, Discard, Remove) | `.text-button.danger`, in a dialog | |
| On or off | `.switch` | a checkbox |
| Two to four short choices | `Segmented` (Theme: System, Light, Dark) | radio buttons |
| Five or more choices, or long ones | `select` | |
| A colour | `Swatches`: presets, and with `custom` the system's picker behind a + | a bare colour input |
| A name, a number, a value | `Field` (`.field`); empty means Off where Off makes sense | a section's own input style |
| A row's actions | `RowMenu`: a ⋯ button and its menu | a row of quiet buttons |
| A label on an item (Hidden, Locked, Not in this build) | `.tag` | |
| A second line under a label | `.secondary` | a section's own class |
| A question | `Dialog` | `confirm()`, or a section's own `<dialog>` |

## Patterns

From the settings audit of 2026-11-13 (owner approved). Every section follows them.

### Saving

- **Everything saves the moment it changes.** No Save buttons, and no select that waits for
  a button.
- **A text field saves on Enter or when you leave it** (`Field`). Escape puts the saved
  value back and leaves the field; it never closes the page from inside one.
- **A choice that throws something away goes inside the confirm dialog,** not in a select
  on the row.
- **What applies only after a relaunch** says so on its row, and one Relaunch button at
  the top of the section applies them all.

### Lists

- **A list is a `.card` of `.row`s.** A row: a leading mark (a colour dot, an icon), the
  name as text, an optional `.secondary` line, then at most one inline control (usually a
  switch) and a `RowMenu`. Names are never always-open fields.
- **The row's menu holds every action on it:** Edit…, Move up and Move down where order
  matters, Details, then a separator and the destructive one last, in `--danger`.
  Right-clicking the row opens the same menu.
- **Adding:** a "New …" button on the right of the list's heading row, in the same place
  for every list. Never a form row after the items.
- **Create and edit share one `Dialog`** with the item's fields, titled "New workspace" or
  "Edit workspace". Enter saves, Escape cancels.
- **A new item gets the next colour no other item has.** Names are unique in a list; a
  duplicate is refused under the field.
- **An empty list is one muted row:** "No workspaces yet". The add button stays where it is.
- **A long name is one line with an ellipsis,** the full name in its tooltip.

### Rows and headings

- **A row** is at least 48px high, padding 8px 16px: the label on the left, the control on
  the right.
- **The nav names the section; the content never repeats it.** An `h2` names a group of
  rows, and a section with one group has none. A heading row may carry one action on its
  right, and is 30px high with or without it.
- **More under a row:** the row is the button, with a chevron on its right that turns
  when it opens. The panel sits under the row on `--accent-tint`. Escape closes it.

### Destructive actions

- **Confirm whatever deletes the user's data or more than one of their changes** in a
  `Dialog` that says what is lost: "Discard 4 changed shortcuts?". The button names the
  action (Delete, Discard, Remove), never OK.
- **A small edit that is easily redone** (one key, one switch) needs no confirm.

### Errors and status

- **Check a value where the rule is known** (required, unique, min, max, pattern, a key in
  use). The error shows under the field in `.field-error`, the typed text stays, and Enter
  keeps focus there.
- **A system error is one plain sentence** in a row of the card it is about, with the
  technical detail in its tooltip. Never a raw Go or system message.
- **A status that asks the user to act carries that action on its own row.** An update
  that is ready has Relaunch there; one that failed has Try again; a key in use links to
  the action that has it.

### Keyboard and focus

- **One focus ring,** `--focus-ring`; inside a menu, the inset one.
- **A group of choices** (`Segmented`, `Swatches`) is one Tab stop, and the arrow keys move
  in it.
- **When an action removes what had focus,** focus goes to a stated place: the next row, or
  the button that opened it. A `Dialog` gives focus back to what opened it.
- **Escape closes the innermost thing open:** a menu, then a dialog, then a field's edit or
  an open row, then the page (`ui/escape.js`).

### Copy

- **Sentence case** everywhere: "New workspace", not "New Workspace".
- **An ellipsis** when the button asks for more before it acts ("New workspace…", "Load
  unpacked…"), none when it acts at once.
- **Plain words for errors.**

### The look ahead

From the owner's reference designs, for the sidebar and toolbar work that follows: soft
surfaces tinted from the frame colour rather than grey; thin line icons, 16px at a 1.5px
stroke; colour carrying identity, a workspace or a tab group as a coloured pill or dot, a
profile as a pill on the toolbar; room to breathe, sidebar rows 36px; the address field a
rounded surface with the page's own actions inside its right end.

## Adding UI

1. Use the classes and variables above. If something is missing, add it to `design.css` (or
   to `tones.js` for a colour) and to this page, then use it.
2. Prefix a feature's own classes (`pbx-` for extensions) and keep them `scoped`.
3. Look at it in the real app, light and dark, and zoom in on the pixels: seams and default
   controls show there first.
