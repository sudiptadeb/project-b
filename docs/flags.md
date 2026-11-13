# Flags

> A flag is a named command-line switch. It is off until the user gives it a value, and it
> applies at launch. A flag's code may be left out of a build.

Every flag is one row of `core/flags/registry.json`, and nothing else defines one. Go embeds the
file, GN reads it (`engine/flags.gni`) and vite reads it (`shell/vite.config.js`). Settings,
Flags draws each row by its type, saves its value in the profile's `flags.json`, and shows
**Relaunch** beside each flag whose saved value isn't the one running.

```json
{ "id": "remoteDebugging", "name": "Remote debugging port",
  "description": "Let DevTools clients connect on this port",
  "switch": "remote-debugging-port", "type": "number", "min": 1024, "max": 65535 }
```

`id`, `name`, `description`, `switch` and `type` are required. A key Go doesn't know stops the
app at its start and fails `TestTheRegistry`.

| Type | Constraints | A value | Becomes | Settings draws |
|---|---|---|---|---|
| `boolean` | | `on` | `--switch` | a switch |
| `number` | `min`, `max` | a whole number in range | `--switch=9222` | a number field |
| `enum` | `choices` | one of the choices | `--switch=fast` | a select, Off first |
| `string` | `pattern` | text the pattern matches | `--switch=text` | a field |
| `numbers` | `min`, `max`, per item | numbers separated by commas | `--switch=1,2,3` | a field |
| `strings` | `pattern`, per item | text separated by commas | `--switch=A,B` | a field |

Empty is off. Spaces around a list's commas are dropped when it is saved. A value the flag
can't take is refused, and the field goes back to the saved one.

The flags today:

| Flag | Type | Becomes |
|---|---|---|
| Extensions | `boolean`, `build` | `--pb-extensions` |
| Remote debugging port | `number`, 1024 to 65535 | `--remote-debugging-port=<port>` |
| Chromium features | `strings` | `--enable-features=<list>` |
| Show window regions | `boolean` | `--pb-show-regions` |

## Build flags

`"build": true` says a flag's code can be left out of a build. Everything is built in unless
`WITHOUT` names it: `WITHOUT=extensions tools/build.sh`. `build.sh` names no flag; it passes
`WITHOUT` to GN as `project_b_without`, and GN passes it on to vite when ninja builds the
shell. From the same file:

- **GN** (`engine/flags.gni`) makes `project_b_built`, the ids built in, and stops at a name in
  `WITHOUT` that isn't a build flag.
- **Go**: each id built in is a build tag of the same name (`engine/BUILD.gn`), and its tagged
  file in `core/flags` tells the Flags page it is there. Left out, the row says "Not in this
  build" and can't be turned on.
- **The shell**: `@<id>` is `src/<id>`, or `src/<id>-off` when left out.
- **The tests**: `tools/test.sh` takes `WITHOUT` too, and runs Go's tests with the tags built in.

Android builds with `project_b_without = "extensions"`.

## How a flag reaches the engine

1. `core/flags` owns them: the registry, `flags.json` in the profile, and the values the app launched with.
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

1. Add a row to `core/flags/registry.json`. For a Chromium switch, that is all.
2. If its switch is ours, read it in the engine with `base::CommandLine`, as `ExtensionsOn()`
   does, and in Go with `App.Flags.IsOn(id)`.
3. For a build flag, add `"build": true`, and the code that asks about it: a line in
   `engine/flags.gni` and `engine/buildflags/BUILD.gn`, a tagged file in `core/flags`, and
   `shell/src/<id>` with `<id>-off`.

## Tests

- `core/flags/flags_test.go`: the registry's rows, each type's good and bad values, saved,
  offered for relaunch, used on the next start, and bad values dropped.
- `core/app/extensions_test.go`: no extension manager with the flag off.
- `shell/e2e/settings.cjs`: the Flags section with a flag of every type (the dev server's
  `-flags` adds the ones the registry has no row of): each drawn by its type, saved, refused,
  and Relaunch beside the waiting flag only.
- `shell/e2e/extensions.cjs`: with the flag off, no toolbar button and no Extensions section.
- `tools/drive_extensions.py`: the real app runs extensions from a profile with the flag on;
  with it off, there is no button and the store's page has no store API, and the Flags
  section's switch and Relaunch start the app again on the same profile.
