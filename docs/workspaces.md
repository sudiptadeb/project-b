# Workspaces: sets of tabs with cookies and site storage of their own

A design note, 2026-11-13, for the fourth step of [profiles.md](profiles.md), brought up to
date as the owner's decisions of 2026-11-14 are built. The owner decided what a workspace is
and where a new tab opens; how it is built and the first way to use it were proposed here
and are built as the defaults below say. Built for macOS; Android gets the signatures. The
research behind it is `project-b-research/workspaces-container-tabs.md`. "Today" is the app
as built so far, and the order of building at the end says which steps are done.

## Today

- Every tab is in a workspace, and a profile always has at least one. A new profile starts
  with one, Main, whose storage is the profile's own: the storage tabs used before there
  were workspaces. Every workspace made later has storage of its own. No workspace is
  special to the user.
- The engine makes a tab in a workspace: `create_tab` takes Go's id for one
  (`engine.Engine.CreateTab(workspace)`). Main's id is "", the engine's meaning of "the
  profile's own storage": the context's own partition, so its tabs get no SiteInstance, as
  before (`engine/browser/engine.cc`). Every other workspace is a storage partition of the
  profile's one browser context, in the domain `workspace`, named by the id and kept on
  disk: `chromium/Storage/ext/workspace/<a hash of the id>/` in the profile's folder, with
  its own cookies and cache there (`engine/browser/content_browser_client.cc`). The tab is
  made with a SiteInstance fixed to that partition
  (`SiteInstance::CreateForFixedStoragePartition`), and content keeps it there for the
  tab's life, across every site and frame.
- A tab a page opens is in its opener's workspace, on every path. Content makes one that
  keeps its opener, by `window.open`, in the opener's SiteInstance. A link opened in a new
  tab the engine makes in the source tab's workspace (`OpenURLFromTab`,
  `engine/browser/page_tab.cc`), and a copy of a tab is in its workspace (`DuplicateTab`).
  A tab opened without an opener, by `target=_blank`, `rel=noopener`, `window.open` with
  `noopener` or a page that COOP cuts off, content makes in the profile's own storage and
  hands to `AddNewContents` before it loads anything; the engine drops it and makes the tab
  in the opener's workspace instead, and content loads that one with the navigation it had
  for its own. Its sessionStorage starts empty there, as such a tab's does anywhere.
- Where nothing picks a workspace, a tab opens in the first one in the list, the user's
  order. In Go that is the first window's tab at start, a new window's first tab, a link
  from another app or a handover, New Tab in a window with no tab, and a tab an extension
  opens with `tabs.create` or `windows.create` (`tabs.Manager.Create`, `OpenIn`,
  `FirstWorkspace`). The engine makes its own tabs there too, as Go tells it which
  workspace is first at start and whenever another comes to the top
  (`engine.Engine.SetFirstWorkspace`): a tab made over DevTools, and one an extension opens
  through the engine, its options page or the page it names for after it is removed
  (`OpenTab`). A window an extension's own page opens stays in the profile's own storage
  (see "Decided while building").
