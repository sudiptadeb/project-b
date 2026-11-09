#ifndef PROJECT_B_ENGINE_BROWSER_WINDOW_H_
#define PROJECT_B_ENGINE_BROWSER_WINDOW_H_

#include <memory>
#include <string>
#include <vector>

#include "base/functional/callback.h"
#include "ui/gfx/geometry/rect_f.h"
#include "ui/gfx/geometry/size_f.h"
#include "ui/gfx/native_ui_types.h"

namespace input {
struct NativeWebKeyboardEvent;
}

namespace project_b {

// The native window: the pages' views at the bottom, one shown at a time, the shell's
// view on top covering everything. One question decides who gets a mouse event: is
// this point in a rectangle the shell claimed?
class Window {
 public:
  explicit Window(base::OnceClosure on_close);
  Window(const Window&) = delete;
  Window& operator=(const Window&) = delete;
  ~Window();

  // Puts the window on screen. Call it once SetFrame has been called.
  void Show();
  void Close();
  void SetTitle(const std::u16string& title);
  void SetShellView(gfx::NativeView view);

  // Page views all sit where SetPageBounds says. AddPageView adds one hidden;
  // ShowPageView shows it and hides the others.
  void AddPageView(gfx::NativeView view);
  void ShowPageView(gfx::NativeView view);
  void RemovePageView(gfx::NativeView view);

  // Where the page goes, in shell coordinates. viewport is the size the shell measured
  // against: a report made for an older window size is ignored, and a fresher one
  // follows because the resize made the shell measure again.
  void SetPageBounds(const gfx::RectF& bounds, const gfx::SizeF& viewport);
  void SetShellRegions(std::vector<gfx::RectF> regions);
  void SetDragRegions(std::vector<gfx::RectF> regions);
  // The frame, 0xRRGGBB and a width in points: the colour the window paints wherever
  // nothing else does, before the shell's first paint and wherever it lags a live
  // resize, and the band the window draws over everything around its edge, rounded
  // with its own corner radius, so the page's square corners never show at the curve.
  void SetFrame(uint32_t rgb, double width);

  // Reports where the native window controls are, in shell coordinates, and the
  // window's own corner radius, now and whenever either changes. In full screen the
  // controls are empty and the radius is 0.
  void ListenForControls(
      base::RepeatingCallback<void(const gfx::RectF&, double corner_radius)> on_change);

  bool HandleUnhandledKey(const input::NativeWebKeyboardEvent& event);

  gfx::NativeWindow native_window() const;
  // Development only, for the driver.
  void ForceKeyForDriver();
  gfx::RectF page_bounds() const;
  size_t ignored_stale_bounds() const;
  // The colour the window painted when Show put it on screen, 0xRRGGBB.
  uint32_t shown_rgb() const;

 private:
  struct Impl;
  std::unique_ptr<Impl> impl_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_WINDOW_H_
