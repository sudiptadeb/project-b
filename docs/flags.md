# Flags

Two kinds, and the settings page shows both in its Flags section.

- **Build flags** say what is compiled in. `tools/build.sh` sets them, and a build can't change
  them. Today there is one: Extensions, on unless `EXTENSIONS=0`.
- **Flags** are read once, as the app starts. Settings saves them in the profile, and
  **Relaunch** appears while what is saved differs from what the app started with.

| Flag | Becomes | Needs |
|---|---|---|
| Extensions | `--pb-extensions` | the Extensions build flag |
| Remote debugging port | `--remote-debugging-port=<port>`, 1024 to 65535 | |
| Chromium features | `--enable-features=<list>` | |

## How a flag reaches the engine

1. `core/flags` owns them: the list, `flags.json` in the profile, and the values the app started with.
2. Before content starts, `ProjectBMain` (`engine/project_b_main.cc`) asks Go for the switches
   (`launch_switches` in `pb_abi.h`, `PBLaunchSwitches` in `core/cmd/lib`) and adds each to the
   browser's command line. A switch already given on the command line wins, so the test tools keep theirs.
3. From then on the engine reads only its command line. Go reads the same file when it starts,
   so both sides agree.
4. Relaunch (`core/relaunch`) starts the program again, with its arguments, once this process
   has exited, then closes the window. The updater uses the same helper.

## The Extensions flag

`ExtensionsOn()` (`engine/common/extensions_on.h`) is the build flag and the switch together.
Off, none of the extension code starts:

- the browser uses the plain `ContentBrowserClient`, not `ExtensionsContentBrowserClient`, and
  creates no `Extensions`;
- no process sets an `ExtensionsClient`, and renderers and utility processes get content's
  default clients, so no store API and no content scripts;
- Go starts no extension manager, so the shell has no toolbar button and settings no
  Extensions section.

The browser passes `--pb-extensions` to every child process it starts, in
`ExtensionsContentBrowserClient::AppendExtraCommandLineSwitches`.

## Adding a flag

1. Add it to `All` in `core/flags/flags.go`: id, name, kind (`switch` or `text`), what it
   becomes, and for text what a value may be. Name a build flag in `Build` if it needs one.
2. If the switch is ours, read it in the engine with `base::CommandLine`, as `ExtensionsOn()`
   does. Chromium's own switches need nothing more.
3. Go reads it with `App.Flags.On(id)`.
4. Add a test to `core/flags/flags_test.go`. The settings page lists it on its own.

## Tests

- `core/flags/flags_test.go`: saved, offered for relaunch, used on the next start, and bad values dropped.
- `core/app/extensions_test.go`: no extension manager with the flag off.
- `shell/e2e/settings.cjs`: the Flags section saves a port, refuses a bad one, and shows
  Relaunch only while something waits.
- `shell/e2e/extensions.cjs`: with the flag off, no toolbar button and no Extensions section.
- `tools/drive_extensions.py`: the real app runs extensions from a profile with the flag on;
  with it off, there is no button and the store's page has no store API, and the Flags
  section's switch and Relaunch start the app again on the same profile.
