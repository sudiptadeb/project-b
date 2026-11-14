// The boundary between the Go core and the C++ adapter: plain C, ids and values.
// Content's types never cross it, and each side only holds a table of the other's
// functions, so the library has no link-time reference to Go.
//
// Everything the adapter tells Go goes through one call, hook: a number and a flat
// list of values. The numbers are the adapter's own events below (PBEvent) and
// content's hooks from 100 up, generated into engine/browser/hooks_gen.h and
// core/content/hooks_gen.go by tools/gen_hooks.py.
//
// Threading: every call in both directions happens on the main thread, except
// post_wakeup and the timeline's calls, which any thread may make.
//
// Strings are only valid for the length of the call. Whoever keeps one copies it.

#ifndef PROJECT_B_ENGINE_ABI_PB_ABI_H_
#define PROJECT_B_ENGINE_ABI_PB_ABI_H_

#include <stddef.h>
#include <stdint.h>

// One value in a hook's argument list or answer. Which field is used follows from the
// hook; the generated code on both sides agrees. A string answer from Go is malloc'd
// and the adapter frees it.
typedef struct PBValue {
  int64_t i;      // integers, booleans, enums, ids
  double d;       // floating point
  const char* s;  // UTF-8 text, not NUL-terminated
  size_t len;
} PBValue;

// The adapter's own events, which are not content hooks. Their values are listed
// beside each. Matches core/content/events.go.
typedef enum PBEvent {
  PB_EVENT_SHELL_MESSAGE = 1,           // i: window, s: json
  PB_EVENT_TAB_OPENED = 2,              // i: opener, i: tab, s: url, s: its workspace,
                                        // "" for the profile's own storage; i: foreground,
                                        // i: in a new window of its own
  PB_EVENT_FAVICON_CHANGED = 3,         // i: tab, s: data url or ""
  PB_EVENT_WINDOW_CHANGED = 4,          // i: window; d: w, d: h, the content area; d: x,
                                        // d: y, d: w, d: h, the window buttons; d: corner
                                        // radius; i: full screen; d: x, d: y, d: w, d: h,
                                        // its frame on screen, y down from the top of the
                                        // main screen; i: minimised; i: zoomed
  PB_EVENT_PING = 5,                    // nothing; the driver measures the crossing
  PB_EVENT_SHELL_CRASHED = 6,           // i: window; its shell page's renderer died
  PB_EVENT_SETTINGS_MESSAGE = 7,        // i: window, s: json, from its settings page
  PB_EVENT_MENU_CHOSEN = 8,             // s: the chosen item's action id
  PB_EVENT_OS_BINDINGS = 9,             // s: json, {title: {code, mods}}, the keys the
                                        // OS gives our menu items (macOS App Shortcuts)
  PB_EVENT_MODS_CHANGED = 10,           // i: the modifiers held now, PBMods
  PB_EVENT_PROCESS_CRASHED = 11,        // s: process type, i: base::TerminationStatus,
                                        // i: exit code, i: pid, s: a renderer's page or ""
  PB_EVENT_DEVTOOLS_COMMAND = 12,       // s: method, s: params json; answers s: the
                                        // reply's body, {"result": ...} or {"error": ...}.
                                        // Only ProjectB.* methods; no answer leaves the
                                        // command to content
  PB_EVENT_WINDOW_CLOSED = 13,          // i: window; it closed, asked by Go or by its own
                                        // close button, and its pages went with it
  PB_EVENT_WINDOW_FOCUSED = 14,         // i: window; it became the key window
  PB_EVENT_MENU_LIST = 15,              // s: a menu list's name, as Go's menu names it;
                                        // answers s: json, Go's engine.Menu, its items
                                        // now. Asked each time a menu holding it opens
  PB_EVENT_PICKER_MESSAGE = 16,         // i: window, s: json, from its picker page
} PBEvent;

