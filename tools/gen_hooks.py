#!/usr/bin/env python3
"""Reads content's hook classes with Chromium's own clang and writes the code that
carries them to Go. Run it after a Chromium upgrade; the output is checked in:

    engine/browser/hooks_gen.h, hooks_gen.cc   C++ classes overriding every hook Go can
                                               carry, one bit check each
    core/content/*_gen.go                      hook numbers, typed Go functions, structs
    docs/content-hooks.md                      one line per hook, with the counts

A hook is carried when every argument is plain data (numbers, strings, enums) or an
object with a copier in COPIERS below, and its return is void or plain. Everything else
is left, with the reason in the doc. Needs the build in chromium/src/out/pb for the
compile flags.
"""

import concurrent.futures
import hashlib
import json
import os
import re
import shlex
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
SRC = os.path.join(ROOT, "chromium/src")
OUT = os.path.join(SRC, "out/pb")
WORK = os.path.join(ROOT, "build/gen_hooks")
CLANG = os.path.join(SRC, "third_party/llvm-build/Release+Asserts/bin")

# The classes whose hooks go to Go. `scope` says what the first argument of every hook
# is: the tab's WebContents for a tab class, the navigation for the throttle, nothing
# for the global client.
CLASSES = [
    ("content::WebContentsObserver", "content/public/browser/web_contents_observer.h", "Observer", "observer"),
    ("content::WebContentsDelegate", "content/public/browser/web_contents_delegate.h", "Delegate", "delegate"),
    ("content::ContentBrowserClient", "content/public/browser/content_browser_client.h", "Browser", "global"),
    ("content::NavigationThrottle", "content/public/browser/navigation_throttle.h", "Throttle", "throttle"),
]
# Listed in the doc only: Go is deliberately absent from the processes these run in.
OTHER_PROCESSES = [
    ("content::ContentRendererClient", "content/public/renderer/content_renderer_client.h", "renderer"),
    ("content::ContentUtilityClient", "content/public/utility/content_utility_client.h", "utility"),
]
FIRST_HOOK = 100  # Below this are the adapter's own events, PBEvent in pb_abi.h.

# Plain types: one value each. (kind, C++ expression that yields the value).
PLAIN = {
    "bool": ("bool", "{v}"),
    "int": ("int", "{v}"), "unsigned int": ("int", "{v}"), "long": ("int", "{v}"),
    "unsigned long": ("int", "{v}"), "int32_t": ("int", "{v}"), "uint32_t": ("int", "{v}"),
    "int64_t": ("int", "{v}"), "uint64_t": ("int", "{v}"), "size_t": ("int", "{v}"),
    "double": ("double", "{v}"), "float": ("double", "{v}"),
    "std::string": ("str", "{v}"), "std::string_view": ("str", "std::string({v})"),
    "std::u16string": ("str", "base::UTF16ToUTF8({v})"),
    "std::u16string_view": ("str", "base::UTF16ToUTF8({v})"),
    "GURL": ("str", "{v}.spec()"), "url::Origin": ("str", "{v}.Serialize()"),
    "base::FilePath": ("str", "{v}.value()"),
    "base::TimeTicks": ("int", "{v}.since_origin().InMicroseconds()"),
    "base::Time": ("int", "({v} - base::Time::UnixEpoch()).InMicroseconds()"),
    "base::TimeDelta": ("int", "{v}.InMicroseconds()"),
    "net::SiteForCookies": ("str", "{v}.RepresentativeUrl().spec()"),
    "net::CookieSettingOverrides": ("int", "{v}.ToEnumBitmask()"),
}
GO_PLAIN = {"bool": "bool", "int": "int64", "double": "float64", "str": "string"}

