// Go's exported functions, from core/cmd/lib. Only the main binary includes this, to
// fill in PBGoCalls. The library reaches them through that table, and the helper has no
// Go at all.

#ifndef PROJECT_B_ENGINE_ABI_PB_EXPORTS_H_
#define PROJECT_B_ENGINE_ABI_PB_EXPORTS_H_

#include "project_b/engine/abi/pb_abi.h"

#ifdef __cplusplus
extern "C" {
#endif

char* PBClaimProfile(const char* root,
                     const char* profile,
                     const char* user_data_dir,
                     const char* urls);
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
void PBTimelineStart(const char* profile_dir, int64_t process_age);
int64_t PBTimelineNow(void);
void PBTimelineRecord(int32_t kind,
                      int64_t t,
                      const char* name,
                      size_t name_length,
                      const char* text,
                      size_t text_length);
void PBTimelinePageOrigin(const char* page, int64_t t);

#ifdef __cplusplus
}
#endif

#endif  // PROJECT_B_ENGINE_ABI_PB_EXPORTS_H_
