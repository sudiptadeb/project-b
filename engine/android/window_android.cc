// The window on Android: one SurfaceView and one compositor drawing the shell's layer over the
// shown page's. Android can't blend two SurfaceViews, so unlike the Mac there's one surface here
// and not a view per page. Touches still go by views: see BrowserWindow.java.

#include <string>
#include <vector>

#include "base/android/jni_android.h"
#include "base/android/jni_array.h"
#include "base/android/jni_string.h"
#include "base/android/scoped_java_ref.h"
#include "base/check.h"
#include "base/functional/bind.h"
#include "base/memory/raw_ptr.h"
#include "cc/slim/layer.h"
#include "cc/slim/solid_color_layer.h"
#include "components/viz/common/frame_timing_details.h"
#include "content/public/browser/android/compositor.h"
#include "content/public/browser/android/compositor_client.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/browser_main_parts.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser/window.h"
#include "third_party/skia/include/core/SkColor.h"
#include "ui/android/view_android.h"
#include "ui/android/window_android.h"
#include "ui/gfx/geometry/point_f.h"
#include "ui/gfx/geometry/size.h"

// Must come after all headers that specialize FromJniType() / ToJniType().
#include "project_b/engine/android/jni_headers/BrowserWindow_jni.h"

using base::android::AttachCurrentThread;
using base::android::JavaRef;
using base::android::ScopedJavaGlobalRef;

namespace project_b {

// The native side of BrowserWindow.java.
class BrowserWindow : public content::CompositorClient {
 public:
  BrowserWindow(JNIEnv* env,
                const JavaRef<jobject>& obj,
                ui::WindowAndroid* window_android)
      : java_(env, obj), root_(cc::slim::SolidColorLayer::Create()) {
    root_->SetBackgroundColor(SkColors::kWhite);
    root_->SetIsDrawable(true);
    compositor_.reset(content::Compositor::Create(this, window_android));
    compositor_->SetRootLayer(root_);
  }

  const JavaRef<jobject>& java() const { return java_; }

  // Children draw in order: the shown page first, the shell last, on top.
  void SetShell(content::WebContents* contents) {
    root_->AddChild(contents->GetNativeView()->GetLayer());
  }

  // Only the shown page's layer is in the tree.
  void ShowPage(content::WebContents* contents) {
    if (shown_) {
      shown_->GetNativeView()->GetLayer()->RemoveFromParent();
    }
    shown_ = contents;
    root_->InsertChild(contents->GetNativeView()->GetLayer(), 0);
  }

  // Over the shell, last in the tree.
  void SetOverlay(content::WebContents* contents) {
    root_->AddChild(contents->GetNativeView()->GetLayer());
    // When the window's first frame with the overlay in it reached the screen, whatever
    // the overlay's page had drawn by then.
    compositor_->RequestSuccessfulPresentationTimeForNextFrame(
        base::BindOnce([](const viz::FrameTimingDetails& details) {
          timeline::MarkAt("settings.open.overlay_on_screen",
                           details.presentation_feedback.timestamp);
        }));
  }

  void RemoveOverlay(content::WebContents* contents) {
    contents->GetNativeView()->GetLayer()->RemoveFromParent();
  }

  void RemovePage(content::WebContents* contents) {
    if (shown_ == contents) {
      contents->GetNativeView()->GetLayer()->RemoveFromParent();
      shown_ = nullptr;
    }
  }

  // Where nothing else draws: before the shell's first frame, and under a crashed page.
  void SetColor(uint32_t rgb) {
    root_->SetBackgroundColor(SkColor4f::FromColor(SkColorSetRGB(
        (rgb >> 16) & 0xff, (rgb >> 8) & 0xff, rgb & 0xff)));
  }

  void ListenForState(base::RepeatingCallback<void(const Window::State&)> cb) {
    on_state_ = std::move(cb);
    ReportState();
  }

  // Called from Java.
  void SurfaceCreated(JNIEnv* env) { surface_format_ = 0; }

  void SurfaceDestroyed(JNIEnv* env) {
    compositor_->PreserveChildSurfaceControls();
    compositor_->SetSurface(nullptr, false, nullptr);
    surface_format_ = 0;
  }

  void SurfaceChanged(JNIEnv* env,
                      int32_t format,
                      int32_t width,
                      int32_t height,
                      const JavaRef<jobject>& surface) {
    if (surface_format_ != format) {
      surface_format_ = format;
      compositor_->SetSurface(surface, /*can_be_used_with_surface_control=*/true,
                              nullptr);
    }
    gfx::Size size(width, height);
    compositor_->SetWindowBounds(size);
    root_->SetBounds(size);
  }