# Objects that cross as a struct copy. Each field is (Go name, type, C++ expression on
# `v`), where the type is a plain kind, an enum, or another copier. `ptr` says the
# C++ side holds a pointer. The copy is what Go can know about the object; Go never
# holds the object itself, because content may destroy it after the hook returns.
COPIERS = {
    "content::WebContents": dict(ptr=True, include="content/public/browser/web_contents.h", fields=[
        # The tab's id, or 0 for the shell and for a WebContents that isn't a tab.
        ("ID", "int", "TabOf(v)"),
        ("URL", "str", "v->GetLastCommittedURL().spec()"),
        # The page's own title, empty when it has none; GetTitle would put the URL there.
        ("Title", "str", "base::UTF16ToUTF8(v->GetController().GetLastCommittedEntry() ? v->GetController().GetLastCommittedEntry()->GetTitle() : std::u16string())"),
        ("CanGoBack", "bool", "v->GetController().CanGoBack()"),
        ("CanGoForward", "bool", "v->GetController().CanGoForward()"),
        ("Loading", "bool", "v->IsLoading()"),
        ("Visibility", "content::Visibility", "v->GetVisibility()"),
    ]),
    "content::NavigationHandle": dict(ptr=True, include="content/public/browser/navigation_handle.h", fields=[
        ("TabID", "int", "TabOf(v->GetWebContents())"),
        ("URL", "str", "v->GetURL().spec()"),
        ("NavigationID", "int", "v->GetNavigationId()"),
        ("IsInPrimaryMainFrame", "bool", "v->IsInPrimaryMainFrame()"),
        ("IsInMainFrame", "bool", "v->IsInMainFrame()"),
        ("IsSameDocument", "bool", "v->IsSameDocument()"),
        ("HasCommitted", "bool", "v->HasCommitted()"),
        ("IsErrorPage", "bool", "v->IsErrorPage()"),
        ("NetError", "int", "v->GetNetErrorCode()"),
        ("NetErrorName", "str", "v->GetNetErrorCode() ? net::ErrorToString(v->GetNetErrorCode()) : std::string()"),
        ("ResponseCode", "int", "v->GetResponseHeaders() ? v->GetResponseHeaders()->response_code() : 0"),
        ("IsRendererInitiated", "bool", "v->IsRendererInitiated()"),
        ("HasUserGesture", "bool", "v->HasUserGesture()"),
        ("WasServerRedirect", "bool", "v->WasServerRedirect()"),
        ("IsPost", "bool", "v->IsPost()"),
        ("IsDownload", "bool", "v->IsDownload()"),
        ("IsServedFromBackForwardCache", "bool", "v->IsServedFromBackForwardCache()"),
        ("PageTransition", "ui::PageTransition", "v->GetPageTransition()"),
    ]),
    "content::RenderFrameHost": dict(ptr=True, include="content/public/browser/render_frame_host.h", fields=[
        ("TabID", "int", "TabOf(content::WebContents::FromRenderFrameHost(v))"),
        ("FrameTreeNodeID", "int", "v->GetFrameTreeNodeId().value()"),
        ("RoutingID", "int", "v->GetRoutingID()"),
        ("ProcessID", "int", "v->GetProcess()->GetDeprecatedID()"),
        ("IsInPrimaryMainFrame", "bool", "v->IsInPrimaryMainFrame()"),
        ("IsActive", "bool", "v->IsActive()"),
        ("URL", "str", "v->GetLastCommittedURL().spec()"),
        ("Origin", "str", "v->GetLastCommittedOrigin().Serialize()"),
    ]),
    "content::RenderWidgetHost": dict(ptr=True, include="content/public/browser/render_widget_host.h", fields=[
        ("RoutingID", "int", "v->GetRoutingID()"),
        ("ProcessID", "int", "v->GetProcess()->GetDeprecatedID()"),
    ]),
    "content::RenderViewHost": dict(ptr=True, include="content/public/browser/render_view_host.h", fields=[
        ("RoutingID", "int", "v->GetRoutingID()"),
        ("ProcessID", "int", "v->GetProcess()->GetDeprecatedID()"),
    ]),
    "blink::mojom::FullscreenOptions": dict(include="third_party/blink/public/mojom/frame/fullscreen.mojom.h", fields=[
        ("PrefersNavigationBar", "bool", "v.prefers_navigation_bar"),
        ("PrefersStatusBar", "bool", "v.prefers_status_bar"),
    ]),
    "content::RenderProcessHost": dict(ptr=True, include="content/public/browser/render_process_host.h", fields=[
        ("ID", "int", "v->GetDeprecatedID()"),
        ("IsReady", "bool", "v->IsReady()"),
    ]),
    "content::NavigationEntry": dict(ptr=True, include="content/public/browser/navigation_entry.h", fields=[
        ("UniqueID", "int", "v->GetUniqueID()"),
        ("URL", "str", "v->GetURL().spec()"),
        ("VirtualURL", "str", "v->GetVirtualURL().spec()"),
        ("Title", "str", "base::UTF16ToUTF8(v->GetTitle())"),
        ("Transition", "ui::PageTransition", "v->GetTransitionType()"),
        ("HasPostData", "bool", "v->GetHasPostData()"),
        ("Timestamp", "base::Time", "v->GetTimestamp()"),
    ]),
    # What a long press or a right click in a page was on: a link, an image, a selection.
    "content::ContextMenuParams": dict(include="content/public/browser/context_menu_params.h", fields=[
        ("X", "int", "v.x"),
        ("Y", "int", "v.y"),
        ("PageURL", "str", "v.page_url.spec()"),
        ("LinkURL", "str", "v.link_url.spec()"),
        ("LinkText", "str", "base::UTF16ToUTF8(v.link_text)"),
        ("SrcURL", "str", "v.src_url.spec()"),
        # blink::mojom::ContextMenuDataMediaType: 0 none, 1 image, 2 video, 3 audio, ...
        ("MediaType", "int", "static_cast<int>(v.media_type)"),
        ("SelectionText", "str", "base::UTF16ToUTF8(v.selection_text)"),
        ("IsEditable", "bool", "v.is_editable"),
    ]),
    "content::SiteInstance": dict(ptr=True, include="content/public/browser/site_instance.h", fields=[
        ("ID", "int", "v->GetId().value()"),
    ]),
    "content::BrowserContext": dict(ptr=True, include="content/public/browser/browser_context.h", fields=[
        ("IsOffTheRecord", "bool", "v->IsOffTheRecord()"),
        ("Path", "str", "v->GetPath().value()"),
    ]),
    "content::Page": dict(ptr=True, include="content/public/browser/page.h", fields=[
        ("MainDocument", "content::RenderFrameHost", "&v->GetMainDocument()"),
    ]),
    "content::LoadCommittedDetails": dict(include="content/public/browser/navigation_details.h", fields=[
        ("Entry", "content::NavigationEntry", "v.entry"),
        ("PreviousMainFrameURL", "str", "v.previous_main_frame_url.spec()"),
        ("IsMainFrame", "bool", "v.is_main_frame"),
        ("DidReplaceEntry", "bool", "v.did_replace_entry"),
        ("IsSameDocument", "bool", "v.is_same_document"),
    ]),
    "content::EntryChangedDetails": dict(include="content/public/browser/navigation_details.h", fields=[
        ("ChangedEntry", "content::NavigationEntry", "v.changed_entry"),
        ("Index", "int", "v.index"),
    ]),
    "content::PrunedDetails": dict(include="content/public/browser/navigation_details.h", fields=[
        ("Index", "int", "v.index"),
        ("Count", "int", "v.count"),
    ]),
    "content::MediaPlayerId": dict(include="content/public/browser/media_player_id.h", fields=[
        ("ProcessID", "int", "v.frame_routing_id.child_id.value()"),
        ("FrameRoutingID", "int", "v.frame_routing_id.frame_routing_id"),
        ("PlayerID", "int", "v.player_id"),
    ]),
    "content::WebContentsObserver::MediaPlayerInfo": dict(fields=[
        ("HasVideo", "bool", "v.has_video"),
        ("HasAudio", "bool", "v.has_audio"),
    ]),
    "content::OpenURLParams": dict(include="content/public/browser/page_navigator.h", fields=[
        ("URL", "str", "v.url.spec()"),
        ("ReferrerURL", "str", "v.referrer.url.spec()"),
        ("FrameTreeNodeID", "int", "v.frame_tree_node_id.value()"),
        ("Disposition", "WindowOpenDisposition", "v.disposition"),
        ("Transition", "ui::PageTransition", "v.transition"),
        ("IsRendererInitiated", "bool", "v.is_renderer_initiated"),
        ("UserGesture", "bool", "v.user_gesture"),
    ]),
    "content::Referrer": dict(include="content/public/common/referrer.h", fields=[
        ("URL", "str", "v.url.spec()"),
        ("Policy", "network::mojom::ReferrerPolicy", "v.policy"),
    ]),
    "content::FocusedNodeDetails": dict(include="content/public/browser/focused_node_details.h", fields=[
        ("EditableLevel", "content::EditableLevel", "v.editable_level"),
        ("BoundsInScreen", "gfx::Rect", "v.node_bounds_in_screen"),
        ("FocusType", "blink::mojom::FocusType", "v.focus_type"),
    ]),
    "content::CookieAccessDetails": dict(include="content/public/browser/cookie_access_details.h", fields=[
        ("Type", "network::mojom::CookieAccessDetails::Type", "v.type"),
        ("URL", "str", "v.url.spec()"),
        ("FirstPartyURL", "str", "v.first_party_url.spec()"),
        ("Count", "int", "v.cookie_access_result_list.size()"),
        ("BlockedByPolicy", "bool", "v.blocked_by_policy"),
        ("IsAdTagged", "bool", "v.is_ad_tagged"),
        ("Source", "content::CookieAccessDetails::Source", "v.source"),
    ]),
    "content::GlobalRequestID": dict(include="content/public/browser/global_request_id.h", fields=[
        ("ChildID", "int", "v.child_id.GetUnsafeValue()"),
        ("RequestID", "int", "v.request_id"),
    ]),
    "content::GlobalRenderFrameHostId": dict(include="content/public/browser/global_routing_id.h", fields=[
        ("ChildID", "int", "v.child_id.value()"),
        ("FrameRoutingID", "int", "v.frame_routing_id"),
    ]),
    "input::NativeWebKeyboardEvent": dict(include=["components/input/native_web_keyboard_event.h", "ui/events/keycodes/keyboard_codes.h"], fields=[
        ("Type", "blink::WebInputEvent::Type", "v.GetType()"),
        ("Modifiers", "blink::WebInputEvent::Modifiers", "v.GetModifiers()"),
        ("WindowsKeyCode", "ui::KeyboardCode", "static_cast<ui::KeyboardCode>(v.windows_key_code)"),
        ("NativeKeyCode", "int", "v.native_key_code"),
        ("DomCode", "int", "v.dom_code"),
        ("Text", "str", "base::UTF16ToUTF8(std::u16string(v.text.data()))"),
        ("IsSystemKey", "bool", "v.is_system_key"),
        ("SkipIfUnhandled", "bool", "v.skip_if_unhandled"),
    ]),
    "blink::WebInputEvent": dict(include="third_party/blink/public/common/input/web_input_event.h", fields=[
        ("Type", "blink::WebInputEvent::Type", "v.GetType()"),
        ("Modifiers", "blink::WebInputEvent::Modifiers", "v.GetModifiers()"),
        ("TimeStamp", "base::TimeTicks", "v.TimeStamp()"),
    ]),
    "blink::mojom::WindowFeatures": dict(include="third_party/blink/public/mojom/window_features/window_features.mojom.h", fields=[
        ("Bounds", "gfx::Rect", "v.bounds"),
        ("HasX", "bool", "v.has_x"), ("HasY", "bool", "v.has_y"),
        ("HasWidth", "bool", "v.has_width"), ("HasHeight", "bool", "v.has_height"),
        ("IsPopup", "bool", "v.is_popup"),
    ]),
    "blink::mojom::ResourceLoadInfo": dict(include="third_party/blink/public/mojom/loader/resource_load_info.mojom.h", fields=[
        ("URL", "str", "v.final_url.spec()"),
        ("OriginalURL", "str", "v.original_url.spec()"),
        ("Referrer", "str", "v.referrer.spec()"),
        ("Method", "str", "v.method"),
        ("MimeType", "str", "v.mime_type"),
        ("Destination", "network::mojom::RequestDestination", "v.request_destination"),
        ("HTTPStatusCode", "int", "v.http_status_code"),
        ("NetError", "int", "v.net_error"),
        ("WasCached", "bool", "v.was_cached"),
        ("TotalReceivedBytes", "int", "v.total_received_bytes.InBytes()"),
        ("RawBodyBytes", "int", "v.raw_body_bytes.InBytes()"),
    ]),
    "network::ResourceRequest": dict(include="services/network/public/cpp/resource_request.h", fields=[
        ("URL", "str", "v.url.spec()"),
        ("Method", "str", "v.method"),
        ("Referrer", "str", "v.referrer.spec()"),
        ("Destination", "network::mojom::RequestDestination", "v.destination"),
        ("HasUserGesture", "bool", "v.has_user_gesture"),
        ("IsOutermostMainFrame", "bool", "v.is_outermost_main_frame"),
    ]),
    "blink::StorageKey": dict(include="third_party/blink/public/common/storage_key/storage_key.h", fields=[
        ("Origin", "str", "v.origin().Serialize()"),
        ("TopLevelSite", "str", "v.top_level_site().Serialize()"),
    ]),
    "gfx::Size": dict(include="ui/gfx/geometry/size.h", fields=[("Width", "int", "v.width()"), ("Height", "int", "v.height()")]),
    "gfx::Point": dict(include="ui/gfx/geometry/point.h", fields=[("X", "int", "v.x()"), ("Y", "int", "v.y()")]),
    "gfx::Rect": dict(include="ui/gfx/geometry/rect.h", fields=[("X", "int", "v.x()"), ("Y", "int", "v.y()"), ("Width", "int", "v.width()"), ("Height", "int", "v.height()")]),
    "gfx::SizeF": dict(include="ui/gfx/geometry/size_f.h", fields=[("Width", "double", "v.width()"), ("Height", "double", "v.height()")]),
    "gfx::PointF": dict(include="ui/gfx/geometry/point_f.h", fields=[("X", "double", "v.x()"), ("Y", "double", "v.y()")]),
    "gfx::RectF": dict(include="ui/gfx/geometry/rect_f.h", fields=[("X", "double", "v.x()"), ("Y", "double", "v.y()"), ("Width", "double", "v.width()"), ("Height", "double", "v.height()")]),
}
# Headers the copiers' field types come from, when the class headers don't bring them.
PROBE_INCLUDES = ["ui/events/keycodes/keyboard_codes.h", "content/public/browser/keyboard_event_processing_result.h", "content/public/browser/site_isolation_mode.h"]
# Return types that are classes but carry as one enum value.
RETURN_AS_ENUM = {
    "content::NavigationThrottle::ThrottleCheckResult": "content::NavigationThrottle::ThrottleAction",
}
# Types written without a namespace that are global, not content's.
GLOBAL_TYPES = {"GURL", "SkBitmap", "SkColor", "SkRegion", "WindowOpenDisposition"}
# Non-hook virtuals the throttle must implement by hand, and anything else to skip.
SKIP_METHODS = {"GetNameForLogging"}

