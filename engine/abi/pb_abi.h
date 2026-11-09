// The boundary between the Go core and the C++ adapter: plain C, ids and strings.
// Content's types never cross it.
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
  // Asks the main thread to call PBRunPosted soon.
  void (*post_wakeup)(void);
} PBEngineCalls;

#endif  // PROJECT_B_ENGINE_ABI_PB_ABI_H_
