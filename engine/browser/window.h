#ifndef PROJECT_B_ENGINE_BROWSER_WINDOW_H_
#define PROJECT_B_ENGINE_BROWSER_WINDOW_H_

#include <memory>
#include <string>
#include <vector>

#include "base/functional/callback_forward.h"
#include "ui/gfx/geometry/rect_f.h"
#include "ui/gfx/geometry/size_f.h"
#include "ui/gfx/native_ui_types.h"

namespace input {
struct NativeWebKeyboardEvent;
}

namespace project_b {

// The native window: the page's view at the bottom, the shell's view on top covering
// everything. One question decides who gets a mouse event: is this point in a
// rectangle the shell claimed?
class Window {
 public:
  explicit Window(base::OnceClosure on_close);
  Window(const Window&) = delete;
  Window& operator=(const Window&) = delete;
  ~Window();

  void Show();
  void SetTitle(const std::u16string& title);
  void SetShellView(gfx::NativeView view);
  void SetPageView(gfx::NativeView view);

  // Where the page goes, in shell coordinates. viewport is the size the shell measured
  // against: a report made for an older window size is ignored, and a fresher one
  // follows because the resize made the shell measure again.
  void SetPageBounds(const gfx::RectF& bounds, const gfx::SizeF& viewport);
  void SetShellRegions(std::vector<gfx::RectF> regions);

  bool HandleUnhandledKey(const input::NativeWebKeyboardEvent& event);

  gfx::NativeWindow native_window() const;
  // Development only, for the driver.
  void ForceKeyForDriver();
  gfx::RectF page_bounds() const;
  size_t ignored_stale_bounds() const;

 private:
  struct Impl;
  std::unique_ptr<Impl> impl_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_WINDOW_H_