// Modifier bits, as Go's engine.Mods has them.
typedef enum PBMods {
  PB_MOD_CTRL = 1,
  PB_MOD_ALT = 2,  // Option on a Mac
  PB_MOD_SHIFT = 4,
  PB_MOD_CMD = 8,
} PBMods;

typedef enum PBEditCommand {
  PB_EDIT_UNDO = 1,
  PB_EDIT_REDO = 2,
  PB_EDIT_CUT = 3,
  PB_EDIT_COPY = 4,
  PB_EDIT_PASTE = 5,
  PB_EDIT_PASTE_AND_MATCH_STYLE = 6,
  PB_EDIT_DELETE = 7,
  PB_EDIT_SELECT_ALL = 8,
} PBEditCommand;

typedef enum PBAppCommand {
  PB_APP_ABOUT = 1,
  PB_APP_HIDE = 2,
  PB_APP_HIDE_OTHERS = 3,
  PB_APP_SHOW_ALL = 4,
  PB_APP_BRING_ALL_TO_FRONT = 5,
  PB_APP_QUIT = 6,  // ends the app, every window at once
} PBAppCommand;

// What a press inside a region does. Matches engine.Area in core/engine/engine.go.
typedef enum PBArea {
  PB_AREA_PAGE = 0,   // it goes to the visible tab's page under it
  PB_AREA_SHELL = 1,  // it goes to the shell
  PB_AREA_DRAG = 2,   // it moves the window
} PBArea;

typedef struct PBRegion {
  double x, y, w, h;
  int32_t area;  // PBArea
} PBRegion;

// A tab whose page a window shows, and where.
typedef struct PBVisibleTab {
  int64_t tab;
  double x, y, w, h;
} PBVisibleTab;

// Everything about how a window behaves, from Go's window manager, with its regions and
// its visible tabs passed beside it. Rectangles are in shell coordinates, y measured
// down from the top.
typedef struct PBWindowLayout {
  // The window size the shell measured; the pages are placed only while it still fits.
  double view_w, view_h;
  // The frame: the colour the window paints under the page and the shell, 0xRRGGBB,
  // and the width of the band it draws around its edge, in points.
  int32_t frame_rgb;
  double frame_width;
  // How the window's own parts (buttons, title bar) are drawn: 0 as the system is,
  // 1 light, 2 dark.
  int32_t theme;
} PBWindowLayout;

// Matches engine.WindowRequest. The first four do what pressing that window button does.
typedef enum PBWindowRequest {
  PB_WINDOW_CLOSE = 1,
  PB_WINDOW_MINIMISE = 2,
  PB_WINDOW_ZOOM = 3,
  PB_WINDOW_FULL_SCREEN = 4,
  PB_WINDOW_FRONT = 5,  // brings the window to the front and makes the app active
} PBWindowRequest;

// How a window sits on screen. Matches engine.WindowMode.
typedef enum PBWindowMode {
  PB_WINDOW_NORMAL = 0,
  PB_WINDOW_MINIMISED = 1,
  PB_WINDOW_ZOOMED = 2,
  PB_WINDOW_IN_FULL_SCREEN = 3,
} PBWindowMode;

// What an entry on the timeline is. Matches timelineKinds in core/cmd/lib/main.go.
typedef enum PBTimelineKind {
  PB_TIMELINE_MARK = 0,   // a moment
  PB_TIMELINE_BEGIN = 1,  // a span's two ends
  PB_TIMELINE_END = 2,
  PB_TIMELINE_INFO = 3,   // a line of log
  PB_TIMELINE_WARN = 4,
  PB_TIMELINE_ERROR = 5,
} PBTimelineKind;

