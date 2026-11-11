#ifndef PROJECT_B_ENGINE_EXTENSIONS_BROWSER_WEB_CONTENTS_OBSERVER_H_
#define PROJECT_B_ENGINE_EXTENSIONS_BROWSER_WEB_CONTENTS_OBSERVER_H_

#include "content/public/browser/web_contents_user_data.h"
#include "extensions/browser/extension_web_contents_observer.h"
#include "extensions/browser/script_executor.h"

namespace project_b {

// What //extensions keeps for each WebContents it may run in: a tab, an extension's
// popup or its background page. It tells renderers which extension a frame belongs to,
// and runs scripts in a tab for chrome.scripting.
class WebContentsObserver
    : public extensions::ExtensionWebContentsObserver,
      public content::WebContentsUserData<WebContentsObserver> {
 public:
  WebContentsObserver(const WebContentsObserver&) = delete;
  WebContentsObserver& operator=(const WebContentsObserver&) = delete;
  ~WebContentsObserver() override;

  extensions::ScriptExecutor* script_executor() { return &script_executor_; }

 private:
  friend class content::WebContentsUserData<WebContentsObserver>;
  explicit WebContentsObserver(content::WebContents* web_contents);

  // content::WebContentsObserver:
  void DidFinishNavigation(content::NavigationHandle* handle) override;
  void WebContentsDestroyed() override;

  extensions::ScriptExecutor script_executor_;

  WEB_CONTENTS_USER_DATA_KEY_DECL();
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_BROWSER_WEB_CONTENTS_OBSERVER_H_
