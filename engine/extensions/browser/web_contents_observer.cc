#include "project_b/engine/extensions/browser/web_contents_observer.h"

#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/web_contents.h"
#include "extensions/browser/api/declarative_net_request/action_tracker.h"
#include "extensions/browser/api/declarative_net_request/rules_monitor_service.h"
#include "project_b/engine/browser/hooks.h"

namespace project_b {

namespace {

// The counts of requests each extension's rules blocked, which are kept by tab.
extensions::declarative_net_request::ActionTracker* BlockedCounts(
    content::WebContents* contents) {
  auto* rules = extensions::declarative_net_request::RulesMonitorService::Get(
      contents->GetBrowserContext());
  return rules ? &rules->action_tracker() : nullptr;
}

}  // namespace

WebContentsObserver::WebContentsObserver(content::WebContents* web_contents)
    : extensions::ExtensionWebContentsObserver(web_contents),
      content::WebContentsUserData<WebContentsObserver>(*web_contents),
      script_executor_(web_contents) {
  Initialize();
}

WebContentsObserver::~WebContentsObserver() = default;

// A tab's counts start again with each page.
void WebContentsObserver::DidFinishNavigation(
    content::NavigationHandle* handle) {
  extensions::ExtensionWebContentsObserver::DidFinishNavigation(handle);
  const int64_t tab = hooks::TabOf(web_contents());
  if (tab <= 0 || !handle->IsInPrimaryMainFrame() || !handle->HasCommitted() ||
      handle->IsSameDocument()) {
    return;
  }
  if (auto* counts = BlockedCounts(web_contents())) {
    counts->ResetTrackedInfoForTab(tab, handle->GetNavigationId());
  }
}

void WebContentsObserver::WebContentsDestroyed() {
  const int64_t tab = hooks::TabOf(web_contents());
  if (tab > 0) {
    if (auto* counts = BlockedCounts(web_contents())) {
      counts->ClearTabData(tab);
    }
  }
}

WEB_CONTENTS_USER_DATA_KEY_IMPL(WebContentsObserver);

}  // namespace project_b
