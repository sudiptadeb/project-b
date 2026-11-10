#include "project_b/engine/browser/settings_page.h"

#include <vector>

#include "base/values.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/web_contents.h"
#include "content/public/browser/web_ui.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/window.h"
#include "url/gurl.h"

namespace project_b {

SettingsPage::SettingsPage(content::BrowserContext* browser_context)
    : contents_(content::WebContents::Create(
          content::WebContents::CreateParams(browser_context))) {
  hooks::TagTab(contents_.get(), kSettingsPageId);
  contents_->SetDelegate(this);
}

SettingsPage::~SettingsPage() {
  contents_->SetDelegate(nullptr);
}

void SettingsPage::Load() {
  contents_->GetController().LoadURLWithParams(
      content::NavigationController::LoadURLParams(
          GURL("chrome://shell/settings")));
}

void SettingsPage::Send(const std::string& json) {
  if (content::WebUI* web_ui = contents_->GetWebUI()) {
    std::vector<base::ValueView> args = {json};
    web_ui->CallJavascriptFunctionUnsafe("__shellReceive", args);
  }
}

// Go gets the key first; one it doesn't use goes to the menu bar.
bool SettingsPage::HandleKeyboardEvent(
    content::WebContents* source,
    const input::NativeWebKeyboardEvent& event) {
  return WebContentsDelegateHooks::HandleKeyboardEvent(source, event) ||
         Engine::Get()->window()->HandleUnhandledKey(event);
}

content::WebContents* SettingsPage::hook_contents() const {
  return contents_.get();
}

}  // namespace project_b
