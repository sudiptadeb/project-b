# Remote builds

`tools/build.sh` sends the compiles to a remote build server when one answers at
`PB_REMOTE` (default `localhost:8980`), and builds here otherwise. Nothing else changes:
the same `args.gn`, the same commands, the same output. A remote `.o` has matched the local
one byte for byte. `PB_REMOTE=off` keeps a build local, and `PB_REMOTE_JOBS` (default 60)
caps how many compiles wait at the server at once.

Nothing in `chromium/src` is changed for it. Everything lives in `tools/remote/`:

| File | Runs on | What it does |
|---|---|---|
| `server.sh` | the build Mac | Buildbarn: `install`, `start`, `stop`, `status`, `logs` |
| `buildbarn/*.jsonnet` | the build Mac | its config, from bb-deployments' `bare` setup, every port on `127.0.0.1` |
| `toolchain.sh` | the laptop | puts this checkout's clang on the build Mac, where the workers run it |
| `siso/main.star` | the laptop | siso's config: Chromium's own, with only the compiles sent to the Mac workers |

## How it fits

    laptop: tools/build.sh -> siso -> localhost:8980 --ssh -L--> build Mac: 127.0.0.1:8980
                                                                 bb_storage (frontend, cache)
                                                                 bb_scheduler
                                                                 bb_worker + bb_runner, native macOS

- The server is [Buildbarn](https://github.com/buildbarn), run natively on the build Mac
  from its released `darwin_arm64` binaries, at the versions `server.sh` pins.
- Only compiles go remote: Chromium's `clang/cc`, `cxx`, `objc`, `objcxx` and
  `cxx_module` rules. Links, assembly and every generator stay on the laptop.
- `args.gn` keeps `use_remoteexec = false`, so the compile commands are exactly a local
  build's. `build.sh` runs siso itself with `-load tools/remote/siso/main.star`, not
  `autoninja`, which forces offline mode for such a build.
- The macOS SDK travels with each step from the laptop's Xcode, through the
  `sdk/xcode_links` link GN makes in the build directory. The worker's own Xcode is
  never used.

## Set up the server, once per build Mac

On the build Mac, with this repo checked out (or just `tools/remote/` copied over):

    tools/remote/server.sh install     # the pinned Buildbarn binaries, into ~/pb-rbe/bin
    tools/remote/server.sh start       # storage, frontend, scheduler, runner, worker
    tools/remote/server.sh status

`PB_RBE_DIR` moves `~/pb-rbe`; `PB_RBE_SLOTS` sets how many steps run at once (default:
the cores less two). After a reboot, run `start` again.

## Each time you build from the laptop

    ssh -N -L 8980:127.0.0.1:8980 <build-mac> &    # the server never listens on the network
    tools/remote/toolchain.sh                      # copies clang only when it changed
    tools/build.sh

`toolchain.sh` reads the build Mac's ssh name from `PB_RBE_SSH`. Re-run it after every
Chromium roll: a new clang gets a new folder. With no tunnel, `build.sh` says "no remote
build server answers" and builds here.

## Security

A remote execution port runs whatever command it is sent. So every Buildbarn port, the
diagnostics ones too, listens on `127.0.0.1` only, and the laptop reaches it through ssh.
Never open it to the network, and keep host names and addresses out of this repo.

## What we met, and why the config is as it is

- **Hardlinked inputs were slow.** The worker's default build directory hardlinks every
  input file of every step; on macOS that kept `bb_worker` at over 90% system CPU. The
  worker uses Buildbarn's virtual build directory instead, served over NFSv4, which macOS
  15 and later mounts without root (`worker.jsonnet`).
- **macOS scans a new executable on its first run.** For Chromium's 100 MB clang that is
  about 70 s, and an input is a new file every step, so each compile took over a minute.
  `toolchain.sh` puts clang in one folder on the build Mac, scanned once, named by
  Chromium's clang package version and checked by hash; `main.star` runs it from there.
  A compile dropped to about 4 s, with the same output.
- **Siso's default queue suits a fleet.** It keeps up to 9,600 steps in flight; against one
  Mac, the ones at the back hit the 15-minute remote timeout. `PB_REMOTE_JOBS=60` keeps
  the queue short, and the build ran at about 8 to 9 steps a second against 4 to 5 locally.
- **"A worker with the same worker ID".** A new worker started while the old one was
  finishing its steps. `server.sh stop` waits for each process to exit, and the worker has
  one runner.
- **A stale mount.** A worker that dies leaves its NFS mount behind; `server.sh` unmounts it
  before starting and after stopping.
- `google.longrunning.Operations Unimplemented` in siso's log after a timeout is noise.

When a remote build fails, `build.sh` matches the output against its known failures, the
missing toolchain and the queue timeout among them, and prints the fix.
