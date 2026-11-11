#ifndef PROJECT_B_ENGINE_COMMON_MAIN_DELEGATE_H_
#define PROJECT_B_ENGINE_COMMON_MAIN_DELEGATE_H_

#include <memory>
#include <optional>
#include <string>
#include <variant>

#include "build/build_config.h"
#include "content/public/app/content_main_delegate.h"
#include "project_b/engine/buildflags/buildflags.h"

namespace content {
class ContentClient;
}

#if BUILDFLAG(PB_EXTENSIONS)
namespace extensions {
class ExtensionsClient;
}
#endif

namespace project_b {

// What content needs at startup in every process. The main binary adds the browser
// side on top; the helper uses this as it is.
class MainDelegate : public content::ContentMainDelegate {
 public:
  MainDelegate();
  MainDelegate(const MainDelegate&) = delete;
  MainDelegate& operator=(const MainDelegate&) = delete;
  ~MainDelegate() override;

 protected:
  std::optional<int> BasicStartupComplete() override;
  void PreSandboxStartup() override;
#if BUILDFLAG(PB_EXTENSIONS)
  // Extensions run code of ours in renderers and in a utility process.
  content::ContentRendererClient* CreateContentRendererClient() override;
  content::ContentUtilityClient* CreateContentUtilityClient() override;
#endif
#if BUILDFLAG(IS_ANDROID)
  std::variant<int, content::MainFunctionParams> RunProcess(
      const std::string& process_type,
      content::MainFunctionParams main_function_params) override;
#endif

 private:
  std::unique_ptr<content::ContentClient> content_client_;
#if BUILDFLAG(PB_EXTENSIONS)
  std::unique_ptr<extensions::ExtensionsClient> extensions_client_;
  std::unique_ptr<content::ContentRendererClient> renderer_client_;
  std::unique_ptr<content::ContentUtilityClient> utility_client_;
#endif
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_COMMON_MAIN_DELEGATE_H_
