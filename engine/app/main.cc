// The main binary: Go and this file. Content and the C++ adapter are in
// libproject_b_content.dylib, which the helper loads too.

#include "project_b/engine/abi/pb_exports.h"
#include "project_b/engine/abi/pb_main.h"

int main(int argc, const char** argv) {
  static const PBGoCalls go = {
      .start = PBStart,
      .stop = PBStop,
      .run_posted = PBRunPosted,
      .hook = PBHook,
      .force_gc = PBForceGC,
      .recover_nil_deref = PBRecoverNilDeref,
      .crash = PBCrash,
      .go_stats = PBGoStats,
  };
  return ProjectBMain(argc, argv, &go);
}
