#ifndef PROJECT_B_ENGINE_BROWSER_SETTINGS_PAGE_H_
#define PROJECT_B_ENGINE_BROWSER_SETTINGS_PAGE_H_

#include <memory>
#include <string>
#include <vector>

#include "build/build_config.h"

#include "project_b/engine/browser/hooks_gen.h"

namespace content {
class BrowserContext;
class WebContents;
}  // namespace content

namespace project_b {

// Hooks give the settings page this id; the shell's is 0 and tabs' are positive.
inline constexpr int64_t kSettingsPageId = -1;

// The settings page: chrome://settings/, an app of its own in a WebContents of its own, over the whole
// window while it is open, so the shell never carries it. It talks to Go as the shell
// does, under its own message name.
class SettingsPage : public WebContentsDelegateHooks {
 public:
  explicit SettingsPage(content::BrowserContext* browser_context);
  SettingsPage(const SettingsPage&) = delete;
  SettingsPage& operator=(const SettingsPage&) = delete;
  ~SettingsPage() override;

  content::WebContents* contents() const { return contents_.get(); }
  void Load();
  void Send(const std::string& json);

 private:
  // content::WebContentsDelegate:
  content::WebContents* hook_contents() const override;
  bool HandleKeyboardEvent(content::WebContents* source,
                           const input::NativeWebKeyboardEvent& event) override;
#if BUILDFLAG(IS_ANDROID)
  // Android has no colour picker inside the page, so the custom colour asks for one.
  std::unique_ptr<content::ColorChooser> OpenColorChooser(
      content::WebContents* web_contents,
      SkColor color,
      const std::vector<blink::mojom::ColorSuggestionPtr>& suggestions)
      override;
#endif

  std::unique_ptr<content::WebContents> contents_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_SETTINGS_PAGE_H_
