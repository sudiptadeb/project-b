#ifndef PROJECT_B_ENGINE_BROWSER_MAIN_DELEGATE_H_
#define PROJECT_B_ENGINE_BROWSER_MAIN_DELEGATE_H_

#include <memory>
#include <optional>

#include "project_b/engine/common/main_delegate.h"

namespace project_b {

class ContentBrowserClient;

// The main process's delegate: everything in common, plus the browser client.
class BrowserMainDelegate : public MainDelegate {
 public:
  BrowserMainDelegate();
  ~BrowserMainDelegate() override;

 private:
  std::optional<int> PreBrowserMain() override;
  content::ContentBrowserClient* CreateContentBrowserClient() override;

  std::unique_ptr<ContentBrowserClient> browser_client_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_MAIN_DELEGATE_H_
