#!/usr/bin/env python3
"""After a Chromium bump, says what the web-features registry (core/webfeatures/registry.json)
is missing and what it names that Chromium no longer has. Reads the source tree only; no
build needed. docs/web-features.md.

    python3 tools/web_features_bump.py           # chromium/src
    python3 tools/web_features_bump.py --list    # and every binder and delegate method read

It lists the interfaces chrome binds for frames, service workers and the other worker
scopes content lets an embedder bind for, each with the platforms whose #if lets it in, and
the virtual methods of WebContentsDelegate and ContentBrowserClient. Then it reports:

  - an interface chrome binds and Blink's renderer asks for, that neither content nor our
    engine binds, and that no row names: a page that calls it here is killed;
  - a delegate method that is new since docs/content-hooks.md was last generated and that no
    row names: a person decides whether a page can tell;
  - what a row names that Chromium no longer has: an interface, a delegate method, a
    feature it turns off, a permission type.

Exits 1 when there is a missing interface or a gone name, so a bump can't pass it silently.
Run it before tools/gen_hooks.py, which rewrites the hooks table it compares with.
"""

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import web_features  # noqa: E402

ROOT = web_features.ROOT
PLATFORMS = ("mac", "android")

# Where chrome adds its binders, by the scope of ContentBrowserClient's
# RegisterBrowserInterfaceBindersFor<Scope>. Chrome's override for frames hands the map to
# PopulateChromeFrameBinders; the WebUI binders are for chrome:// pages, which we don't have.
CHROME_BINDERS = {
    "Frame": [("chrome/browser/chrome_browser_interface_binders.cc", "PopulateChromeFrameBinders")],
    "ServiceWorker": [("chrome/browser/chrome_content_browser_client_receiver_bindings.cc",
                       "RegisterBrowserInterfaceBindersForServiceWorker")],
}
CONTENT_BINDERS = "content/browser/browser_interface_binders.cc"
OUR_BINDERS = ["engine/browser/content_browser_client.cc", "engine/extensions/browser/content_hooks.cc"]
DELEGATES = {
    "WebContentsDelegate": "content/public/browser/web_contents_delegate.h",
    "ContentBrowserClient": "content/public/browser/content_browser_client.h",
    "BrowserContext": "content/public/browser/browser_context.h",
    "JavaScriptDialogManager": "content/public/browser/javascript_dialog_manager.h",
}
# The two whose new methods are reported.
LISTED_DELEGATES = ("WebContentsDelegate", "ContentBrowserClient")
RUNTIME_FEATURES = "third_party/blink/renderer/platform/runtime_enabled_features.json5"
PERMISSION_TYPES = "third_party/blink/public/common/permissions/permission_utils.h"
# Where the .mojom files that define the registry's interfaces live.
MOJOM_DIRS = ["third_party/blink/public", "components", "chrome/common", "content/public", "media", "services", "device"]
# Where base features are declared.
FEATURE_DIRS = ["content/public/common", "content/common", "third_party/blink/common", "services/network/public/cpp"]

# What a build flag is on each platform. Any other is taken as on.
BUILDFLAGS = {
    "mac": {"IS_MAC": True, "IS_APPLE": True, "IS_POSIX": True, "IS_ANDROID": False, "IS_DESKTOP_ANDROID": False,
            "ENABLE_VR": False},
    "android": {"IS_ANDROID": True, "IS_POSIX": True, "IS_MAC": False, "IS_APPLE": False, "ENABLE_EXTENSIONS": False,
                "ENABLE_EXTENSIONS_CORE": False, "ENABLE_PRINT_PREVIEW": False, "IS_DESKTOP_ANDROID": False},
}
OFF_EVERYWHERE = ("IS_WIN", "IS_LINUX", "IS_CHROMEOS", "IS_FUCHSIA", "IS_IOS")


def read(src, path):
    with open(os.path.join(src, path), errors="replace") as f:
        return f.read()


def lines_of(text):
    """The file's lines, with preprocessor lines continued by a backslash joined."""
    out = []
    for line in text.split("\n"):
        if out and out[-1].endswith("\\"):
            out[-1] = out[-1][:-1] + " " + line.strip()
        else:
            out.append(line)
    return out


def body_start(lines, func):
    """The index of the line where func's definition begins: its name, then its parameters,
    then a brace."""
    text = "\n".join(lines)
    for m in re.finditer(r"\b%s\s*\(" % re.escape(func), text):
        depth, i = 0, m.end() - 1
        while i < len(text):
            depth += {"(": 1, ")": -1}.get(text[i], 0)
            i += 1
            if depth == 0:
                break
        if re.match(r"\s*(const\s*)?(override\s*)?\{", text[i:]):
            return text.count("\n", 0, m.start())
    raise SystemExit("can't find %s's definition" % func)


