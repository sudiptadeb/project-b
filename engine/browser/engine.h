#ifndef PROJECT_B_ENGINE_BROWSER_ENGINE_H_
#define PROJECT_B_ENGINE_BROWSER_ENGINE_H_

#include <cstdint>
#include <map>
#include <memory>
#include <vector>

#include "base/memory/raw_ptr.h"
#include "project_b/engine/abi/pb_abi.h"
#include "url/gurl.h"

namespace content {
class BrowserContext;
class RenderWidgetHost;
class WebContents;
}  // namespace content

namespace project_b {

class PageTab;
class ShellPage;
class Window;

// What the Go core calls. It owns the tabs and is the one place every call from Go
// comes through, so it's also where the main-thread rule is checked.
class Engine {
 public:
  Engine(content::BrowserContext* browser_context,
         Window* window,
         ShellPage* shell);
  Engine(const Engine&) = delete;
  Engine& operator=(const Engine&) = delete;
  ~Engine();

  static Engine* Get();
  // Tells Go a page opened a tab, which the adapter already made.
  static void TabOpened(int64_t opener, int64_t tab, const GURL& url, bool foreground);

  // The table handed to Go. Lives as long as the process.
  static const PBEngineCalls* Calls();

  content::BrowserContext* browser_context() const { return browser_context_; }
  Window* window() const { return window_; }
  ShellPage* shell() const { return shell_; }
  // The tab whose page is shown.
  PageTab* active_tab() const;
  // What the window shows: the shell's widget and the shown page's.
  std::vector<content::RenderWidgetHost*> DrawnWidgets() const;

  void FocusShellForTraversal(bool reverse);
  void FocusPageForTraversal(bool reverse);

  int64_t CreateTab();
  // Takes a WebContents content made for a page's target=_blank or window.open and
  // makes it a hidden tab.
  PageTab* AdoptTab(std::unique_ptr<content::WebContents> contents);
  void CloseTab(int64_t tab);
  void ShowTab(int64_t tab);
  void LoadURL(int64_t tab, const char* url);
  void GoBack(int64_t tab);
  void GoForward(int64_t tab);
  void Reload(int64_t tab);
  void CloseWindow();
  void FocusPage();
  void FocusShell();
  void SendToShell(const char* json, size_t length);
  void SendToTab(int64_t tab, const char* json, size_t length);

 private:
  PageTab* Add(std::unique_ptr<content::WebContents> contents);

  const raw_ptr<content::BrowserContext> browser_context_;
  const raw_ptr<Window> window_;
  const raw_ptr<ShellPage> shell_;

  std::map<int64_t, std::unique_ptr<PageTab>> tabs_;
  int64_t next_tab_ = 1;
  int64_t active_tab_ = 0;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_ENGINE_H_
