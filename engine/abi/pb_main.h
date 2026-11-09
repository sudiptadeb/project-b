// The one function libproject_b_content.dylib exports. Both binaries call it from main.

#ifndef PROJECT_B_ENGINE_ABI_PB_MAIN_H_
#define PROJECT_B_ENGINE_ABI_PB_MAIN_H_

#include "project_b/engine/abi/pb_abi.h"

#ifdef __cplusplus
extern "C" {
#endif

// Runs content and returns when it's done. The main binary passes its table of Go
// functions. The helper passes NULL and runs whichever child process --type names.
__attribute__((visibility("default")))
int ProjectBMain(int argc, const char** argv, const PBGoCalls* go);

#ifdef __cplusplus
}
#endif

#endif  // PROJECT_B_ENGINE_ABI_PB_MAIN_H_