  void SizeChanged(JNIEnv* env, double width, double height) {
    size_ = gfx::SizeF(width, height);
    ReportState();
  }

  void ContentsMoved(JNIEnv* env,
                     const JavaRef<jobject>& jcontents,
                     int32_t left,
                     int32_t top,
                     int32_t width,
                     int32_t height) {
    content::WebContents* contents =
        content::WebContents::FromJavaWebContents(jcontents);
    ui::ViewAndroid* view = contents->GetNativeView();
    view->GetLayer()->SetPosition(gfx::PointF(left, top));
    view->OnPhysicalBackingSizeChanged(gfx::Size(width, height));
  }

  // The phone's back key closes settings while it is open, as its close button does; then
  // goes to the shell while it has something open the key closes; and is otherwise the
  // shell's back button while the tab has somewhere to go back to. Android has one
  // window, 1.
  bool BackKey(JNIEnv* env) {
    if (Engine::Get()->window(1)->settings) {
      hooks::Args args;
      args.Int(1);
      args.Str(R"({"t":"close"})");
      hooks::Fire(PB_EVENT_SETTINGS_MESSAGE, args);
      return true;
    }
    if (Engine::Get()->window(1)->shell_back) {
      hooks::Args args;
      args.Int(1);
      args.Str(R"({"t":"backKey"})");
      hooks::Fire(PB_EVENT_SHELL_MESSAGE, args);
      return true;
    }
    PageTab* tab = Engine::Get()->FocusedTab(Engine::Get()->window(1));
    if (!tab || !tab->contents()->GetController().CanGoBack()) {
      return false;
    }
    hooks::Args args;
    args.Int(1);
    args.Str(R"({"t":"back"})");
    hooks::Fire(PB_EVENT_SHELL_MESSAGE, args);
    return true;
  }

  void Stop(JNIEnv* env) { Go().stop(); }

 private:
  // A phone's window fills the screen and never moves, which is what full screen means to the
  // window manager: nothing drags. There are no window buttons and the corners are square.
  void ReportState() {
    if (on_state_ && !size_.IsEmpty()) {
      on_state_.Run({.size = size_,
                     .controls = gfx::RectF(),
                     .corner_radius = 0,
                     .full_screen = true});
    }
  }

