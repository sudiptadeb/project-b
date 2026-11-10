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

#if BUILDFLAG(IS_ANDROID)
#include "components/embedder_support/android/delegate/html_color_picker_bridge.h"
#endif

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
          GURL("chrome://settings/")));
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

#if BUILDFLAG(IS_ANDROID)
std::unique_ptr<content::ColorChooser> SettingsPage::OpenColorChooser(
    content::WebContents* web_contents,
    SkColor color,
    const std::vector<blink::mojom::ColorSuggestionPtr>& suggestions) {
  return std::make_unique<web_contents_delegate_android::HtmlColorPickerBridge>(
      web_contents, color, suggestions);
}
#endif

content::WebContents* SettingsPage::hook_contents() const {
  return contents_.get();
}

}  // namespace project_b
