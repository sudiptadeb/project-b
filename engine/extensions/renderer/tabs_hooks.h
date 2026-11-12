#ifndef PROJECT_B_ENGINE_EXTENSIONS_RENDERER_TABS_HOOKS_H_
#define PROJECT_B_ENGINE_EXTENSIONS_RENDERER_TABS_HOOKS_H_

#include <string>

#include "base/memory/raw_ptr.h"
#include "extensions/renderer/bindings/api_binding_hooks_delegate.h"
#include "extensions/renderer/bindings/api_signature.h"

namespace extensions {
class NativeRendererMessagingService;
class ScriptContext;
}  // namespace extensions

namespace project_b {

// tabs.sendMessage and tabs.connect, from an extension to a tab. They open a message
// channel, as runtime.sendMessage does the other way, and never reach Go as a call:
// the browser finds the tab by Go's id (api_client.cc). As Chrome's
// chrome/renderer/extensions/api/tabs_hooks_delegate.cc, without the old sendRequest.
class TabsHooks : public extensions::APIBindingHooksDelegate {
 public:
  explicit TabsHooks(extensions::NativeRendererMessagingService* messaging);
  TabsHooks(const TabsHooks&) = delete;
  TabsHooks& operator=(const TabsHooks&) = delete;
  ~TabsHooks() override;

  extensions::APIBindingHooks::RequestResult HandleRequest(
      const std::string& method_name,
      const extensions::APISignature* signature,
      v8::Local<v8::Context> context,
      v8::LocalVector<v8::Value>* arguments,
      const extensions::APITypeReferenceMap& refs) override;

 private:
  extensions::APIBindingHooks::RequestResult SendMessage(
      extensions::ScriptContext* script_context,
      const extensions::APISignature::V8ParseResult& parsed);
  extensions::APIBindingHooks::RequestResult Connect(
      extensions::ScriptContext* script_context,
      const extensions::APISignature::V8ParseResult& parsed);

  // Outlives this: both belong to the bindings system.
  const raw_ptr<extensions::NativeRendererMessagingService> messaging_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_RENDERER_TABS_HOOKS_H_