  ScopedJavaGlobalRef<jobject> java_;
  scoped_refptr<cc::slim::SolidColorLayer> root_;
  std::unique_ptr<content::Compositor> compositor_;
  raw_ptr<content::WebContents> shown_ = nullptr;
  gfx::SizeF size_;
  base::RepeatingCallback<void(const Window::State&)> on_state_;
  int surface_format_ = 0;
};

namespace {
BrowserWindow* g_browser_window = nullptr;
}  // namespace

static int64_t JNI_BrowserWindow_Start(JNIEnv* env,
                                       const JavaRef<jobject>& obj,
                                       const JavaRef<jobject>& jwindow_android,
                                       int64_t go_table,
                                       int64_t process_age_ms,
                                       const JavaRef<jstring>& jblocked_hosts) {
  SetGo(reinterpret_cast<const PBGoCalls*>(go_table));
  timeline::Start(ProfileDir(), base::Milliseconds(process_age_ms));
  g_browser_window = new BrowserWindow(
      env, obj, ui::WindowAndroid::FromJavaWindowAndroid(jwindow_android));
  BrowserMainParts::Get()->StartApp(
      base::android::ConvertJavaStringToUTF8(env, jblocked_hosts));
  return reinterpret_cast<intptr_t>(g_browser_window);
}

struct Window::Impl {
  uint32_t frame_rgb = 0;
  bool has_frame = false;
  int theme = -1;
  // The last layout, which SetPages hands Java with the page.
  Layout layout;
  raw_ptr<content::WebContents> shown = nullptr;
};

// A phone's one window never closes, nor stops being the one in front, nor is minimised.
Window::Window(base::OnceClosure on_close,
               base::RepeatingClosure on_key,
               base::RepeatingCallback<void(bool)> on_minimised)
    : impl_(std::make_unique<Impl>()) {
  CHECK(g_browser_window);
}

Window::~Window() = default;

// A phone has one window, so there is never another to place it near.
void Window::PlaceNear(Window* other) {}

// Android put the activity's window on screen before content started.
void Window::Show() {}

void Window::Close() {
  Java_BrowserWindow_close(AttachCurrentThread(), g_browser_window->java());
}

// A phone shows no window title.
void Window::SetTitle(const std::u16string& title) {}

void Window::SetShellContents(content::WebContents* contents) {
  g_browser_window->SetShell(contents);
  Java_BrowserWindow_setShell(AttachCurrentThread(), g_browser_window->java(),
                              contents->GetJavaWebContents());
}

void Window::AddPageContents(content::WebContents* contents) {
  Java_BrowserWindow_addPage(AttachCurrentThread(), g_browser_window->java(),
                             contents->GetJavaWebContents());
}

void Window::SetOverlayContents(content::WebContents* contents) {
  g_browser_window->SetOverlay(contents);
  Java_BrowserWindow_setOverlay(AttachCurrentThread(), g_browser_window->java(),
                                contents->GetJavaWebContents());
}

void Window::RemoveOverlayContents(content::WebContents* contents) {
  g_browser_window->RemoveOverlay(contents);
  Java_BrowserWindow_removeOverlay(AttachCurrentThread(), g_browser_window->java());
}

void Window::RemovePageContents(content::WebContents* contents) {
  if (impl_->shown == contents) {
    impl_->shown = nullptr;
  }
  g_browser_window->RemovePage(contents);
  Java_BrowserWindow_removePage(AttachCurrentThread(), g_browser_window->java(),
                                contents->GetJavaWebContents());
}

// The frame's band is the shell's own edge strips here: with square corners there is no curve
// for the window to draw over the page.
// Java places the page and ignores a stale one itself, so the pages always follow.
bool Window::SetLayout(Layout layout) {
  JNIEnv* env = AttachCurrentThread();
  if (!impl_->has_frame || layout.frame_rgb != impl_->frame_rgb) {
    impl_->has_frame = true;
    impl_->frame_rgb = layout.frame_rgb;
    g_browser_window->SetColor(layout.frame_rgb);
    Java_BrowserWindow_setFrameColor(env, g_browser_window->java(),
                                     static_cast<int32_t>(layout.frame_rgb));
  }
  if (layout.theme != impl_->theme) {
    impl_->theme = layout.theme;
    Java_BrowserWindow_setTheme(env, g_browser_window->java(), layout.theme);
  }

  impl_->layout = std::move(layout);
  return true;
}

// A phone shows one page at a time: the first the layout lists.
void Window::SetPages(const std::vector<Page>& pages) {
  JNIEnv* env = AttachCurrentThread();
  const Layout& layout = impl_->layout;
  if (!pages.empty() && pages.front().contents != impl_->shown) {
    impl_->shown = pages.front().contents;
    g_browser_window->ShowPage(impl_->shown);
    Java_BrowserWindow_showPage(env, g_browser_window->java(),
                                impl_->shown->GetJavaWebContents());
  }
  std::vector<double> flat;
  flat.reserve(layout.regions.size() * 5);
  for (const Region& r : layout.regions) {
    flat.push_back(r.rect.x());
    flat.push_back(r.rect.y());
    flat.push_back(r.rect.width());
    flat.push_back(r.rect.height());
    flat.push_back(static_cast<double>(r.area));
  }
  const gfx::RectF page = pages.empty() ? gfx::RectF() : pages.front().rect;
  Java_BrowserWindow_setLayout(env, g_browser_window->java(),
                               layout.viewport.width(), layout.viewport.height(),
                               page.x(), page.y(), page.width(), page.height(),
                               base::android::ToJavaDoubleArray(env, flat));
}

void Window::ListenForState(
    base::RepeatingCallback<void(const State&)> on_change) {
  g_browser_window->ListenForState(std::move(on_change));
}

void Window::InstallPackage(const std::string& path) {
  JNIEnv* env = AttachCurrentThread();
  Java_BrowserWindow_installPackage(env, g_browser_window->java(),
                                    base::android::ConvertUTF8ToJavaString(env, path));
}

void Window::Minimize() {
  Java_BrowserWindow_minimize(AttachCurrentThread(), g_browser_window->java());
}

// A phone's window always fills the screen, so there is nothing to zoom or make full screen.
void Window::Zoom() {}

void Window::ToggleFullScreen() {}

// Android runs one copy of the app, so no other copy hands over to bring it forward.
void Window::BringToFront() {}

// static
bool Window::HandleUnhandledKey(const input::NativeWebKeyboardEvent& event) {
  return false;
}

}  // namespace project_b

DEFINE_JNI(BrowserWindow)
