# Workspaces: tabs that share no cookies with the rest of the profile

A design note, 2026-11-13, for the fourth step of [profiles.md](profiles.md). The owner
decided what a workspace is; how it is built and the first way to use it are proposed here,
and the three defaults under "Proposed defaults" wait for his word. Built for macOS;
Android gets the signatures. The research behind it is
`project-b-research/workspaces-container-tabs.md`. "Today" is the app as built so far, and
the order of building at the end says which steps are done.

## Today

- The engine makes a tab in a workspace: `create_tab` takes Go's id for one, or "" for the
  default workspace (`engine.Engine.CreateTab(workspace)`). A workspace is a storage
  partition of the profile's one browser context, in the domain `workspace`, named by the
  id and kept on disk: `chromium/Storage/ext/workspace/<a hash of the id>/` in the
  profile's folder, with its own cookies and cache there
  (`engine/browser/content_browser_client.cc`). The tab is made with a SiteInstance fixed
  to that partition (`SiteInstance::CreateForFixedStoragePartition`), and content keeps it
  there for the tab's life, across every site and frame. The default workspace is the
  context's own partition: its tabs get no SiteInstance, as before
  (`engine/browser/engine.cc`).
- A tab a page opens keeping its opener, by `window.open`, is in its opener's workspace:
  content makes it in the opener's SiteInstance. So is a link opened in a new tab, which the
  engine makes in the source tab's workspace (`OpenURLFromTab`,
  `engine/browser/page_tab.cc`), and a copy of a tab (`DuplicateTab`). A link that opens a
  new tab without an opener (`target=_blank`, `rel=noopener`) still lands in the default
  workspace. A tab made over DevTools, and one an extension opens, is in the default.
- The tab-opened event says which workspace the new tab is in (`PB_EVENT_TAB_OPENED`,
  `engine.Listener.TabOpened`); the engine reads it from the tab's partition. Go doesn't
  keep it yet, and nothing in the app opens a tab in a workspace yet: the driver does, by
  id (`opentab`). The mock engine keeps each tab's workspace, and a tab its page opens is
  in it (`mockengine.Engine.Workspace`).

## Decided

- A profile has workspaces. A tab opened in another workspace shares no cookies or site
  storage with the default workspace, as Firefox's containers and Brave's do.
- Separate per workspace: cookies, local storage, IndexedDB, caches, service workers.
  Shared across the profile: extensions, permissions, downloads, settings and history.
- The first way to use a feature is a simple command; other ways come later.

## What changes

**A workspace is a storage partition inside the profile's one browser context.** Content
keeps a tab in its partition for the tab's life, across sites, frames and the windows it
opens (`SiteInstance::CreateForFixedStoragePartition`). Brave's containers use the same
call. The profile's own storage is the default workspace; it always exists and has no
partition of its own.

**Go owns the list.** A workspace has an id Go makes and never changes, a name and a
colour. The list is one small file in the profile folder, beside the prefs.

**A tab's workspace is given when the tab is made and never changes.** `CreateTab` takes a
workspace, and a tab opening says which workspace it is in. Moving a tab to another window
keeps it.

**A tab a page opens takes its opener's workspace.** That covers a link opened with the
keyboard or the mouse in a new tab, `window.open`, and a link that opens a new tab without
an opener, which content would otherwise put in the default workspace
(`content/browser/web_contents/web_contents_impl.cc`). How the last one is closed, a
delegate override or a small Chromium patch, is decided from the code when it is built.

**Everything else opens in the default workspace:** a new tab (Cmd+T), a link from another
app, a tab made over DevTools, a new window's first tab.

**Using it, first version.**
- Tab menu: "New Tab in Workspace", a list of the workspaces read when the menu opens, as
  the Profiles menu is.
- Settings: a Workspaces section, with the list, New Workspace (a name and a colour) and
  rename.
- The shell shows a tab's workspace as a dot in the workspace's colour on the tab. A tab
  in the default workspace has none.

**Extensions see every tab and aren't told its workspace,** as decided. Content scripts
and network rules work in every workspace.

## Proposed defaults, for the owner's word

1. Cmd+T opens in the default workspace, as Firefox does, not in the workspace of the tab
   in front.
2. A workspace is shown by a coloured dot on the tab, and by nothing else for now.
3. A new profile starts with no workspaces but the default.

## Order of building

Each commit builds, leaves a working app, and carries its tests.

1. Done. The engine: a tab made in a workspace's partition, a tab a page opens taking its
   opener's, the tab-opened event saying which. A check that two workspaces don't share a
   cookie or local storage, and that one survives a restart.
2. The link that opens a new tab without an opener stays in its workspace.
3. Go: the workspace list in the profile, tabs carrying their workspace, the Tab menu's
   list.
4. The shell: the dot on a tab.
5. Settings: the Workspaces section.

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

## Not in this step

Deleting a workspace (its partition has to be emptied), a throwaway workspace, rules that
send a site to a workspace, telling extensions a tab's workspace, DevTools' storage
commands for a workspace, a workspace per window.
