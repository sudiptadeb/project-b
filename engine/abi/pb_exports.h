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
             const char* blocked_hosts,
             const char* hook_table);
void PBStop(void);
void PBRunPosted(void);
int PBHook(int32_t hook, const PBValue* args, size_t count, PBValue* result);

void PBForceGC(void);
int PBRecoverNilDeref(void);
void PBCrash(int32_t kind);
char* PBGoStats(void);
char* PBVersion(void);
char* PBLaunchSwitches(const char* profile_dir);

#ifdef __cplusplus
}
#endif

#endif  // PROJECT_B_ENGINE_ABI_PB_EXPORTS_H_
