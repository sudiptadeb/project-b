# Tab groups

Tabs of one window kept together under a name and a colour, which every shell draws its
own way: the Mobile shell's strip and switcher cards, Glide's sidebar folders. Owner's
decision, 2026-11-14: a group is Go's, since every shell is expected to have it
([shells.md](shells.md): a concept moves into Go when every shell is).

## What Go keeps

`core/tabs/groups.go`, in the tabs manager, beside the tabs.

- **A group** is `{id, name, color, collapsed, tabs}`: an id Go makes (`g1`, `g2`, ...),
  a name that may be empty, a colour `#rrggbb`, whether a shell that folds groups shows it
  folded, and its tabs' ids in the group's own order, which needn't be the window's.
- **A tab** carries `group`, its group's id, or none.
- **One window, one workspace.** A group's tabs are all in its window and in the workspace
  of the tab that started it; Go refuses a tab from elsewhere.
- **A tab is in one group at most:** joining one takes it out of the other.
- **A group ends** when it drops below two tabs: one closed, moved to another window,
  taken out. A group made of one tab stays until a tab leaves it.
- **Colour:** given, or the first of the seven workspace colours no other group of the
  window has (`GroupColors`, the same list as settings' Workspaces).
- **A tab a page opens** (`target=_blank`, `window.open`) from a tab in a group joins that
  group, right after its opener.
- **A new tab in a group:** `newTab` with `groupWith`, a tab's id. The new tab goes in that
  tab's workspace and group, at the group's end; when that tab is in none, the two start
  one, as Chrome's "Open in new tab in group" does.
- **Lifetime:** in memory with the tabs, until sessions are saved.

## What the shells hear and say

The `tabs` message and the snapshot carry `groups`, this window's groups in the order
they were made. The shell sends `createGroup`, `addToGroup`, `removeFromGroup`,
`editGroup`, `ungroup` and `closeGroup`, and `newTab` with `groupWith`
([shells.md](shells.md#the-messages)).

Moving a tab with `moveTab` follows the group where it lands: dropped between two tabs of a
group it joins it, and out of its group's run it leaves it; a group's tabs keep the
window's order.

## Not in this step

Moving a group to another window, reordering groups, dragging tabs between workspaces or
onto groups in a sidebar (in todo.md, for discussion), and saving groups across a restart.
