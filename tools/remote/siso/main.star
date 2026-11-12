# -*- bazel-starlark -*-
"""Siso config for remote builds on our own Mac workers, set up from outside chromium/src.

The build is configured as usual (use_remoteexec stays false in args.gn, so every compile
command is exactly what a local build runs), and this file decides where steps run: it
loads Chromium's config (build/config/siso/main.star) and sends the compile steps to the
Mac workers of tools/remote/buildbarn. Everything else stays local, including the steps
Chromium would run remotely on Linux (mojo, grit, Blink and TypeScript generators), and
linking, which siso does not run remotely on a Mac.

Nothing is written into chromium/src. What a compile step needs travels with it:
- clang's own headers and libc++'s config, from Chromium's clang input deps;
- the macOS SDK, through the link GN makes in the build directory (sdk/xcode_links),
  so the worker uses this Mac's SDK, never its own Xcode.

tools/build.sh passes this file to siso with -load when a remote build server answers at
PB_REMOTE. docs/remote-build.md has the whole setup.
"""

load("@builtin//encoding.star", "json")
load("@builtin//struct.star", "module")
load("@config//main.star", chromium_init = "init")
load("@config//clang_all.star", "clang_all")
load("@config//clang_unix.star", "clang_unix")
load("@config//gn_logs.star", "gn_logs")

# Chromium's clang rules that compile (build/config/siso/clang_unix.star). Assembly and
# links stay local, as they do in Chromium's own remote config on a Mac.
_COMPILES = ("clang/cc", "clang/cxx", "clang/cxx_module", "clang/objc", "clang/objcxx")

# What tools/remote/buildbarn/worker.jsonnet serves.
_PLATFORM = "mac_worker"
_PLATFORM_PROPERTIES = {"OSFamily": "Darwin"}

# Where tools/remote/toolchain.sh puts this checkout's clang on the workers, one folder per
# Chromium clang package version. Workers run clang from there rather than from the step's
# inputs: macOS scans an executable the first time it runs from a new file, which for
# Chromium's 100 MB clang takes over a minute, and inputs are a new file every step.
_TOOLCHAINS = "/Users/Shared/pb-rbe/toolchains"

# Chromium allows a remote compile 2 minutes, then retries it locally. The biggest Blink
# files take several minutes, so allow more before giving up on the worker.
_TIMEOUT = "15m"

# The same globs Chromium uses for a remote SDK (build/config/siso/mac_sdk.star).
_SDK_HEADERS = [
    "*.framework",
    "*.h",
    "*.json",
    "*.modulemap",
    "Current",
    "Frameworks",
    "Headers",
    "Modules",
    "crt*.o",
    "usr/include/c++/v1/*",
    "usr/include/c++/v1/*/*",
]

def _clang_version(ctx):
    """Chromium's clang package version, as tools/clang/scripts/update.py names it."""
    rev = None
    sub = None
    for line in str(ctx.fs.read("tools/clang/scripts/update.py")).splitlines():
        if line.startswith("CLANG_REVISION = "):
            rev = line.removeprefix("CLANG_REVISION = ").strip("'\"")
        elif line.startswith("CLANG_SUB_REVISION = "):
            sub = line.removeprefix("CLANG_SUB_REVISION = ").strip()
    if not rev or not sub:
        fail("project_b: can't read clang's version from tools/clang/scripts/update.py")
    return "%s-%s" % (rev, sub)

def init(ctx):
    chromium = chromium_init(ctx)
    toolchain = "%s/%s/bin/" % (_TOOLCHAINS, _clang_version(ctx))
    step_config = json.decode(chromium.step_config)

    for rule in step_config["rules"]:
        rule["remote"] = False

    # On a Mac, Chromium adds its clang rules only when a reclient config sits in
    # chromium/src (clang_mac.star), so take its compile rules straight from clang_unix.
    # They go first, as Chromium puts them; their command prefixes match only clang.
    compiles = []
    for rule in clang_unix.rules(ctx):
        if rule["name"] in _COMPILES:
            # The rule runs clang or clang++, named by the start of its command.
            compiler = rule["command_prefix"].split(" ")[0].split("/")[-1]
            rule["remote_command"] = toolchain + compiler
            rule["remote"] = True
            rule["platform_ref"] = _PLATFORM
            rule["timeout"] = _TIMEOUT
            compiles.append(rule)
    if len(compiles) != len(_COMPILES):
        fail("project_b: expected Chromium's compile rules %s, found %s" %
             (_COMPILES, [r["name"] for r in compiles]))
    step_config["rules"] = compiles + step_config["rules"]

    step_config["platforms"][_PLATFORM] = _PLATFORM_PROPERTIES
    step_config["input_deps"].update(clang_all.input_deps(ctx))

    sdk = gn_logs.read(ctx).get("mac_sdk_path")
    if not sdk or not sdk.startswith("sdk/xcode_links/"):
        fail("project_b: the macOS SDK must be linked into the build directory " +
             "(mac_sdk_path=%s); remote builds need GN's system Xcode setup" % sdk)
    filegroups = dict(chromium.filegroups)
    filegroups[ctx.fs.canonpath("./" + sdk) + ":headers"] = {
        "type": "glob",
        "includes": _SDK_HEADERS,
    }

    print("project_b: compiles run on the Mac workers; everything else runs here")
    return module(
        "config",
        step_config = json.encode(step_config),
        filegroups = filegroups,
        handlers = chromium.handlers,
    )
