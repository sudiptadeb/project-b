#ifndef PROJECT_B_ENGINE_BROWSER_ENGINE_H_
#define PROJECT_B_ENGINE_BROWSER_ENGINE_H_

#include <cstdint>
#include <map>
#include <memory>
#include <string>
#include <vector>

#include "base/files/file_path.h"
#include "base/functional/callback.h"
#include "base/memory/raw_ptr.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/window.h"
#include "ui/gfx/geometry/rect_f.h"
#include "url/gurl.h"

namespace content {
class BrowserContext;
class WebContents;
}  // namespace content

namespace project_b {

class PageTab;
class SettingsPage;
class ShellPage;

// What the Go core calls. It owns the tabs and the windows, and is the one place every
// call from Go comes through, so it's also where the main-thread rule is checked.
class Engine {
 public:
  // A tab a window's layout shows, and where, in shell coordinates.
  struct VisibleTab {
    int64_t tab;
    gfx::RectF rect;
  };

  // One of Go's windows, by Go's id for it: the native window, the shell page on it, the
  // settings page over it while that is open, and the tabs it shows.
  struct AppWindow {
    explicit AppWindow(int64_t id);
    ~AppWindow();
    const int64_t id;
    std::unique_ptr<Window> window;
    std::unique_ptr<ShellPage> shell;
    std::unique_ptr<SettingsPage> settings;
    std::vector<VisibleTab> visible;
    // The visible tab whose page has, or last had, the keyboard here, or 0. The window
    // is titled after it.
    int64_t focused_tab = 0;
    // Whether the phone's back key goes to the shell first (set_shell_back).
    bool shell_back = false;
  };

  // Makes the profile's context, with Chromium's files in a folder of the profile's.
  using OpenProfileCallback =
      base::RepeatingCallback<content::BrowserContext*(const base::FilePath&)>;

  // Our pages (the shells, the settings pages and the picker) load in pages_context, and
  // the tabs in the profile's context, which open_profile makes when Go opens the profile.
  // quit ends the app.
  Engine(content::BrowserContext* pages_context,
         OpenProfileCallback open_profile,
         base::RepeatingClosure quit);
  Engine(const Engine&) = delete;
  Engine& operator=(const Engine&) = delete;
  ~Engine();

  static Engine* Get();
  // Tells Go a page or a client opened a tab, which the adapter already made, in a window
  // of its own when new_window, and which workspace it is in.
  static void TabOpened(int64_t opener,
                        int64_t tab,
                        const GURL& url,
                        bool foreground,
                        bool new_window);

  // The table handed to Go. Lives as long as the process.
  static const PBEngineCalls* Calls();

  content::BrowserContext* pages_context() const { return pages_context_; }
  // The profile's context, or null before Go opens the profile.
  content::BrowserContext* browser_context() const { return browser_context_; }
  // A window by Go's id, or null for one that isn't open.
  AppWindow* window(int64_t id) const;
  const std::map<int64_t, std::unique_ptr<AppWindow>>& windows() const {
    return windows_;
  }
  // A tab by Go's id, or null.
  PageTab* tab(int64_t id) const;
  // The window whose layout last showed a tab, or null when none has.
  AppWindow* WindowOf(int64_t tab) const;
  // A window's focused tab, or null when it shows none.
  PageTab* FocusedTab(const AppWindow* window) const;

  // Puts the tabs a window's layout lists in it, and tells each tab that comes or goes
  // that it is shown or hidden.
  void ShowTabs(AppWindow* window, std::vector<VisibleTab> tabs);
  // A tab's page took the keyboard; a tab's title changed.
  void TabFocused(PageTab* tab);
  void TabTitleChanged(PageTab* tab);

  void OpenProfile(const char* dir);
  // Makes window id with its page, chrome://<page>/, and puts it on screen with its first
  // layout, a little down and right of window near, or where a first window goes for 0.
  void CreateWindow(int64_t id,
                    int64_t near,
                    const std::string& page,
                    Window::Layout layout,
                    std::vector<VisibleTab> tabs);
  // Ends the app, every window at once.
  void Quit();

  // Tab moves the keyboard out of a tab's page into its window's shell, and out of a
  // shell into its window's page.
  void FocusShellForTraversal(int64_t from_tab, bool reverse);
  void FocusPageForTraversal(int64_t window, bool reverse);

  // Makes a hidden tab in a workspace, by Go's id for it: "" is the profile's own storage.
  // Any other workspace is a storage partition of the profile's context, which the tab
  // keeps for its life, across every site it visits and every tab it opens.
  PageTab* CreateTab(const std::string& workspace);
  // Takes a WebContents content made for a page's window.open or link, or for an
  // extension's page, and makes it a hidden tab, in the workspace content made it in.
  PageTab* AdoptTab(std::unique_ptr<content::WebContents> contents);
  // The workspace a page is in, by the partition content made it in: Go's id for one, or
  // "" for the profile's own storage.
  std::string WorkspaceOf(content::WebContents* contents) const;
  // The workspace a tab the engine makes on its own goes in, one a client of the
  // debugging port or an extension asks for: the first in the profile's list, as Go says.
  void SetFirstWorkspace(const std::string& workspace);
  const std::string& first_workspace() const { return first_workspace_; }
  // Erases a workspace's site data and cache, once the tabs Go has just closed are gone,
  // and marks workspace.erased on the timeline when it is done. For "", the profile's own
  // storage, extensions' own data stays.
  void EraseWorkspace(const std::string& workspace);
  void CloseTab(int64_t tab);
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
  void WriteClipboard(const std::string& text);
  void OpenSettings(int64_t window);
  void CloseSettings(int64_t window);
  void SendToSettings(int64_t window, const char* json, size_t length);
  void CloseWindow(int64_t window);
  void FocusTab(int64_t tab);
  void FocusShell(int64_t window);
  void SendToShell(int64_t window, const char* json, size_t length);

  // A tab's page, or null for an id that isn't a tab's.
  content::WebContents* Contents(int64_t tab) const;

 private:
  PageTab* Add(std::unique_ptr<content::WebContents> contents,
               const std::string& workspace);
  void Erase(const std::string& workspace);
  // Whichever of our pages has the keyboard: a settings page, a shell or a tab's.
  content::WebContents* FocusedContents() const;
  void UpdateTitle(AppWindow* window);
  // The native window closed: Go hears, and the window's pages go with it.
  void WindowClosed(int64_t id);
  // Tells Go a window's state, now and whenever it changes.
  void ListenToWindow(int64_t id);
  // A minimised window's tabs are told they are hidden, and shown again when it comes
  // back.
  void WindowMinimised(int64_t id, bool minimised);

  const raw_ptr<content::BrowserContext> pages_context_;
  const OpenProfileCallback open_profile_;
  raw_ptr<content::BrowserContext> browser_context_ = nullptr;
  const base::RepeatingClosure quit_;
  std::map<int64_t, std::unique_ptr<AppWindow>> windows_;

  std::map<int64_t, std::unique_ptr<PageTab>> tabs_;
  int64_t next_tab_ = 1;
  std::string first_workspace_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_ENGINE_H_
