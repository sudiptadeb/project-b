# The daily driver

What a browser needs before someone can use it all day: right-click, downloads, file upload and
the page's own dialogs. Built in this order, one branch each, in Standard first, then Glide, then
the other shells through their owners. Mac first, then Android where it applies.

One rule for all four. **Go owns each request a page makes:** what is asked, the choices, how long
it lives, and what the answer does. The shell only draws it and sends back the choice. Each live
request has an id and sits in the state the SDK keeps; the shell answers by that id, and Go ignores
an answer to one it has dropped. What the OS already draws stays native: the file chooser and the
Save panel.

## Right-click

A right-click in a page, or a long press on a phone, reaches Go through content's
`HandleContextMenu`, which Go answers as handled. Go works out what can run there and sends it to
the window's shell as `contextMenu`. The shell designs and draws the menu, and sends back the
action chosen. Go runs it.

**The context.** `state.contextMenu` is `null`, or:

| Field | What it is |
|---|---|
| `id` | This menu's number. A newer right-click replaces it |
| `tab`, `x`, `y` | The tab, and where in its page, in the page's CSS pixels |
| `link`, `linkText` | The link pressed on, and its words |
| `src`, `media` | An image's address, or another medium's, and which: `image`, `video`, `audio` |
| `selection` | The text selected |
| `editable` | The press was in a field, for cut, copy, paste and select all |
| `actions` | What can run here: `{id, title, group, enabled}`, and `actions` of its own for a submenu |

**The actions.** Go lists only what it can run here and now. Ids never change once released, so a
shell may give an id an icon, or leave it out.

| Group | Actions |
|---|---|
| `link` | `link.newTab` Open Link in New Tab, `link.workspace` Open Link in Workspace (a submenu, `link.workspace.<id>` per other workspace), `link.newWindow` Open Link in New Window, `link.copy` Copy Link |
| `image` | `image.newTab` Open Image in New Tab, `image.copyAddress` Copy Image Address |
| `selection` | `selection.copy` Copy, `selection.search` Search for "…" |
| `edit` | `edit.undo`, `edit.redo`, `edit.cut`, `edit.copy`, `edit.paste`, `edit.selectAll` |
| `page` | `page.back` Back, `page.forward` Forward, `page.reload` Reload |

Back and Forward come disabled when there is nowhere to go. A press on a link in a field gets both
groups. Save Link As and Save Image As come with downloads, Copy Image and Inspect when the engine
has them, and Print and View Source later.

**The messages.** The shell sends `contextMenuChoose {id, item}` to run an action, or
`contextMenuClose {id}` when the menu goes without one. Go then sends `contextMenu` with `null`.
Go also drops the menu, and sends `null`, when its tab closes, navigates or stops being the active
one. Copying is Go's, through the engine's clipboard, so no shell needs `navigator.clipboard`.

**The design is the shell's.** It may order, group and leave out Go's actions, and add actions of
its own (Glide's Pin, say). Separators are its choice; `group` is there to help. In Standard it is
the design system's menu ([design-system.md](design-system.md)), at the press, kept inside the
window.

**A shell that draws nothing** shows no menu: content's own on the Mac is none. The contract's
rule 16 ([shells.md](shells.md#what-every-shell-does)) is what catches it.

**Android.** Go sends only presses on a link or an image. A long press on text or in a field keeps
Android's own selection bar, which already copies, pastes and selects. The Mobile shell's link
sheet opens on the same message, and still builds its own items from the press.

## Downloads

Today the engine has no `DownloadManagerDelegate`, so content refuses every download. The
engine gets one; Go decides where each file goes and keeps the list; the shell draws the list;
the Save panel is the system's own.

**The flow.** A page starts a download, or a menu's Save Link As does. The engine asks Go
where it goes (`PB_EVENT_DOWNLOAD_TARGET`: the download's id, its tab, address, suggested
name and type). Go answers with a path, with whether to show the Save panel at it first, or
with nothing to refuse it, as policy says. By default a file goes to `~/Downloads` under its
own name, with ` (1)`, ` (2)` added when that name is taken, and no panel; Save Link As and
Save Image As show the panel. The engine then reports each change
(`PB_EVENT_DOWNLOAD_UPDATED`: progress, done, cancelled, failed, paused), and Go keeps the
list.

**The list.** `state.downloads` is this profile's, newest first, each `{id, name, url,
path, state, received, total, paused}`, `state` being `progress`, `done`, `cancelled` or
`failed`. Go keeps the finished ones in the profile's `downloads.json`, the last 100, so the
list is there after a restart; one still running when the app quit comes back `failed`.
The shell asks for one action at a time, `downloadAction {id, action}`: `open` the file,
`reveal` it in Finder, `cancel`, `pause`, `resume`, or `remove` it from the list (the file
stays). `clearDownloads` removes every finished one.

**The shell.** A control named Downloads shows once the list has one, and opens the list:
each download a `listitem` by its file name, its progress while it runs, Open and Show in
Finder once done, Cancel while it runs. Standard puts the control in its toolbar and shows a
download's progress on it. The contract's rule 17 checks it, by role and name.

**Android.** Downloads go to the app's own files and open through Android's share sheet.
Built after the Mac's.

## File upload

The panel is the system's own, so no shell draws anything. A page's `<input type=file>`
reaches the engine (`RunFileChooser`), which asks Go first (`PB_EVENT_FILE_CHOOSER`: the tab,
whether it picks one file, several, a folder or a file to save, and the types it accepts).
Go lets it open while the page's address is one policy allows, and names the folder: the one
files were last chosen from in this run, or the system's choice. The engine then shows the
open panel as a sheet on the page's window, limited to the accepted types with All Files
still offered, and hands the page what was chosen; a folder upload gets every file under
it. The engine tells Go where they came from (`PB_EVENT_FILES_CHOSEN`) for the next panel.
Cancelling, or a refusal, answers the page with nothing, as a cancelled panel does.

**Android.** The phone's own picker already opens through content, without Go. Bringing it
through Go's policy comes with the Android pass.