def directive(stack, s):
    """Keeps stack, the #if conditions around a line, up to date with line s, and says
    whether s was a preprocessor line."""
    s = re.sub(r"^#\s+", "#", s)
    if s.startswith("#ifdef"):
        stack.append("defined(%s)" % s[6:].strip())
    elif s.startswith("#ifndef"):
        stack.append("!defined(%s)" % s[7:].strip())
    elif s.startswith("#if"):
        stack.append(s[3:].strip())
    elif s.startswith("#elif"):
        stack[-1] = "!(%s) && (%s)" % (stack[-1], s[5:].strip())
    elif s.startswith("#else"):
        stack[-1] = "!(%s)" % stack[-1]
    elif s.startswith("#endif"):
        stack.pop()
    return s.startswith("#")


def binders(lines, start=0, whole=False):
    """The interfaces added to a binder map from line start to the end of its function (or
    the file, with whole), as (C++ name, line, #if conditions, empty binder)."""
    found, stack, depth, began, statement, first = [], [], 0, False, "", 0
    for i in range(start, len(lines)):
        s = re.sub(r"//.*", "", lines[i]).strip()
        if directive(stack, s):
            continue
        depth += s.count("{") - s.count("}")
        began = began or "{" in s
        if not statement:
            first = i + 1
        statement += " " + s
        if ";" in s:
            for m in re.finditer(r"(?:map|registry)->Add(?:Interface)?<\s*([\w:]+)\s*>\s*\(\s*([^;]*)", statement):
                empty = bool(re.search(r"EmptyBinder|DoNothing|DropReceiver", m.group(2)))
                found.append((m.group(1), first, list(stack), empty))
        if ";" in s or s == "" or ("Add" not in statement and ("{" in s or "}" in s)):
            statement = ""
        if began and depth == 0 and not whole:
            break
    return found


def on(platform, conditions):
    """Whether #if conditions let a line in on platform."""
    def flag(m):
        name = m.group(1)
        if name in OFF_EVERYWHERE:
            return "False"
        return str(BUILDFLAGS[platform].get(name, True))
    for c in conditions:
        expr = re.sub(r"BUILDFLAG\((\w+)\)", flag, c)
        expr = re.sub(r"defined\s*\(\s*\w+\s*\)", "False", expr)
        expr = re.sub(r"//.*", "", expr).replace("&&", " and ").replace("||", " or ")
        expr = re.sub(r"!(?!=)", " not ", expr)
        try:
            if not eval(expr, {"__builtins__": {}}):
                return False
        except Exception:
            return True
    return True


def short(name):
    """An interface's own name, without its namespaces: blink::mojom::BadgeService and
    blink.mojom.BadgeService are both BadgeService."""
    return re.split(r"::|\.", name)[-1]


def dotted(name):
    return name.replace("::", ".")


def blink_asks(src):
    """The interfaces Blink's renderer names, by their own name."""
    names = set()
    for root, _, files in os.walk(os.path.join(src, "third_party/blink/renderer")):
        for n in files:
            if n.endswith((".cc", ".h")):
                with open(os.path.join(root, n), errors="replace") as f:
                    names.update(re.findall(r"mojom::(?:blink::)?(\w+)", f.read()))
    return names


def methods(src, header):
    """A class header's virtual methods, by name, each with the platforms whose #if lets it
    in."""
    found, stack, pending = {}, [], None
    for line in lines_of(read(src, header)):
        s = re.sub(r"//.*", "", line).strip()
        if directive(stack, s):
            continue
        if pending is not None or re.match(r"virtual\b", s):
            # The name is the word before the first ( outside a template's <>.
            pending = (pending or "") + " " + s
            angle = 0
            for i, c in enumerate(pending):
                angle += {"<": 1, ">": -1}.get(c, 0)
                if c == "(" and angle == 0:
                    name = re.search(r"(~?\w+)\s*$", pending[:i])
                    if name and not name.group(1).startswith("~"):
                        found[name.group(1)] = [p for p in PLATFORMS if on(p, stack)]
                    pending = None
                    break
            else:
                if ";" in s or "{" in s:
                    pending = None
    return found


def hooks_table():
    """The delegate methods in docs/content-hooks.md's table, by class."""
    known = {}
    with open(os.path.join(ROOT, "docs/content-hooks.md")) as f:
        for cls, method in re.findall(r"^\| `(\w+)` \| `(\w+)` \|", f.read(), re.M):
            known.setdefault(cls, set()).add(method)
    return known


def mojom_interfaces(src):
    names = set()
    for top in MOJOM_DIRS:
        for root, _, files in os.walk(os.path.join(src, top)):
            for n in files:
                if n.endswith(".mojom"):
                    with open(os.path.join(root, n), errors="replace") as f:
                        names.update(re.findall(r"^\s*interface\s+(\w+)", f.read(), re.M))
    return names


def base_features(src):
    names = set()
    for top in FEATURE_DIRS:
        for root, _, files in os.walk(os.path.join(src, top)):
            for n in files:
                if n.endswith(".cc"):
                    with open(os.path.join(root, n), errors="replace") as f:
                        for m in re.finditer(r"BASE_FEATURE\(\s*k(\w+)\s*,\s*(\"(\w+)\")?", f.read()):
                            names.add(m.group(3) or m.group(1))
    return names


