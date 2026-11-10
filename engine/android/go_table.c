// The table of Go's functions. Only the main process loads this library, and loading it starts
// Go's runtime. Java carries the table's address across to the content library, which has no
// link-time reference to Go, the same way app/main.cc does it on the Mac.

#include <jni.h>
#include <stdint.h>

#include "project_b/engine/abi/pb_exports.h"

JNIEXPORT jlong JNICALL Java_dev_projectb_browser_GoCore_table(JNIEnv* env, jclass cls) {
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
  return (jlong)(intptr_t)&go;
}
