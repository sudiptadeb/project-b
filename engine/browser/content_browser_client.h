#ifndef PROJECT_B_ENGINE_BROWSER_CONTENT_BROWSER_CLIENT_H_
#define PROJECT_B_ENGINE_BROWSER_CONTENT_BROWSER_CLIENT_H_

#include <memory>
#include <string>
#include <vector>

#include "project_b/engine/browser/hooks_gen.h"

namespace project_b {

// The general counter: content asks this object questions. The generated base carries
// them to Go; what is answered here needs content's objects or runs before Go starts.
class ContentBrowserClient : public ContentBrowserClientHooks {
 public:
  ContentBrowserClient();
  ContentBrowserClient(const ContentBrowserClient&) = delete;
  ContentBrowserClient& operator=(const ContentBrowserClient&) = delete;
  ~ContentBrowserClient() override;

 private:
  std::unique_ptr<content::BrowserMainParts> CreateBrowserMainParts(
      bool is_integration_test) override;
  base::FilePath GetChildProcessPath(int flags) override;
  void CreateThrottlesForNavigation(
      content::NavigationThrottleRegistry& registry) override;
  std::string GetUserAgent() override;
  std::vector<base::FilePath> GetNetworkContextsParentDirectory() override;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_CONTENT_BROWSER_CLIENT_H_