def main():
    args = [a for a in sys.argv[1:] if a != "--list"]
    listing = "--list" in sys.argv[1:]
    src = args[0] if args else os.path.join(ROOT, "chromium/src")
    rows = web_features.load()
    named = {}
    for r in rows:
        if "interface" in r:
            named.setdefault(short(r["interface"]["name"]), []).append(r["id"])
    problems = 0

    # Interfaces.
    scopes = re.findall(r"virtual void RegisterBrowserInterfaceBindersFor(\w+)\(",
                        read(src, DELEGATES["ContentBrowserClient"]))
    ours = {short(n) for path in OUR_BINDERS for n, *_ in binders(lines_of(read(ROOT, path)), whole=True)}
    content = {short(n): empty for n, _, _, empty in binders(lines_of(read(src, CONTENT_BINDERS)), whole=True)}
    asks = blink_asks(src)
    missing = []
    print("Chrome's binders, by the scope content lets an embedder bind for:")
    for scope in scopes:
        if scope not in CHROME_BINDERS:
            print("  %s: a scope this check doesn't know; add where chrome binds for it to CHROME_BINDERS" % scope)
            problems += 1
            continue
        found = []
        for path, func in CHROME_BINDERS[scope]:
            lines = lines_of(read(src, path))
            found += [(n, "%s:%d" % (path, line), conds) for n, line, conds, _ in binders(lines, body_start(lines, func))]
        print("  %s: %d interfaces, from %s" % (scope, len(found), ", ".join(f for _, f in CHROME_BINDERS[scope])))
        for name, where, conds in found:
            plats = [p for p in PLATFORMS if on(p, conds)]
            s = short(name)
            state = ("registry: " + ", ".join(named[s]) if s in named else
                     "ours" if s in ours else
                     "content binds it" + (" (empty)" if content[s] else "") if s in content else
                     "Blink asks" if s in asks else "Blink doesn't ask")
            if listing:
                print("    %-60s %-12s %-26s %s" % (dotted(name), "+".join(plats) or "none", state, where))
            if plats and state == "Blink asks":
                missing.append((dotted(name), scope, plats, where))

    # Delegate methods.
    known = hooks_table()
    delegates = {cls: methods(src, h) for cls, h in DELEGATES.items()}
    in_rows = {r["delegate"] for r in rows if "delegate" in r}
    new = []
    print("\nDelegate methods (the hooks table is generated for the Mac, so only those the Mac has are compared):")
    for cls in LISTED_DELEGATES:
        mac = {m for m, plats in delegates[cls].items() if "mac" in plats}
        print("  %s: %d virtual methods, %d of them on the Mac" % (cls, len(delegates[cls]), len(mac)))
        if listing:
            for m, plats in sorted(delegates[cls].items()):
                print("    %-60s %s" % (m, "+".join(plats) or "none"))
        new += ["%s::%s" % (cls, m) for m in sorted(mac - known.get(cls, set()))
                if "%s::%s" % (cls, m) not in in_rows]

    # What the rows name.
    mojoms, runtime = mojom_interfaces(src), set(re.findall(r'name:\s*"(\w+)"', read(src, RUNTIME_FEATURES)))
    features, permissions = base_features(src), set(re.findall(r"^\s+(\w+)\s*=\s*\d+,", read(src, PERMISSION_TYPES), re.M))
    gone = []
    for r in rows:
        if "interface" in r and short(r["interface"]["name"]) not in mojoms:
            gone.append("%s: interface %s" % (r["id"], r["interface"]["name"]))
        if "delegate" in r:
            cls, method = r["delegate"].split("::", 1)
            if cls in delegates and method not in delegates[cls]:
                gone.append("%s: %s" % (r["id"], r["delegate"]))
            elif cls not in delegates:
                gone.append("%s: %s, a class this check doesn't read; add its header to DELEGATES" % (r["id"], r["delegate"]))
        gone += ["%s: Blink runtime feature %s" % (r["id"], f) for f in r.get("disable_blink_features", []) if f not in runtime]
        gone += ["%s: base feature %s" % (r["id"], f) for f in r.get("disable_features", []) if f not in features]
        gone += ["%s: permission type %s" % (r["id"], p) for p in r.get("permissions", []) if p not in permissions]

    print("\nChrome binds these, Blink asks for them, and nothing here binds them or names them in the registry:")
    for name, scope, plats, where in missing:
        print("  %s (%s, %s)  %s" % (name, scope, "+".join(plats), where))
    print("  none" if not missing else "  A page that asks for one is killed: add a row, and turn it off or give it an empty binder.")
    print("\nDelegate methods new since docs/content-hooks.md, in no row:")
    for m in new:
        print("  " + m)
    print("  none" if not new else "  Add a row for each that a page can tell.")
    print("\nWhat the registry names that Chromium no longer has:")
    for g in gone:
        print("  " + g)
    print("  none" if not gone else "  Fix the row.")
    return 1 if missing or gone or problems else 0


if __name__ == "__main__":
    sys.exit(main())