- The tab-opened event says which workspace the new tab is in (`PB_EVENT_TAB_OPENED`,
  `engine.Listener.TabOpened`); the engine reads it from the tab's partition. The driver
  opens a tab in a workspace by id (`opentab`, "" for the profile's own storage). The mock
  engine keeps each tab's workspace, and a tab its page opens is in it
  (`mockengine.Engine.Workspace`); it keeps the first workspace as Go last said, for the
  tests (`FirstWorkspace`).
- Go keeps the list, in `workspaces.json` in the profile folder beside `prefs.json`:
  `{"version": 2, "workspaces": [{"id": "", "name": "Main", "color": "#3470d0"}, {"id":
  "0a1b2c3d", "name": "Work", "color": "#5b7fd6"}]}`, in the user's order, which starts as
  Main and then the order they were made. Main's colour is the first the settings page
  offers. Every other id is eight hex digits Go makes, and never changes; a workspace needs
  a name no other has, whatever its case, and a `#rrggbb` colour (`core/workspaces`,
  `workspaces.Manager`: `List`, `First`, `Create`, `SetName`, `SetColor`, `Move`,
  `OnChanged`). A profile from before Main, a list without `version` or no file, gets Main
  at the top of its list once, when the list is read: the one upgrade step
  (`workspaces.New`). From the next change on the file says version 2. A copy on a folder
  named outright (`--user-data-dir`) has the folder's.
- A tab carries its workspace's id, "" for Main (`tabs.Tab.Workspace`, `workspace` in what
  the shell and every other manager hear of a tab). It is given when the tab is made,
  `tabs.Manager.CreateIn(window, workspace)`; a tab a page opens, and a copy, take the one
  the engine says. Moving a tab to another window keeps it. New Tab (Cmd+T, File, New Tab,
  the shell's button) opens in the workspace of the window's active tab, and in the first
  in a window with no tab (`tabs.Manager.Create(window)`). The shell's `newTab` can name a
  workspace instead, by its id, so a button can open a tab in a chosen one. History is the
  profile's one: a visit in a workspace goes in as any other.
- The Tab menu has New Tab in Workspace, a submenu holding the list `workspaces`
  (`{"list": "workspaces"}` in the menu template), read when the menu opens, as the
  Profiles menu is. It lists every workspace, a single one too. Choosing one opens a new tab
  in it, at the end of the window in front.
- While the profile has two or more workspaces, a tab shows a small dot in its workspace's
  colour after its title, in the sidebar and in the strip on top, named for the workspace
  to a screen reader. With one, every tab is in it, and no tab shows a dot. The SDK's
  `workspaceOf` says so for every shell (`shell/src/sdk/shell.js`): the fox shell's line on
  a tab and the workspace's name in its address field, and the minimal shells' square, go
  with it. A tab in a workspace the list doesn't hold shows none. Every shell has the list
  in its snapshot, and hears it again, `{"t": "workspaces", "workspaces": [...]}`, when it
  changes, so the dots follow at once (`core/app/workspaces.go`,
  `shell/src/apps/shell/standard/`). The mock's page fires
  a menu item by its action, as the menu bar does (`/mock/fire`, kind `menu`).
- Settings has a Workspaces section, after Profile: a list of rows, each with the
  workspace's colour, its name and the number of tabs open in it, and a menu (the ⋯
  button, or a right-click on the row) with New tab in workspace, Edit…, Move up and Move
  down. Main is a row like any other. New tab in workspace opens a tab in it in the page's
  window and closes the page (`openInWorkspace`). New workspace…
  on the heading's right and Edit… open one dialog with the name and the colour (the
  presets, or any other). A new one starts on the first preset no workspace has. Names are
  unique whatever their case: the dialog says so under the name, and Go refuses one too.
  The order is the user's, and the Tab menu lists them in it. Go answers with the list as
  it is now (`createWorkspace`, `editWorkspace` with the name and colour together,
  `moveWorkspace` by -1 or +1, and `renameWorkspace`; the page and every shell hear
  `{"t": "workspaces", ...}`), so the dots change at once and the Tab menu has the change
  the next time it opens. The page has the counts in its snapshot and hears them again,
  `{"t": "workspaceTabs", ...}`, when a tab opening or closing changes them. No workspace
  can be deleted. Every copy has the section, a copy on a folder named outright
  (`--user-data-dir`) too: the list is the folder's, as its prefs are, and nothing about
  it needs the profile list (`core/app/workspaces.go`,
  `shell/src/apps/settings/Workspaces.vue`).

## Decided

- A profile has workspaces. A tab in one workspace shares no cookies or site storage with
  a tab in another, as Firefox's containers and Brave's do.
- Separate per workspace: cookies, local storage, IndexedDB, caches, service workers.
  Shared across the profile: extensions, permissions, downloads, settings and history.
- The first way to use a feature is a simple command; other ways come later.
- New Tab (Cmd+T) opens in the workspace of the window's active tab. The shell can name a
  workspace for a new tab instead, so a button can open one in a chosen workspace.
- 2026-11-14: "default workspace should be created by default... and later users should be
  able to delete it and deleting deletes all tabs on that workspace", then: "yes no concept
  of default... workspaces needs to have atleast one space.... when its one workspace only
  we donot need to show the dot...", and "delete should be danger button". So every tab is
  in a workspace, a profile always has one at least, and no workspace is special: each can
  be renamed, recoloured, moved and deleted, but the last can't be deleted.