CPP_KEYWORDS = {"default", "new", "delete", "class", "template", "export", "register"}


def run(cmd, **kw):
    r = subprocess.run(cmd, capture_output=True, text=True, errors="replace", **kw)
    if r.returncode != 0:
        errors = [l for l in r.stderr.splitlines() if "error:" in l]
        raise SystemExit(f"{cmd[0]} failed:\n" + "\n".join(errors[:30] or r.stderr.splitlines()[-30:]))
    return r


def copier_includes(names=None):
    out = set()
    for name, c in COPIERS.items():
        if names is not None and name not in names:
            continue
        inc = c.get("include")
        out |= set([inc] if isinstance(inc, str) else inc or [])
    return out


def clang_args():
    """The compile flags the build uses, taken from one of our own objects."""
    env = dict(os.environ, PATH=os.path.join(ROOT, "depot_tools") + ":" + os.environ["PATH"])
    line = run(["ninja", "-t", "commands", "-s", "obj/project_b/engine/browser/page_tab.o"], cwd=OUT, env=env).stdout.strip().splitlines()[-1]
    words = shlex.split(line)
    keep, skip = [], False
    for w in words:
        if skip:
            skip = False
        elif w in ("-MMD", "-c") or w.endswith("page_tab.cc"):
            pass
        elif w in ("-MF", "-o"):
            skip = True
        else:
            keep.append(w)
    return keep


def dump(args, probe, filt):
    """clang's AST as JSON for every declaration whose qualified name contains filt.
    Each run takes seconds, so the raw output is kept under build/gen_hooks/cache;
    delete that folder after changing the probe or Chromium."""
    probe_hash = hashlib.sha256(open(probe, "rb").read()).hexdigest()[:8]
    cache = os.path.join(WORK, "cache", f"{os.path.basename(probe)}.{probe_hash}." + re.sub(r"[^\w]", "_", filt) + ".json")
    if os.path.exists(cache):
        raw = open(cache).read()
    else:
        r = run(args + ["-fsyntax-only", "-Xclang", "-ast-dump=json", "-Xclang", "-ast-dump-filter=" + filt, probe], cwd=OUT)
        raw = r.stdout
        os.makedirs(os.path.dirname(cache), exist_ok=True)
        with open(cache, "w") as f:
            f.write(raw)
    dec = json.JSONDecoder()
    out, i = [], 0
    while i < len(raw):
        while i < len(raw) and raw[i].isspace():
            i += 1
        if i >= len(raw):
            break
        o, i = dec.raw_decode(raw, i)
        out.append(o)
    return out


def demangle(names):
    """Fully qualified signatures from mangled names. Given as arguments in batches:
    llvm-cxxfilt stalls on a large stdin."""
    out = {}
    for i in range(0, len(names), 100):
        batch = names[i:i + 100]
        r = run([os.path.join(CLANG, "llvm-cxxfilt")] + batch)
        out.update(zip(batch, r.stdout.splitlines()))
    return out


def split_params(sig):
    """Splits a demangled 'ns::Class::Method(A, B<C, D>)' into its parameter types."""
    inner = sig[sig.index("(") + 1:sig.rindex(")")]
    parts, depth, cur = [], 0, ""
    for ch in inner:
        if ch in "<(":
            depth += 1
        elif ch in ">)":
            depth -= 1
        if ch == "," and depth == 0:
            parts.append(cur.strip())
            cur = ""
        else:
            cur += ch
    if cur.strip():
        parts.append(cur.strip())
    return parts


def strip_cvref(t):
    t = t.strip()
    for prefix in ("const ", "volatile "):
        if t.startswith(prefix):
            t = t[len(prefix):]
    t = re.sub(r"\s*(const)?\s*[&*]+$", "", t).strip()
    t = re.sub(r"\s+const$", "", t).strip()
    return t


def qualify(written, demangled):
    """Fully qualifies a type as the header wrote it, using the demangled signature to
    learn which namespace an unqualified name came from."""
    def fix(m):
        path = m.group(0)
        head = path.split("::")[0]
        if head in GLOBAL_TYPES or head in ("std", "base", "blink", "net", "url", "gfx", "ui", "network", "mojo", "media", "device", "viz", "input", "content", "storage", "download", "cc", "optimization_guide", "sandbox", "attribution_reporting", "web_pref", "printing", "webauthn", "webid", "blink_mojom", "mojo_base", "privacy_sandbox", "permissions", "font_access", "payments", "cert_verifier", "error_page", "digital_identity", "service_manager", "sync_pb", "ukm", "origin_trials", "speech", "on_device_model", "site_isolation", "web_app", "ax", "components") or not head[:1].isupper():
            return path
        # The demangled signature says which namespace, or enclosing class, the
        # header meant.
        found = re.search(r"\b((?:[A-Za-z_]\w*::)+)" + re.escape(path) + r"\b", demangled or "")
        if found:
            return found.group(1) + path
        return "content::" + path
    out = re.sub(r"\b[A-Za-z_]\w*(::[A-Za-z_]\w*)*\b(?![\w:]*\()", fix, written)
    return out.replace(" *", "*").replace(" &", "&")


