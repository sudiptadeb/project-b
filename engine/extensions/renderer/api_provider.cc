#include "project_b/engine/extensions/renderer/api_provider.h"

#include <memory>

#include "extensions/renderer/bindings/api_bindings_system.h"
#include "extensions/renderer/native_extension_bindings_system.h"
#include "project_b/engine/extensions/renderer/action_hooks.h"
#include "project_b/engine/extensions/renderer/tabs_hooks.h"

namespace project_b {

void RendererAPIProvider::AddBindingsSystemHooks(
    extensions::Dispatcher* dispatcher,
    extensions::NativeExtensionBindingsSystem* bindings_system) const {
  extensions::APIBindingsSystem* bindings = bindings_system->api_system();
  bindings->RegisterHooksDelegate("action", std::make_unique<ActionHooks>());
  bindings->RegisterHooksDelegate(
      "tabs",
      std::make_unique<TabsHooks>(bindings_system->messaging_service()));
}

}  // namespace project_b
