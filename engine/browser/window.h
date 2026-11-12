#ifndef PROJECT_B_ENGINE_BROWSER_WINDOW_H_
#define PROJECT_B_ENGINE_BROWSER_WINDOW_H_

#include <memory>
#include <string>
#include <vector>

#include "base/functional/callback.h"
#include "base/memory/raw_ptr.h"
#include "base/time/time.h"
#include "build/build_config.h"
#include "project_b/engine/buildflags/buildflags.h"
#include "ui/gfx/geometry/point_f.h"
#include "ui/gfx/geometry/rect.h"
#include "ui/gfx/geometry/rect_f.h"
#include "ui/gfx/geometry/size.h"
#include "ui/gfx/geometry/size_f.h"
#include "ui/gfx/native_ui_types.h"

namespace content {
class WebContents;
}

namespace input {
struct NativeWebKeyboardEvent;
}

namespace project_b {

// A native window: the views of the pages it shows at the bottom, each where the layout
// says, the shell's view on top covering everything. Go's window manager describes how it behaves in one
// Layout; the window only follows it, and reports its State back.
class Window {
 public:
  // What a press inside a region does. Matches PBArea.
  enum class Area { kPage = 0, kShell = 1, kDrag = 2 };
  struct Region {
    gfx::RectF rect;
    Area area;
  };
  struct Layout {
    // The size the shell measured against. Page positions measured for an older
    // window size are ignored, and fresher ones follow because the resize made the
    // shell measure again.
    gfx::SizeF viewport;
    // In order: the first region that holds a point decides. None holds it: the page.
    std::vector<Region> regions;
    // The colour the window paints wherever nothing else does, before the shell's
    // first paint and wherever it lags a live resize, 0xRRGGBB, and the width in
    // points of the band it draws over everything around its edge, rounded with its
    // own corner radius, so the page's square corners never show at the curve.
    uint32_t frame_rgb;
    double frame_width;
    // How the window's own parts are drawn: 0 as the system is, 1 light, 2 dark. Only
    // this window: pages keep following the system.
    int theme;
  };
  // A page the window shows, and where, in shell coordinates.
  struct Page {
    raw_ptr<content::WebContents> contents;
    gfx::RectF rect;
  };
  struct State {
    gfx::SizeF size;  // The content area.
    gfx::RectF controls;  // The window buttons, in shell coordinates; empty in full screen.
    double corner_radius;  // 0 in full screen.
    bool full_screen;
    // The window's frame on screen, y down from the top of the main screen.
    gfx::Rect bounds;
    bool minimised;
    bool zoomed;
  };

  // on_close runs when the window closes, on_key when it becomes the key window, and
  // on_minimised when it is minimised or comes back.
  Window(base::OnceClosure on_close,
         base::RepeatingClosure on_key,
         base::RepeatingCallback<void(bool)> on_minimised);
  Window(const Window&) = delete;
  Window& operator=(const Window&) = delete;
  ~Window();

  // Puts the window, the same size as another, a little down and right of it. Call it
  // before Show.
  void PlaceNear(Window* other);
  // Puts the window on screen. Call it once SetLayout has been called.
  void Show();
  void Close();
  void SetTitle(const std::u16string& title);
#if BUILDFLAG(IS_ANDROID)
  // Android's views need the Java WebContents, which a native view can't lead back to.
  void SetShellContents(content::WebContents* contents);
  void AddPageContents(content::WebContents* contents);
  void RemovePageContents(content::WebContents* contents);

  // The overlay covers the whole window, over the shell, and takes every touch while it
  // is there. The settings page is one.
  void SetOverlayContents(content::WebContents* contents);
  void RemoveOverlayContents(content::WebContents* contents);

  // Opens Android's package installer for the APK at path, where the user confirms.
  void InstallPackage(const std::string& path);
#else
  void SetShellView(gfx::NativeView view);

  // A page view leaves the window when its tab closes.
  void RemovePageView(gfx::NativeView view);
#endif

  // The overlay covers the whole window, over the shell, and takes every press while it
  // is there. The settings page is one.
  void SetOverlayView(gfx::NativeView view);
  void RemoveOverlayView(gfx::NativeView view);
#if BUILDFLAG(PB_EXTENSIONS)
  // A popup floats over the shell and the page, under an anchor rectangle given in
  // shell coordinates: its right edge on the anchor's, kept inside the window. Presses
  // inside it are its own. Calling SetPopupView again moves or resizes it.
  void SetPopupView(gfx::NativeView view,
                    const gfx::RectF& anchor,
                    const gfx::Size& size);
  void RemovePopupView(gfx::NativeView view);
  // Development only, for the driver: where the popup is, in shell coordinates, or
  // empty when there is none.
  gfx::RectF popup_bounds() const;
#endif

  // Returns whether the layout was measured for the window's size now, so the pages
  // can be placed.
  bool SetLayout(Layout layout);
  // Shows these pages, each where it says, and hides every other the window holds.
  void SetPages(const std::vector<Page>& pages);

  // Reports the window's state now and whenever it changes.
  void ListenForState(base::RepeatingCallback<void(const State&)> on_change);

  // What the window buttons do: minimise, and zoom and full screen go in or out.
  void Minimize();
  void Zoom();
  void ToggleFullScreen();
  // Brings the window to the front and makes the app the active one, as when another
  // copy started on this profile hands over to this one.
  void BringToFront();

  // A key none of our pages used goes to the menu bar, which is the app's, not a window's.
  static bool HandleUnhandledKey(const input::NativeWebKeyboardEvent& event);

#if !BUILDFLAG(IS_ANDROID)
  // The window's frame on screen, y measured down from the top of the main screen, and
  // its state, as Chrome's views report and change them on a Mac
  // (ui/views/widget/native_widget_mac.mm): maximised is AppKit's zoomed, and Restore
  // leaves full screen, the Dock and zoom alike. For DevTools' Browser domain.
  gfx::Rect GetBounds() const;
  void SetBounds(const gfx::Rect& bounds);
  bool IsMinimized() const;
  bool IsMaximized() const;
  bool IsFullscreen() const;
  void Maximize();
  void SetFullscreen(bool fullscreen);
  void Restore();
  // Puts the window into a PBWindowMode from whatever it is in now. Leaving full screen
  // for the Dock or zoom waits for macOS to finish leaving it.
  void SetMode(int mode);

  // A click the OS made at this time, at this point in the window's own coordinates, is
  // about to be delivered. One the shell or the settings page will get goes on the
  // timeline, as "input.click".
  void NoteClick(const gfx::PointF& in_window, base::TimeTicks made);

  gfx::NativeWindow native_window() const;
  // Development only, for the driver.
  // Makes the window say it is the key window, or stop saying so.
  void ForceKeyForDriver(bool key);
  size_t ignored_stale_bounds() const;
  // The colour the window painted when Show put it on screen, 0xRRGGBB.
  uint32_t shown_rgb() const;
#endif

 private:
  struct Impl;
  std::unique_ptr<Impl> impl_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_WINDOW_H_