class Type:
    """One type as a hook declares it, classified for carrying."""

    def __init__(self, written, desugared, demangled, enums, records):
        self.written = written
        self.qualified = qualify(written, demangled)
        self.base = strip_cvref(self.qualified)
        self.is_pointer = written.rstrip().endswith("*")
        self.is_ref = written.rstrip().endswith("&")
        self.is_const = written.startswith("const ")
        self.kind = None      # plain kind, "enum", "copier", "optional", "vector"
        self.inner = None     # for optional and vector
        self.reason = None    # why it can't be carried
        self.classify(desugared, enums, records)

    def classify(self, desugared, enums, records):
        b = self.base
        if re.search(r"Callback|Closure", b):
            self.reason = "callback"
        elif b.startswith("mojo::") or "mojo::" in b or re.search(r"Pending(Receiver|Remote|AssociatedReceiver|AssociatedRemote)", b):
            self.reason = "mojo"
        elif re.search(r"unique_ptr|scoped_refptr|WeakPtr|unique_ref|raw_ptr", b):
            self.reason = "ownership"
        elif b in PLAIN:
            self.kind = PLAIN[b][0]
            if self.is_pointer:
                self.reason = "out-param"
        elif b in GO_PLAIN:
            # A copier field written as a plain kind: the expression is the value.
            self.kind = b
        elif b in enums:
            self.kind = "enum"
            if self.is_pointer:
                self.reason = "out-param"
        elif b in COPIERS:
            self.kind = "copier"
            if self.is_ref and not self.is_const and not COPIERS[b].get("ptr"):
                self.reason = "mutable reference"
        elif b.startswith("std::optional<") or b.startswith("std::vector<"):
            inner = Type(b[b.index("<") + 1:-1], None, None, enums, records)
            self.inner = inner
            self.kind = "optional" if b.startswith("std::optional<") else "vector"
            if inner.reason:
                self.reason = inner.reason
            elif inner.kind not in GO_PLAIN and inner.kind != "enum":
                self.reason = "vector of objects" if self.kind == "vector" else "optional object"
            elif self.is_pointer:
                self.reason = "out-param"
        elif desugared and desugared.startswith("base::IdType<"):
            self.kind = "int"
            self.id_type = True
        elif desugared and desugared != self.written and strip_cvref(desugared) in PLAIN:
            self.kind = PLAIN[strip_cvref(desugared)][0]
            self.desugared = strip_cvref(desugared)
        else:
            self.reason = "no copier for " + b

    def go_type(self):
        if self.kind in GO_PLAIN:
            return GO_PLAIN[self.kind]
        if self.kind == "enum":
            return go_enum_name(self.base)
        if self.kind == "copier":
            return go_struct_name(self.base)
        if self.kind == "optional":
            return "Optional[" + self.inner.go_type() + "]"
        if self.kind == "vector":
            return "[]" + self.inner.go_type()
        raise ValueError(self.written)


def go_enum_name(qualified):
    parts = [p for p in qualified.split("::") if p not in ("content", "blink", "mojom", "ui", "net", "network", "url", "base", "gfx", "media", "device", "input", "viz", "storage")]
    return "".join(parts)


def go_struct_name(qualified):
    parts = [p for p in qualified.split("::") if p not in ("content", "blink", "mojom", "input", "network", "gfx")]
    if qualified.startswith("gfx::"):
        parts = ["Gfx"] + parts
    return "".join(parts)


def pack_lines(t, expr, indent="  "):
    """C++ statements that add one value of type t, read from expr, to args."""
    e = expr
    if t.kind in GO_PLAIN:
        plain = PLAIN.get(t.base) or PLAIN.get(getattr(t, "desugared", ""))
        if getattr(t, "id_type", False):
            value = e + ".value()"
        else:
            value = plain[1].format(v=e) if plain else e
        return [f"{indent}args.{t.kind.capitalize()}({value});"]
    if t.kind == "enum":
        return [f"{indent}args.Int(static_cast<int64_t>({e}));"]
    if t.kind == "copier":
        return [f"{indent}hooks::Pack(args, {'&' if t.is_ref and COPIERS[t.base].get('ptr') else ''}{e});"]
    if t.kind == "optional":
        inner = t.inner
        lines = [f"{indent}args.Bool({e}.has_value());"]
        if inner.kind == "enum":
            lines.append(f"{indent}args.Int({e} ? static_cast<int64_t>(*{e}) : 0);")
        else:
            zero = {"bool": "false", "int": "0", "double": "0", "str": "std::string()"}[inner.kind]
            plain = PLAIN.get(inner.base)
            value = plain[1].format(v=f"(*{e})") if plain else f"(*{e})"
            lines.append(f"{indent}args.{inner.kind.capitalize()}({e} ? {value} : {zero});")
        return lines
    if t.kind == "vector":
        lines = [f"{indent}args.Int(static_cast<int64_t>({e}.size()));", f"{indent}for (const auto& item : {e}) {{"]
        lines += pack_lines(t.inner, "item", indent + "  ")
        lines.append(f"{indent}}}")
        return lines
    raise ValueError(t.written)


def read_expr(t):
    """The Go expression that reads one value of type t from the reader r."""
    if t.kind in GO_PLAIN:
        return {"bool": "r.bool()", "int": "r.int()", "double": "r.float()", "str": "r.str()"}[t.kind]
    if t.kind == "enum":
        return f"{go_enum_name(t.base)}(r.int())"
    if t.kind == "copier":
        return f"r.{lower_first(go_struct_name(t.base))}()"
    if t.kind == "optional":
        return f"readOptional(r, func() {t.inner.go_type()} {{ return {read_expr(t.inner)} }})"
    if t.kind == "vector":
        return f"readVector(r, func() {t.inner.go_type()} {{ return {read_expr(t.inner)} }})"
    raise ValueError(t.written)


def lower_first(s):
    return s[0].lower() + s[1:]


def camel(snake):
    parts = [p for p in snake.split("_") if p]
    if not parts:
        return "arg"
    out = parts[0] + "".join(p[:1].upper() + p[1:] for p in parts[1:])
    if out in ("type", "func", "map", "range", "string", "len", "cap", "new", "make", "r", "h", "ok", "v", "c", "f", "tab", "navigation"):
        out += "_"
    return out


def go_const_name(enum_go, enumerator, common_prefix):
    name = enumerator
    if common_prefix and name.startswith(common_prefix) and len(name) > len(common_prefix):
        name = name[len(common_prefix):]
    if name.startswith("k") and len(name) > 1 and name[1].isupper():
        name = name[1:]
    if "_" in name or name.isupper():
        name = "".join(p[:1].upper() + p[1:].lower() for p in name.split("_") if p)
    else:
        name = name[:1].upper() + name[1:]
    return enum_go + name


class Method:
    def __init__(self, cls, node, header_lines, demangled):
        self.cls = cls
        self.name = node["name"]
        sig = node["type"]["qualType"]
        self.ret_written = sig[:sig.index("(")].strip()
        self.is_const = sig.rstrip().endswith("const")
        self.pure = bool(node.get("pure"))
        self.demangled = demangled
        self.params = [p for p in node.get("inner", []) if p.get("kind") == "ParmVarDecl"]
        start = node["range"]["begin"].get("offset", node["loc"].get("offset"))
        self.line = header_lines.offset_to_line(start)
        self.comment = header_lines.comment_above(self.line)
        self.guard = header_lines.guard(self.line)
        self.signature = sig

    def key(self):
        return (self.name, self.signature)