## What changes

**A workspace is a set of tabs with storage of its own inside the profile's one browser
context.** Main, which a new profile starts with, has the profile's own storage, the
context's own partition. Every other is a storage partition of the context. Content keeps
a tab in its partition for the tab's life, across sites, frames and the windows it opens
(`SiteInstance::CreateForFixedStoragePartition`). Brave's containers use the same call.

**Go owns the list, and it is never empty.** A workspace has an id Go makes and never
changes, a name and a colour. The list is one small file in the profile folder, beside the
prefs.

**A tab's workspace is given when the tab is made and never changes.** `CreateTab` takes a
workspace, and a tab opening says which workspace it is in. Moving a tab to another window
keeps it.

**A tab a page opens takes its opener's workspace.** That covers a link opened with the
keyboard or the mouse in a new tab, `window.open`, and a link that opens a new tab without
an opener, which content would otherwise put in the profile's own storage
(`content/browser/web_contents/web_contents_impl.cc`). How the last one is closed, a
delegate override or a small Chromium patch, is decided from the code when it is built.

**A new tab takes the workspace of its window's active tab.** Cmd+T from a tab in a
workspace opens in it. The shell can name a workspace for a new tab instead, for a button
that opens one in a chosen workspace.

**Everything else opens in the first workspace in the user's order:** a link from another
app, a tab made over DevTools, a tab an extension opens, a new window's first tab.

**Using it, first version.**
- Tab menu: "New Tab in Workspace", a list of the workspaces read when the menu opens, as
  the Profiles menu is.
- Settings: a Workspaces section, with the list, New workspace (a name and a colour), and
  edit and reorder.
- The shell shows a tab's workspace as a dot in the workspace's colour on the tab, while
  there are two or more.

**Extensions see every tab and aren't told its workspace,** as decided. Content scripts
and network rules work in every workspace.

## Defaults

Proposed with the design, and settled by the owner's word of 2026-11-14:

1. A workspace is shown by a coloured dot on the tab, and by nothing else for now. With
   one workspace only there is no dot.
2. A new profile starts with one workspace, Main, the profile's own storage. A profile from
   before gets Main at the top of its list once.

## Order of building

Each commit builds, leaves a working app, and carries its tests.

1. Done. The engine: a tab made in a workspace's partition, a tab a page opens taking its
   opener's, the tab-opened event saying which. A check that two workspaces don't share a
   cookie or local storage, and that one survives a restart.
2. Done. The link that opens a new tab without an opener stays in its workspace.
3. Done. Go: the workspace list in the profile, tabs carrying their workspace, the Tab
   menu's list.
4. Done. The shell: the dot on a tab.
5. Done. Settings: the Workspaces section.
6. Done. New Tab in the workspace of the window's active tab, and the shell's `newTab`
   naming one.
7. Done. No default: Main in every profile, the profile's own storage, and a tab nothing
   places in the first workspace, in Go and in the engine.
8. Done. The dot only while there are two or more workspaces.

## Decided while building

- The engine reads a tab's workspace from the partition content made it in, where it takes
  a WebContents content made (`Engine::AdoptTab`), rather than each caller saying. A tab's
  workspace is then where its storage really is, whichever path made it.
- A workspace's partition is on disk, never in memory, as the profile's own is.
- In a workspace tab the blank page, which every new tab loads, is loaded in the tab's own
  SiteInstance (`source_site_instance`, `Engine::LoadURL`). Content would give it a new one,
  and the blank page leaves that one with no site; content then reads the partition for the
  tab's next navigation from the empty site, the default one, and stops on a `CHECK` against
  the workspace's (`BrowsingInstance::ComputeSiteInfoForURL`). The tab's own SiteInstance has
  a site in the workspace's partition from the start. Only the blank page as a workspace
  tab's first navigation was seen to stop the app, from the SiteInstance the tab is made
  with. Once a tab has shown a page, a blank page loaded without the engine leaves it
  working: DevTools' `Page.navigate` to `about:blank`, a reload of the blank page (over
  DevTools: Go reloads no blank tab), and going
  back and forward onto it each have a driver check, and none needed a fix. A
  workspace tab's first navigation is always one the engine starts: Go's through
  `Engine::LoadURL`, or content's own, with its opener's origin, for a tab a page opens. So
  `Engine::LoadURL` is the one place that needs it; no lower hook (a throttle, the
  delegate) can choose a navigation's SiteInstance.
  a site in the workspace's partition from the start.
