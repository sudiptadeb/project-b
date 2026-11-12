#ifndef PROJECT_B_ENGINE_BROWSER_SHELL_PAGE_H_
#define PROJECT_B_ENGINE_BROWSER_SHELL_PAGE_H_

#include <memory>
#include <string>
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

// Makes our pages loadable, chrome://shell/ and chrome://settings/, each served from its
// own folder under shell/ beside the binary.
void RegisterShellWebUI(content::BrowserContext* browser_context);

// A window's shell: one transparent page covering the window. It talks to the Go core
// with chrome.send, and the core answers by calling window.__shellReceive.
class ShellPage : public content::WebContentsObserver,
                  public WebContentsDelegateHooks {
 public:
  // window is Go's id for the window the shell is on.
  ShellPage(content::BrowserContext* browser_context, int64_t window);
  ShellPage(const ShellPage&) = delete;
  ShellPage& operator=(const ShellPage&) = delete;
  ~ShellPage() override;

  content::WebContents* contents() const { return contents_.get(); }
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
  std::unique_ptr<content::WebContents> contents_;
  base::WeakPtrFactory<ShellPage> weak_factory_{this};
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_SHELL_PAGE_H_
