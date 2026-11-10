# Chrome keyboard shortcuts and main menus: full inventory (Chrome 155, macOS / Windows / Linux)

Source: local checkout `chromium/src` at `chrome/VERSION` 155.0.8059.41 (the checkout is shallow, so there is no git history to date individual key changes).
Paths below are relative to `chromium/src`. Context menus are out of scope. ChromeOS-only keys are listed once in section 6 and kept out of the master table.

Cross-check: Google's "Chrome keyboard shortcuts" help page (support.google.com/chrome/answer/157179), fetched 2026-11-10, both platform tabs (section 5).

---

## 0. How Chrome routes a key (what Project B has to copy)

Accelerator sources, by platform:

| Platform | Where the keys come from |
|---|---|
| Win/Linux | `kAcceleratorMap` + `kDevToolsAcceleratorMap` + `kTabGroupAcceleratorMap` + split-tab key + (debug) `kUIDebugAcceleratorMap` (accelerator_table.cc:51-301, 317-365). These are registered on the views FocusManager in `BrowserView::LoadAccelerators` (browser_view.cc:5625-5650). Windows also maps WM_APPCOMMAND keys (browser_view.cc:5678-5720). Cut/Copy/Paste are not accelerators. Blink handles Ctrl+C/X/V/A/Z/Y itself (editing_behavior.cc:171-182). The table entries for them only feed the labels shown in the menu (accelerator_table.cc:372-395). |
| Mac | (a) Main menu key equivalents. A menu item with an IDC tag takes its key from `AcceleratorsCocoa` (main_menu_builder.mm:719-732, accelerators_cocoa.mm:32-120). Otherwise the key comes from the builder's `key_equivalent`. (b) The hidden list `GetShortcutsNotPresentInMainMenu` (global_keyboard_shortcuts_mac.mm:111-180). (c) The "delayed" list: Cmd+Left and Cmd+Right fire only if the page did not consume the key (global_keyboard_shortcuts_mac.mm:81-92; chrome_command_dispatcher_delegate.mm:155-158). (d) Only the modifier-less entries in accelerator_table.cc, all outside `#if !BUILDFLAG(IS_MAC)`: F7 (caret browsing), Esc (close find bar or stop) and F12 (DevTools). |
| Mac Edit menu | Undo/Redo/Cut/Copy/Paste/Select All etc. are **not** Chrome commands. They are AppKit selectors (`copy:`, `paste:` …) that go down the responder chain to content's `RenderWidgetHostViewCocoa` (render_widget_host_view_cocoa.mm:2914-2961). Blink deliberately does not bind Cmd+C/V on Mac: "On OS X, we pipe these back to the browser" (editing_behavior.cc:171-173). **So a Mac shell with no Edit menu has no Cmd+C/V/X/A/Z at all.** |

Reserved keys: the page never gets them (`BrowserCommandController::IsReservedCommandOrKey`, browser_command_controller.cc:477-543):

- **Reserved outside fullscreen:** IDC_CLOSE_TAB, IDC_CLOSE_WINDOW, IDC_NEW_INCOGNITO_WINDOW, IDC_NEW_ISOLATED_WINDOW, IDC_NEW_TAB, IDC_NEW_WINDOW, IDC_RESTORE_TAB, IDC_SELECT_NEXT_TAB, IDC_SELECT_PREVIOUS_TAB, IDC_CYCLE_TO_NEXT_TAB, IDC_CYCLE_TO_PREV_TAB, IDC_EXIT (535-542).
- **Fullscreen:** only IDC_EXIT and IDC_FULLSCREEN stay reserved; every other key goes to the page (500-521). On Mac, when the toolbar is showing in fullscreen, IDC_FULLSCREEN is reserved and the normal list applies (506-517).
- **App / app-popup windows:** nothing is reserved (480-484).
- **Linux:** a key that the user's GTK key theme binds to a text-edit command is not reserved (523-533).
- On Mac, the menu items Close Window (`performClose:`) and Quit (`terminate:`) are mapped back to IDC_CLOSE_WINDOW and IDC_EXIT, so they can be reserved (global_keyboard_shortcuts_mac.mm:55-65).
- Everything else is "post-target": the page sees the key first, and the browser acts only if the page didn't call preventDefault (docs/ui/learn/keyboard_shortcuts.md:308-313; chrome_command_dispatcher_delegate.mm:144-182).
- On Mac, key repeats of New Tab / New Window / New Incognito are dropped (chrome_command_dispatcher_delegate.mm:99-105).

Below, "Reserved" uses these values: **not in FS** (reserved except in fullscreen), **always**, **FS only** (reserved only while in fullscreen), **no**.

---

## 1. Mac menu bar, exactly as `BuildMainMenu` builds it

Order: App, File, Edit, View, History, Bookmarks, People, Tab, Window, Help (main_menu_builder.mm:639-650). Bookmarks, Tab and Help are dropped for PWA windows (433-435, 529-531, 598-600). "pwa-" means the item is removed in PWA windows. "alt" means an Option-alternate (`is_alternate`), which replaces the item above it while Option is held. The Mac key comes from accelerators_cocoa.mm unless the row says "builder".

### 1.1 App menu ("Chrome", tag IDC_CHROME_MENU) (main_menu_builder.mm:57-127)
| # | Title | IDC / selector | Mac key | Reserved |
|---|---|---|---|---|
| 1 | About Google Chrome | IDC_ABOUT (`orderFrontStandardAboutPanel:`) | none | no |
| – | separator | | | |
| 2 | Preferences… (AppKit shows it as "Settings…" on macOS 13+), pwa- | IDC_OPTIONS (`showPreferences:`) | Cmd+, (accelerators_cocoa.mm:64) | no |
| 3 | Preferences… (PWA only) | IDC_WEB_APP_SETTINGS | Cmd+, (:65) | no |
| – | separator | | | |
| 4 | Delete Browsing Data…, pwa- | IDC_CLEAR_BROWSING_DATA | Cmd+Shift+Backspace(Delete) (:34) | no |
| 5 | Import Bookmarks and Settings…, pwa- | IDC_IMPORT_SETTINGS | none | no |
| – | separator | | | |
| 6 | Services ▸ (filled by AppKit) | none (`servicesMenu`) | n/a | no |
| – | separator | | | |
| 7 | Hide Google Chrome | IDC_HIDE_APP (`hide:`) | Cmd+H (:66) | no |
| 8 | Hide Others | `hideOtherApplications:` | Cmd+Opt+H (builder :102) | no |
| 9 | Show All | `unhideAllApplications:` | none | no |
| – | separator | | | |
| 10 | Warn Before Quitting (⌘Q), pwa- | `toggleConfirmToQuit:` | none (toggle) | no |
| – | separator | | | |
| 11 | Quit Google Chrome | IDC_EXIT (`terminate:`) | Cmd+Q (:67) | **always** |

### 1.2 File (IDC_FILE_MENU) (main_menu_builder.mm:129-189)
| # | Title | IDC / selector | Mac key | Reserved |
|---|---|---|---|---|
| 1 | New Tab, pwa- | IDC_NEW_TAB | Cmd+T (:44) | **not in FS** |
| 2 | New Window | IDC_NEW_WINDOW | Cmd+N (:45) | **not in FS** |
| 3 | New Incognito Window, pwa- | IDC_NEW_INCOGNITO_WINDOW | Cmd+Shift+N (:42) | **not in FS** |
| 4 | New Isolated Window, pwa- (enabled only for the enterprise isolated mode) | IDC_NEW_ISOLATED_WINDOW | none | **not in FS** |
| 5 | Reopen Closed Tab, pwa- | IDC_RESTORE_TAB | Cmd+Shift+T (:48) | **not in FS** |
| 6 | Open File…, pwa- | IDC_OPEN_FILE | Cmd+O (:68) | no |
| 7 | Open Location…, pwa- | IDC_FOCUS_LOCATION | Cmd+L (:69) | no |
| – | separator | | | |
| 8 | Close Window | IDC_CLOSE_WINDOW (`performClose:`) | Cmd+Shift+W; Cmd+W in a PWA (accelerators_cocoa.mm:122-130) | **not in FS** |
| 9 | Close All (alt of 8) | `closeAll:` | Cmd+Opt+Shift+W (builder :166, "W" = Shift) | no |
| 10 | Close Tab, pwa- | IDC_CLOSE_TAB | Cmd+W (:75) | **not in FS** |
| 11 | Save Page As… | IDC_SAVE_PAGE | Cmd+S (:49) | no |
| – | separator, pwa- | | | |
| 12 | Share ▸, pwa- (ShareMenuController: "Email Link", the NSSharingService list, "More…") | IDC_EMAIL_PAGE_LOCATION for Email Link | Email Link: Cmd+Shift+I (accelerators_cocoa.mm:77; share_menu_controller.mm:80-160) | no |
| – | separator | | | |
| 13 | Print… | IDC_PRINT | Cmd+P (:47) | no |
| 14 | Print Using System Dialog… (alt of 13), pwa- | IDC_BASIC_PRINT | Cmd+Opt+P (:80, ENABLE_PRINTING) | no |

