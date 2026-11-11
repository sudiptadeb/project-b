#include "project_b/engine/extensions/browser/api_client.h"

#include <optional>
#include <utility>
#include <vector>

#include "base/values.h"
#include "components/safe_browsing/buildflags.h"
#include "content/public/browser/web_contents.h"
#include "extensions/browser/api/messaging/messaging_delegate.h"
#include "extensions/browser/api/webstore_private/webstore_private_api_delegate.h"
#include "project_b/engine/abi/pb_extensions_abi.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/extensions/browser/go_function.h"

namespace project_b {

namespace {

// Messages between a tab's content script and its extension name the tab by Go's id.
class MessagingDelegate : public extensions::MessagingDelegate {
  // The tab a message's sender is in, as chrome.tabs.Tab, which is Go's to describe.
  std::optional<base::DictValue> MaybeGetTabInfo(
      content::WebContents* web_contents) override {
    const int64_t tab = hooks::TabOf(web_contents);
    if (tab <= 0) {
      return std::nullopt;
    }
    base::ListValue args;
    args.Append(static_cast<int>(tab));
    GoAnswer answer = AskGo(std::string(), "tabs.get", args, 0);
    if (!answer.result || !answer.result->is_dict()) {
      return std::nullopt;
    }
    return std::move(*answer.result).TakeDict();
  }

  content::WebContents* GetWebContentsByTabId(
      content::BrowserContext* browser_context,
      int tab_id) override {
    return Engine::Get()->Contents(tab_id);
  }
};

// //extensions carries the web store's private API whether or not anything can use
// it, and its services ask for this at start. Nothing here is a web store, so no page
// ever reaches the API and none of these is called.
class WebstoreDelegate : public extensions::WebstorePrivateAPIDelegate {
  std::vector<KeyedServiceBaseFactory*> GetWebStoreAPIFactoryDependencies()
      override {
    return {};
  }
  extensions::ExtensionAllowlist* GetExtensionAllowlist(
      content::BrowserContext* context) override {
    return nullptr;
  }
  signin::IdentityManager* GetIdentityManager(
      content::BrowserContext* context) override {
    return nullptr;
  }
  void ShowExtensionInstallBlockedDialog(
      content::WebContents* web_contents,
      const extensions::Extension* extension,
      const std::u16string& custom_error_message,
      const gfx::ImageSkia& icon,
      base::OnceClosure done_callback) override {}
  void ShowExtensionInstallFrictionDialog(
      content::WebContents* web_contents,
      base::OnceCallback<void(bool)> callback) override {}
  void ReportFrictionAcceptedEvent(content::BrowserContext* context) override {}
#if BUILDFLAG(SAFE_BROWSING_AVAILABLE)
  bool IsSafeBrowsingEnabledAndReady(
      content::BrowserContext* context) override {
    return false;
  }
  safe_browsing::SafeBrowsingNavigationObserverManager*
  GetSafeBrowsingNavigationObserverManager(
      content::BrowserContext* context) override {
    return nullptr;
  }
#endif
  std::unique_ptr<enterprise_promotion::PromotionEligibilityChecker>
  CreatePromotionEligibilityChecker(content::BrowserContext* context,
                                    bool dismissed_banner_pref,
                                    bool feature_enabled) override {
    return nullptr;
  }
};

}  // namespace

ExtensionsAPIClient::ExtensionsAPIClient()
    : messaging_delegate_(std::make_unique<MessagingDelegate>()),
      webstore_delegate_(std::make_unique<WebstoreDelegate>()) {}

ExtensionsAPIClient::~ExtensionsAPIClient() = default;

// declarativeNetRequest counts the requests an extension's rules block in each tab, for
// an extension that asked to show the count on its button. Go draws the button.
void ExtensionsAPIClient::UpdateActionCount(
    content::BrowserContext* context,
    const extensions::ExtensionId& extension_id,
    int tab_id,
    int action_count,
    bool clear_badge_text) {
  hooks::Args args;
  args.Str(extension_id);
  args.Int(tab_id);
  args.Int(action_count);
  hooks::Fire(PB_EVENT_EXTENSION_BLOCKED_COUNT, args);
}

extensions::MessagingDelegate* ExtensionsAPIClient::GetMessagingDelegate() {
  return messaging_delegate_.get();
}

extensions::WebstorePrivateAPIDelegate*
ExtensionsAPIClient::GetWebstorePrivateAPIDelegate() {
  return webstore_delegate_.get();
}

}  // namespace project_b
