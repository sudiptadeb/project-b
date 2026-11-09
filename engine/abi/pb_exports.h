// Go's exported functions, from core/cmd/lib. Only the main binary includes this, to
// fill in PBGoCalls. The library reaches them through that table, and the helper has no
// Go at all.

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
int PBNavigationStarting(int64_t tab, const char* url);
void PBNavigationFinished(int64_t tab, const char* url);
void PBNavigationFailed(int64_t tab, const char* url, const char* reason);
void PBHistoryChanged(int64_t tab, int can_go_back, int can_go_forward);
void PBTitleChanged(int64_t tab, const char* title);
void PBLoadingChanged(int64_t tab, int loading);
void PBPageCrashed(int64_t tab);
void PBFaviconChanged(int64_t tab, const char* data_url);
void PBTabOpened(int64_t opener, int64_t tab, const char* url, int foreground);
void PBShortcutPressed(int shortcut);
void PBWindowControlsChanged(double x, double y, double w, double h);

void PBPing(void);
void PBForceGC(void);
int PBRecoverNilDeref(void);
char* PBGoStats(void);

#ifdef __cplusplus
}
#endif

#endif  // PROJECT_B_ENGINE_ABI_PB_EXPORTS_H_