### 1.3 Edit (IDC_EDIT_MENU) (main_menu_builder.mm:191-300). All responder-chain selectors, none reserved
| # | Title | Tag / selector | Mac key |
|---|---|---|---|
| 1 | Undo | IDC_CONTENT_CONTEXT_UNDO / `undo:` | Cmd+Z (:82) |
| 2 | Redo | IDC_CONTENT_CONTEXT_REDO / `redo:` | Cmd+Shift+Z (:83) |
| – | separator | | |
| 3 | Cut | IDC_CONTENT_CONTEXT_CUT / `cut:` | Cmd+X (:85) |
| 4 | Copy | IDC_CONTENT_CONTEXT_COPY / `copy:` | Cmd+C (:86) |
| 5 | Paste | IDC_CONTENT_CONTEXT_PASTE / `paste:` | Cmd+V (:87) |
| 6 | Paste and Match Style | IDC_CONTENT_CONTEXT_PASTE_AND_MATCH_STYLE / `pasteAndMatchStyle:` | Cmd+Shift+V (:88) |
| 7 | Paste and Match Style (alt of 6) | `pasteAndMatchStyle:` | Cmd+Opt+Shift+V (builder :223) |
| 8 | Delete | IDC_CONTENT_CONTEXT_DELETE / `delete:` | none |
| 9 | Select All | IDC_CONTENT_CONTEXT_SELECTALL / `selectAll:` | Cmd+A (:90) |
| – | separator | | |
| 10 | Find ▸ (IDC_FIND_MENU) | | |
| 10.1 | Search the Web… | IDC_FOCUS_SEARCH | Cmd+Opt+F (:91) |
| – | separator | | |
| 10.2 | Find… | IDC_FIND | Cmd+F (:41) |
| 10.3 | Find Next | IDC_FIND_NEXT | Cmd+G (:92) |
| 10.4 | Find Previous | IDC_FIND_PREVIOUS | Cmd+Shift+G (:93) |
| 10.5 | Use Selection for Find | `copyToFindPboard:` | Cmd+E (builder :246) |
| 10.6 | Jump to Selection | `centerSelectionInVisibleArea:` | Cmd+J (builder :249) |
| 11 | Spelling and Grammar ▸ (IDC_SPELLCHECK_MENU) | | |
| 11.1 | Show Spelling and Grammar | `showGuessPanel:` | Cmd+Shift+; (":") (builder :256) |
| 11.2 | Check Document Now | `checkSpelling:` | Cmd+; (builder :259) |
| 11.3 | Check Spelling While Typing | `toggleContinuousSpellChecking:` | none |
| 11.4 | Check Grammar With Spelling | `toggleGrammarChecking:` | none |
| 12 | Substitutions ▸ | | |
| 12.1 | Show Substitutions | `orderFrontSubstitutionsPanel:` | none |
| – | separator | | |
| 12.2 | Smart Quotes | `toggleAutomaticQuoteSubstitution:` | none |
| 12.3 | Smart Dashes | `toggleAutomaticDashSubstitution:` | none |
| 12.4 | Text Replacement | `toggleAutomaticTextReplacement:` | none |
| 13 | Transformations ▸ | | |
| 13.1 | Make Uppercase | `uppercaseWord:` | none |
| 13.2 | Make Lowercase | `lowercaseWord:` | none |
| 13.3 | Capitalize | `capitalizeWord:` | none |
| 14 | Speech ▸ | | |
| 14.1 | Start Speaking | `startSpeaking:` | none |
| 14.2 | Stop Speaking | `stopSpeaking:` | none |
| (AppKit) | AutoFill / Start Dictation… / Emoji & Symbols are inserted by AppKit (comment :294-295) | | Fn/Globe+E etc. (OS) |

