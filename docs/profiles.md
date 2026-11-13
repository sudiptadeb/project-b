# Profiles, workspaces, windows and shells

A design note, 2026-11-12, from a discussion with the owner. Nothing here is built. The
owner's decisions are marked **Decided**; the rest is proposed and waits for his yes. The
research behind it is in `project-b-research/`: `ulaa-modes.md`,
`profiles-chromium-internals.md`, `profiles-other-browsers.md` and
`workspaces-container-tabs.md`.

## Today

- One profile folder, `~/.project-b/default`, and one browser context made from it at start
  (`engine/browser/browser_main_parts.mm`). The shell, every tab, DevTools and settings use it.
- One set of Go managers reading that folder (`core/app/app.go`).
- One window. Its layout lists the tabs it shows, each with a rectangle, and the engine puts
  each listed tab's view there and hides the rest (`engine/browser/engine.cc`). Which tabs
  show is already our decision; content only renders.
- Cookies and the cache are saved in the profile's `chromium/` folder (`Network/Cookies`,
  `Cache`), unencrypted, so a login survives a quit
  (`engine/browser/content_browser_client.cc`).
- One copy of the app per profile folder: a second copy started on it hands its URLs to
  the running one and quits (`core/instance`).

## The four things

| Thing | What it is | Who owns it |
|---|---|---|
| Profile | An isolated folder, and one process running on it | The folder; a global list names them |
| Workspace | A separate set of cookies and site storage inside a profile | Go, in the profile |
| Window | One native window with one shell page and the tabs it shows | Go's window manager |
| Shell | The page that shows Go's data and takes the user's input | Chosen per profile |

Each process also has a context of its own for our pages, apart from the profile's: see
"Our own pages".

## Profiles

**Decided.**

- A profile is an isolated folder. There can be several.
- One process per profile. Opening a second profile starts a second copy of the app on that
  profile's folder. A profile can have several windows.
- A profile has a type, chosen when it is created. The type is a template: creating the
  profile applies the template's values, such as the colour scheme and prefs.
- A profile can be reset to a different template later. Reset changes prefs and look. It
  never touches data: history, cookies, logins and installed extensions stay.
- A template can carry a list of suggested extensions. The list is a pref. The user installs
  one, or all at once, through the normal install path. Installed extensions are data, so a
  reset leaves them.
- What Go keeps for a profile, such as prefs, history, and the windows and tabs when they
  are saved, lives in the profile's folder, beside Chromium's files.
- Later, a profile links to a cloud account for sync: a fresh account, or one another
  profile already uses. Login and sync aren't designed here.
- Later, a profile can be locked, and locking means its data is encrypted. See "Locking a
  profile" below. A to-do, not part of this.
- A template is only values. The profile remembers its template's name to show it, and no
  code ever asks which type a profile is. Everything a type changes is an ordinary pref the
  template fills in. A new type is then a file of values and no code. Ulaa does the opposite,
  and pays for it with about 64 mode checks in C++ and 37 hard-coded mode numbers in its
  pages (`ulaa-modes.md`).

- The first templates are Personal and Work, with different colours and tab positions.
- The app starts by choosing its profile. A picker screen exists; the start either shows
  it or opens the right profile itself. A master process will sit there later.
- On the Mac, each running profile is its own app in the Dock: our logo with a small
  accent in the profile's colour. A picture per profile comes later.
- An update is app-wide: it restarts every running profile.
- Deleting a profile comes later.

How these are built is in [profiles-build.md](profiles-build.md).

**Proposed.**

- *Reset writes all of the template's values,* after a confirmation. Nothing tracks which
  prefs the user changed by hand.
- *Folders.* A profile is a folder directly under `~/.project-b/`; today's `default` is the
  first one. Beside the folders sit the things every profile shares: the profile list (name,
  template name, colour, and later the account) and the update state, since every copy runs
  the same installed app.
- *One owner per folder.* A lock file in the profile folder. A copy that starts on a folder
  another copy holds hands its request over ("open a window", "open this link") and quits.
  This is the single-instance lock already on the to-do list, per profile folder instead of
  per app, and it is also how one profile opens another: hand over if it is running, start
  it if not.
- *For sync, later:* a profile has an identity of its own, apart from the account, so one
  account can hold several profiles that each sync their own data. Chrome and Firefox allow
  one profile per account because their sync has one data set per account
  (`profiles-other-browsers.md`); Ulaa and Safari keep several.

## Workspaces

**Decided.**

- A profile has workspaces. A tab opened in another workspace shares no cookies or site
  storage with the default workspace, as Firefox's containers and Brave's do.
- Separate per workspace: cookies, local storage, IndexedDB, caches, service workers. Shared
  across the profile: extensions, permissions, downloads, settings and history. Firefox and
  Brave share the same things.

**Proposed.**

- *A workspace is a storage partition inside the profile's one browser context.* Content
  keeps a tab in its partition for the tab's life, across sites, frames and popups
  (`SiteInstance::CreateForFixedStoragePartition`). Brave's containers use the same call.
- *Go chooses.* A tab's workspace is given when the tab is made and never changes. The engine
  interface gains one value on creating a tab and on a tab opening. A tab a page opens takes
  its opener's workspace.
