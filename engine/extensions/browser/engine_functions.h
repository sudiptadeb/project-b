#ifndef PROJECT_B_ENGINE_EXTENSIONS_BROWSER_ENGINE_FUNCTIONS_H_
#define PROJECT_B_ENGINE_EXTENSIONS_BROWSER_ENGINE_FUNCTIONS_H_

#include <string>

#include "project_b/engine/extensions/browser/go_function.h"

namespace project_b {

// The functions of Go's APIs that need the engine for a part only C++ can do. Each is
// listed in go_function.cc's kEngineFunctions.

// action.setIcon: an icon named by a file inside the extension is read here, and Go
// gets the picture as a data: URL under "icon". One the extension drew comes from the
// renderer as that already.
class ActionSetIconFunction : public GoFunction {
 private:
  ~ActionSetIconFunction() override = default;
  ResponseAction Run() override;
  void OnIconRead(std::string data_url);
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_BROWSER_ENGINE_FUNCTIONS_H_
