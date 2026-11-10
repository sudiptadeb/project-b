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
// post_wakeup, which any thread may call.
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
  PB_EVENT_SHELL_MESSAGE = 1,           // s: json
  PB_EVENT_TAB_OPENED = 2,              // i: opener, i: tab, s: url, i: foreground
  PB_EVENT_FAVICON_CHANGED = 3,         // i: tab, s: data url or ""
  PB_EVENT_WINDOW_CHANGED = 4,          // d: w, d: h, the content area; d: x, d: y, d: w,
                                        // d: h, the window buttons; d: corner radius;
                                        // i: full screen
  PB_EVENT_PING = 5,                    // nothing; the driver measures the crossing
  PB_EVENT_SHELL_CRASHED = 6,           // nothing; the shell page's renderer died
} PBEvent;

// What a press inside a region does. Matches engine.Area in core/engine/engine.go.
typedef enum PBArea {
  PB_AREA_PAGE = 0,   // it goes to the shown page
  PB_AREA_SHELL = 1,  // it goes to the shell
  PB_AREA_DRAG = 2,   // it moves the window
} PBArea;

typedef struct PBRegion {
  double x, y, w, h;
  int32_t area;  // PBArea
} PBRegion;

// Everything about how the window behaves, from Go's window manager, with its regions
// passed beside it. Rectangles are in shell coordinates, y measured down from the top.
typedef struct PBWindowLayout {
  // The window size the shell measured; the page is placed only while it still fits.
  double view_w, view_h;
  double page_x, page_y, page_w, page_h;
  // The frame: the colour the window paints under the page and the shell, 0xRRGGBB,
  // and the width of the band it draws around its edge, in points.
  int32_t frame_rgb;
  double frame_width;
} PBWindowLayout;

// Matches engine.WindowRequest. Each does what pressing that window button does.
typedef enum PBWindowRequest {
  PB_WINDOW_CLOSE = 1,
  PB_WINDOW_MINIMISE = 2,
  PB_WINDOW_ZOOM = 3,
  PB_WINDOW_FULL_SCREEN = 4,
} PBWindowRequest;

// What Go can ask the adapter to do. Handed over once, at start.
typedef struct PBEngineCalls {
  // Makes a hidden tab. show_tab shows one tab and hides the rest.
  int64_t (*create_tab)(void);
  void (*close_tab)(int64_t tab);
  void (*show_tab)(int64_t tab);
  void (*load_url)(int64_t tab, const char* url);
  void (*go_back)(int64_t tab);
  void (*go_forward)(int64_t tab);
  void (*reload)(int64_t tab);
  void (*request_window)(int32_t request);  // PBWindowRequest
  // regions are in order: the first that holds a point decides. None holds it: the page.
  void (*set_window_layout)(const PBWindowLayout* layout,
                            const PBRegion* regions,
                            size_t region_count);
  void (*focus_page)(void);
  void (*focus_shell)(void);
  void (*send_to_shell)(const char* json, size_t length);
  // Lets a content hook cross to Go, or stops it. Unsubscribed hooks never cross.
  void (*set_hook_subscribed)(int32_t hook, int on);
  // Asks the main thread to call run_posted soon.
  void (*post_wakeup)(void);
} PBEngineCalls;

// What the adapter can tell the Go core. The main binary fills it in from Go's exports
// (pb_exports.h) and hands it to ProjectBMain. Helpers have no Go and no table.
typedef struct PBGoCalls {
  // hook_table is the hash of the generated hook list the adapter was built with; Go
  // refuses a library whose list differs from its own.
  void (*start)(const PBEngineCalls* calls,
                const char* profile_dir,
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
  // Returns a string the caller frees with free().
  char* (*go_stats)(void);
} PBGoCalls;

#endif  // PROJECT_B_ENGINE_ABI_PB_ABI_H_
