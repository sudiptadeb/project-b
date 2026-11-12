#ifndef PROJECT_B_ENGINE_EXTENSIONS_BROWSER_GO_FUNCTION_H_
#define PROJECT_B_ENGINE_EXTENSIONS_BROWSER_GO_FUNCTION_H_

#include <cstdint>
#include <optional>
#include <string>

#include "base/values.h"
#include "extensions/browser/extension_function.h"
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

// A function of an API Go answers. As it is, it has no logic: the function's name and
// arguments go to Go as they came, and Go's answer goes back. A function that needs
// the engine first, such as reading a file or taking a picture, is a subclass, listed
// in go_function.cc's kEngineFunctions.
class GoFunction : public ExtensionFunction {
 public:
  GoFunction();

 protected:
  ~GoFunction() override;

  ResponseAction Run() override;

  // Asks Go this function with these arguments, from this caller.
  ResponseValue Ask(const base::ListValue& args);
};

// Registers every function of the APIs Go answers (engine/BUILD.gn's
// project_b_go_apis): a GoFunction, or the subclass kEngineFunctions names.
class GoAPIProvider : public extensions::ExtensionsBrowserAPIProvider {
 public:
  void RegisterExtensionFunctions(
      ExtensionFunctionRegistry* registry) override;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_BROWSER_GO_FUNCTION_H_
