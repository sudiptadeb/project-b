#ifndef PROJECT_B_ENGINE_EXTENSIONS_RENDERER_ACTION_HOOKS_H_
#define PROJECT_B_ENGINE_EXTENSIONS_RENDERER_ACTION_HOOKS_H_

#include <string>

#include "extensions/renderer/bindings/api_binding_hooks_delegate.h"

namespace project_b {

// action.setIcon with imageData, a picture an extension drew on a canvas. Only the
// renderer can read an ImageData, so the picture is made a PNG here, and the call goes
// on with it as "icon", the data: URL Go draws. An icon by path goes on as it came.
class ActionHooks : public extensions::APIBindingHooksDelegate {
 public:
  extensions::APIBindingHooks::RequestResult HandleRequest(
      const std::string& method_name,
      const extensions::APISignature* signature,
      v8::Local<v8::Context> context,
      v8::LocalVector<v8::Value>* arguments,
      const extensions::APITypeReferenceMap& refs) override;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_RENDERER_ACTION_HOOKS_H_
