#ifndef PROJECT_B_ENGINE_BROWSER_ENGINE_H_
#define PROJECT_B_ENGINE_BROWSER_ENGINE_H_

#include <cstdint>
#include <map>
#include <memory>

#include "base/memory/raw_ptr.h"
#include "project_b/engine/abi/pb_abi.h"

namespace content {
class BrowserContext;
class WebContents;
}  // namespace content

namespace input {
struct NativeWebKeyboardEvent;
}

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
  static bool IsFocusAddressBarKey(const input::NativeWebKeyboardEvent& event);

  // The table handed to Go. Lives as long as the process.
  static const PBEngineCalls* Calls();

  Window* window() const { return window_; }
  ShellPage* shell() const { return shell_; }
  PageTab* active_tab() const;
  PageTab* TabFor(content::WebContents* contents) const;

  void FocusAddressBar();
  void FocusShellForTraversal(bool reverse);
  void FocusPageForTraversal(bool reverse);

  int64_t CreateTab();
  void LoadURL(int64_t tab, const char* url);
  void FocusPage();
  void SendToShell(const char* json, size_t length);

 private:
  const raw_ptr<content::BrowserContext> browser_context_;
  const raw_ptr<Window> window_;
  const raw_ptr<ShellPage> shell_;

  std::map<int64_t, std::unique_ptr<PageTab>> tabs_;
  int64_t next_tab_ = 1;
  int64_t active_tab_ = 0;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_ENGINE_H_
