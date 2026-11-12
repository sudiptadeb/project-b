#ifndef PROJECT_B_ENGINE_EXTENSIONS_RENDERER_API_PROVIDER_H_
#define PROJECT_B_ENGINE_EXTENSIONS_RENDERER_API_PROVIDER_H_

#include "extensions/renderer/extensions_renderer_api_provider.h"

namespace project_b {

// The parts of the browser's own APIs that Chrome answers in the renderer, with a hooks
// delegate per API (docs/extensions.md, "Adding an API"). The rest of each API goes to
// the browser, and from there to Go.
class RendererAPIProvider : public extensions::ExtensionsRendererAPIProvider {
 public:
  void RegisterNativeHandlers(
      extensions::ModuleSystem* module_system,
      extensions::NativeExtensionBindingsSystem* bindings_system,
      extensions::V8SchemaRegistry* v8_schema_registry,
      extensions::ScriptContext* context) const override {}
  void AddBindingsSystemHooks(
      extensions::Dispatcher* dispatcher,
      extensions::NativeExtensionBindingsSystem* bindings_system) const override;
  void PopulateSourceMap(
      extensions::ResourceBundleSourceMap* source_map) const override {}
  void EnableCustomElementAllowlist() const override {}
  void RequireWebViewModules(
      extensions::ScriptContext* context) const override {}
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_RENDERER_API_PROVIDER_H_
