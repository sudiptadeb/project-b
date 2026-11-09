// The adapter's way to the Go core. Only the main process has one, and only code that
// runs in the main process may call Go().

#ifndef PROJECT_B_ENGINE_BROWSER_GO_H_
#define PROJECT_B_ENGINE_BROWSER_GO_H_

#include "project_b/engine/abi/pb_abi.h"

namespace project_b {

void SetGo(const PBGoCalls* go);
const PBGoCalls& Go();

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_GO_H_
