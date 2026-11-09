#include "project_b/engine/browser_main_delegate.h"

#include "project_b/engine/browser/content_browser_client.h"
#include "project_b/engine/browser/mac_app.h"

namespace project_b {

BrowserMainDelegate::BrowserMainDelegate() = default;
BrowserMainDelegate::~BrowserMainDelegate() = default;

std::optional<int> BrowserMainDelegate::PreBrowserMain() {
  InitMacApp();
  return MainDelegate::PreBrowserMain();
}

content::ContentBrowserClient*
BrowserMainDelegate::CreateContentBrowserClient() {
  browser_client_ = std::make_unique<ContentBrowserClient>();
  return browser_client_.get();
}

}  // namespace project_b
