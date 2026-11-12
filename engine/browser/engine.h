#ifndef PROJECT_B_ENGINE_BROWSER_ENGINE_H_
#define PROJECT_B_ENGINE_BROWSER_ENGINE_H_

#include <cstdint>
#include <map>
#include <memory>

#include "base/functional/callback.h"
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

// What the Go core calls. It owns the tabs and the windows, and is the one place every
// call from Go comes through, so it's also where the main-thread rule is checked.
class Engine {
 public:
  // One of Go's windows, by Go's id for it: the native window, the shell page on it, and
  // the settings page over it while that is open.
  struct AppWindow {
    explicit AppWindow(int64_t id);
    ~AppWindow();
    const int64_t id;
    std::unique_ptr<Window> window;
    std::unique_ptr<ShellPage> shell;
    std::unique_ptr<SettingsPage> settings;
  };

  explicit Engine(content::BrowserContext* browser_context);
  Engine(const Engine&) = delete;
  Engine& operator=(const Engine&) = delete;
  ~Engine();

  static Engine* Get();
  // Tells Go a page opened a tab, which the adapter already made.
  static void TabOpened(int64_t opener, int64_t tab, const GURL& url, bool foreground);

  // The table handed to Go. Lives as long as the process.
  static const PBEngineCalls* Calls();

  content::BrowserContext* browser_context() const { return browser_context_; }
  // A window by Go's id, or null for one that isn't open.
  AppWindow* window(int64_t id) const;
  const std::map<int64_t, std::unique_ptr<AppWindow>>& windows() const {
    return windows_;
  }
  // The tab whose page is shown.
  PageTab* active_tab() const;
  // The window a tab is in.
  AppWindow* WindowOf(int64_t tab) const;

  // Makes window id, with its shell page loading on it, off screen. on_close runs when
  // the native window closes.
  void OpenWindow(int64_t id, base::OnceClosure on_close);
  // Puts window id on screen, once Go has given it its first layout, and from then on
  // tells Go its state.
  void ShowWindow(int64_t id);

  // Tab moves the keyboard out of a tab's page into its window's shell, and out of a
  // shell into its window's page.
  void FocusShellForTraversal(int64_t from_tab, bool reverse);
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
  void OpenSettings(int64_t window);
  void CloseSettings(int64_t window);
  void SendToSettings(int64_t window, const char* json, size_t length);
  void CloseWindow(int64_t window);
  void FocusPage();
  void FocusShell(int64_t window);
  void SendToShell(int64_t window, const char* json, size_t length);

  // A tab's page, or null for an id that isn't a tab's.
  content::WebContents* Contents(int64_t tab) const;

 private:
  PageTab* Add(std::unique_ptr<content::WebContents> contents);
  // Whichever of our pages has the keyboard: a settings page, a shell or the tab.
  content::WebContents* FocusedContents() const;

  const raw_ptr<content::BrowserContext> browser_context_;
  std::map<int64_t, std::unique_ptr<AppWindow>> windows_;

  std::map<int64_t, std::unique_ptr<PageTab>> tabs_;
  int64_t next_tab_ = 1;
  int64_t active_tab_ = 0;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_ENGINE_H_