// What Go can ask the adapter to do. Handed over once, at start. A window is named by
// Go's id for it, 1 for the first.
typedef struct PBEngineCalls {
  // Makes the profile's context, with Chromium's files in the profile folder dir, and the
  // extension runtime with it. Go calls it once, as the copy becomes that profile's, before
  // anything asks for a tab.
  void (*open_profile)(const char* dir);
  // Makes a hidden tab in a workspace, by Go's id for it: "" is the profile's own storage.
  // A window's layout shows it.
  int64_t (*create_tab)(const char* workspace);
  // The workspace a tab the engine makes on its own goes in, one a client of the debugging
  // port or an extension asks for: the first in the profile's list. Go says which at start,
  // before anything asks for such a tab, and again whenever the first changes.
  void (*set_first_workspace)(const char* workspace);
  // Erases a workspace's site data and cache: its cookies, every kind of site storage,
  // service workers and the HTTP cache. Go asks once it has closed the workspace's tabs.
  // For "", the profile's own storage, extensions' own data stays.
  void (*erase_workspace)(const char* workspace);
  void (*close_tab)(int64_t tab);
  void (*load_url)(int64_t tab, const char* url);
  void (*go_back)(int64_t tab);
  void (*go_forward)(int64_t tab);
  void (*reload)(int64_t tab);
  void (*reload_bypassing_cache)(int64_t tab);
  void (*stop)(int64_t tab);
  // Page zoom as a factor, 1 for actual size, kept for the page's host.
  void (*set_zoom)(int64_t tab, double factor);
  double (*get_zoom)(int64_t tab);
  // Makes a hidden copy of the tab, history included, and returns its id.
  int64_t (*duplicate_tab)(int64_t tab);
  // Runs a PBEditCommand in whichever of our pages has the keyboard.
  void (*edit)(int32_t command);
  // Makes a window and the page covering it, chrome://<page>/: a shell, "shell/<id>", or
  // "picker" in a copy that hasn't chosen its profile. It puts it on screen with its first layout, a
  // little down and right of window near, or where a first window goes when near is 0.
  // Its first state follows once this call has returned.
  void (*create_window)(int64_t window,
                        int64_t near,
                        const char* page,
                        const PBWindowLayout* layout,
                        const PBRegion* regions,
                        size_t region_count,
                        const PBVisibleTab* tabs,
                        size_t tab_count);
  // Loads another page in the window in place of the one covering it, as create_window
  // names one. The tabs stay where its layout last put them.
  void (*set_window_page)(int64_t window, const char* page);
  void (*request_window)(int64_t window, int32_t request);  // PBWindowRequest
  // Puts the window's frame on screen there, y down from the top of the main screen.
  void (*set_window_bounds)(int64_t window, double x, double y, double w, double h);
  // Puts the window into a PBWindowMode from whatever it is in now.
  void (*set_window_mode)(int64_t window, int32_t mode);
  // regions are in order: the first that holds a point decides. None holds it: the page
  // there. tabs are the tabs the window shows; every other tab is hidden.
  void (*set_window_layout)(int64_t window,
                            const PBWindowLayout* layout,
                            const PBRegion* regions,
                            size_t region_count,
                            const PBVisibleTab* tabs,
                            size_t tab_count);
  // Gives the keyboard to a visible tab's page.
  void (*focus_tab)(int64_t tab);
  void (*focus_shell)(int64_t window);
  void (*send_to_shell)(int64_t window, const char* json, size_t length);
  // Whether the phone's back key goes to the shell first: while on, the window reports
  // the key on the shell's channel as {"t":"backKey"}. Only Android has the key.
  void (*set_shell_back)(int64_t window, int on);
  // A window's settings page, over the whole window in its own page while open.
  void (*open_settings)(int64_t window);
  void (*close_settings)(int64_t window);
  void (*send_to_settings)(int64_t window, const char* json, size_t length);
  // Replaces the native menu bar: json is Go's engine.Menu.
  void (*set_menu)(const char* json, size_t length);
  void (*app_command)(int32_t command);  // PBAppCommand
  // Draws the app's Dock icon with a small dot in the profile's colour, 0xRRGGBB, at its
  // bottom right. Null on Android, where nothing calls it.
  void (*set_dock_color)(int32_t rgb);
  // Lets a content hook cross to Go, or stops it. Unsubscribed hooks never cross.
  void (*set_hook_subscribed)(int32_t hook, int on);
  // Asks the main thread to call run_posted soon.
  void (*post_wakeup)(void);
  // Android only: opens the system's package installer for the APK at path, where the user
  // confirms. The Mac swaps its own bundle, and leaves this null.
  void (*install_update)(const char* path);
  // What a build with extensions adds: the table in pb_extensions_abi.h. Null in a build
  // without, where nothing calls it.
  const struct PBExtensionCalls* (*extensions)(void);
} PBEngineCalls;

