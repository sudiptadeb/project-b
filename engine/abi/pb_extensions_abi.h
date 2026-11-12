// The part of the boundary a build with extensions adds (project_b_extensions in
// engine/BUILD.gn, the Go build tag "extensions"). Only such a build includes it.
//
// Go asks through PBExtensionCalls, which PBEngineCalls leads to. The adapter tells Go
// through the same hook call as everything else, with the events below. As in pb_abi.h,
// every call is on the main thread and strings are valid only for the call.

#ifndef PROJECT_B_ENGINE_ABI_PB_EXTENSIONS_ABI_H_
#define PROJECT_B_ENGINE_ABI_PB_EXTENSIONS_ABI_H_

#include <stddef.h>
#include <stdint.h>

// Matches core/content/extensions.go.
typedef enum PBExtensionEvent {
  PB_EVENT_EXTENSIONS_CHANGED = 50,        // nothing
  PB_EVENT_EXTENSION_INSTALL_PROMPT = 51,  // i: prompt, s: json, Go's engine.InstallPrompt
  PB_EVENT_EXTENSION_INSTALL_DONE = 52,    // s: the path asked with, s: extension id or "",
                                           // s: error or "", i: cancelled
  // An extension called a browser API that Go answers. s: extension id, s: the
  // function's name, s: its arguments as a JSON list, i: the caller's tab or 0. Go
  // answers with a JSON object: {"result": ...}, {} for none, or {"error": "..."}.
  PB_EVENT_EXTENSION_CALL = 53,
  PB_EVENT_EXTENSION_BLOCKED_COUNT = 54,   // s: extension id, i: tab, i: count
  PB_EVENT_EXTENSION_POPUP_CLOSED = 55,    // nothing
  PB_EVENT_PATH_CHOSEN = 56,               // s: path, or "" when nothing was picked
} PBExtensionEvent;

// Matches engine.PathKind.
typedef enum PBPathKind {
  PB_PATH_FOLDER = 1,
  PB_PATH_PACKAGE = 2,
} PBPathKind;

// Matches engine.Extensions in core/engine/extensions.go.
typedef struct PBExtensionCalls {
  void (*install)(const char* path);
  void (*answer_install_prompt)(int64_t prompt, int accept);
  void (*load_unpacked)(const char* dir);
  void (*uninstall)(const char* id);
  void (*set_enabled)(const char* id, int on);
  // Every installed extension, as a JSON list of Go's engine.Extension. The caller frees
  // it with free().
  char* (*list)(void);
  void (*dispatch_event)(const char* id, const char* event, const char* args_json);
  // Over a window, under the anchor in its shell's coordinates, y measured down from
  // the top.
  void (*open_popup)(int64_t window,
                     const char* id,
                     const char* url,
                     double x,
                     double y,
                     double w,
                     double h);
  void (*close_popup)(void);
  // On a window.
  void (*choose_path)(int64_t window, int32_t kind);  // PBPathKind
} PBExtensionCalls;

#endif  // PROJECT_B_ENGINE_ABI_PB_EXTENSIONS_ABI_H_
