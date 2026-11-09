#ifndef PROJECT_B_ENGINE_COMMON_MAIN_DELEGATE_H_
#define PROJECT_B_ENGINE_COMMON_MAIN_DELEGATE_H_

#include <memory>
#include <optional>

#include "content/public/app/content_main_delegate.h"

namespace content {
class ContentClient;
}

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

 private:
  std::unique_ptr<content::ContentClient> content_client_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_COMMON_MAIN_DELEGATE_H_
