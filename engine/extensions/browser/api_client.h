#ifndef PROJECT_B_ENGINE_EXTENSIONS_BROWSER_API_CLIENT_H_
#define PROJECT_B_ENGINE_EXTENSIONS_BROWSER_API_CLIENT_H_

#include <memory>

#include "extensions/browser/api/extensions_api_client.h"

namespace project_b {

// What //extensions' own APIs ask the browser: which tab a message came from, and where
// to show the count of requests an extension's rules blocked.
class ExtensionsAPIClient : public extensions::ExtensionsAPIClient {
 public:
  ExtensionsAPIClient();
  ExtensionsAPIClient(const ExtensionsAPIClient&) = delete;
  ExtensionsAPIClient& operator=(const ExtensionsAPIClient&) = delete;
  ~ExtensionsAPIClient() override;

 private:
  void UpdateActionCount(content::BrowserContext* context,
                         const extensions::ExtensionId& extension_id,
                         int tab_id,
                         int action_count,
                         bool clear_badge_text) override;
  extensions::MessagingDelegate* GetMessagingDelegate() override;
  extensions::WebstorePrivateAPIDelegate* GetWebstorePrivateAPIDelegate()
      override;
  extensions::ManagementAPIDelegate* CreateManagementAPIDelegate()
      const override;
  std::unique_ptr<extensions::SupervisedUserExtensionsDelegate>
  CreateSupervisedUserExtensionsDelegate(
      content::BrowserContext* browser_context) const override;

  std::unique_ptr<extensions::MessagingDelegate> messaging_delegate_;
  std::unique_ptr<extensions::WebstorePrivateAPIDelegate> webstore_delegate_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_BROWSER_API_CLIENT_H_
