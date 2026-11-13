# Workspaces: tabs that share no cookies with the rest of the profile

A design note, 2026-11-13, for the fourth step of [profiles.md](profiles.md). The owner
decided what a workspace is; how it is built and the first way to use it are proposed here,
and the three defaults under "Proposed defaults" wait for his word. Built for macOS;
Android gets the signatures. The research behind it is
`project-b-research/workspaces-container-tabs.md`. Nothing here is built yet; "Today" is
the app before this step.

## Today

- A profile has one browser context, and every tab uses its one set of cookies and site
  storage. `CreateTab` takes nothing, and a tab a page opens is made in the same context
  (`engine/browser/engine.cc`, `engine/browser/page_tab.cc`).
- Cookies and the cache are saved per storage partition already: a partition that isn't
  the default gets a subfolder (`engine/browser/content_browser_client.cc`).

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

1. The engine: a tab made in a workspace's partition, a tab a page opens taking its
   opener's, the tab-opened event saying which. A check that two workspaces don't share a
   cookie or local storage, and that one survives a restart.
2. The link that opens a new tab without an opener stays in its workspace.
3. Go: the workspace list in the profile, tabs carrying their workspace, the Tab menu's
   list.
4. The shell: the dot on a tab.
5. Settings: the Workspaces section.

## Not in this step

Deleting a workspace (its partition has to be emptied), a throwaway workspace, rules that
send a site to a workspace, telling extensions a tab's workspace, DevTools' storage
commands for a workspace, a workspace per window.
