#ifndef PROJECT_B_ENGINE_BROWSER_SHELL_PAGE_H_
#define PROJECT_B_ENGINE_BROWSER_SHELL_PAGE_H_

#include <memory>
#include <string>
#include <string_view>
#include <vector>

#include "base/memory/weak_ptr.h"
#include "build/build_config.h"
#include "content/public/browser/web_contents_observer.h"
#include "project_b/engine/browser/hooks_gen.h"

namespace content {
class BrowserContext;
class WebContents;
}  // namespace content

namespace project_b {

// Makes our pages loadable in pages_context, and only there: each shell at
// chrome://shell/<id>/, chrome://settings/ and chrome://picker/, each served from its own
// folder under shell/ beside the binary.
// Call it once.
void RegisterShellWebUI(content::BrowserContext* pages_context);

// A window's page: one transparent page covering the window, the shell, or the picker in a
// copy that hasn't chosen its profile. It talks to the Go core with chrome.send, and the
// core answers by calling window.__shellReceive.
class ShellPage : public content::WebContentsObserver,
                  public WebContentsDelegateHooks {
 public:
  // window is Go's id for the window the page is on, and page what covers it, as Go
  // names it: "shell/<id>" for a shell, or "picker".
  ShellPage(content::BrowserContext* browser_context,
            int64_t window,
            const std::string& page);
  ShellPage(const ShellPage&) = delete;
  ShellPage& operator=(const ShellPage&) = delete;
  ~ShellPage() override;

  content::WebContents* contents() const { return contents_.get(); }
  const std::string& page() const { return page_; }
  // Loads page in place of the one there.
  void SetPage(const std::string& page);
  // The app the page is, whose channel it speaks on: "shell" for every shell, or
  // "picker".
  std::string_view app() const;
  void Load();
  void Send(const std::string& json);

 private:
  // content::WebContentsObserver:
  void RenderFrameCreated(content::RenderFrameHost* frame) override;
  void DidFinishNavigation(content::NavigationHandle* handle) override;
  void PrimaryMainFrameRenderProcessGone(
      base::TerminationStatus status) override;

  // content::WebContentsDelegate:
  content::WebContents* hook_contents() const override;
  bool HandleKeyboardEvent(content::WebContents* source,
                           const input::NativeWebKeyboardEvent& event) override;
  bool TakeFocus(content::WebContents* source, bool reverse) override;

  const int64_t window_;
  std::string page_;
  std::unique_ptr<content::WebContents> contents_;
  base::WeakPtrFactory<ShellPage> weak_factory_{this};
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_SHELL_PAGE_H_