- A tab opened without an opener stays in its workspace through the delegate, with no
  Chromium patch: the repo builds Chromium unpatched and keeps no patches. In
  `AddNewContents` the engine drops the WebContents content made in the profile's own
  storage and returns one it made in the opener's; content then loads that one, because its
  own is gone (`WebContentsImpl::CreateNewWindow`, under `kPwaNavigationCapturing`, which is on by
  default), with the referrer, the initiator and a form's data as it had them. This leans
  on that feature being on: with it off, content loads nothing in a WebContents it didn't
  make, and the tab stays blank. A Chromium update that turns it off, or drops the branch,
  shows as the three no-opener checks in `tools/drive_first_app.py workspaces` failing (the
  `target=_blank` link's, the `rel=noopener` link's, and `window.open` with `noopener`):
  each opened tab never loads the page. Overriding
  the creation itself (`IsWebContentsCreationOverridden`) was the other way: content asks it
  before it knows whether the opener is kept, so the engine would have had to build every
  tab a workspace's page opens, opener and all, which content already does right. The
  dropped WebContents never loaded anything, and the sessionStorage namespace content made
  for it goes with it, so no tab is left with storage from one partition and pages in
  another.
- The list has a manager and a file of its own (`core/workspaces`, `workspaces.json`),
  rather than being the tabs manager's or a pref: the tabs manager keeps only each tab's
  workspace id, and the list is read by the menu, the shell and settings, none of which
  needs the tabs.
- Main's id is "", the engine's own meaning of the profile's own storage, so nothing that
  crosses to the engine changes for its tabs. In Go "" is then an id like any other: a name
  is checked against every other workspace by its place in the list, not by an id standing
  for none, and a tab's `workspace` is always in what the shell hears, "" too.
- The upgrade step adds Main as the list is read and writes nothing: the file says version
  2 from the next change on. Until then reading the list again gives the same.
- The tabs manager asks for the first workspace as it needs it (`tabs.New(engine,
  first)`), so nothing keeps a copy of it but the engine, which Go tells at start and
  whenever another comes to the top.
- The engine keeps which workspace is first, as Go last said (`set_first_workspace`,
  `Engine::SetFirstWorkspace`), and makes the tabs it makes on its own there: one a client
  of the debugging port asks for (`DevToolsManagerDelegate::CreateNewTarget`), and one an
  extension opens through the engine, its options page or the page it names for after
  it is removed (`OpenTab`).
- A window an extension's own page opens, its popup's or its background page's
  `window.open`, stays in the profile's own storage whichever workspace is first. Content
  makes it there, the page's own, and a tab made in the first workspace in its place
  would never load: `ExtensionHost::AddNewContents` answers content with no tab whatever
  its delegate does, so content has none to load the navigation in once its own is gone
  (`WebContentsImpl::CreateNewWindow`), which is what a page's tab without an opener
  leans on.
- Erasing a workspace is content's `BrowsingDataRemover` on its partition: every kind of
  data a partition holds (cookies, local and session storage, IndexedDB, Cache Storage,
  service workers, file systems, the privacy sandbox's stores) and the HTTP cache, for web
  origins only (`erase_workspace`, `Engine::EraseWorkspace`). An extension's
  `chrome-extension://` origin isn't a web origin, so in the profile's own storage an
  extension keeps what it stored. The remover runs on a task after the closed tabs' own,
  so they are gone first, and `workspace.erased` on the timeline says when it is done.
  The partition's folder stays, as the remover leaves it.

## Not in this step

Deleting a workspace (its partition has to be emptied), a throwaway workspace, rules that
send a site to a workspace, telling extensions a tab's workspace, DevTools' storage
commands for a workspace, a workspace per window.
