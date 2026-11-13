# Several profiles, and how the app starts on one

A design note, 2026-11-13, for the third step of [profiles.md](profiles.md), with the
owner's decisions. Built for macOS; Android keeps its one profile. "Today" is the app as
built so far, and the order of building at the end says which steps are done.

## Today

- Go chooses the profile, in `PBClaimProfile` before content starts (`core/profiles`,
  `core/cmd/lib/main.go`): the folder `--user-data-dir` names, as it is and never listed;
  else the listed profile `--profile=<id>` names, refusing an id that isn't listed; else
  the picker, when the copy was started with `--picker`; else the profile used last; else,
  when the list has no profile used yet, the picker. The list is `profiles.json` in the
  root, `~/.project-b` unless `--root=<dir>` names another, beside the profile folders:
  each profile's id (its folder's name), name, a copy of its frame colour, the name of the
  template it was made from, and when a copy last started on it. A first start, with no
  list, makes it holding `default`, named Personal, from the Personal template, and opens
  it; a `default` that already holds a `prefs.json` is left as it is. Every change to the
  list holds a lock on it (`profiles.lock`), since several copies write it. Everything Go
  keeps for a profile takes its one folder (`core/app/app.go`).
- Templates are files of values in `core/commands/templates/profile/`, beside the keymap
  and menu templates and built in the same way: a name and pref values, nothing else.
  Personal is today's defaults; Work has the tabs on top and the frame colour `#5b7fd6`.
  They hold the profile's own prefs only (the tabs, the frame colour, the theme), not
  automatic updates, which become app-wide. Making a profile from one writes its values as
  the profile's `prefs.json`; the list keeps the template's name only to show it
  (`core/profiles/templates.go`).
- Prefs are four values in `<profile>/prefs.json`: where the tabs go, automatic updates,
  the frame colour, the theme (`core/prefs`).
- One copy of the app per folder, with a handover (`core/instance`).
- Opening another profile starts another copy. The menu bar has a Profiles menu, from the
  menu template, listing the profiles with a tick on this one. A menu template places a
  list that changes by its id (`{"list": "profiles"}`, `commands.List`); the engine asks
  Go for its items each time the menu opens (`PB_EVENT_MENU_LIST`, `engine.Listener.MenuList`),
  so the menu shows the list as `profiles.json` has it then, whichever copy changed it.
  Choosing one starts a copy of the app on it, `--root=<root> --profile=<id>`, at once and
  apart from this one (`relaunch.Copy`); a copy already running there takes the handover
  and comes to the front, and the new copy quits. Started from its binary, each copy is an
  app of its own, with its own Dock icon. Quit ends this profile's copy only.
- The picker is a copy of the app that has no profile (`app.Picker`, `core/app/picker.go`).
  Go's start is split: `PBStart` starts what every copy has, the engine, our pages and the
  list, then either the profile's managers (`app.Start`) or the picker. The engine makes
  only our pages' context at start; the profile's waits for Go (`open_profile`), so a
  picker copy has none. It opens one window holding the picker page, an app of its own in
  `shell/src/apps/picker/` at `chrome://picker/`, whose messages reach Go as
  `PB_EVENT_PICKER_MESSAGE` (`create_window` names the page a window holds). The page lists
  the profiles in their colours, and New Profile asks for a name and a template; Go makes
  the folder, writes the template's values as its prefs and lists it
  (`profiles.Manager.Create`; its id is eight hex digits). Choosing a profile, or making
  one, starts a copy on it, as the Profiles menu does, and the picker copy quits: a copy
  already running there takes the handover. The picker copy has the menu bar's app, Edit
  and Window menus from the templates, with no keymap file (`commands.NewFromTemplates`),
  and a Profiles menu whose items do what the page's do. Its Dock icon is the plain one.
  "Profiles…" in a profile's Profiles menu starts a picker copy. A copy keeps its own files,
  its timeline and `DevToolsActivePort`, in its profile's folder, and a picker copy in the
  root (`CopyDir` in the engine).
- Each copy draws its Dock icon as the app's with a small dot in its profile's colour,
  bottom right (`set_dock_color`, `engine.Engine.SetDockColor`). Go says the colour at
  start and when the frame colour changes, and keeps the list's copy of it the same.
- The update state lives in the profile folder, and "relaunch to update" swaps the
  installed app and restarts this one copy (`core/update`).
- Our pages, the shells and the settings pages, load in a context of their own, off the
  record and with no folder, made before the profile's
  (`engine/browser/browser_main_parts.mm`). They are registered only there, so in the
  profile's context, where the tabs, extension pages and popups are, their addresses load
  nothing; Go's gate still refuses the shell's address in a tab too. The extension hooks
  serve the profile's context only. DevTools' front end is not one of our pages: the app
  has none, only the endpoint. On Android the shell and the tabs keep one context.

## What changes