class HeaderLines:
    def __init__(self, path):
        self.text = open(path, encoding="utf-8").read()
        self.lines = self.text.split("\n")
        self.starts = [0]
        for line in self.lines[:-1]:
            self.starts.append(self.starts[-1] + len(line) + 1)

    def offset_to_line(self, offset):
        lo, hi = 0, len(self.starts) - 1
        while lo < hi:
            mid = (lo + hi + 1) // 2
            if self.starts[mid] <= offset:
                lo = mid
            else:
                hi = mid - 1
        return lo + 1

    def guard(self, line):
        """The preprocessor condition the header declares this line under, "" for none, so
        a hook that only some platforms have is only overridden there. The header's own
        include guard doesn't count."""
        stack = []  # One list of conditions per open #if: the branches before, negated, and this one.
        for text in self.lines[:line - 1]:
            m = re.match(r"\s*#\s*(if|ifdef|ifndef|elif|else|endif)\b\s*(.*?)\s*(//.*)?$", text)
            if not m:
                continue
            word, cond = m.group(1), m.group(2)
            if word == "ifdef":
                cond = f"defined({cond})"
            elif word == "ifndef":
                cond = "" if cond.endswith("_H_") else f"!defined({cond})"
            if word in ("if", "ifdef", "ifndef"):
                stack.append([cond] if cond else [])
            elif word == "elif":
                prev = stack[-1]
                stack[-1] = [f"!({c})" for c in prev[-1:]] + prev[:-1] + [cond]
            elif word == "else":
                prev = stack[-1]
                stack[-1] = [f"!({c})" for c in prev[-1:]] + prev[:-1]
            elif word == "endif":
                stack.pop()
        conds = [c for branch in stack for c in branch]
        return " && ".join(f"({c})" if "||" in c and len(conds) > 1 else c for c in conds)

    def comment_above(self, line):
        out = []
        i = line - 2
        while i >= 0 and self.lines[i].strip().startswith("//"):
            out.insert(0, self.lines[i].strip()[2:].strip())
            i -= 1
        return " ".join(out)


OFF_MAIN = re.compile(r"\b(IO thread|on the IO|from the IO|any thread|ThreadPool|thread pool|background thread|worker thread|UI or IO|IO or UI|a thread other than)\b", re.I)


def main():
    os.makedirs(WORK, exist_ok=True)
    args = clang_args()
    probe = os.path.join(WORK, "probe.cc")
    with open(probe, "w") as f:
        for _, header, _, _ in CLASSES:
            f.write(f'#include "{header}"\n')
        for _, header, _ in OTHER_PROCESSES:
            f.write(f'#include "{header}"\n')
        for inc in sorted(copier_includes() | set(PROBE_INCLUDES)):
            f.write(f'#include "{inc}"\n')
    version = open(os.path.join(SRC, "chrome/VERSION")).read()
    version = ".".join(re.findall(r"=(\d+)", version))

    # Pass 1: the classes.
    pool = concurrent.futures.ThreadPoolExecutor(16)
    names = [c[0] for c in CLASSES] + [c[0] for c in OTHER_PROCESSES]
    dumps = dict(zip(names, pool.map(lambda n: dump(args, probe, n), names)))

    def class_decl(nodes, qualified):
        short = qualified.split("::")[-1]
        for n in nodes:
            if n.get("kind") == "CXXRecordDecl" and n.get("name") == short and n.get("completeDefinition"):
                return n
        raise SystemExit("no definition for " + qualified)

    methods = {}
    all_mangled = []
    for qualified, header, go, scope in CLASSES + [(c[0], c[1], None, c[2]) for c in OTHER_PROCESSES]:
        decl = class_decl(dumps[qualified], qualified)
        nodes = [d for d in decl.get("inner", []) if d.get("kind") == "CXXMethodDecl" and d.get("virtual")]
        methods[qualified] = (nodes, HeaderLines(os.path.join(SRC, header)))
        all_mangled += [n["mangledName"] for n in nodes if "mangledName" in n]
    demangled = demangle(all_mangled)

    classes = []
    for qualified, header, go, scope in CLASSES:
        nodes, lines = methods[qualified]
        ms = [Method(qualified, n, lines, demangled.get(n.get("mangledName"), "")) for n in nodes]
        ms = [m for m in ms if m.name not in SKIP_METHODS]
        ms.sort(key=Method.key)
        classes.append((qualified, header, go, scope, ms))

    # Pass 2: what the unknown types are. One clang run per type, in parallel.
    seen = {}
    for _, _, _, _, ms in classes:
        for m in ms:
            dparams = split_params(m.demangled) if m.demangled else []
            for i, p in enumerate(m.params):
                written = p["type"]["qualType"]
                q = strip_cvref(qualify(written, dparams[i] if i < len(dparams) else ""))
                seen.setdefault(q, p["type"].get("desugaredQualType"))
                for inner in re.findall(r"<(.+)>$", q):
                    seen.setdefault(strip_cvref(qualify(inner, "")), None)
            q = strip_cvref(qualify(m.ret_written, ""))
            seen.setdefault(q, None)
    for c in COPIERS.values():
        for _, ftype, _ in c["fields"]:
            seen.setdefault(ftype, None)
    for v in RETURN_AS_ENUM.values():
        seen.setdefault(v, None)
    unknown = sorted(t for t in seen if t not in PLAIN and t not in COPIERS and t not in GO_PLAIN and not t.startswith("std::") and t and not re.search(r"Callback|Closure|mojo::|unique_ptr|scoped_refptr|WeakPtr|raw_ptr", t) and t not in ("void",))
    enums, records, enum_target, alias_candidates = {}, set(), {}, {}
    pending = unknown
    for _ in range(3):  # An alias of an alias of an enum.
        results = dict(zip(pending, pool.map(lambda n: dump(args, probe, n), pending)))
        pending = []
        for t, nodes in results.items():
            short = t.split("::")[-1]
            for n in nodes:
                if n.get("kind") == "EnumDecl" and n.get("name") == short and any(e.get("kind") == "EnumConstantDecl" for e in n.get("inner", [])):
                    enums[t] = [e["name"] for e in n["inner"] if e.get("kind") == "EnumConstantDecl"]
                    enum_target.setdefault(t, t)
                    break
                if n.get("kind") == "CXXRecordDecl" and n.get("name") == short and n.get("completeDefinition"):
                    records.add(t)
                    break
                if n.get("kind") in ("TypeAliasDecl", "TypedefDecl") and n.get("name") == short and n.get("inner"):
                    inner = n["inner"][0]
                    target = inner.get("qualifier", "") + inner.get("decl", {}).get("name", "") or inner.get("type", {}).get("qualType")
                    if target and inner.get("kind") == "EnumType":
                        # An unqualified target lives in the alias's own namespace or an
                        # enclosing one; each is tried.
                        scopes = t.split("::")[:-1]
                        candidates = [target] if "::" in target else ["::".join(scopes[:k] + [target]) for k in range(len(scopes), -1, -1)]
                        alias_candidates[t] = candidates
                        pending += candidates
                    break
        if not pending:
            break
    for t, candidates in alias_candidates.items():
        for c in candidates:
            if c in enums:
                enums[t] = enums[c]
                enum_target[t] = enum_target.get(c, c)
                break
    # A bare alias candidate matched an enum by substring, but its real name is
    # qualified. Enums a hook names bare (WindowOpenDisposition) are global and stay.
    for t in [t for t in enums if "::" not in t and t not in unknown]:
        del enums[t]

    # Pass 3: enumerator values, from the compiler.
    probe3 = os.path.join(WORK, "enums.cc")
    wanted = []
    with open(probe3, "w") as f:
        f.write(f'#include "{probe}"\n')
        for _, header in [(None, h) for _, h, _, _ in CLASSES]:
            f.write(f'#include "{header}"\n')
        for inc in sorted(copier_includes()):
            f.write(f'#include "{inc}"\n')
        f.write("namespace pb_enum_probe {\n")
        for t in sorted(enums):
            for e in enums[t]:
                wanted.append((t, e))
                # The value comes back folded into the array type, char[2^40 + value].
                f.write(f"extern char pb_e_{len(wanted) - 1}[static_cast<unsigned long long>(static_cast<long long>({enum_target[t]}::{e}) + (1LL << 40))];\n")
        f.write("}\n")
    values = {}
    for n in dump(args, probe3, "pb_enum_probe::pb_e_"):
        if n.get("kind") != "VarDecl":
            continue
        idx = int(n["name"][len("pb_e_"):])
        m = re.match(r"char\[(\d+)\]", n["type"]["qualType"])
        values[wanted[idx]] = int(m.group(1)) - (1 << 40) if m else None
    enum_values = {t: [(e, values.get((t, e))) for e in es] for t, es in enums.items()}

    # Classify every hook.
    hook_id = FIRST_HOOK
    hooks = []       # carried
    for qualified, header, go, scope, ms in classes:
        counts = {}
        for m in ms:
            counts[m.name] = counts.get(m.name, 0) + 1
        seen_names = {}
        for m in ms:
            dparams = split_params(m.demangled) if m.demangled else []
            m.types = []
            for i, p in enumerate(m.params):
                written = p["type"]["qualType"]
                t = Type(written, p["type"].get("desugaredQualType"), dparams[i] if i < len(dparams) else "", enums, records)
                t.param_name = p.get("name") or f"arg{i}"
                m.types.append(t)
            ret_base = strip_cvref(qualify(m.ret_written, ""))
            nested = qualified + "::" + strip_cvref(m.ret_written)
            if nested in RETURN_AS_ENUM:
                ret_base = nested
            if ret_base in RETURN_AS_ENUM:
                m.ret = Type(RETURN_AS_ENUM[ret_base], None, "", enums, records)
                m.ret.as_result = ret_base
            else:
                m.ret = None if m.ret_written == "void" else Type(m.ret_written, None, "", enums, records)
            m.hook_name = m.name
            seen_names[m.name] = seen_names.get(m.name, 0) + 1
            if counts[m.name] > 1:
                m.hook_name = f"{m.name}{seen_names[m.name]}"
            m.go_name = go + m.hook_name
            m.cpp_id = f"k{qualified.split('::')[-1]}_{m.hook_name}"
            m.reason = None
            if m.pure:
                m.reason = "pure virtual, implemented by hand"
            elif scope == "global" and OFF_MAIN.search(m.comment):
                m.reason = "off the main thread"
            else:
                for t in m.types:
                    if t.reason:
                        m.reason = t.reason
                        break
                if not m.reason and m.ret is not None:
                    if m.ret.reason:
                        m.reason = "returns " + (m.ret.reason if m.ret.reason.startswith("no copier") else m.ret.reason)
                    elif m.ret.kind not in GO_PLAIN and m.ret.kind != "enum":
                        m.reason = "returns " + m.ret.kind
                    elif m.ret.is_pointer or (m.ret.is_ref):
                        m.reason = "returns a pointer"
                if not m.reason and m.ret is not None and m.ret.kind in ("bool", "int", "double", "str", "enum") and m.ret.base not in PLAIN and m.ret.kind != "enum" and not getattr(m.ret, "desugared", None):
                    m.reason = "returns " + m.ret.base
            if m.reason is None:
                m.id = hook_id
                hook_id += 1
                m.struct_copy = any(t.kind in ("copier",) or (t.kind in ("optional", "vector") and t.inner.kind == "copier") for t in m.types)
                hooks.append(m)
        print(f"{qualified}: {sum(1 for m in ms if m.reason is None)} of {len(ms)} carried", file=sys.stderr)

    # Which copiers and enums the carried hooks use.
    used_copiers, used_enums = set(), set()

    def visit(t):
        if t.kind == "copier":
            if t.base not in used_copiers:
                used_copiers.add(t.base)
                for _, ftype, _ in COPIERS[t.base]["fields"]:
                    visit(Type(ftype, None, "", enums, records))
        elif t.kind == "enum":
            used_enums.add(t.base)
        elif t.kind in ("optional", "vector"):
            visit(t.inner)

    for m in hooks:
        for t in m.types:
            visit(t)
        if m.ret is not None:
            visit(m.ret)
    for scope_type in ("content::WebContents", "content::NavigationHandle"):
        visit(Type(scope_type + "*", None, "", enums, records))

    table_hash = hashlib.sha256("\n".join(f"{m.id} {m.cls}::{m.name} {m.signature}" for m in hooks).encode()).hexdigest()[:12]

    write_cpp(classes, hooks, used_copiers, enums, records, version, table_hash)
    write_go(classes, hooks, used_copiers, used_enums, enum_values, enums, records, version, table_hash)
    write_doc(classes, hooks, methods, demangled, version, enums, records)
    print(f"{len(hooks)} hooks carried, table {table_hash}", file=sys.stderr)


