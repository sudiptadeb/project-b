# Glide: a shell to choose

`glide`, built 2026-11-14 from the spec in `project-b-research/arc-shell/spec.md`, under its
section 0, the owner's decisions of that day. Arc gave the look; the behaviour is ours and
kept simple. Nothing of Arc's name, icons or assets is used.

## What it is

- **One sidebar on the left, the page a card in the frame.** The frame is one flat tone of
  the Space's colour (`tokens.js`), the page inset 8px with 8px corners, a hairline round
  it. Radii are 4, 6 and 8. Light, dark or the system's, from the one theme pref.
- **The sidebar, top to bottom:** the window's buttons, Hide Sidebar, Back, Forward,
  Reload; the address (a button that opens the Command Bar on it) and Copy Address; the
  Space's Favorites; the Space's name and its menu; its pinned tabs; New Tab; its tabs,
  with folders and splits among them; at the foot the extensions, a dot per Space, New
  Space and Settings.
- **The Command Bar** finds open tabs in every Space, actions by name (a leading `/` lists
  only them, with their keys), the history, and the address or search typed. Opened on the
  address (Cmd+L, the address button, a new empty tab) Enter takes the active tab there;
  opened empty (Open Command Bar) it opens a new tab in the Space. Cmd+Enter opens it in a
  split beside the active tab. It covers the page, not the sidebar.

## The model

- **A Space is a workspace, one to one,** with its colour, Main among them. **The Space
  shown is the active tab's**: there is no
  separate "current Space", so Go and the shell can't disagree. Showing a Space activates
  the tab it showed last, or opens a tab in it when it has none.
- **A tab is a tab.** Closing closes it.
- **Favorites** are saved addresses, per Space, up to 12. A press switches to the Space's
  tab at that address (or on that site), else opens it in a new tab.
- **Pinned** keeps a tab at the top of its Space. Closing a pinned tab closes it and
  unpins it. After a restart a pinned entry shows as a row in the muted ink, with no tab
  until a press opens it.
- **Folders are Go's tab groups** (`docs/groups.md`), drawn as a coloured folder where the
  group's first tab is; a press folds it, and a folded folder still shows its active tab.
  From a tab's menu: Add to New Folder, New Tab in Folder, Move to a folder, Remove from
  Folder; from the folder's: rename and colour, Ungroup, Close Folder.
- **Split view** shows up to four tabs of a Space side by side, 8px apart, each a card,
  the active one ringed in the accent. It is one row in the sidebar, a part per tab. Made
  by Add to Split View (the tab last shown beside the active one), a tab's menu, or
  Cmd+Enter in the Command Bar; the gaps drag to resize, no pane under 240px. A press in a
  page makes it the active tab, as Go does by itself. Separate Split View ends it.

## Where its data lives

- **For the profile (new in Go, `core/shelldata`):** Favorites and pinned addresses, per
  Space. A shell saves any JSON for the whole profile with `saveProfileData`; Go writes it
  to `shells/<id>.json` in the profile folder and sends it to the shell in every window, and
  it is in every snapshot as `profileData`. Go never reads it. It survives a restart, a
  closed window and a switch to another shell and back. The smallest design that does: one
  message, one file per shell, no Glide concept in Go. It names addresses, never tab ids,
  which last only as long as a tab.
- **For the window (`saveData`, in memory):** the sidebar's width and whether it shows, the
  tab each Space showed last, the tab each pinned entry is open in, and the splits.

## Go changes it brought

It uses what Go already gives every shell: the workspace messages (`createWorkspace`,
`editWorkspace`), the tab groups, and `newTab` with `text`. New with it:

- `profileData` and `saveProfileData`, above.
- The snapshot's `actions`: every action that can run, `{id, title}`, for the Command Bar.
- Glide's own actions in the registry, under View, Glide in the menu, with keys in every
  keymap where they are free: Show or Hide Sidebar Cmd+S, Copy Address Shift+Cmd+C, Space
  1 to 9 Ctrl+1..9, Add to Split View Ctrl+Shift+=, Separate Split View Ctrl+Shift+-.
- **A `glide` keymap template,** chosen in Settings, Keyboard like any other; the keymap
  and the shell stay separate prefs. Over Project B's keys it gives Cmd+T to Open Command
  Bar, Cmd+D to Pin or Unpin Tab, and Option+Cmd+Left and Right to the previous and next
  Space. With another keymap, Cmd+T opens a new empty tab, which opens the Command Bar.

## Left out, on purpose

Archive, auto-archive, Clear, Peek, the little window, a gradient or grain, 12px radii,
pinned tabs that reset to their address, the media card, and dragging (between Spaces,
onto folders, to reorder or to split: parked in `todo.md`). The sidebar hidden is a slim
bar on top, not a sidebar that slides in on hover. Splits are side by side only.
