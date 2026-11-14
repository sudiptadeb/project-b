#ifndef PROJECT_B_ENGINE_BROWSER_NATIVE_SHELL_MAC_H_
#define PROJECT_B_ENGINE_BROWSER_NATIVE_SHELL_MAC_H_

#include <cstdint>
#include <memory>
#include <string>

#include "ui/gfx/native_ui_types.h"

namespace project_b {

// Spike (docs/native-shell.md): a shell drawn in AppKit, by Swift, in place of a shell's
// page. It covers the window as the page does and speaks the same messages: what Go sends
// the shell reaches its receive:, and what it sends goes to Go as PB_EVENT_SHELL_MESSAGE,
// with the window, as chrome.send('shell') does.
class NativeShell {
 public:
  // Whether a page Go names is the native shell's: "shell/native".
  static bool Is(const std::string& page);
  // Takes a shell page's view out of its window, before the native shell takes its place.
  static void Detach(gfx::NativeView view);

  explicit NativeShell(int64_t window);
  NativeShell(const NativeShell&) = delete;
  NativeShell& operator=(const NativeShell&) = delete;
  ~NativeShell();

  gfx::NativeView view() const;
  void Send(const std::string& json);
  // Gives the shell the keyboard: its address field.
  void Focus();

 private:
  struct Impl;
  std::unique_ptr<Impl> impl_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_NATIVE_SHELL_MAC_H_
