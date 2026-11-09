#include "project_b/engine/browser/go.h"

namespace project_b {

namespace {
const PBGoCalls* g_go = nullptr;
}  // namespace

void SetGo(const PBGoCalls* go) {
  g_go = go;
}

const PBGoCalls& Go() {
  return *g_go;
}

}  // namespace project_b