**Our own pages get their own context.** Each process makes a small off-the-record
context first, and the shell, settings and the picker load there. Our pages are registered
only there. The profile's context is made after it, once the process knows its profile.

**The folders.** `~/.project-b/` holds the profile folders and, beside them, what every
profile shares: the profile list and the update state. A profile's folder name is an id Go
makes and never changes; `default` is the first. The list holds, for each profile: its
folder, name, colour, template name, and when it was used last.

**A profile's colour is its frame colour.** The list keeps a copy, so the picker and the
Profiles menu can show a profile that isn't running.

**Every copy starts the same way, by choosing its profile.** In order: the profile named
on the command line; else the picker, when the copy was started to pick; else the profile
used last; else, on a first start, a new profile from the Personal template in `default`.
This step is where a master process will sit later. Today it is the first thing each copy
does, in Go, before the profile's context is made.

**A folder named outright is used as it is.** `--user-data-dir` keeps meaning "this folder
is the profile", for ChromeDriver and the tests, and such a folder never enters the list.

**The picker is a copy of the app that hasn't chosen yet.** It shows one window with the
picker page, in the context for our own pages: the profiles, and New Profile. Choosing one
makes this copy that profile, or, when that profile is already running, hands over to its
copy and quits. The lock screen will later wait in the same place.

**Opening another profile starts another copy.** The Profiles menu lists the profiles,
with a tick on this one; choosing one starts a copy on it, and the handover that exists
brings its window to the front if it is already running. "Profiles…" in that menu starts
the picker. Quit ends this profile's copy only.

**In the Dock, each running profile is its own app.** Each copy puts a small dot in its
profile's colour on the app's icon. The picker's copy shows the plain icon. A picture per
profile comes later.

**Templates.** A template is a file of values shipped in the app, beside the keymap and
menu templates: a name and the pref values. Two ship: Personal, with today's defaults, and
Work, with another colour and the tabs on top. New Profile asks for a name and a template;
Go makes the folder, writes the template's values as the profile's prefs, adds it to the
list and opens it. The profile remembers the template's name only to show it, and no code
asks which template a profile came from.

**This profile, in settings.** A Profile section: its name, and "Reset to a template",
which after a confirmation writes the chosen template's values over the profile's prefs.
History, cookies, logins and installed extensions stay. The colour is the frame colour
setting that is already there.

**Updates are app-wide.** The update state moves beside the profile folders, and so does
the automatic-updates setting. One copy at a time checks and downloads. "Relaunch to
update" restarts every running copy: the copy that was asked tells the others to quit,
swaps the installed app when they are gone, and starts each profile again. A copy comes
back with a new tab, as a relaunch does today.

## Behaviour, as decided

- Work and Personal are the first templates, with different colours and tab positions.
- One Dock icon per running profile, our logo with a small coloured accent.
- A picker screen exists, and the start either shows it or opens the right profile itself.
  Today the start opens the profile used last, and the picker shows when asked for.
- A link from another app opens in whichever copy macOS hands it to.
- An update restarts the whole app, every profile.
- A profile comes back from a restart with a new tab. Bringing back its windows and tabs,
  and reopening a closed tab or window with a key, comes with saved sessions, later.
- Deleting a profile comes later.

## Order of building

Each commit builds, leaves a working app, and carries its tests.

1. Done. Our own pages in the off-the-record context.
2. Done. The profile list beside the folders, and the start choosing the profile: named,
   else used last, else a first one.
3. Done. Templates, Personal and Work; the first profile is made from Personal.
4. Done. Opening another profile: the Profiles menu, the second copy, the Dock icon's dot.
5. Done. The picker: a copy that hasn't chosen, its page, New Profile from a template.
6. The Profile section in settings: the name, and reset to a template.
7. Updates app-wide: the shared state and setting, and the restart of every copy.

## Decided while building

- The start shows the picker by itself only when no profile is named and the list has no
  profile used last. A first start, with no list, makes `default` and opens it.
- Choosing a profile in the picker starts that profile's copy, and the picker copy quits,
  where the note had the picker copy become the profile. Becoming it in the same process
  doesn't hold up. Content forbids blocking work on the main thread once it runs, and
  making a profile reads its folder there: Chromium's own code only gets past that as a
  friend of the rule, which we are not. And the flags saved in a profile become switches
  before content starts, so a picker copy runs without the chosen profile's, the
  extensions flag among them. A copy started on the profile has both right, and the
  handover already covers a profile that is running.
- Go's gate still refuses the shell's address in a tab, as a second wall on the Mac and the
  only one on Android, where the shell and the tabs share a context.
- A copy started on a folder named outright (`--user-data-dir`) still draws its dot, in its
  frame colour, and its Profiles menu lists the root's profiles with no tick.

## Not in this step

Deleting a profile, a picture per profile, choosing which profile takes links, saved
sessions, cloud login and sync, the lock and the seal, workspaces, a second shell.
