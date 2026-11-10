#include "project_b/engine/browser_main_delegate.h"

#include "build/build_config.h"
#include "project_b/engine/browser/content_browser_client.h"

#if BUILDFLAG(IS_MAC)
#include "project_b/engine/browser/mac_app.h"
#endif

namespace project_b {

BrowserMainDelegate::BrowserMainDelegate() = default;
BrowserMainDelegate::~BrowserMainDelegate() = default;

std::optional<int> BrowserMainDelegate::PreBrowserMain() {
#if BUILDFLAG(IS_MAC)
  InitMacApp();
#endif
  return MainDelegate::PreBrowserMain();
}

content::ContentBrowserClient*
BrowserMainDelegate::CreateContentBrowserClient() {
  browser_client_ = std::make_unique<ContentBrowserClient>();
  return browser_client_.get();
}

}  // namespace project_b