def cpp_param_list(m):
    out = []
    for t in m.types:
        name = t.param_name
        if name in CPP_KEYWORDS:
            name += "_"
        out.append(f"{t.qualified} {name}")
    return ", ".join(out)


def cpp_args_list(m):
    return ", ".join((t.param_name + "_" if t.param_name in CPP_KEYWORDS else t.param_name) for t in m.types)


def cpp_class_name(qualified):
    return qualified.split("::")[-1] + "Hooks"


def write_cpp(classes, hooks, used_copiers, enums, records, version, table_hash):
    h, cc = [], []
    head = [f"// Generated by tools/gen_hooks.py from Chromium {version}. Do not edit.",
            "//",
            "// Every hook content can carry to Go, as an override that costs one bit check when",
            "// Go hasn't subscribed. Hand-written classes derive from these and override by hand",
            "// only what needs content's objects (engine/browser/page_tab.h).", ""]
    h += head + ["#ifndef PROJECT_B_ENGINE_BROWSER_HOOKS_GEN_H_", "#define PROJECT_B_ENGINE_BROWSER_HOOKS_GEN_H_", "", "#include <cstdint>", ""]
    for qualified, header, go, scope, ms in classes:
        h.append(f'#include "{header}"')
    h += ['#include "project_b/engine/browser/hooks.h"', "", "namespace project_b {", "namespace hooks {", ""]
    h.append(f'constexpr char kTableHash[] = "{table_hash}";')
    h.append(f"constexpr int32_t kFirstHook = {FIRST_HOOK};")
    h.append(f"constexpr int32_t kHookEnd = {FIRST_HOOK + len(hooks)};")
    h += ["", "// Each carried hook's number. The Go side has the same list, core/content/hooks_gen.go.", "enum Hook : int32_t {"]
    for m in hooks:
        h.append(f"  {m.cpp_id} = {m.id},")
    h += ["};", "", "// Content's objects, copied into plain values when a hook fires."]
    for c in sorted(used_copiers):
        ptr = COPIERS[c].get("ptr")
        h.append(f"void Pack(Args& args, const {c}{'*' if ptr else '&'} v);")
    h += ["", "}  // namespace hooks", ""]

    cc += head + ['#include "project_b/engine/browser/hooks_gen.h"', "", "#include <string>", "", '#include "base/strings/utf_string_conversions.h"', '#include "net/base/net_errors.h"', '#include "net/http/http_response_headers.h"']
    for inc in sorted(copier_includes(used_copiers)):
        cc.append(f'#include "{inc}"')
    cc += ["", "namespace project_b {", "namespace hooks {", ""]
    for c in sorted(used_copiers):
        ptr = COPIERS[c].get("ptr")
        if ptr:
            # Content's getters are not all const, and a copy changes nothing.
            cc.append(f"void Pack(Args& args, const {c}* const_v) {{")
            cc.append(f"  auto* v = const_cast<{c}*>(const_v);")
            cc.append("  if (!v) {")
        else:
            cc.append(f"void Pack(Args& args, const {c}& v) {{")
        if ptr:
            n = sum(value_count(Type(ft, None, '', enums, records), enums, records) for _, ft, _ in COPIERS[c]["fields"])
            cc.append(f"    args.Zeros({n});")
            cc.append("    return;")
            cc.append("  }")
        for _, ftype, expr in COPIERS[c]["fields"]:
            cc += pack_lines(Type(ftype, None, "", enums, records), expr)
        cc += ["}", ""]
    cc += ["}  // namespace hooks", ""]

    for qualified, header, go, scope, ms in classes:
        name = cpp_class_name(qualified)
        carried = [m for m in ms if m.reason is None]
        h.append(f"class {name} : public {qualified} {{")
        h.append(" public:")
        if scope == "observer":
            h.append(f"  explicit {name}(content::WebContents* contents) : {qualified}(contents) {{}}")
        elif scope == "throttle":
            h.append(f"  explicit {name}(content::NavigationThrottleRegistry& registry) : {qualified}(registry) {{}}")
            h.append("  const char* GetNameForLogging() override;")
        elif scope == "delegate":
            h.append("  // The tab this delegate belongs to; the first argument of every hook.")
            h.append("  virtual content::WebContents* hook_contents() const = 0;")
        h.append("")
        h.append(f"  // {qualified}:")
        for m in carried:
            ret = m.ret_written if m.ret is None or not hasattr(m.ret, "as_result") else m.ret.as_result
            ret = qualify(ret, "")
            if m.guard:
                h.append(f"#if {m.guard}")
            h.append(f"  {ret} {m.name}({cpp_param_list(m)}){' const' if m.is_const else ''} override;")
            if m.guard:
                h.append("#endif")
        h += ["};", ""]

        for m in carried:
            ret = m.ret_written if m.ret is None or not hasattr(m.ret, "as_result") else m.ret.as_result
            ret = qualify(ret, "")
            if m.guard:
                cc.append(f"#if {m.guard}")
            cc.append(f"{ret} {name}::{m.name}({cpp_param_list(m)}){' const' if m.is_const else ''} {{")
            default = f"{qualified}::{m.name}({cpp_args_list(m)})"
            if m.ret is None:
                cc.append(f"  if (!hooks::Subscribed(hooks::{m.cpp_id})) {{")
                cc.append(f"    return {default};")
                cc.append("  }")
            else:
                cc.append(f"  if (hooks::Subscribed(hooks::{m.cpp_id})) {{")
            indent = "  " if m.ret is None else "    "
            cc.append(f"{indent}hooks::Args args;")
            if scope == "observer":
                cc.append(f"{indent}hooks::Pack(args, web_contents());")
            elif scope == "delegate":
                cc.append(f"{indent}hooks::Pack(args, hook_contents());")
            elif scope == "throttle":
                cc.append(f"{indent}hooks::Pack(args, navigation_handle());")
            for t in m.types:
                pname = t.param_name + "_" if t.param_name in CPP_KEYWORDS else t.param_name
                cc += pack_lines(t, pname, indent)
            if m.ret is None:
                cc.append(f"{indent}hooks::Fire(hooks::{m.cpp_id}, args);")
                cc.append(f"{indent}{default};")
            else:
                cc.append(f"{indent}PBValue result;")
                cc.append(f"{indent}if (hooks::Ask(hooks::{m.cpp_id}, args, &result)) {{")
                cc.append(f"{indent}  return {return_expr(m.ret, ret)};")
                cc.append(f"{indent}}}")
                cc.append("  }")
                cc.append(f"  return {default};")
            cc += ["}", "#endif", ""] if m.guard else ["}", ""]

    h += ["}  // namespace project_b", "", "#endif  // PROJECT_B_ENGINE_BROWSER_HOOKS_GEN_H_", ""]
    cc += ["const char* NavigationThrottleHooks::GetNameForLogging() {", '  return "project_b::NavigationThrottleHooks";', "}", "", "}  // namespace project_b", ""]
    write(os.path.join(ROOT, "engine/browser/hooks_gen.h"), "\n".join(h))
    write(os.path.join(ROOT, "engine/browser/hooks_gen.cc"), "\n".join(cc))