// What the adapter can tell the Go core. The main binary fills it in from Go's exports
// (pb_exports.h) and hands it to ProjectBMain. Helpers have no Go and no table.
typedef struct PBGoCalls {
  // The first call, before anything is written to the profile. Go chooses the profile
  // from the command line's switches, each "" when not given: root, the folder that holds
  // the profiles and their list (--root); profile, a listed profile's id (--profile);
  // user_data_dir, a folder that is the profile as it is (--user-data-dir); picker, 1 for
  // a copy started to pick its profile (--picker). It returns the folder this copy keeps
  // its own files in, malloc'd: the profile's, once this copy owns it, or the root for a
  // copy that picks its profile later, through open_profile. It returns NULL when another
  // copy owns the profile and has been handed urls, one per line, possibly none: this
  // copy then quits.
  char* (*claim_profile)(const char* root,
                         const char* profile,
                         const char* user_data_dir,
                         int picker,
                         const char* urls);

  // hook_table is the hash of the generated hook list the adapter was built with; Go
  // refuses a library whose list differs from its own.
  void (*start)(const PBEngineCalls* calls,
                const char* blocked_hosts,
                const char* hook_table);
  void (*stop)(void);
  void (*run_posted)(void);

  // Every event: the adapter's own (PBEvent) and content's hooks. Returns 1 when Go
  // answered and result holds the answer, 0 to let content do its default.
  int (*hook)(int32_t hook, const PBValue* args, size_t count, PBValue* result);

  // Development only, used by the driver to measure the boundary.
  void (*force_gc)(void);
  // Returns 1 if a nil dereference in Go was recovered as a normal panic.
  int (*recover_nil_deref)(void);
  // Crashes on purpose, to test the crash log: 0 a nil dereference in Go on the calling
  // thread, 1 a panic there, 2 a nil dereference in a new goroutine, 3 a nil
  // dereference in C called from Go.
  void (*crash)(int32_t kind);
  // Returns a string the caller frees with free().
  char* (*go_stats)(void);

  // What --version prints first, such as "Project B 0.2.17". The caller frees it.
  char* (*version)(void);

  // The switches the flags saved in dir ask for, one per line, such as "--pb-extensions".
  // Called once, before content starts, with the folder claim_profile returned. The caller
  // frees it.
  char* (*launch_switches)(const char* profile_dir);

  // The timeline, Go's record of what happened and when (core/timeline). The adapter
  // uses these through engine/browser/timeline.h. Times are nanoseconds since process
  // start. timeline_start comes first: the file goes in the folder claim_profile returned,
  // and the process is already process_age old.
  void (*timeline_start)(const char* profile_dir, int64_t process_age);
  int64_t (*timeline_now)(void);
  // kind is a PBTimelineKind; t is the entry's time, or -1 for now.
  void (*timeline_record)(int32_t kind,
                          int64_t t,
                          const char* name,
                          size_t name_length,
                          const char* text,
                          size_t text_length);
  // The moment one of our pages' own clock started, by its channel's name.
  void (*timeline_page_origin)(const char* page, int64_t t);

  // Frees a string Go returned, with the allocator that made it: Go's C library's. The
  // content library's own free is Chromium's allocator on Android, which can't.
  void (*free)(void* p);
} PBGoCalls;

#endif  // PROJECT_B_ENGINE_ABI_PB_ABI_H_