- *Known holes to close,* from reading the code, not from a test: a link that opens a new tab
  without an opener falls to the default workspace in content
  (`content/browser/web_contents/web_contents_impl.cc`), and so do our own paths for
  ctrl-click, a new tab, and a tab made over DevTools.
- Not in this design: a throwaway workspace, rules that send a site to a workspace, and what
  extensions are told about a tab's workspace.

## Windows and visible tabs

**Decided.**

- A profile can have several windows.
- Showing more than one tab at once, in one window or in several, is the shell's and the
  window manager's business. The engine and content only ever hear "this tab's view goes at
  this rectangle in this window".

**Proposed.**

- *Go's data is flat:* the windows, and in each a list of tabs. No panes, no splits.
- *The shell reports which tabs are visible and where:* a list of rectangles, each with a
  tab, in place of today's one page rectangle. A shell that shows one tab at a time reports
  one. Go passes the list to the engine and doesn't know why it has that shape.
- *The shell reports which visible tab has the keyboard.* Reload, back, find and the address
  bar act on it. Go's "active tab" becomes these two things: the visible set, and the
  focused tab.
- A tab is in one place at a time, because a page has one view.
- Each window has its own shell page, and every message between Go and a shell says which
  window it is for.

## Shells

**Decided.**

- The shell is only the representation of the data and the surface the user acts on.
- There can be several shells to choose from, such as one like Firefox, one like Arc, one
  like Chrome, one like iTerm.
- The choice is per profile, and it switches live.
- A shell may keep data only it understands, such as how it splits a window into panes.
  Changing shell loses that data. The next shell starts from Go's data alone: moving from a
  shell with panes to one without lays the tabs out in a line.
- A concept moves into Go, and becomes a contract between shells, only when every shell is
  expected to have it. Tab groups are a likely first. Defined when it comes up.

**Proposed.**

- *A shell is an app of its own,* as settings is: a folder under `shell/src/apps/`, built on
  its own, speaking the same messages. Which one a profile loads is a pref, so a template
  can set it.
- *Go stores a shell's own data and never reads it:* one blob per profile, handed to the
  shell when it loads and cleared when the shell changes or the profile is reset.
- No second shell is part of this design. It makes the choice a pref and the messages the
  contract.

## Our own pages

**Decided.**

- Each process has a second, small browser context for our own pages: the shell, settings,
  the lock screen, and a profile picker when there is one. It is off the record, so content
  keeps its storage in memory and it has no folder. It is made at start, before the
  profile's context.
- The shell then shares no cookies, storage or service workers with the web, extensions
  can't see or reach into it, and our pages can't load in a tab, because they are registered
  only there.
- It is per process, not one for all profiles: two processes can't share a context's folder,
  and it needs no folder, since Go stores what a shell keeps.
- Chrome does the same for its profile picker, which runs off the record in its "System
  Profile" (`chrome/browser/ui/views/profiles/profile_picker_view.cc`). Its browser window
  is native and tied to the user's profile; ours is a page, so all of our UI moves over.
- Nothing in the extensions work is in the way: the shell and settings get icons and state
  from Go, and a popup is a page of its own in the profile's context. Whatever opens an
  extension's page from the shell asks Go to open it in the profile's context.

## Locking a profile

A to-do, recorded so that storage is built with it in mind. Nothing here is built with the
first steps.

**Decided.**

- A profile has a screen lock: a lock screen lies over it and the password takes it away.
  The pages keep running underneath and nothing is lost.
- A profile can also be sealed: its data is encrypted, cookies, site storage, everything.
  An option says when: on lock, some time after the lock, or never.
- Sealing loses the state of the pages. The tabs reload when the profile is opened again.
- Keeping someone on a page or a set of pages, as a children's type would, is a separate
  kiosk feature, not the lock.

**Proposed, for when this is designed.**

- *Sealing ends the profile's processes.* Pausing tabs isn't enough: the network service,
  service workers and storage keep the files open outside any tab, and a paused page still
  holds the data in memory. Go saves the windows and tabs, the process quits and starts
  again as far as the lock screen only, in the context for our own pages. The profile's
  context is made after the password, and the tabs come back from the saved list.
- *The lock screen is an app of its own,* as settings is, since every shell needs the same
  one. What it shows before the password, the profile's name and colour, comes from the
  profile list, outside the sealed folder.
- *What content gives.* Cookies can take a key of ours (`cookie_encryption_provider` in
  `services/network/public/mojom/network_context.mojom`). Local storage, IndexedDB, the
  caches and service workers have no encryption in Chromium, so "everything" means
  encrypting the profile's folder underneath Chromium, not each store.

## Order of building

Each step is usable on its own.

1. Cookies and the cache saved to the profile folder, and the lock with its handover.
2. Several windows, and the list of visible tabs in place of the one page rectangle.
3. Profiles: the list, creating one from a template, one process each, opening another,
   reset. Its own note: [profiles-build.md](profiles-build.md).
4. Workspaces. Its own note: [workspaces.md](workspaces.md).
5. The shell as a pref, with the shell's own data blob.

The owner agreed to this order on 2026-11-12. Step 1 saves cookies as Chromium writes them,
unencrypted; the lock adds encryption when it is designed.
