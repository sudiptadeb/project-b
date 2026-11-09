// The boundary between the Go core and the C++ adapter: plain C, ids and strings.
// Content's types never cross it, and each side only holds a table of the other's
// functions, so the library has no link-time reference to Go.
//
// Threading: every call in both directions happens on the main thread, except
// post_wakeup, which any thread may call.
//
// Strings are only valid for the length of the call. Whoever keeps one copies it.

#ifndef PROJECT_B_ENGINE_ABI_PB_ABI_H_
#define PROJECT_B_ENGINE_ABI_PB_ABI_H_

#include <stddef.h>
#include <stdint.h>

// What Go can ask the adapter to do. Handed over once, at start.
typedef struct PBEngineCalls {
  int64_t (*create_tab)(void);
  void (*load_url)(int64_t tab, const char* url);
  void (*go_back)(int64_t tab);
  void (*go_forward)(int64_t tab);
  void (*reload)(int64_t tab);
  // view_w and view_h are the window size the shell measured against.
  void (*set_page_bounds)(double x,
                          double y,
                          double w,
                          double h,
                          double view_w,
                          double view_h);
  // rects is x, y, w, h repeated count times.
  void (*set_shell_regions)(const double* rects, size_t count);
  void (*focus_page)(void);
  void (*send_to_shell)(const char* json, size_t length);
  // Asks the main thread to call run_posted soon.
  void (*post_wakeup)(void);
} PBEngineCalls;

// What the adapter can tell the Go core. The main binary fills it in from Go's exports
// (pb_exports.h) and hands it to ProjectBMain. Helpers have no Go and no table.
typedef struct PBGoCalls {
  void (*start)(const PBEngineCalls* calls,
                const char* profile_dir,
                const char* blocked_hosts);
  void (*stop)(void);
  void (*run_posted)(void);

  void (*shell_message)(const char* json, size_t length);
  // Returns 1 to allow the navigation and 0 to block it.
  int (*navigation_starting)(int64_t tab, const char* url);
  void (*navigation_finished)(int64_t tab, const char* url);
  void (*navigation_failed)(int64_t tab, const char* url, const char* reason);
  void (*history_changed)(int64_t tab, int can_go_back, int can_go_forward);
  void (*title_changed)(int64_t tab, const char* title);
  void (*loading_changed)(int64_t tab, int loading);
  void (*page_crashed)(int64_t tab);
  void (*focus_address_bar_requested)(void);

  // Development only, used by the driver to measure the boundary.
  void (*ping)(void);
  void (*force_gc)(void);
  // Returns 1 if a nil dereference in Go was recovered as a normal panic.
  int (*recover_nil_deref)(void);
  // Returns a string the caller frees with free().
  char* (*go_stats)(void);
} PBGoCalls;

#endif  // PROJECT_B_ENGINE_ABI_PB_ABI_H_
