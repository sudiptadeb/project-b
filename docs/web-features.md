# Web features

> A web feature here is something a page can call or do whose browser side Chrome provides
> from `//chrome` or `//components`, not content. Project B has no `//chrome`, so each one is
> missing, half there, turned off, or, at worst, gets the page's renderer killed.

## After a Chromium bump

    python3 tools/web_features_bump.py           # reads chromium/src; no build needed
    python3 tools/web_features_bump.py --list    # and everything it read

Run it before `tools/gen_hooks.py` ([content-hooks.md](content-hooks.md), "When Chromium is
upgraded"), since it compares with the hooks table that run rewrites. It reads:

- **Chrome's binders**, for each scope content lets an embedder bind for
  (`ContentBrowserClient::RegisterBrowserInterfaceBindersFor<Scope>`): frames, from
  `PopulateChromeFrameBinders`, and service workers, from chrome's
  `RegisterBrowserInterfaceBindersForServiceWorker`. Dedicated and shared workers have no such
  hook in content, so chrome binds nothing for them; a new scope is reported until
  `CHROME_BINDERS` in the script says where chrome fills it. Each interface comes with the
  platforms its `#if` lets it in on.
- **Content's binders** (`content/browser/browser_interface_binders.cc`) and **ours**
  (`engine/browser/content_browser_client.cc`, `engine/extensions/browser/content_hooks.cc`).
- **What Blink's renderer asks for**: every `mojom::` name in `third_party/blink/renderer`.
- **The virtual methods** of `WebContentsDelegate` and `ContentBrowserClient`, with the
  platforms each is on.

And reports three lists:

1. **Chrome binds it, Blink asks for it, and nothing here binds it or names it.** A page that
   asks for one is killed. Add a row, and turn the feature off or give it an empty binder.
2. **Delegate methods new since `docs/content-hooks.md`**, in no row. The table is generated
   for the Mac, so only the methods the Mac has are compared. A person decides whether a page
   can tell; if it can, add a row.
3. **What a row names that Chromium no longer has**: an interface no `.mojom` defines, a
   delegate method gone from its class, a Blink or base feature that isn't declared, a
   permission type gone from `blink::PermissionType`. Fix the row.

It exits 1 when the first or the third list has anything, so a bump can't pass it quietly.
On Chromium 155.0.8059.41 all three are empty.
