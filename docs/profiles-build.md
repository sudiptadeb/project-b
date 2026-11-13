# Several profiles, and how the app starts on one

A design note, 2026-11-13, for the third step of [profiles.md](profiles.md), with the
owner's decisions. Built for macOS; Android keeps its one profile. Nothing here is built
yet; "Today" is the app before this step.

## Today

- One profile folder: `--user-data-dir`, else `--profile`, else `~/.project-b/default`,
  chosen in C++ before Go starts (`engine/browser/browser_main_parts.mm`). Everything Go
  keeps takes that one folder (`core/app/app.go`).
- Prefs are four values in `<profile>/prefs.json`: where the tabs go, automatic updates,
  the frame colour, the theme (`core/prefs`).
- One copy of the app per folder, with a handover (`core/instance`). Nothing starts a
  second copy on another folder, and nothing lists folders.
- The update state lives in the profile folder, and "relaunch to update" swaps the
  installed app and restarts this one copy (`core/update`).
- The shell, settings and every tab share the profile's one browser context.

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

1. Our own pages in the off-the-record context.
2. The profile list beside the folders, and the start choosing the profile: named, else
   used last, else a first one.
3. Templates, Personal and Work; the first profile is made from Personal.
4. Opening another profile: the Profiles menu, the second copy, the Dock icon's dot.
5. The picker: a copy that hasn't chosen, its page, New Profile from a template.
6. The Profile section in settings: the name, and reset to a template.
7. Updates app-wide: the shared state and setting, and the restart of every copy.

## Not in this step

Deleting a profile, a picture per profile, choosing which profile takes links, saved
sessions, cloud login and sync, the lock and the seal, workspaces, a second shell.
