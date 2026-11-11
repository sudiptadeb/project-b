# Design system

How every piece of our own UI looks: the shell, the settings page, and any app added under
`shell/src/apps/`. Read this before drawing anything. Web pages are not ours and get none of it.

## Where it lives

| File | What it holds |
|---|---|
| `shell/src/tones.js` | Every colour, made from the one frame colour Go owns, light and dark |
| `shell/src/theme.js` | Puts those colours on the root as CSS variables and follows light or dark |
| `shell/src/design.css` | Type, the floating shadow, the focus ring, and the shared controls: `.icon-button`, `.text-button` with `.primary` and `.quiet`, and their touch sizes |
| `shell/src/apps/settings/SettingsApp.vue` | The settings page's own parts: `.nav-item`, `.card`, `.row`, `.label`, `select`, `.switch` |
| `shell/src/extensions/InstallPrompt.vue` | The reference dialog |
| `shell/src/extensions/ExtensionsToolbar.vue` | The reference floating menu (`.pbx-menu`) |

Each app's `main.js` imports `@/design.css` before mounting. A class two apps need goes into
`design.css`, never into one app's global style. That is how the install prompt once drew
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
| `--accent-soft` | Pressed or selected: the current settings section, an open popup's button |
| `--accent-tint` | Hover on any control, an open row's details |
| `--bar` | The toolbar and the settings page's background |
| `--surface` | Cards, menus, dialogs, text buttons |
| `--text` | Body text |
| `--muted` | Secondary text: section headings, versions, descriptions, quiet buttons |
| `--icon` | Icons |
| `--line` | Every hairline border and divider |
| `--hover`, `--press` | The toolbar's own hover and press |
| `--danger` | Errors, and only errors |

Text on `--accent` is `--surface`. Check light and dark with three frame colours: the default
`#4f9e9e`, a dark one and a light one. `tones.js` must keep every pair readable.

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
| Settings row | at least 48px high, padding 8px 14px |
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

Use these classes; never leave a `<button>`, `<select>` or `<input>` to the browser's look.

- `.icon-button`: an icon alone, with an `aria-label`.
- `.text-button`: words. Add `.primary` for the one answer a dialog wants, and only there.
  Add `.quiet` for an action inside a row (Details, Remove, Reset).
- `.switch`: a checkbox drawn as a switch (settings).
- `select`: settings' style.

## Patterns

- **Dialog** (`InstallPrompt.vue`): a modal `<dialog>`, 360px wide, padding 20. A head with a
  32px icon and a 15px title, the body, then the buttons at the bottom right: Cancel as
  `.text-button`, the answer as `.text-button.primary`. Escape cancels. A list goes in a
  hairline box with radius 6 that scrolls.
- **Menu** (`.pbx-menu`): floating, 280px wide, padding 4, items 32px high.
- **Settings section:** an `h2` heading in `--muted`, then a `.card` of `.row`s. A row is a
  `.label`, then its control on the right.

## Adding UI

1. Use the classes and variables above. If something is missing, add it to `design.css` (or
   to `tones.js` for a colour) and to this page, then use it.
2. Prefix a feature's own classes (`pbx-` for extensions) and keep them `scoped`.
3. Look at it in the real app, light and dark, and zoom in on the pixels: seams and default
   controls show there first.