def value_count(t, enums, records):
    if t.kind in GO_PLAIN or t.kind == "enum":
        return 1
    if t.kind == "copier":
        return sum(value_count(Type(ft, None, "", enums, records), enums, records) for _, ft, _ in COPIERS[t.base]["fields"])
    if t.kind == "optional":
        return 2
    raise ValueError(t.written)


def return_expr(t, cpp_type):
    if t.kind == "bool":
        return "result.i != 0"
    if t.kind == "int":
        return f"static_cast<{cpp_type}>(result.i)"
    if t.kind == "double":
        return f"static_cast<{cpp_type}>(result.d)"
    if t.kind == "enum":
        return f"static_cast<{t.base}>(result.i)"
    if t.kind == "str":
        if t.base == "std::u16string":
            return "base::UTF8ToUTF16(hooks::TakeString(result))"
        if t.base == "GURL":
            return "GURL(hooks::TakeString(result))"
        if t.base == "base::FilePath":
            return "base::FilePath(hooks::TakeString(result))"
        return "hooks::TakeString(result)"
    raise ValueError(t.written)


def go_ret_type(t):
    return t.go_type()


def write_go(classes, hooks, used_copiers, used_enums, enum_values, enums, records, version, table_hash):
    head = [f"// Code generated by tools/gen_hooks.py from Chromium {version}. DO NOT EDIT.", "", "package content", ""]
    g = head + [f'// TableHash names the hook table this file was generated with. The library carries',
                "// the same hash, and start refuses a library built from a different table.",
                f'const TableHash = "{table_hash}"', "",
                "// Each carried hook's number, matching engine/browser/hooks_gen.h.", "const ("]
    for m in hooks:
        g.append(f"\t{m.go_name} Hook = {m.id}")
    g += [")", "", "// Where each class's numbers start, so Dispatch can pick the class.", "const ("]
    g.append(f"\tFirstHook Hook = {FIRST_HOOK}")
    for qualified, header, go, scope, ms in classes:
        carried = [m for m in ms if m.reason is None]
        g.append(f"\tfirst{go} Hook = {carried[0].id if carried else FIRST_HOOK}")
    g += [")", "", "// Names gives each hook's content name, for stats and logs.", "var Names = map[Hook]string{"]
    for m in hooks:
        g.append(f'\t{m.go_name}: "{m.cls.split("::")[-1]}.{m.name}",')
    g += ["}", ""]
    # Enums.
    for e in sorted(used_enums):
        values = enum_values.get(e, [])
        name = go_enum_name(e)
        names = [v for v, _ in values]
        prefix = os.path.commonprefix(names) if len(names) > 1 else ""
        prefix = prefix[:prefix.rfind("_") + 1] if "_" in prefix else ""
        g.append(f"// {name} is {e}.")
        g.append(f"type {name} int64")
        g.append("")
        g.append("const (")
        taken = set()
        for v, val in values:
            cname = go_const_name(name, v, prefix)
            if cname in taken or val is None:
                continue
            taken.add(cname)
            g.append(f"\t{cname} {name} = {val}")
        g += [")", ""]
    # Structs and readers.
    for c in sorted(used_copiers):
        name = go_struct_name(c)
        g.append(f"// {name} is a copy of {c} taken when the hook fired.")
        g.append(f"type {name} struct {{")
        for fname, ftype, _ in COPIERS[c]["fields"]:
            t = Type(ftype, None, "", enums, records)
            g.append(f"\t{fname} {t.go_type()}")
        g += ["}", ""]
        g.append(f"func (r *reader) {lower_first(name)}() {name} {{")
        g.append(f"\treturn {name}{{")
        for fname, ftype, _ in COPIERS[c]["fields"]:
            t = Type(ftype, None, "", enums, records)
            g.append(f"\t\t{fname}: {read_expr(t)},")
        g += ["\t}", "}", ""]
    write(os.path.join(ROOT, "core/content/hooks_gen.go"), "\n".join(g))

    for qualified, header, go, scope, ms in classes:
        carried = [m for m in ms if m.reason is None]
        f = head[:]
        short = qualified.split("::")[-1]
        f.append(f"// {go}Hooks is {qualified}: subscribe with On (hooks that inform) and Answer (hooks")
        f.append("// that want a value back; return false to let content use its default).")
        f.append(f"type {go}Hooks struct {{")
        f.append("\thooks *Hooks")
        for m in carried:
            f.append(f"\t{field_name(m)} {handler_type(m, scope)}")
        f += ["}", ""]
        for m in carried:
            doc = m.comment.strip()
            if doc:
                f.append(f"// {m.hook_name}: " + wrap(doc, 85, "// "))
            else:
                f.append(f"// {m.hook_name} is {qualified}::{m.name}.")
            if m.ret is None:
                f.append(f"func (c *{go}Hooks) {on_name(m)}(f {func_type(m, scope)}) {{")
                f.append(f"\tc.hooks.subscribe({m.go_name})")
                f.append(f"\tc.{field_name(m)} = append(c.{field_name(m)}, f)")
            else:
                f.append(f"func (c *{go}Hooks) Answer{m.hook_name}(f {func_type(m, scope)}) {{")
                f.append(f"\tc.hooks.subscribe({m.go_name})")
                f.append(f"\tc.{field_name(m)} = f")
            f += ["}", ""]
        f.append(f"func (c *{go}Hooks) dispatch(id Hook, r *reader) (Value, bool) {{")
        f.append("\tswitch id {")
        for m in carried:
            f.append(f"\tcase {m.go_name}:")
            call_args = []
            if scope in ("observer", "delegate"):
                f.append("\t\ttab := r.webContents()")
                call_args.append("tab")
            elif scope == "throttle":
                f.append("\t\tnavigation := r.navigationHandle()")
                call_args.append("navigation")
            for t in m.types:
                f.append(f"\t\t{camel(t.param_name)} := {read_expr(t)}")
                call_args.append(camel(t.param_name))
            if m.ret is None:
                f.append(f"\t\tfor _, f := range c.{field_name(m)} {{")
                f.append(f"\t\t\tf({', '.join(call_args)})")
                f.append("\t\t}")
            else:
                f.append(f"\t\tif c.{field_name(m)} == nil {{")
                f.append("\t\t\treturn Value{}, false")
                f.append("\t\t}")
                f.append(f"\t\tv, ok := c.{field_name(m)}({', '.join(call_args)})")
                f.append(f"\t\treturn {value_from(m.ret)}, ok")
        f += ["\t}", "\treturn Value{}, false", "}", ""]
        write(os.path.join(ROOT, f"core/content/{go.lower()}_gen.go"), "\n".join(f))


