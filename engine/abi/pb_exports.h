// What the adapter can tell the Go core. Exported by core/cmd/lib, so only the main
// binary can include this. The helper binary has no Go.

#ifndef PROJECT_B_ENGINE_ABI_PB_EXPORTS_H_
#define PROJECT_B_ENGINE_ABI_PB_EXPORTS_H_

#include "project_b/engine/abi/pb_abi.h"

#ifdef __cplusplus
extern "C" {
#endif

void PBStart(const PBEngineCalls* calls,
             const char* profile_dir,
             const char* blocked_hosts);
void PBStop(void);
void PBRunPosted(void);

void PBShellMessage(const char* json, size_t length);
// Returns 1 to allow the navigation and 0 to block it.
int PBNavigationStarting(int64_t tab, const char* url);
void PBNavigationFinished(int64_t tab, const char* url);
void PBNavigationFailed(int64_t tab, const char* url, const char* reason);
void PBTitleChanged(int64_t tab, const char* title);
void PBLoadingChanged(int64_t tab, int loading);
void PBPageCrashed(int64_t tab);
void PBFocusAddressBarRequested(void);

// Development only, used by the driver to measure the boundary.
void PBPing(void);
void PBForceGC(void);
// Returns 1 if a nil dereference in Go was recovered as a normal panic.
int PBRecoverNilDeref(void);
// Returns a string the caller frees with free().
char* PBGoStats(void);

#ifdef __cplusplus
}
#endif

#endif  // PROJECT_B_ENGINE_ABI_PB_EXPORTS_H_
