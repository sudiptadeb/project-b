#ifndef PROJECT_B_ENGINE_BROWSER_CONTENT_BROWSER_CLIENT_H_
#define PROJECT_B_ENGINE_BROWSER_CONTENT_BROWSER_CLIENT_H_

#include <memory>
#include <string>
#include <vector>

#include "content/public/browser/content_browser_client.h"

namespace project_b {

// The general counter: content asks this object questions. Each answer is short or
// forwards to someone who knows.
class ContentBrowserClient : public content::ContentBrowserClient {
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
