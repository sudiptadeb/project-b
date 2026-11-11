#ifndef PROJECT_B_ENGINE_EXTENSIONS_BROWSER_GO_FUNCTION_H_
#define PROJECT_B_ENGINE_EXTENSIONS_BROWSER_GO_FUNCTION_H_

#include <cstdint>
#include <optional>
#include <string>

#include "base/values.h"
#include "extensions/browser/extensions_browser_api_provider.h"

namespace project_b {

// Go's answer to one call: a result, nothing, or an error.
struct GoAnswer {
  std::optional<base::Value> result;
  std::string error;
};

// Asks Go one function of the browser's own APIs, such as "tabs.query", for an
// extension, or with no extension id for the engine itself. tab is the tab the caller
// runs in, or 0.
GoAnswer AskGo(const std::string& extension_id,
               const std::string& name,
               const base::ListValue& args,
               int64_t tab);

// Registers every function of the APIs Go answers (tabs, windows, action, commands)
// as the same C++ function, which carries the call to Go.
class GoAPIProvider : public extensions::ExtensionsBrowserAPIProvider {
 public:
  void RegisterExtensionFunctions(
      ExtensionFunctionRegistry* registry) override;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_BROWSER_GO_FUNCTION_H_
