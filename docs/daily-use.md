# What a daily user needs

The features a person expects from a browser they use every day, and where Project B stands
on each. Checked on the 0.2.59 package on macOS, 2026-11-14: the areas `tools/test.sh basics`
covers, one check each, by that run; the rest by hand in the app through the driver. Each
feature is judged in the Standard shell, the default; Glide and Fox are optional styles.
Update this list when a check's result changes or a plan is decided.

**Status:** works, partly or missing. **Plan:** who builds it, or "needs discussion" when nobody
has a plan yet. "Proposed" means the plan is written down but waits on the owner's approval.

The web APIs a page calls whose browser side is Chrome's (badges, the clipboard, permissions,
devices, payments and the rest) are kept one per row, with what a page gets of each on Mac
and Android, in the web-features registry: [web-features.md](web-features.md).

## What stops daily use, worst first

| # | Feature | Status | Plan |
|---|---|---|---|
| 1 | Downloads: saving a file from a link | missing: no file arrives (basics 15) | proposed, waiting on him: session r6, Mac then Android, fourth |
| 2 | File upload: a file input opens a chooser | missing: no chooser (basics 14) | proposed, waiting on him: session r6, third |
| 3 | JavaScript dialogs: alert, confirm, prompt, and "leave this page?" on close | missing: alert returns at once, a tab closes without asking (basics 13, 1) | proposed, waiting on him: session r6, second |
| 4 | Right-click menu on a page: open link in a new tab, copy link, copy and save images | missing: a right-click never reaches Go (basics 22) | proposed, waiting on him: session r6, first. System menu or a shell-drawn one is also his call; mocks in `project-b-ux-audit/shots/context-menus/` |
| 5 | Find in page | missing: no action, menu item or key | needs discussion |
| 6 | Passwords to sites that ask (HTTP auth) | missing: the 401 page shows without asking (basics 12) | needs discussion |
| 7 | Camera and microphone, for Meet and Zoom on the web | missing: NotSupportedError, no prompt (basics 17) | needs discussion |
| 8 | Video full screen | missing: requestFullscreen stays pending (basics 18) | needs discussion |
| 9 | Tabs come back after a crash | missing: the app comes back with 1 tab (basics 32) | needs discussion |
| 10 | Bookmarks | missing in Standard. Glide, an optional style, has favorites and pins | needs discussion |
| 11 | A history page | partly: history is kept and feeds the address bar's suggestions; the History menu has only Back and Forward | needs discussion |
| 12 | Saved passwords and autofill | missing: nothing of ours; the Edit menu's Passwords… and Contact… are macOS's AutoFill | needs discussion |
| 13 | Printing | missing: `window.print()` returns at once, and there is no Print item | needs discussion |
| 14 | Error pages: a failed load, a bad certificate | missing on the desktop shells: an empty page (basics 7, 10). The Mobile shell shows a notice with Try again since 0.2.58. Certificate page mock in `project-b-research/morning-2026-11-10/` | needs discussion (in [todo.md](todo.md)) |
| 15 | A new tab page | missing: a new tab is `about:blank` | needs discussion (in [todo.md](todo.md)) |

## Safety, which a person doesn't see until it matters

| Feature | Status | Plan |
|---|---|---|
| Safe Browsing warnings | missing: the test malware page loads (basics 28) | needs discussion |
| "Not secure" on http pages | missing (basics 9) | needs discussion |
| The address bar hides a password in a URL | missing: it shows it (basics 8) | needs discussion |
| Popup blocking | missing: `window.open` without a click opens a tab (basics 6) | needs discussion |
| Blocked hosts' images and scripts | missing: only navigations are blocked (basics 27) | needs discussion (URL policy, in [todo.md](todo.md)) |

## Less often needed

| Feature | Status | Plan |
|---|---|---|
| Notification permission | missing: no prompt, always denied (basics 16) | needs discussion |
| Picture-in-picture | missing: NotSupportedError (basics 36) | needs discussion |
| A proxy | missing: `--proxy-server` isn't used (basics 26) | needs discussion |
| A second browser context, for automation | missing (basics 23) | needs discussion (in [todo.md](todo.md)) |
| Client certificates | not checked yet (basics 11 needs a test certificate) | needs discussion |
| Captive portals | not checked yet (basics 38) | needs discussion |

## What works

Checked by the same run: zoom (Cmd+= and Cmd+0), back and forward, Cmd+click to a background
tab, a tab closing while it loads, a second navigation winning, page keys not taking the
browser's shortcuts, the address bar keeping focus while typing, clicks landing where they
should, cookies across a restart, internal pages closed to tabs, no shell bridge for pages,
reload after a page crashes, Go's faults logged, a clean quit with a load in flight, updates,
a name on every shell control, resizing, and DevTools off unless switched on. Profiles,
workspaces and the shells have their own checks ([vm-testing.md](vm-testing.md)).