### 1.4 View (IDC_VIEW_MENU) (main_menu_builder.mm:302-381)
| # | Title | IDC / selector | Mac key | Reserved |
|---|---|---|---|---|
| 1 | Always Show Bookmarks Bar, pwa- | IDC_SHOW_BOOKMARK_BAR | Cmd+Shift+B (:50) | no |
| 2 | Always Show Toolbar in Full Screen | IDC_TOGGLE_FULLSCREEN_TOOLBAR | Cmd+Shift+F (:110) | no |
| 3 | Always Show Full URLs | IDC_SHOW_FULL_URLS | none | no |
| 4 | Always Show Google Lens Shortcut | IDC_SHOW_GOOGLE_LENS_SHORTCUT | none | no |
| 5 | Always Show AI Mode | IDC_SHOW_AI_MODE_OMNIBOX_BUTTON | none | no |
| 6 | Show Search Shortcuts | IDC_SHOW_SEARCH_TOOLS | none | no |
| 7 | Show Tabs Vertically | IDC_TOGGLE_VERTICAL_TABS | none | no |
| 8 | Collapse Vertical Tabs | IDC_TOGGLE_VERTICAL_TABS_COLLAPSE | Cmd+Shift+L (:117; the builder's `@"L"` at :331 is overridden by the tag) | no |
| 9 | Customize Touch Bar, pwa- | IDC_CUSTOMIZE_TOUCH_BAR (`toggleTouchBarCustomizationPalette:`) | none | no |
| – | separator | | | |
| 10 | Stop | IDC_STOP | Cmd+. (:94) | no |
| 11 | Reload This Page | IDC_RELOAD | Cmd+R (:95) | no |
| 12 | Force Reload This Page (alt of 11) | IDC_RELOAD_BYPASSING_CACHE | Cmd+Shift+R (:96) | no |
| – | separator | | | |
| 13 | Enter Full Screen | IDC_FULLSCREEN (`toggleFullScreen:`) | Cmd+Ctrl+F with the Fn flag (`AcceleratorForEnterFullscreen`, accelerators_cocoa.mm:132-135); macOS presents this as Globe/Fn+F | **FS only** |
| 14 | Enter Full Screen (alt of 13) | `toggleFullScreen:` | Cmd+Ctrl+F (builder :351) | no |
| 15 | Actual Size | IDC_ZOOM_NORMAL | Cmd+0 (:98) | no |
| 16 | Zoom In | IDC_ZOOM_PLUS | Cmd+Shift+= shown as Cmd++; Cmd+= also works (:59-61) | no |
| 17 | Zoom Out | IDC_ZOOM_MINUS | Cmd+- (:58) | no |
| – | separator | | | |
| 18 | Cast… | IDC_ROUTE_MEDIA | none | no |
| – | separator | | | |
| 19 | Developer ▸ (IDC_DEVELOPER_MENU) | | | |
| 19.1 | View Source | IDC_VIEW_SOURCE | Cmd+Opt+U (:57) | no |
| 19.2 | Developer Tools | IDC_DEV_TOOLS | Cmd+Opt+I (:38) | no |
| 19.3 | Inspect Elements | IDC_DEV_TOOLS_INSPECT | Cmd+Opt+C (:40) | no |
| 19.4 | JavaScript Console | IDC_DEV_TOOLS_CONSOLE | Cmd+Opt+J (:39) | no |
| 19.5 | Allow JavaScript from Apple Events | IDC_TOGGLE_JAVASCRIPT_APPLE_EVENTS | none | no |

### 1.5 History (IDC_HISTORY_MENU) (main_menu_builder.mm:383-426), filled by HistoryMenuBridge
| # | Title | IDC | Mac key | Reserved |
|---|---|---|---|---|
| 1 | Home, pwa- | IDC_HOME | Cmd+Shift+H (:99) | no |
| 2 | Back | IDC_BACK | Cmd+[ (:100). Swapped with Forward in RTL locales (:154-164) | no |
| 3 | Forward | IDC_FORWARD | Cmd+] (:101) | no |
| – | separator (kRecentlyClosedSeparator), pwa- | | | |
| 4 | "Recently Closed" section header, followed by dynamic closed tabs/windows/groups/splits, pwa- | (dynamic) | none | no |
| – | separator (kVisitedSeparator), pwa- | | | |
| 5 | "Recently Visited" section header, followed by dynamic history entries, pwa- | (dynamic) | none | no |
| – | separator (kShowFullSeparator), pwa- | | | |
| 6 | Show Full History, pwa- | IDC_SHOW_HISTORY | Cmd+Y (:56) | no |

### 1.6 Bookmarks (kBookmarksMenuPlaceholder) (main_menu_builder.mm:428-459), not in PWA; BookmarkMenuBridge appends the bookmark tree
| # | Title | IDC | Mac key | Reserved |
|---|---|---|---|---|
| 1 | Open Bookmark Manager (string V2 when MenuSimplification is on) | IDC_SHOW_BOOKMARK_MANAGER | Cmd+Opt+B (:52) | no |
| – | separator | | | |
| 2 | Bookmark This Tab… | IDC_BOOKMARK_THIS_TAB | Cmd+D (:54) | no |
| 3 | Bookmark All Tabs… | IDC_BOOKMARK_ALL_TABS | Cmd+Shift+D (:102) | no |
| – | separator, followed by the dynamic bookmark bar / other bookmarks tree | | | |

### 1.7 People / "Profiles" (IDC_PROFILE_MAIN_MENU) (main_menu_builder.mm:462-475)
Built empty. ProfileMenuController fills it with a profile list, separator, "Manage Profiles…", separator, "Add Profile…" (profile_menu_controller.mm:166-252). No keys, none reserved.

### 1.8 Tab (IDC_TAB_MENU) (main_menu_builder.mm:524-591), not in PWA; TabMenuBridge appends the open tabs after the last separator (app_controller_mac.mm:1137-1141)
| # | Title | IDC | Mac key | Reserved |
|---|---|---|---|---|
| 1 | New Tab to the Right (to the Left in RTL) | IDC_NEW_TAB_TO_RIGHT | none | no |
| 2 | New Tab Below (hidden; swapped in for vertical tabs) | IDC_NEW_TAB_TO_RIGHT | none | no |
| 3 | Select Next Tab | IDC_CYCLE_TO_NEXT_TAB | Ctrl+Tab (:105) | **not in FS** |
| 4 | Select Previous Tab | IDC_CYCLE_TO_PREV_TAB | Ctrl+Shift+Tab (:106) | **not in FS** |
| 5 | Duplicate Tab | IDC_DUPLICATE_TAB | none | no |
| 6 | Duplicate Selected Tab (alt) | IDC_DUPLICATE_TARGET_TAB | Option-only alternate | no |
| 7 | Mute Site | IDC_WINDOW_MUTE_SITE | none | no |
| 8 | Mute Selected Site (alt) | IDC_MUTE_TARGET_SITE | alt | no |
| 9 | Pin Tab | IDC_WINDOW_PIN_TAB | none | no |
| 10 | Pin Selected Tab (alt) | IDC_PIN_TARGET_TAB | alt | no |
| 11 | Group Tab | IDC_WINDOW_GROUP_TAB | none | no |
| 12 | Group Selected Tab (alt) | IDC_GROUP_TARGET_TAB | alt | no |
| 13 | Close Other Tabs | IDC_WINDOW_CLOSE_OTHER_TABS | none | no |
| 14 | Close Tabs to the Right (Left in RTL) | IDC_WINDOW_CLOSE_TABS_TO_RIGHT | none | no |
| 15 | Close Tabs Below (hidden; vertical tabs) | IDC_WINDOW_CLOSE_TABS_TO_RIGHT | none | no |
| 16 | Move Tab to New Window | IDC_MOVE_TAB_TO_NEW_WINDOW | none | no |
| 17 | Add Tab to New Split View | IDC_NEW_SPLIT_TAB | Cmd+Opt+N (:119) | no |
| 18 | Search Tabs… | IDC_TAB_SEARCH | Cmd+Shift+A (:116) | no |
| – | separator, followed by the dynamic list of open tabs | | | |

### 1.9 Window (IDC_WINDOW_MENU) (main_menu_builder.mm:477-522); AppKit appends the window list (`windowsMenu`)
| # | Title | IDC / selector | Mac key | Reserved |
|---|---|---|---|---|
| 1 | Minimize | IDC_MINIMIZE_WINDOW (`performMiniaturize:`) | Cmd+M (:104) | no |
| 2 | Zoom | IDC_MAXIMIZE_WINDOW (`performZoom:`) | none | no |
| – | separator | | | |
| 3 | Show As Tab, pwa- | IDC_SHOW_AS_TAB | none | no |
| 4 | Name Window…, pwa- | IDC_NAME_WINDOW | none | no |
| – | separator, pwa- | | | |
| 5 | Downloads, pwa- | IDC_SHOW_DOWNLOADS | Cmd+Shift+J (:55) | no |
| 6 | Extensions, pwa- | IDC_MANAGE_EXTENSIONS | none | no |
| 7 | Task Manager, pwa- | IDC_TASK_MANAGER_MAIN_MENU | none | no |
| – | separator, pwa- | | | |
| 8 | Bring All to Front | IDC_ALL_WINDOWS_FRONT (`arrangeInFront:`) | none | no |
| – | separator, followed by the AppKit window list | | | |

### 1.10 Help (main_menu_builder.mm:593-627), not in PWA; macOS adds its own search field
| # | Title | IDC | Mac key | Reserved |
|---|---|---|---|---|
| 1 | Report an Issue… (Google Chrome branding only) | IDC_FEEDBACK | Cmd+Opt+Shift+I (:113, branded) | no |
| 2 | Report an unsafe site… (branded, if ReportUnsafeSiteDialog is enabled for the profile) | IDC_REPORT_UNSAFE_SITE | none | no |
| 3 | Google Chrome Help | IDC_HELP_PAGE_VIA_MENU | Cmd+Shift+/ (Cmd+?) (:108) | no |

### 1.11 Mac keys that are not on any menu (global_keyboard_shortcuts_mac.mm:111-180, 81-92; accelerator_table.cc:57-60, 279-280)
| Key | IDC | Reserved |
|---|---|---|
| Cmd+Shift+] / Cmd+Shift+[ | IDC_SELECT_NEXT_TAB / IDC_SELECT_PREVIOUS_TAB (:117-118) | not in FS |
| Ctrl+PgDn / Ctrl+PgUp | IDC_SELECT_NEXT_TAB / IDC_SELECT_PREVIOUS_TAB (:119-120) | not in FS |
| Cmd+Opt+Right / Cmd+Opt+Left | IDC_SELECT_NEXT_TAB / IDC_SELECT_PREVIOUS_TAB (:121-122) | not in FS |
| Ctrl+Shift+PgDn / Ctrl+Shift+PgUp | IDC_MOVE_TAB_NEXT / IDC_MOVE_TAB_PREVIOUS (:123-124) | no |
| Cmd+1 … Cmd+8 (and keypad) | IDC_SELECT_TAB_0 … _7 (:127-142) | no |
| Cmd+9 (and keypad) | IDC_SELECT_LAST_TAB (:143-144) | no |
| Cmd+Shift+M | IDC_SHOW_AVATAR_MENU (:146) | no |
| Cmd+Opt+L | IDC_SHOW_DOWNLOADS, a second key for it (:147) | no |
| Cmd+Shift+C, Cmd+Opt+C | IDC_DEV_TOOLS_INSPECT (:148-149; Cmd+Opt+C is also the menu key) | no |
| Cmd+Opt+Down / Cmd+Opt+Up | IDC_FOCUS_NEXT_PANE / IDC_FOCUS_PREVIOUS_PANE (:150-151) | no |
| Cmd+Opt+Shift+A | IDC_FOCUS_INACTIVE_POPUP_FOR_ACCESSIBILITY (:152) | no |
| Cmd+Opt+R | IDC_SHOW_READING_MODE_KEYBOARD (:153) | no |
| Cmd+Ctrl+C / P / W / X / Z | IDC_ADD_NEW_TAB_TO_GROUP / IDC_CREATE_NEW_TAB_GROUP / IDC_CLOSE_TAB_GROUP / IDC_FOCUS_NEXT_TAB_GROUP / IDC_FOCUS_PREV_TAB_GROUP (:158-166) | no |
| Ctrl+Opt+Shift+T / V / M (only with the kUIDebugTools feature) | IDC_DEBUG_* (:168-175) | no |
| Cmd+Left / Cmd+Right ("delayed": only if the page doesn't consume them, e.g. not in a text field) | IDC_BACK / IDC_FORWARD (:87-88) | no |
| F7 | IDC_CARET_BROWSING_TOGGLE (accelerator_table.cc:58) | no |
| Esc | IDC_CLOSE_FIND_OR_STOP (accelerator_table.cc:60) | no |
| F12 | IDC_DEV_TOOLS_TOGGLE (accelerator_table.cc:280) | no |

---

## 2. Windows / Linux app menu (three-dot), `AppMenuModel::Build` (app_menu_model.cc:2053-2367)

The keys shown are the first match from `GetAcceleratorForCommandId` (accelerator_table.cc:401-419). Both platforms build the same tree; "Exit" depends on `browser_defaults::kShowExitMenuItem`.

| # | Item | IDC | Win/Linux key | Reserved |
|---|---|---|---|---|
| 0a | (cond.) Update Chrome, plus spacing separator | IDC_UPGRADE_DIALOG (2057-2071) | none | no |
| 0b | (cond.) Safety Hub notification / global-error items / "Set Chrome as your default browser", plus separator | notification command / error IDs / IDC_SET_BROWSER_AS_DEFAULT (2073-2076, 2400-2472) | none | no |
| 1 | New tab (New Incognito tab / New Isolated tab in those profiles) | IDC_NEW_TAB (2077-2087) | Ctrl+T | **not in FS** |
| 2 | New window | IDC_NEW_WINDOW (2089) | Ctrl+N | **not in FS** |
| 3 | New Incognito window (hidden in Guest) | IDC_NEW_INCOGNITO_WINDOW (2095-2102) | Ctrl+Shift+N | **not in FS** |
| 4 | (cond.) New Isolated window | IDC_NEW_ISOLATED_WINDOW (2104-2114) | none | **not in FS** |
| – | separator | | | |
| 5 | Profile ▸ (profile name + avatar; not on ChromeOS) (2119-2134) | kProfileMenuPlaceholder | | |
| 5.1 | sign-in section (title + sign-in/sync button), separator | info.command_id (app_menu_model.cc:663-680) | none | no |
| 5.2 | Manage your Google Account (branded, signed in) | IDC_MANAGE_GOOGLE_ACCOUNT (711-735) | none | no |
| 5.3 | Customize your Chrome | IDC_CUSTOMIZE_CHROME (689-698) | none | no |
| 5.4 | Close this profile (N windows) | IDC_CLOSE_PROFILE (700-709) | none | no |
| – | separator; "Other Chrome profiles" title; one item per profile (558-604) | dynamic | none | no |
| – | separator | | | |
| 5.5 | Add new profile | IDC_ADD_NEW_PROFILE (609-614) | none | no |
| 5.6 | Open Guest profile | IDC_OPEN_GUEST_PROFILE (682-687) | none | no |
| 5.7 | Manage Chrome profiles | IDC_MANAGE_CHROME_PROFILES (619-621) | none | no |
| – | spacing separator | | | |
| 6 | Passwords and autofill ▸ (not in Guest) (2136-2148) | kPasswordsAndAutofillMenuPlaceholder | | |
| 6.1 | Google Password Manager | IDC_SHOW_PASSWORD_MANAGER (743) | none | no |
| 6.2 | Payments | IDC_SHOW_PAYMENT_METHODS (751) | none | no |
| 6.3 | Contact info | IDC_SHOW_CONTACT_INFO (757) | none | no |
| 6.4 | Identity docs | IDC_SHOW_IDENTITY_DOCS (765) | none | no |
| 6.5 | Travel | IDC_SHOW_TRAVEL (771) | none | no |
| 7 | History ▸ (not off-the-record) (2150-2163), RecentTabsSubMenuModel (recent_tabs_sub_menu_model.cc:425-478) | kRecentTabsMenuPlaceholder | | |
| 7.1 | Open history page | IDC_SHOW_HISTORY | Ctrl+H | no |
| 7.2 | (cond.) Show history in side panel | IDC_SHOW_HISTORY_CLUSTERS_SIDE_PANEL | none | no |
| 7.3 | (cond.) Show tabs from other devices in side panel | IDC_SHOW_TABS_FROM_OTHER_DEVICES_SIDE_PANEL | none | no |
| – | separator; "Recent tabs" (shows the Ctrl+Shift+T hint when empty); recently closed entries; separator; "Your devices" + synced sessions or sign-in item | dynamic / IDC_RECENT_TABS_* | none | no |
| 8 | Downloads | IDC_SHOW_DOWNLOADS (2164) | Ctrl+J | no |
| 9 | Bookmarks and lists ▸ (not in Guest) (2169-2181), BookmarkSubMenuModel (bookmark_sub_menu_model.cc:46-110) | kBookmarksMenuPlaceholder | | |
| 9.1 | Bookmark this tab… | IDC_BOOKMARK_THIS_TAB | Ctrl+D | no |
| 9.2 | Bookmark all tabs… | IDC_BOOKMARK_ALL_TABS | Ctrl+Shift+D | no |
| – | separator | | | |
| 9.3 | Show/Hide bookmarks bar, or (kNtpSimplificationBookmarkBar) a "Bookmarks bar ▸" submenu: Always show / Always hide / Only show on New Tab Page | IDC_SHOW_BOOKMARK_BAR, or IDC_BOOKMARK_BAR_SUBMENU_* | Ctrl+Shift+B | no |
| 9.4 | Show bookmarks in side panel | IDC_SHOW_BOOKMARK_SIDE_PANEL | none | no |
| 9.5 | Open bookmark(s) manager | IDC_SHOW_BOOKMARK_MANAGER | Ctrl+Shift+O | no |
| 9.6 | Import bookmarks and settings… | IDC_IMPORT_SETTINGS | none | no |
| – | separator | | | |
| 9.7 | Reading list ▸: Add tab to reading list / Show reading list | IDC_READING_LIST_MENU_ADD_TAB / IDC_READING_LIST_MENU_SHOW_UI | none | no |
| – | (dynamic bookmark folders follow) | | | |
| 10 | Tab groups ▸ (regular profile; dynamic saved groups) (2183-2194) | kSavedTabGroupsMenuPlaceholder | none | no |
| 11 | Extensions ▸ (Manage extensions / Visit Chrome Web Store), or a flat "Explore Extensions" item when there are no extensions and kExtensionsCollapseMainMenu is on (2196-2218, 1169-1189) | IDC_EXTENSIONS_SUBMENU_MANAGE_EXTENSIONS / IDC_EXTENSIONS_SUBMENU_VISIT_CHROME_WEB_STORE / IDC_FIND_EXTENSIONS | none | no |
| 12 | Delete browsing data… | IDC_CLEAR_BROWSING_DATA (2220) | Ctrl+Shift+Delete | no |
| – | separator | | | |
| 13 | Zoom row: [−] [+] [fullscreen ⛶] (2386-2398) | IDC_ZOOM_MINUS / IDC_ZOOM_PLUS / IDC_FULLSCREEN | Ctrl+- / Ctrl+= / F11 | FULLSCREEN: FS only |
| – | separator | | | |
| 14 | Print… | IDC_PRINT (2231) | Ctrl+P | no |
| 15 | (cond.) Open Gemini in Chrome | IDC_OPEN_GLIC (2235-2244) | none | no |
| 16 | (cond.) Search with Google Lens | IDC_CONTENT_CONTEXT_LENS_OVERLAY (2246-2266) | none | no |
| 17 | Translate… | IDC_SHOW_TRANSLATE (2268) | none | no |
| 18 | Find and edit ▸ (2378-2384, 787-805) | kFindAndEditMenuPlaceholder | | |
| 18.1 | Find… | IDC_FIND | Ctrl+F | no |
| – | separator | | | |
| 18.2-4 | Cut / Copy / Paste | IDC_CUT / IDC_COPY / IDC_PASTE | Ctrl+X / Ctrl+C / Ctrl+V (labels only) | no |
| 19 | Cast, save, and share ▸ (or "Save and share" without Media Router) (2273-2284, 859-940) | kSaveAndShareMenuPlaceholder | | |
| 19.1 | "Cast" title, Cast…, separator (if Media Router) | IDC_ROUTE_MEDIA | none | no |
| 19.2 | "Save" title, Save page as…, separator | IDC_SAVE_PAGE | Ctrl+S | no |
| 19.3 | Install <app>… or Open in <app> | IDC_INSTALL_PWA / IDC_OPEN_IN_PWA_WINDOW | none | no |
| 19.4 | Create shortcut… | IDC_CREATE_SHORTCUT | none | no |
| 19.5 | separator, "Share" title, Copy link, Send to your devices (item or device submenu), Create QR code, Screenshot (cond.) | IDC_COPY_URL / IDC_SEND_TAB_TO_SELF / IDC_QRCODE_GENERATOR / IDC_SHARING_HUB_SCREENSHOT | none | no |
| 20 | More tools ▸ (2301-2308, 1046-1151) | kMoreToolsMenuPlaceholder | | |
| 20.1 | Tab search | IDC_TAB_SEARCH | Ctrl+Shift+A | no |
| 20.2 | Name window… | IDC_NAME_WINDOW | none | no |
| 20.3 | Show tabs vertically / horizontally | IDC_TOGGLE_VERTICAL_TABS | none | no |
| 20.4 | (cond.) Customize Chrome | IDC_SHOW_CUSTOMIZE_CHROME_SIDE_PANEL | none | no |
| – | separator | | | |
| 20.5 | Reading mode | IDC_SHOW_READING_MODE_SIDE_PANEL | (keyboard variant: Alt+Shift+R) | no |
| – | separator | | | |
| 20.6 | Performance | IDC_PERFORMANCE | none | no |
| 20.7 | Task manager | IDC_TASK_MANAGER_APP_MENU | (Shift+Esc goes to IDC_TASK_MANAGER_SHORTCUT) | no |
| – | separator | | | |
| 20.8 | Developer tools | IDC_DEV_TOOLS | Ctrl+Shift+I | no |
| – | (cond.) separator + Profiling enabled (check item) | IDC_PROFILING_ENABLED | none | no |
| – | (cond.) separator + Chrome Labs… | IDC_SHOW_CHROME_LABS | none | no |
| – | separator | | | |
| 21 | Help ▸ (branded) (2314-2320, 981-1027): About Google Chrome, What's new (cond.), Help center, Report an issue… (cond.), Report an unsafe site… (cond.). Unbranded builds show a flat "About Chromium" instead (2321-2330) | IDC_ABOUT / IDC_CHROME_WHATS_NEW / IDC_HELP_PAGE_VIA_MENU / IDC_FEEDBACK / IDC_REPORT_UNSAFE_SITE | Help center: F1 (via IDC_HELP_PAGE_VIA_KEYBOARD); Feedback: Alt+Shift+I | no |
| 22 | Settings | IDC_OPTIONS (2332) | none | no |
| 23 | Exit (if kShowExitMenuItem) | IDC_EXIT (2336-2340) | none (menu mnemonic Alt+F, X) | **always** |
| 24 | (managed) separator + "Managed by …" ; Linux: Chrome Enterprise release notes (cond.) | IDC_SHOW_MANAGEMENT_PAGE / IDC_CHROME_ENTERPRISE_RELEASE_NOTES (2344-2364) | none | no |

There is also a dead `CreateCutCopyPasteMenu()` (app_menu_model.cc:2369-2376): it is defined but never called.

---

## 3. Master table (union of all sources)

Columns: **Mac menu** = whether the action is on the Mac menu bar. **Needs**, one of four buckets:
- `content API` — content/public already has the call.
- `shell/Go only` — our own UI: tabs, windows, address bar, settings.
- `feature we don't have` — the feature is named.
- `OS provides`.

Where `content API` is given, content does the work but the shell still has to bind the key and the menu item. WebContents line refs are content/public/browser/web_contents.h.

Platform-only keys are marked (W) Windows-only, (L) Linux-only.

### 3.1 Tabs and windows
| Action id | Title | IDC | Mac keys | Windows keys | Linux keys | Reserved | Mac menu | Needs |
|---|---|---|---|---|---|---|---|---|
| tab.new | New Tab | IDC_NEW_TAB | Cmd+T | Ctrl+T; APPCOMMAND_NEW (W) | Ctrl+T; XF86New (VKEY_NEW) (L) | not in FS | yes | shell/Go only |
| tab.newToRight | New Tab to the Right / Below | IDC_NEW_TAB_TO_RIGHT | none | none | none | no | yes | shell/Go only |
| window.new | New Window | IDC_NEW_WINDOW | Cmd+N | Ctrl+N | Ctrl+N | not in FS | yes | shell/Go only |
| window.newIncognito | New Incognito Window | IDC_NEW_INCOGNITO_WINDOW | Cmd+Shift+N | Ctrl+Shift+N | Ctrl+Shift+N | not in FS | yes | feature we don't have: incognito (OTR BrowserContext + UI) |
| window.newIsolated | New Isolated Window | IDC_NEW_ISOLATED_WINDOW | none | none | none | not in FS | yes | feature we don't have: enterprise isolated mode |
| tab.reopenClosed | Reopen Closed Tab | IDC_RESTORE_TAB | Cmd+Shift+T | Ctrl+Shift+T | Ctrl+Shift+T | not in FS | yes | shell/Go only (closed-tab stack; content NavigationController::Restore for entries) |
| tab.close | Close Tab | IDC_CLOSE_TAB | Cmd+W | Ctrl+W, Ctrl+F4; APPCOMMAND_CLOSE (W) | Ctrl+W, Ctrl+F4; XF86Close (L) | not in FS | yes | shell/Go only |
| window.close | Close Window | IDC_CLOSE_WINDOW | Cmd+Shift+W (Cmd+W in PWA) | Ctrl+Shift+W, Alt+F4 | Ctrl+Shift+W, Alt+F4 | not in FS | yes | shell/Go only |
| window.closeAll | Close All | (`closeAll:`) | Cmd+Opt+Shift+W | none | none | no | yes (alt) | shell/Go only |
| app.quit | Quit / Exit | IDC_EXIT | Cmd+Q | none (Alt+F then X via menu) | none (Alt+F then X) | always | yes | shell/Go only |
| app.confirmToQuit | Warn Before Quitting | (`toggleConfirmToQuit:`) | none | none | none | no | yes | shell/Go only |
| tab.next | Select Next Tab (cycle) | IDC_CYCLE_TO_NEXT_TAB | Ctrl+Tab | Ctrl+Tab | Ctrl+Tab | not in FS | yes | shell/Go only |
| tab.previous | Select Previous Tab (cycle) | IDC_CYCLE_TO_PREV_TAB | Ctrl+Shift+Tab | Ctrl+Shift+Tab | Ctrl+Shift+Tab | not in FS | yes | shell/Go only |
| tab.selectNext | Select next tab (2nd IDC) | IDC_SELECT_NEXT_TAB | Cmd+Shift+], Cmd+Opt+Right, Ctrl+PgDn | Ctrl+PgDn | Ctrl+PgDn | not in FS | no | shell/Go only |
| tab.selectPrevious | Select previous tab (2nd IDC) | IDC_SELECT_PREVIOUS_TAB | Cmd+Shift+[, Cmd+Opt+Left, Ctrl+PgUp | Ctrl+PgUp | Ctrl+PgUp | not in FS | no | shell/Go only |
| tab.select1…tab.select8 | Select tab N (8 IDCs) | IDC_SELECT_TAB_0 … _7 | Cmd+1…8 (+keypad) | Ctrl+1…8 (+keypad) | Ctrl+1…8, Alt+1…8 (L) (+keypad) | no | no | shell/Go only |
| tab.selectLast | Select last tab | IDC_SELECT_LAST_TAB | Cmd+9 (+keypad) | Ctrl+9 (+keypad) | Ctrl+9, Alt+9 (L) (+keypad) | no | no | shell/Go only |
| tab.moveNext | Move tab right | IDC_MOVE_TAB_NEXT | Ctrl+Shift+PgDn | Ctrl+Shift+PgDn | Ctrl+Shift+PgDn | no | no | shell/Go only |
| tab.movePrevious | Move tab left | IDC_MOVE_TAB_PREVIOUS | Ctrl+Shift+PgUp | Ctrl+Shift+PgUp | Ctrl+Shift+PgUp | no | no | shell/Go only |
| tab.duplicate | Duplicate Tab | IDC_DUPLICATE_TAB | none | none | none | no | yes | content API: WebContents::Clone (:1178) |
| tab.duplicateSelected | Duplicate Selected Tab | IDC_DUPLICATE_TARGET_TAB | Option-alt | none | none | no | yes (alt) | content API: WebContents::Clone |
| tab.muteSite | Mute Site | IDC_WINDOW_MUTE_SITE | none | none | none | no | yes | content API: WebContents::SetAudioMuted (:958) (Chrome mutes per site via content settings; per-tab mute is enough for us) |
| tab.muteSelectedSite | Mute Selected Site | IDC_MUTE_TARGET_SITE | Option-alt | none | none | no | yes (alt) | content API: WebContents::SetAudioMuted |
| tab.pin | Pin Tab | IDC_WINDOW_PIN_TAB | none | none | none | no | yes | shell/Go only |
| tab.pinSelected | Pin Selected Tab | IDC_PIN_TARGET_TAB | Option-alt | none | none | no | yes (alt) | shell/Go only |
| tab.closeOthers | Close Other Tabs | IDC_WINDOW_CLOSE_OTHER_TABS | none | none | none | no | yes | shell/Go only |
| tab.closeToRight | Close Tabs to the Right / Below | IDC_WINDOW_CLOSE_TABS_TO_RIGHT | none | none | none | no | yes | shell/Go only |
| tab.moveToNewWindow | Move Tab to New Window | IDC_MOVE_TAB_TO_NEW_WINDOW | none | none | none | no | yes | shell/Go only |
| tab.search | Search Tabs… | IDC_TAB_SEARCH | Cmd+Shift+A | Ctrl+Shift+A | Ctrl+Shift+A | no | yes | shell/Go only |
| tab.list | Open-tab list (Tab menu tail) | (TabMenuBridge) | none | none | none | no | yes | shell/Go only |
| tabs.toggleVertical | Show Tabs Vertically / Horizontally | IDC_TOGGLE_VERTICAL_TABS | none | none | none | no | yes | shell/Go only |
| tabs.toggleVerticalCollapse | Collapse Vertical Tabs | IDC_TOGGLE_VERTICAL_TABS_COLLAPSE | Cmd+Shift+L | Ctrl+Shift+L | Ctrl+Shift+L | no | yes | shell/Go only |
| tab.newSplit | Add Tab to New Split View | IDC_NEW_SPLIT_TAB | Cmd+Opt+N | Alt+Shift+N | Alt+Shift+N | no | yes | feature we don't have: split view |
| tab.group | Group Tab | IDC_WINDOW_GROUP_TAB | none | none | none | no | yes | feature we don't have: tab groups |
| tab.groupSelected | Group Selected Tab | IDC_GROUP_TARGET_TAB | Option-alt | none | none | no | yes (alt) | feature we don't have: tab groups |
| tabGroup.addNewTab | Add new tab to group | IDC_ADD_NEW_TAB_TO_GROUP | Cmd+Ctrl+C | Alt+Shift+C | Alt+Shift+C | no | no | feature we don't have: tab groups |
| tabGroup.create | Create new tab group | IDC_CREATE_NEW_TAB_GROUP | Cmd+Ctrl+P | Alt+Shift+P | Alt+Shift+P | no | no | feature we don't have: tab groups |
| tabGroup.focusNext | Focus next tab group | IDC_FOCUS_NEXT_TAB_GROUP | Cmd+Ctrl+X | Alt+Shift+X | Alt+Shift+X | no | no | feature we don't have: tab groups |
| tabGroup.focusPrevious | Focus previous tab group | IDC_FOCUS_PREV_TAB_GROUP | Cmd+Ctrl+Z | Alt+Shift+Z | Alt+Shift+Z | no | no | feature we don't have: tab groups |
| tabGroup.close | Close tab group | IDC_CLOSE_TAB_GROUP | Cmd+Ctrl+W | Alt+Shift+W | Alt+Shift+W | no | no | feature we don't have: tab groups |
| tabGroups.saved | Tab groups ▸ (saved) | kSavedTabGroupsMenuPlaceholder | n/a | menu | menu | no | no | feature we don't have: saved tab groups (sync) |
| window.minimize | Minimize | IDC_MINIMIZE_WINDOW | Cmd+M | (Alt+Space, N: OS) | (WM) | no | yes | OS provides |
| window.zoom | Zoom (maximize) | IDC_MAXIMIZE_WINDOW | none | (Alt+Space, X: OS) | (WM) | no | yes | OS provides |
| window.bringAllToFront | Bring All to Front | IDC_ALL_WINDOWS_FRONT | none | n/a | n/a | no | yes | OS provides |
| window.list | Window list (Window menu tail) | (AppKit) | none | n/a | n/a | no | yes | OS provides |
| window.showAsTab | Show As Tab | IDC_SHOW_AS_TAB | none | none | none | no | yes | shell/Go only |
| window.name | Name Window… | IDC_NAME_WINDOW | none | none (More tools) | none (More tools) | no | yes | shell/Go only |
| window.fullscreen | Enter/Exit Full Screen | IDC_FULLSCREEN | Fn+Cmd+Ctrl+F (shown as Globe+F), Cmd+Ctrl+F | F11 | F11 | FS only | yes | shell/Go only (plus content WebContents::ExitFullscreen (:1562) and WebContentsDelegate::EnterFullscreenModeForTab for page fullscreen) |
| window.fullscreenToolbar | Always Show Toolbar in Full Screen | IDC_TOGGLE_FULLSCREEN_TOOLBAR | Cmd+Shift+F | none | none | no | yes | shell/Go only |
| app.hide | Hide Chrome | IDC_HIDE_APP | Cmd+H | n/a | n/a | no | yes | OS provides |
| app.hideOthers | Hide Others | (`hideOtherApplications:`) | Cmd+Opt+H | n/a | n/a | no | yes | OS provides |
| app.showAll | Show All | (`unhideAllApplications:`) | none | n/a | n/a | no | yes | OS provides |
| app.services | Services ▸ | (`servicesMenu`) | (per service) | n/a | n/a | no | yes | OS provides |
| view.customizeTouchBar | Customize Touch Bar | IDC_CUSTOMIZE_TOUCH_BAR | none | n/a | n/a | no | yes | OS provides |

### 3.2 Navigation, page, find, zoom
| Action id | Title | IDC | Mac keys | Windows keys | Linux keys | Reserved | Mac menu | Needs |
|---|---|---|---|---|---|---|---|---|
| nav.back | Back | IDC_BACK | Cmd+[, Cmd+Left (delayed); RTL swaps with Forward | Alt+Left, AltGr+Left; APPCOMMAND_BROWSER_BACKWARD (W) | Alt+Left, AltGr+Left; XF86Back (L) | no | yes | content API: NavigationController::GoBack |
| nav.forward | Forward | IDC_FORWARD | Cmd+], Cmd+Right (delayed) | Alt+Right, AltGr+Right; APPCOMMAND_BROWSER_FORWARD (W) | Alt+Right, AltGr+Right; XF86Forward (L) | no | yes | content API: NavigationController::GoForward |
| nav.home | Home | IDC_HOME | Cmd+Shift+H | Alt+Home; APPCOMMAND_BROWSER_HOME (W) | Alt+Home; XF86HomePage (L) | no | yes | shell/Go only (home-page setting, then LoadURL) |
| nav.reload | Reload This Page | IDC_RELOAD | Cmd+R | Ctrl+R, F5; APPCOMMAND_BROWSER_REFRESH (W) | Ctrl+R, F5; XF86Reload (L) | no | yes | content API: NavigationController::Reload(ReloadType::NORMAL) |
| nav.reloadHard | Force Reload This Page | IDC_RELOAD_BYPASSING_CACHE | Cmd+Shift+R | Ctrl+Shift+R, Ctrl+F5, Shift+F5 | Ctrl+Shift+R, Ctrl+F5, Shift+F5; Ctrl/Shift+XF86Reload (L) | no | yes (alt) | content API: NavigationController::Reload(ReloadType::BYPASSING_CACHE) |
| nav.stop | Stop | IDC_STOP | Cmd+. | (Esc via find.closeOrStop); APPCOMMAND_BROWSER_STOP (W) | (Esc) | no | yes | content API: WebContents::Stop (:1169) |
| find.closeOrStop | Close find bar, else stop loading | IDC_CLOSE_FIND_OR_STOP | Esc | Esc | Esc | no | no | content API: WebContents::StopFinding (:1497) / WebContents::Stop |
| nav.focusLocation | Open Location / focus address bar | IDC_FOCUS_LOCATION | Cmd+L | Ctrl+L, Alt+D (F6 cycles to it) | Ctrl+L, Alt+D | no | yes | shell/Go only |
| find.searchWeb | Search the Web… (omnibox in search mode) | IDC_FOCUS_SEARCH | Cmd+Opt+F | Ctrl+E, Ctrl+K, BrowserSearch key | Ctrl+E, Ctrl+K, XF86Search | no | yes | shell/Go only |
| file.open | Open File… | IDC_OPEN_FILE | Cmd+O | Ctrl+O; APPCOMMAND_OPEN (W) | Ctrl+O | no | yes | shell/Go only (file picker, then LoadURL file://) |
| page.save | Save Page As… | IDC_SAVE_PAGE | Cmd+S | Ctrl+S; APPCOMMAND_SAVE (W) | Ctrl+S | no | yes | content API: WebContents::SavePage (:1307) / OnSavePage (:1302) / GenerateMHTML |
| page.print | Print… | IDC_PRINT | Cmd+P | Ctrl+P; APPCOMMAND_PRINT (W) | Ctrl+P | no | yes | feature we don't have: printing / print preview (components/printing) |
| page.printSystemDialog | Print Using System Dialog… | IDC_BASIC_PRINT | Cmd+Opt+P | Ctrl+Shift+P | Ctrl+Shift+P | no | yes (alt) | feature we don't have: printing |
| page.share | Share ▸ (NSSharingService) | (ShareMenuController) | none | n/a | n/a | no | yes | OS provides |
| page.emailLink | Email Link | IDC_EMAIL_PAGE_LOCATION | Cmd+Shift+I | none | none | no | yes (Share ▸) | shell/Go only (open a mailto: URL) |
| page.copyUrl | Copy link | IDC_COPY_URL | none | menu | menu | no | no | shell/Go only |
| page.caretBrowsing | Caret browsing toggle | IDC_CARET_BROWSING_TOGGLE | F7 | F7 | F7 | no | no | content API: blink::RendererPreferences::caret_browsing_enabled (renderer_preferences.h:93) + WebContents::SyncRendererPrefs (:1164) |
| find.open | Find… | IDC_FIND | Cmd+F | Ctrl+F | Ctrl+F | no | yes | content API: WebContents::Find (:1490) (find bar UI is ours) |
| find.next | Find Next | IDC_FIND_NEXT | Cmd+G | Ctrl+G, F3 | Ctrl+G, F3 | no | yes | content API: WebContents::Find (find_next) |
| find.previous | Find Previous | IDC_FIND_PREVIOUS | Cmd+Shift+G | Ctrl+Shift+G, Shift+F3 | Ctrl+Shift+G, Shift+F3 | no | yes | content API: WebContents::Find (forward=false) |
| find.useSelection | Use Selection for Find | (`copyToFindPboard:`) | Cmd+E | n/a | n/a | no | yes | content API: RenderWidgetHostViewCocoa copyToFindPboard: (render_widget_host_view_cocoa.mm:2930) |
| find.jumpToSelection | Jump to Selection | (`centerSelectionInVisibleArea:`) | Cmd+J | n/a | n/a | no | yes | content API: RenderWidgetHostViewCocoa centerSelectionInVisibleArea: (:2934) |
| view.zoomIn | Zoom In | IDC_ZOOM_PLUS | Cmd++ (Cmd+Shift+=), Cmd+= | Ctrl+=, Ctrl+Shift+=, Ctrl+NumPad+ | Ctrl+=, Ctrl+Shift+=, Ctrl+NumPad+ | no | yes | content API: HostZoomMap (GetForWebContents, SetZoomLevelForHost / SetTemporaryZoomLevel) |
| view.zoomOut | Zoom Out | IDC_ZOOM_MINUS | Cmd+- | Ctrl+-, Ctrl+Shift+-, Ctrl+NumPad- | Ctrl+-, Ctrl+Shift+-, Ctrl+NumPad- | no | yes | content API: HostZoomMap |
| view.zoomReset | Actual Size | IDC_ZOOM_NORMAL | Cmd+0 | Ctrl+0, Ctrl+NumPad0 | Ctrl+0, Ctrl+NumPad0 | no | yes | content API: HostZoomMap |
| view.showFullUrls | Always Show Full URLs | IDC_SHOW_FULL_URLS | none | none | none | no | yes | shell/Go only |
| view.showLensShortcut | Always Show Google Lens Shortcut | IDC_SHOW_GOOGLE_LENS_SHORTCUT | none | none | none | no | yes | feature we don't have: Google Lens |
| view.showAiModeButton | Always Show AI Mode | IDC_SHOW_AI_MODE_OMNIBOX_BUTTON | none | none | none | no | yes | feature we don't have: AI Mode omnibox entry |
| view.showSearchTools | Show Search Shortcuts | IDC_SHOW_SEARCH_TOOLS | none | none | none | no | yes | feature we don't have: omnibox search shortcuts |

### 3.3 Edit
| Action id | Title | IDC | Mac keys | Windows keys | Linux keys | Reserved | Mac menu | Needs |
|---|---|---|---|---|---|---|---|---|
| edit.undo | Undo | IDC_CONTENT_CONTEXT_UNDO (Mac tag) | Cmd+Z | Ctrl+Z, Alt+Backspace (W) (Blink) | Ctrl+Z (Blink) | no | yes | content API: WebContents::Undo (:1185) |
| edit.redo | Redo | IDC_CONTENT_CONTEXT_REDO (Mac tag) | Cmd+Shift+Z | Ctrl+Y, Ctrl+Shift+Z, Alt+Shift+Backspace (W) (Blink) | Ctrl+Y, Ctrl+Shift+Z (Blink) | no | yes | content API: WebContents::Redo (:1186) |
| edit.cut | Cut | IDC_CUT / IDC_CONTENT_CONTEXT_CUT | Cmd+X | Ctrl+X, Shift+Del (Blink); APPCOMMAND_CUT (W) | Ctrl+X, Shift+Del (Blink) | no | yes | content API: WebContents::Cut (:1187) |
| edit.copy | Copy | IDC_COPY / IDC_CONTENT_CONTEXT_COPY | Cmd+C | Ctrl+C, Ctrl+Ins (Blink); APPCOMMAND_COPY (W) | Ctrl+C, Ctrl+Ins (Blink) | no | yes | content API: WebContents::Copy (:1188) |
| edit.paste | Paste | IDC_PASTE / IDC_CONTENT_CONTEXT_PASTE | Cmd+V | Ctrl+V, Shift+Ins (Blink); APPCOMMAND_PASTE (W) | Ctrl+V, Shift+Ins (Blink) | no | yes | content API: WebContents::Paste (:1191) |
| edit.pasteMatchStyle | Paste and Match Style | IDC_CONTENT_CONTEXT_PASTE_AND_MATCH_STYLE | Cmd+Shift+V, Cmd+Opt+Shift+V | Ctrl+Shift+V (Blink) | Ctrl+Shift+V (Blink) | no | yes | content API: WebContents::PasteAndMatchStyle (:1192) |
| edit.delete | Delete | IDC_CONTENT_CONTEXT_DELETE | none | (Del, Blink) | (Del, Blink) | no | yes | content API: WebContents::Delete (:1193) |
| edit.selectAll | Select All | IDC_CONTENT_CONTEXT_SELECTALL | Cmd+A | Ctrl+A (Blink) | Ctrl+A (Blink) | no | yes | content API: WebContents::SelectAll (:1194) |
| edit.spelling.showPanel | Show Spelling and Grammar | (`showGuessPanel:`) | Cmd+Shift+; | n/a | n/a | no | yes | feature we don't have: spellcheck (chrome_render_widget_host_view_mac_delegate.mm) |
| edit.spelling.checkNow | Check Document Now | (`checkSpelling:`) | Cmd+; | n/a | n/a | no | yes | feature we don't have: spellcheck |
| edit.spelling.whileTyping | Check Spelling While Typing | (`toggleContinuousSpellChecking:`) | none | n/a | n/a | no | yes | feature we don't have: spellcheck |
| edit.spelling.grammar | Check Grammar With Spelling | (`toggleGrammarChecking:`) | none | n/a | n/a | no | yes | feature we don't have: spellcheck |
| edit.subst.showPanel | Show Substitutions | (`orderFrontSubstitutionsPanel:`) | none | n/a | n/a | no | yes | content API: RenderWidgetHostViewCocoa (:685) |
| edit.subst.smartQuotes | Smart Quotes | (`toggleAutomaticQuoteSubstitution:`) | none | n/a | n/a | no | yes | content API: RenderWidgetHostViewCocoa (:2213) |
| edit.subst.smartDashes | Smart Dashes | (`toggleAutomaticDashSubstitution:`) | none | n/a | n/a | no | yes | content API: RenderWidgetHostViewCocoa (:2229) |
| edit.subst.textReplacement | Text Replacement | (`toggleAutomaticTextReplacement:`) | none | n/a | n/a | no | yes | content API: RenderWidgetHostViewCocoa (:2249) |
| edit.transform.upper | Make Uppercase | (`uppercaseWord:`) | none | n/a | n/a | no | yes | content API: RenderWidgetHostViewCocoa (:702) |
| edit.transform.lower | Make Lowercase | (`lowercaseWord:`) | none | n/a | n/a | no | yes | content API: RenderWidgetHostViewCocoa (:711) |
| edit.transform.capitalize | Capitalize | (`capitalizeWord:`) | none | n/a | n/a | no | yes | content API: RenderWidgetHostViewCocoa (:720) |
| edit.speech.start | Start Speaking | (`startSpeaking:`) | none | n/a | n/a | no | yes | content API: RenderWidgetHostViewCocoa (:2957) |
| edit.speech.stop | Stop Speaking | (`stopSpeaking:`) | none | n/a | n/a | no | yes | content API: RenderWidgetHostViewCocoa (:2961) |
| edit.osExtras | AutoFill / Start Dictation / Emoji & Symbols | (AppKit-inserted) | Fn/Globe+E etc. | Win+. (OS) | (IME) | no | yes | OS provides |

### 3.4 Focus, menus, accessibility
| Action id | Title | IDC | Mac keys | Windows keys | Linux keys | Reserved | Mac menu | Needs |
|---|---|---|---|---|---|---|---|---|
| focus.nextPane | Focus next pane (toolbar, bookmarks, web, dialogs) | IDC_FOCUS_NEXT_PANE | Cmd+Opt+Down | F6 | F6 | no | no | shell/Go only |
| focus.previousPane | Focus previous pane | IDC_FOCUS_PREVIOUS_PANE | Cmd+Opt+Up | Shift+F6 | Shift+F6 | no | no | shell/Go only |
| focus.webContents | Skip to web contents | IDC_FOCUS_WEB_CONTENTS_PANE | none | Ctrl+F6 | Ctrl+F6 | no | no | content API: WebContents::Focus (:1269) |
| focus.toolbar | Focus first toolbar item | IDC_FOCUS_TOOLBAR | none | Alt+Shift+T | Alt+Shift+T | no | no | shell/Go only |
| focus.menuBar | Focus app-menu button | IDC_FOCUS_MENU_BAR | (Ctrl+F2: OS menu bar) | F10, Alt (tap) | F10, Alt (tap) | no | no | shell/Go only |
| menu.showApp | Open the three-dot menu | IDC_SHOW_APP_MENU | n/a | Alt+F, Alt+E | Alt+F, Alt+E | no | no | shell/Go only |
| focus.inactivePopup | Focus inactive popup/dialog | IDC_FOCUS_INACTIVE_POPUP_FOR_ACCESSIBILITY | Cmd+Opt+Shift+A | Alt+Shift+A | Alt+Shift+A | no | no | shell/Go only |
| focus.bookmarks | Focus bookmarks bar | IDC_FOCUS_BOOKMARKS | none | Alt+Shift+B | Alt+Shift+B | no | no | feature we don't have: bookmarks |

### 3.5 Developer
| Action id | Title | IDC | Mac keys | Windows keys | Linux keys | Reserved | Mac menu | Needs |
|---|---|---|---|---|---|---|---|---|
| dev.viewSource | View Source | IDC_VIEW_SOURCE | Cmd+Opt+U | Ctrl+U | Ctrl+U | no | yes | content API: view-source: URL scheme (NavigationController::LoadURL) |
| dev.tools | Developer Tools | IDC_DEV_TOOLS | Cmd+Opt+I | Ctrl+Shift+I | Ctrl+Shift+I | no | yes | content API: DevToolsAgentHost::GetOrCreateForTab + a DevTools frontend host (content_shell pattern) |
| dev.toggle | Toggle DevTools | IDC_DEV_TOOLS_TOGGLE | F12 | F12 | F12 | no | no | content API: DevToolsAgentHost |
| dev.inspect | Inspect Elements | IDC_DEV_TOOLS_INSPECT | Cmd+Opt+C, Cmd+Shift+C | Ctrl+Shift+C | Ctrl+Shift+C | no | yes | content API: DevToolsAgentHost (inspect mode) |
| dev.console | JavaScript Console | IDC_DEV_TOOLS_CONSOLE | Cmd+Opt+J | Ctrl+Shift+J | Ctrl+Shift+J | no | yes | content API: DevToolsAgentHost (console panel) |
| dev.appleEventsJs | Allow JavaScript from Apple Events | IDC_TOGGLE_JAVASCRIPT_APPLE_EVENTS | none | n/a | n/a | no | yes | feature we don't have: AppleScript support |
| dev.taskManager | Task Manager | IDC_TASK_MANAGER_MAIN_MENU / _APP_MENU / _SHORTCUT | none | Shift+Esc | Shift+Esc | no | yes | feature we don't have: task manager |
| dev.performance | Performance | IDC_PERFORMANCE | n/a | menu | menu | no | no | feature we don't have: performance settings (memory saver) |
| dev.profiling | Profiling enabled | IDC_PROFILING_ENABLED | n/a | menu | menu | no | no | feature we don't have: profiling build support |
| dev.chromeLabs | Chrome Labs… | IDC_SHOW_CHROME_LABS | n/a | menu | menu | no | no | feature we don't have: Chrome Labs |
| debug.uiTools | Toggle tablet mode / print view tree / details / window / layer hierarchy (5 IDCs, kUIDebugTools only) | IDC_DEBUG_* | Ctrl+Opt+Shift+T/V/M (+W/L from the shared table) | Ctrl+Alt+Shift+T/V/M/W/L | Ctrl+Alt+Shift+T/V/M/W/L | no | no | feature we don't have: views UI debug tools |

### 3.6 Profiles, bookmarks, history, downloads, other Chrome features
| Action id | Title | IDC | Mac keys | Windows keys | Linux keys | Reserved | Mac menu | Needs |
|---|---|---|---|---|---|---|---|---|
| app.about | About | IDC_ABOUT | none | menu | menu | no | yes | shell/Go only |
| app.settings | Settings / Preferences | IDC_OPTIONS (IDC_WEB_APP_SETTINGS in PWA) | Cmd+, | none | none | no | yes | shell/Go only |
| app.clearBrowsingData | Delete Browsing Data… | IDC_CLEAR_BROWSING_DATA | Cmd+Shift+Backspace | Ctrl+Shift+Del | Ctrl+Shift+Del | no | yes | content API: BrowsingDataRemover (content/public/browser/browsing_data_remover.h) |
| app.importSettings | Import Bookmarks and Settings… | IDC_IMPORT_SETTINGS | none | menu | menu | no | yes | feature we don't have: importer |
| app.setDefaultBrowser | Set as default browser | IDC_SET_BROWSER_AS_DEFAULT | n/a | menu (cond.) | menu (cond.) | no | no | shell/Go only |
| app.upgrade | Update Chrome | IDC_UPGRADE_DIALOG | n/a | menu (cond.) | menu (cond.) | no | no | feature we don't have: updater |
| app.safetyHub | Safety Hub notification | (dynamic) | n/a | menu (cond.) | menu (cond.) | no | no | feature we don't have: Safety Hub |
| app.managed | Managed by your organization / Enterprise release notes | IDC_SHOW_MANAGEMENT_PAGE / IDC_CHROME_ENTERPRISE_RELEASE_NOTES | n/a | menu (cond.) | menu (cond.) | no | no | feature we don't have: enterprise management |
| help.page | Chrome Help / Help center | IDC_HELP_PAGE_VIA_MENU / IDC_HELP_PAGE_VIA_KEYBOARD | Cmd+Shift+/ | F1; APPCOMMAND_HELP (W) | F1 | no | yes | shell/Go only |
| help.feedback | Report an Issue… | IDC_FEEDBACK | Cmd+Opt+Shift+I (branded) | Alt+Shift+I (branded) | Alt+Shift+I (branded) | no | yes | feature we don't have: feedback |
| help.reportUnsafeSite | Report an unsafe site… | IDC_REPORT_UNSAFE_SITE | none | menu (cond.) | menu (cond.) | no | yes | feature we don't have: Safe Browsing reporting |
| help.whatsNew | What's new | IDC_CHROME_WHATS_NEW | n/a | menu (branded) | menu (branded) | no | no | feature we don't have: What's New |
| profiles.menu | Profiles ▸ / Profile ▸ | IDC_PROFILE_MAIN_MENU / kProfileMenuPlaceholder | none | menu | menu | no | yes | feature we don't have: profiles |
| profiles.avatarMenu | Open avatar/profile menu | IDC_SHOW_AVATAR_MENU | Cmd+Shift+M | Ctrl+Shift+M | Ctrl+Shift+M | no | no | feature we don't have: profiles |
| profiles.add | Add new profile | IDC_ADD_NEW_PROFILE | (People menu) | menu | menu | no | yes | feature we don't have: profiles |
| profiles.manage | Manage profiles | IDC_MANAGE_CHROME_PROFILES | (People menu) | menu | menu | no | yes | feature we don't have: profiles |
| profiles.guest | Open Guest profile | IDC_OPEN_GUEST_PROFILE | n/a | menu | menu | no | no | feature we don't have: guest mode |
| profiles.customize | Customize your Chrome | IDC_CUSTOMIZE_CHROME | n/a | menu | menu | no | no | feature we don't have: profiles |
| profiles.close | Close this profile | IDC_CLOSE_PROFILE | n/a | menu | menu | no | no | feature we don't have: profiles |
| account.manage | Manage your Google Account | IDC_MANAGE_GOOGLE_ACCOUNT | n/a | menu | menu | no | no | feature we don't have: Google account / sync |
| sync.signin | Sign-in / sync row | (sign-in section) | n/a | menu | menu | no | no | feature we don't have: sync |
| autofill.passwords | Google Password Manager | IDC_SHOW_PASSWORD_MANAGER | n/a | menu | menu | no | no | feature we don't have: password manager |
| autofill.payments | Payments | IDC_SHOW_PAYMENT_METHODS | n/a | menu | menu | no | no | feature we don't have: autofill |
| autofill.contactInfo | Contact info | IDC_SHOW_CONTACT_INFO | n/a | menu | menu | no | no | feature we don't have: autofill |
| autofill.identityDocs | Identity docs | IDC_SHOW_IDENTITY_DOCS | n/a | menu | menu | no | no | feature we don't have: autofill |
| autofill.travel | Travel | IDC_SHOW_TRAVEL | n/a | menu | menu | no | no | feature we don't have: autofill |
| history.show | Show Full History / Open history page | IDC_SHOW_HISTORY | Cmd+Y | Ctrl+H | Ctrl+H | no | yes | feature we don't have: history page |
| history.recentlyClosed | Recently Closed list | (HistoryMenuBridge / RecentTabsSubMenuModel) | none | menu | menu | no | yes | shell/Go only |
| history.recentlyVisited | Recently Visited list | (HistoryMenuBridge) | none | n/a | n/a | no | yes | feature we don't have: history service |
| history.sidePanel | Show history in side panel | IDC_SHOW_HISTORY_CLUSTERS_SIDE_PANEL | n/a | menu (cond.) | menu (cond.) | no | no | feature we don't have: history clusters / side panel |
| history.otherDevices | Tabs from other devices / Your devices | IDC_SHOW_TABS_FROM_OTHER_DEVICES_SIDE_PANEL, IDC_RECENT_TABS_* | n/a | menu | menu | no | no | feature we don't have: sync (sessions) |
| bookmarks.thisTab | Bookmark This Tab… | IDC_BOOKMARK_THIS_TAB | Cmd+D | Ctrl+D | Ctrl+D | no | yes | feature we don't have: bookmarks |
| bookmarks.allTabs | Bookmark All Tabs… | IDC_BOOKMARK_ALL_TABS | Cmd+Shift+D | Ctrl+Shift+D | Ctrl+Shift+D | no | yes | feature we don't have: bookmarks |
| bookmarks.manager | Bookmark Manager | IDC_SHOW_BOOKMARK_MANAGER | Cmd+Opt+B | Ctrl+Shift+O | Ctrl+Shift+O | no | yes | feature we don't have: bookmarks |
| bookmarks.toggleBar | Show/Hide bookmarks bar | IDC_SHOW_BOOKMARK_BAR (+ IDC_BOOKMARK_BAR_SUBMENU_*) | Cmd+Shift+B | Ctrl+Shift+B | Ctrl+Shift+B, XF86Favorites (L) | no | yes | feature we don't have: bookmarks |
| bookmarks.sidePanel | Show bookmarks in side panel | IDC_SHOW_BOOKMARK_SIDE_PANEL | n/a | menu | menu | no | no | feature we don't have: bookmarks / side panel |
| bookmarks.tree | Bookmark folders (dynamic) | (BookmarkMenuBridge / BookmarkSubMenuModel) | none | menu | menu | no | yes | feature we don't have: bookmarks |
| readingList.addTab | Add tab to reading list | IDC_READING_LIST_MENU_ADD_TAB | n/a | menu | menu | no | no | feature we don't have: reading list |
| readingList.show | Show reading list | IDC_READING_LIST_MENU_SHOW_UI | n/a | menu | menu | no | no | feature we don't have: reading list |
| readingMode.show | Reading mode | IDC_SHOW_READING_MODE_SIDE_PANEL / IDC_SHOW_READING_MODE_KEYBOARD | Cmd+Opt+R | Alt+Shift+R | Alt+Shift+R | no | no | feature we don't have: reading mode |
| downloads.show | Downloads | IDC_SHOW_DOWNLOADS | Cmd+Shift+J, Cmd+Opt+L | Ctrl+J | Ctrl+J | no | yes | feature we don't have: downloads page (content DownloadManager is the backend) |
| extensions.manage | Extensions / Manage extensions | IDC_MANAGE_EXTENSIONS / IDC_EXTENSIONS_SUBMENU_MANAGE_EXTENSIONS | none | menu | menu | no | yes | feature we don't have: extensions |
| extensions.webStore | Visit Chrome Web Store / Explore Extensions | IDC_EXTENSIONS_SUBMENU_VISIT_CHROME_WEB_STORE / IDC_FIND_EXTENSIONS | n/a | menu | menu | no | no | feature we don't have: extensions |
| media.cast | Cast… | IDC_ROUTE_MEDIA | none | menu | menu | no | yes | feature we don't have: Cast (media router) |
| page.translate | Translate… | IDC_SHOW_TRANSLATE | n/a | menu | menu | no | no | feature we don't have: Translate |
| page.lens | Search with Google Lens | IDC_CONTENT_CONTEXT_LENS_OVERLAY | n/a | menu (cond.) | menu (cond.) | no | no | feature we don't have: Lens overlay |
| glic.open | Open Gemini in Chrome | IDC_OPEN_GLIC | n/a | menu (cond.) | menu (cond.) | no | no | feature we don't have: Gemini (glic) |
| webapp.install | Install app / Open in app | IDC_INSTALL_PWA / IDC_OPEN_IN_PWA_WINDOW | n/a | menu | menu | no | no | feature we don't have: web apps (PWA) |
| webapp.createShortcut | Create shortcut… | IDC_CREATE_SHORTCUT | n/a | menu | menu | no | no | feature we don't have: web apps / OS shortcuts |
| share.sendTabToSelf | Send to your devices | IDC_SEND_TAB_TO_SELF | n/a | menu | menu | no | no | feature we don't have: sync (send tab to self) |
| share.qrCode | Create QR code | IDC_QRCODE_GENERATOR | n/a | menu | menu | no | no | feature we don't have: QR code generator |
| share.screenshot | Screenshot | IDC_SHARING_HUB_SCREENSHOT | n/a | menu (cond.) | menu (cond.) | no | no | feature we don't have: sharing-hub screenshots |
| ntp.customize | Customize Chrome (side panel) | IDC_SHOW_CUSTOMIZE_CHROME_SIDE_PANEL | n/a | menu (cond.) | menu (cond.) | no | no | feature we don't have: NTP customization / side panel |

### 3.7 Keys on Google's help page that are not browser accelerators (omnibox, Blink, OS, mouse)
| Action id | Title | IDC | Mac keys | Windows keys | Linux keys | Reserved | Mac menu | Needs |
|---|---|---|---|---|---|---|---|---|
| omnibox.go | Navigate / search | none (omnibox) | Return | Enter | Enter | n/a | no | shell/Go only |
| omnibox.keyword | Search with a different engine | none (omnibox) | Tab after the engine keyword | Tab | Tab | n/a | no | shell/Go only |
| omnibox.completeCom | Add www. and .com, open | none (omnibox) | Ctrl+Return | Ctrl+Enter | Ctrl+Enter | n/a | no | shell/Go only |
| omnibox.completeComNewWindow | Add www./.com, open in new window | none (omnibox) | Ctrl+Shift+Return | Ctrl+Shift+Enter | Ctrl+Shift+Enter | n/a | no | shell/Go only |
| omnibox.openNewTab | Open in new tab (help page: Mac background tab) | none (omnibox) | Cmd+Return | Alt+Enter | Alt+Enter | n/a | no | shell/Go only |
| omnibox.removeSuggestion | Remove highlighted prediction | none (omnibox) | Shift+Fn+Delete | Shift+Del | Shift+Del | n/a | no | shell/Go only |
| tabstrip.keyboardNav | Tab-strip keyboard nav: F6 into tabs, Tab/Shift+Tab, Menu key, Space/Enter to collapse a group, Ctrl+Left/Right to move, "F6 twice then Shift+Ctrl+H" multi-select | none (tab strip views) | Tab (VoiceOver) | as listed | as listed | n/a | no | shell/Go only |
| page.scroll | Scroll: Space / Shift+Space / PgDn / PgUp / Home / End / arrows / Shift+wheel horizontal | none (Blink) | Space, Shift+Space | same | same | n/a | no | content API: Blink default key and wheel handling (nothing to do) |
| page.tabFocus | Next/previous focusable item | none (Blink) | Tab / Shift+Tab | same | same | n/a | no | content API: Blink focus + WebContentsDelegate::TakeFocus to leave the page |
| text.wordNav | Word left/right, delete word | none (Blink/OS) | Opt+Left/Right, Opt+Delete | Ctrl+Left/Right, Ctrl+Backspace | same | n/a | no | content API: Blink editing (nothing to do) |
| view.zoomWheel | Zoom with Ctrl/Cmd + wheel or pinch | none | Cmd+wheel, pinch | Ctrl+wheel | Ctrl+wheel | n/a | no | content API: WebContentsDelegate::ContentsZoomChange (web_contents_delegate.h:253) + HostZoomMap |
| mouse.linkDispositions | Ctrl/Cmd+click background tab, +Shift foreground, Shift+click new window, Alt/Opt+click download | none | Cmd(+Shift)+click, Shift+click, Opt+click | Ctrl(+Shift)+click, Shift+click, Alt+click | same | n/a | no | content API: WebContentsDelegate::OpenURLFromTab with WindowOpenDisposition (download: DownloadManager) |
| mouse.backHistory | Right-click or long-press Back/Forward for the history list | none | mouse | mouse | mouse | n/a | no | shell/Go only (NavigationController entries) |
| mouse.tabDrag | Drag tabs/links between windows, Esc cancels, double-click strip to maximize | none | mouse | mouse | mouse | n/a | no | shell/Go only |
| window.exitFullscreenHoldEsc | Press and hold Esc to exit fullscreen | (Esc pressed/released accelerators, browser_view.cc:5652-5668) | hold Esc | hold Esc | hold Esc | n/a | no | shell/Go only (plus content keyboard lock) |
| os.windowMenu | Windows system menu (Alt+Space, then N/X) | none | n/a | Alt+Space | (WM) | n/a | no | OS provides |
| os.macMenuBarFocus | Focus macOS menu bar / Full Keyboard Access | none | Ctrl+F2, Cmd+F7 | n/a | n/a | n/a | no | OS provides |

---

## 4. Counts

Rows in the master table (section 3), with grouped rows counted once. tab.select1…8 counts as 1 row covering 8 IDCs. debug.uiTools is 1 row covering 5 IDCs.

| Bucket | 3.1 tabs/windows | 3.2 nav/page | 3.3 edit | 3.4 focus | 3.5 dev | 3.6 features | 3.7 non-accel | **Total** |
|---|---|---|---|---|---|---|---|---|
| content API | 4 | 16 | 17 | 1 | 5 | 1 | 5 | **49** |
| shell/Go only | 30 | 7 | 0 | 6 | 0 | 5 | 10 | **58** |
| feature we don't have | 11 | 5 | 4 | 1 | 6 | 47 | 0 | **74** |
| OS provides | 9 | 1 | 1 | 0 | 0 | 0 | 2 | **13** |
| **Rows** | 54 | 29 | 22 | 8 | 11 | 53 | 17 | **194** |

Without section 3.7 (pure Chrome commands, menu items and accelerators only): **177** rows. That is 44 content API, 48 shell/Go only, 74 feature we don't have, 11 OS provides. Counted mechanically from the "Needs" column.

Mac main menu (section 1, excluding the 10 top-level titles and separators): 120 rows. 6 of them are submenu headers, 1 is Services, and 2 are History section headers. 90 rows carry an IDC tag, and 21 leaf items are pure AppKit selectors with no IDC. There are 17 non-menu Mac key rows in 1.11.

---

## 5. Cross-check against Google's help page (support.google.com/chrome/answer/157179)

**On the page and in source:** almost every table row matches the source. The page's Mac F7 caret browsing and Esc stop both match the shared accelerator table entries (accelerator_table.cc:58, 60).

**On the page but not Chrome accelerators** (handled elsewhere; all are in 3.7):
- Omnibox: Ctrl+Enter, Ctrl+Shift+Enter, Alt+Enter (Mac: Cmd+Return background tab), Tab keyword search, Shift+Delete to remove a prediction.
- Blink: Space/Shift+Space/PgUp/PgDn/Home/End scrolling, Tab/Shift+Tab, Ctrl/Opt+arrow word moves, Ctrl+Backspace / Opt+Delete.
- Zoom: Ctrl+mouse wheel goes through `WebContentsDelegate::ContentsZoomChange`.
- OS: Windows Alt+Space N/X; macOS Ctrl+F2, Cmd+F7, Fn+F, Cmd+M, Cmd+H.
- Menu mnemonics: Alt+F then X to quit.
- Tab-strip keyboard navigation: F6 into tabs, Menu key, Space/Enter to collapse, Ctrl+Left/Right to move.

**Page vs source disagreements:**
- Page "Ctrl+F5: Move cursor to the address bar" (both tabs). Source: Ctrl+F5 = IDC_RELOAD_BYPASSING_CACHE (accelerator_table.cc:150), with no Mac binding. The page looks wrong. On Mac, Ctrl+F5 is the macOS "move focus to window toolbar" key.
- Page "Ctrl+Shift+J or F12: Open Developer Tools". Source: Ctrl+Shift+J = IDC_DEV_TOOLS_CONSOLE (:283). Plain DevTools is Ctrl+Shift+I (:282), which the page omits.
- Page "Turn on full-screen: Fn+F" (Mac). Source: the menu key is Cmd+Ctrl+F with the Fn flag (accelerators_cocoa.mm:132-135), plus the alternate Cmd+Ctrl+F.
- Page "Open the Page Setup dialog: Cmd+Opt+P". Source: that key is "Print Using System Dialog…" (IDC_BASIC_PRINT, accelerators_cocoa.mm:80).
- Page "Log in a different user or browse as Guest: Ctrl+Shift+M". Source: IDC_SHOW_AVATAR_MENU (opens the profile menu) (:164).
- Page "Next tab: Cmd+Opt+Right" only. Source also has Cmd+Shift+], Ctrl+PgDn and Ctrl+Tab (the menu item).

**In source but missing from the page:**
- Tab search: Ctrl/Cmd+Shift+A.
- Collapse vertical tabs: Ctrl/Cmd+Shift+L.
- Reading mode: Alt+Shift+R / Cmd+Opt+R.
- Focus bookmarks bar: Alt+Shift+B.
- System print dialog: Ctrl+Shift+P.
- Tab-group keys: Alt+Shift+C/P/X/Z/W, and Cmd+Ctrl+C/P/X/Z/W on Mac.
- Inspect: Ctrl+Shift+C, Cmd+Opt+C, Cmd+Shift+C.
- DevTools: Ctrl+Shift+I.
- Search the web: Ctrl+E / Cmd+Opt+F (the page has only Ctrl+K/E on Windows).
- Mac: Cmd+. stop, Cmd+Shift+F toolbar in full screen, Cmd+Shift+/ help, Cmd+Opt+L downloads, Cmd+Shift+[ / ] tabs, Cmd+Opt+B bookmark manager, Cmd+Shift+V / Cmd+Opt+Shift+V paste and match style, Cmd+E / Cmd+J find helpers, Cmd+; spelling.
- Alt alone / F10 to focus the menu button.
- Windows APPCOMMAND keys; Linux XF86 Back/Forward/Reload/HomePage/Search/Favorites/New/Close keys and Alt+1…9.
- Shift+F3 find previous; Shift+F6 previous pane.
- (Split view is on the page: Shift+Alt+N, and ⌘+Option+N on Mac.)

---

## 6. ChromeOS-only (excluded from the master table)
- Ctrl+F7 caret browsing (accelerator_table.cc:160)
- Print key → print (:185)
- Search+Ctrl+I feedback (:190)
- Ctrl+Shift+Backspace delete browsing data (:195)
- Ctrl+/ and Ctrl+Shift+/ help (:204-206)
- BrowserFavorites → bookmark manager (:207)
- BrowserStop → stop (:208)
- Search+Esc task manager (:210)
- Search+Z multitask menu (:211)
- Ctrl+Alt+N split view (:332-334)
- Top-row BrowserBack/Forward/Refresh are reserved (browser_command_controller.cc:486-498)
