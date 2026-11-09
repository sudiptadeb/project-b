// The main binary: Go and this file. Content and the C++ adapter are in
// libproject_b_content.dylib, which the helper loads too.

#include "project_b/engine/abi/pb_exports.h"
#include "project_b/engine/abi/pb_main.h"

int main(int argc, const char** argv) {
  static const PBGoCalls go = {
      .start = PBStart,
      .stop = PBStop,
      .run_posted = PBRunPosted,
      .shell_message = PBShellMessage,
      .navigation_starting = PBNavigationStarting,
      .navigation_finished = PBNavigationFinished,
      .navigation_failed = PBNavigationFailed,
      .history_changed = PBHistoryChanged,
      .title_changed = PBTitleChanged,
      .loading_changed = PBLoadingChanged,
      .page_crashed = PBPageCrashed,
      .focus_address_bar_requested = PBFocusAddressBarRequested,
      .window_controls_changed = PBWindowControlsChanged,
      .ping = PBPing,
      .force_gc = PBForceGC,
      .recover_nil_deref = PBRecoverNilDeref,
      .go_stats = PBGoStats,
  };
  return ProjectBMain(argc, argv, &go);
}