def on_name(m):
    """On plus the hook's name, without doubling an On content already put there."""
    return m.hook_name if m.hook_name.startswith("On") and m.hook_name[2:3].isupper() else "On" + m.hook_name


def field_name(m):
    return "on" + m.hook_name if m.ret is None else "answer" + m.hook_name


def func_type(m, scope):
    params = []
    if scope in ("observer", "delegate"):
        params.append("tab WebContents")
    elif scope == "throttle":
        params.append("navigation NavigationHandle")
    for t in m.types:
        params.append(f"{camel(t.param_name)} {t.go_type()}")
    sig = f"func({', '.join(params)})"
    if m.ret is not None:
        sig += f" ({go_ret_type(m.ret)}, bool)"
    return sig


def handler_type(m, scope):
    return ("[]" if m.ret is None else "") + func_type(m, scope)


def value_from(t):
    if t.kind == "bool":
        return "boolValue(v)"
    if t.kind in ("int", "enum"):
        return "Value{I: int64(v)}"
    if t.kind == "double":
        return "Value{D: float64(v)}"
    if t.kind == "str":
        return "Value{S: v}"
    raise ValueError(t.written)


def wrap(text, width, prefix):
    words, lines, cur = text.split(), [], ""
    for w in words:
        if len(cur) + len(w) + 1 > width and cur:
            lines.append(cur)
            cur = w
        else:
            cur = (cur + " " + w).strip()
    lines.append(cur)
    return ("\n" + prefix).join(lines)


def grep_uses(method):
    """Where in core/ a hook is subscribed, so the doc says who listens."""
    out = []
    for root, _, files in os.walk(os.path.join(ROOT, "core")):
        for fn in files:
            if fn.endswith(".go") and not fn.endswith("_gen.go"):
                path = os.path.join(root, fn)
                text = open(path).read()
                if re.search(r"\." + re.escape(method) + r"\(", text):
                    out.append(os.path.relpath(path, ROOT))
    return out


def write_doc(classes, hooks, methods, demangled, version, enums, records):
    path = os.path.join(ROOT, "docs/content-hooks.md")
    hand = ""
    if os.path.exists(path):
        old = open(path).read()
        m = re.search(r"<!-- hand -->.*?<!-- /hand -->", old, re.S)
        hand = m.group(0) if m else ""
    if not hand:
        hand = "<!-- hand -->\n<!-- /hand -->"

    groups = {}
    rows = []
    totals = []
    for qualified, header, go, scope, ms in classes:
        short = qualified.split("::")[-1]
        carried = [m for m in ms if m.reason is None]
        plain = [m for m in carried if not m.struct_copy]
        copy = [m for m in carried if m.struct_copy]
        left = [m for m in ms if m.reason is not None]
        used = sum(1 for m in carried if grep_uses(on_name(m) if m.ret is None else "Answer" + m.hook_name))
        totals.append((short, len(ms), len(plain), len(copy), len(left), used))
        for m in left:
            groups.setdefault(group_of(m.reason), {}).setdefault(short, []).append(m)
        for m in ms:
            if m.reason is None:
                status = "carried, struct copy" if m.struct_copy else "carried"
                method = on_name(m) if m.ret is None else "Answer" + m.hook_name
                where = f"`{cpp_class_name(qualified)}::{m.name}` · `Hooks.{go}.{method}`"
                uses = grep_uses(method)
                if uses:
                    where += " · used by " + ", ".join(f"`{u}`" for u in uses)
            else:
                status = "left: " + m.reason
                where = f"`{header}:{m.line}`"
            rows.append((short, m.name, status, where))
    other = []
    for qualified, header, process in OTHER_PROCESSES:
        nodes, lines = methods[qualified]
        other.append((qualified.split("::")[-1], len(nodes), process, header))

    out = [f"# Content hooks", "",
           f"Generated by `tools/gen_hooks.py` from Chromium {version}; the prose between the hand markers is",
           "written by hand and kept. One line per virtual hook in the main-process classes, with",
           "where it is carried or why it is left. Carried means content's call reaches Go as a",
           "generated hook; Go acts on it only where it subscribes or answers, counted as used by Go.", "",
           "## Counts", "",
           "| Class | Hooks | Carried, plain data | Carried, struct copy | Left | Used by Go |",
           "|---|---:|---:|---:|---:|---:|"]
    for short, n, p, c, l, u in totals:
        out.append(f"| `{short}` | {n} | {p} | {c} | {l} | {u} |")
    tn, tp, tc, tl, tu = (sum(x[i] for x in totals) for i in range(1, 6))
    out.append(f"| **Total** | **{tn}** | **{tp}** | **{tc}** | **{tl}** | **{tu}** |")
    for short, n, process, header in other:
        out.append(f"| `{short}` ({process} process) | {n} | 0 | 0 | {n} | 0 |")
    out += ["", "Left, by group:", ""]
    out.append("| Group | " + " | ".join(f"`{s}`" for s, *_ in totals) + " | Total |")
    out.append("|---|" + "---:|" * (len(totals) + 1))
    for g in sorted(groups, key=lambda g: -sum(len(v) for v in groups[g].values())):
        cells = [str(len(groups[g].get(s, []))) for s, *_ in totals]
        out.append(f"| {g} | " + " | ".join(cells) + f" | {sum(len(v) for v in groups[g].values())} |")
    out += ["", hand, "", "## Every hook", "", "| Class | Hook | Status | Where |", "|---|---|---|---|"]
    for short, name, status, where in rows:
        out.append(f"| `{short}` | `{name}` | {status} | {where} |")
    out.append("")
    write(path, "\n".join(out))


def group_of(reason):
    if reason.startswith("returns"):
        if "pointer" in reason or "ownership" in reason or "no copier" in reason or "vector" in reason or "optional" in reason:
            return "factories and object returns"
        return "other returns"
    if reason == "callback":
        return "callbacks"
    if reason == "mojo":
        return "Mojo"
    if reason == "ownership":
        return "ownership (unique_ptr, scoped_refptr, WeakPtr)"
    if reason == "off the main thread":
        return "off the main thread"
    if reason in ("out-param", "mutable reference"):
        return "out-parameters"
    if reason.startswith("no copier") or reason in ("vector of objects", "optional object"):
        return "objects without a copier"
    if reason.startswith("pure"):
        return "pure virtual"
    return reason


def write(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        f.write(text)
    if path.endswith(".go"):
        subprocess.run(["/opt/homebrew/bin/gofmt", "-w", path], check=True)
    print("wrote", os.path.relpath(path, ROOT), file=sys.stderr)


if __name__ == "__main__":
    main()
