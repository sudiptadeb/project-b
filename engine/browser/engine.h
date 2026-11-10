#ifndef PROJECT_B_ENGINE_BROWSER_ENGINE_H_
#define PROJECT_B_ENGINE_BROWSER_ENGINE_H_

#include <cstdint>
#include <map>
#include <memory>

#include "base/memory/raw_ptr.h"
#include "project_b/engine/abi/pb_abi.h"
#include "url/gurl.h"

namespace content {
class BrowserContext;
class WebContents;
}  // namespace content

namespace project_b {

class PageTab;
class SettingsPage;
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
  // The settings page while it is open, else null.
  SettingsPage* settings() const { return settings_.get(); }
  // The tab whose page is shown.
  PageTab* active_tab() const;

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
  void ReloadBypassingCache(int64_t tab);
  void Stop(int64_t tab);
  void SetZoom(int64_t tab, double factor);
  double GetZoom(int64_t tab);
  int64_t DuplicateTab(int64_t tab);
  void Edit(int32_t command);
  void OpenSettings();
  void CloseSettings();
  void SendToSettings(const char* json, size_t length);
  void CloseWindow();
  void FocusPage();
  void FocusShell();
  void SendToShell(const char* json, size_t length);

 private:
  PageTab* Add(std::unique_ptr<content::WebContents> contents);
  content::WebContents* Contents(int64_t tab) const;
  // Whichever of our pages has the keyboard: the settings page, the shell or the tab.
  content::WebContents* FocusedContents() const;

  const raw_ptr<content::BrowserContext> browser_context_;
  const raw_ptr<Window> window_;
  const raw_ptr<ShellPage> shell_;
  std::unique_ptr<SettingsPage> settings_;

  std::map<int64_t, std::unique_ptr<PageTab>> tabs_;
  int64_t next_tab_ = 1;
  int64_t active_tab_ = 0;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_ENGINE_H_
