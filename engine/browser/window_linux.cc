#include "project_b/engine/browser/window.h"

#include <algorithm>
#include <cmath>
#include <memory>
#include <utility>
#include <vector>

#include "base/check.h"
#include "base/command_line.h"
#include "base/functional/bind.h"
#include "base/functional/callback.h"
#include "base/memory/raw_ptr.h"
#include "base/memory/weak_ptr.h"
#include "base/no_destructor.h"
#include "base/task/sequenced_task_runner.h"
#include "components/input/native_web_keyboard_event.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/timeline.h"
#include "third_party/skia/include/core/SkColor.h"
#include "ui/aura/client/default_capture_client.h"
#include "ui/aura/client/focus_client.h"
#include "ui/aura/client/window_parenting_client.h"
#include "ui/aura/env.h"
#include "ui/aura/window.h"
#include "ui/aura/window_targeter.h"
#include "ui/aura/window_tracker.h"
#include "ui/aura/window_tree_host_platform.h"
#include "ui/compositor/layer.h"
#include "ui/compositor/layer_solid_color.h"
#include "ui/display/screen.h"
#include "ui/events/event.h"
#include "ui/gfx/geometry/rect_conversions.h"
#include "ui/native_theme/native_theme.h"
#include "ui/platform_window/platform_window.h"
#include "ui/platform_window/platform_window_init_properties.h"
#include "ui/wm/core/base_focus_rules.h"
#include "ui/wm/core/focus_controller.h"
#include "ui/wm/public/activation_client.h"

// Linux's window (docs/research/linux.md, 2): one Aura window tree per top-level window, on
// Ozone, the same calls from Go as window_mac.mm. Bottom to top: the frame's colour, the pages
// holder (our pages' background, then each page's view), the shell's view, and an extension's
// popup. Go's regions decide who gets each press, as the Mac's hitTest: does: each holder's
// targeter lets an event into it only where the layout gives the point to it.
//
// Step 2 of the Linux work is the tree only. The top row's own parts (moving from a drag area,
// the resize border, a double-click to maximise) and the menu strip come in step 3.

namespace project_b {

namespace {

constexpr gfx::Rect kFirstBounds(80, 120, 1280, 720);

SkColor4f Hex(uint32_t rgb) {
  return SkColor4f::FromColor(SkColorSetRGB((rgb >> 16) & 0xff, (rgb >> 8) & 0xff, rgb & 0xff));
}

// Every window of ours is a top-level window of its own, so each root activates its children.
class FocusRules : public wm::BaseFocusRules {
 public:
  bool SupportsChildActivation(const aura::Window* window) const override {
    return window->IsRootWindow();
  }
};

// New windows content makes without a parent (a page's select box, its tooltip) go into the
// root of the window that holds the context they belong to.
class ParentingClient : public aura::client::WindowParentingClient {
 public:
  explicit ParentingClient(aura::Window* root) : root_(root) {
    aura::client::SetWindowParentingClient(root_, this);
  }
  ~ParentingClient() override { aura::client::SetWindowParentingClient(root_, nullptr); }

  aura::Window* GetDefaultParent(aura::Window* window,
                                 const gfx::Rect& bounds,
                                 int64_t display_id) override {
    return root_;
  }

 private:
  raw_ptr<aura::Window> root_;
};

}  // namespace

struct Window::Impl {
  // The platform window, which reports its state and the close button to the window.
  class Host : public aura::WindowTreeHostPlatform {
   public:
    Host(ui::PlatformWindowInitProperties properties, Impl* impl)
        : aura::WindowTreeHostPlatform(std::move(properties)), impl_(impl) {}

    ui::PlatformWindow* platform() { return platform_window(); }
    const ui::PlatformWindow* platform() const { return platform_window(); }

   private:
    void OnBoundsChanged(const BoundsChange& change) override {
      const gfx::Size before = impl_->root()->bounds().size();
      aura::WindowTreeHostPlatform::OnBoundsChanged(change);
      impl_->Relayout(before);
      impl_->ReportState();
    }
    void OnWindowStateChanged(ui::PlatformWindowState old_state,
                              ui::PlatformWindowState new_state) override {
      aura::WindowTreeHostPlatform::OnWindowStateChanged(old_state, new_state);
      const bool was = old_state == ui::PlatformWindowState::kMinimized;
      const bool is = new_state == ui::PlatformWindowState::kMinimized;
      if (was != is) {
        impl_->on_minimised.Run(is);
      }
      impl_->ReportState();
    }
    // The window manager's close: the same as Close.
    void OnCloseRequest() override { impl_->Closed(); }
    void OnActivationChanged(bool active) override {
      aura::WindowTreeHostPlatform::OnActivationChanged(active);
      if (active) {
        impl_->on_key.Run();
      }
    }

    raw_ptr<Impl> impl_;
  };

  // Lets an event into a holder only where the layout gives the point to it.
  class Targeter : public aura::WindowTargeter {
   public:
    Targeter(Impl* impl, Area area) : impl_(impl), area_(area) {}

    bool SubtreeShouldBeExploredForEvent(aura::Window* window,
                                         const ui::LocatedEvent& event) override {
      gfx::PointF p = event.location_f();
      aura::Window::ConvertPointToTarget(window->parent(), impl_->root(), &p);
      return impl_->AreaAt(p) == area_ &&
             aura::WindowTargeter::SubtreeShouldBeExploredForEvent(window, event);
    }

   private:
    raw_ptr<Impl> impl_;
    Area area_;
  };

  Impl(base::OnceClosure on_close,
       base::RepeatingClosure on_key,
       base::RepeatingCallback<void(bool)> on_minimised)
      : on_close(std::move(on_close)),
        on_key(std::move(on_key)),
        on_minimised(std::move(on_minimised)) {}

  aura::Window* root() const { return host->window(); }

  // The area a point in the window, in shell coordinates, belongs to: the first region that
  // holds it, else the page. A popup's own rectangle is the popup's.
  Area AreaAt(const gfx::PointF& p) const {
    for (const Region& r : regions) {
      if (r.rect.Contains(p)) {
        return r.area;
      }
    }
    return Area::kPage;
  }

  // The holders cover the window. Each page keeps its four margins, as the Mac's do through
  // autoresizing, so a page follows a resize in the same frame, before the shell has said
  // anything; the shell's next layout then places it.
  void Relayout(const gfx::Size& before) {
    const gfx::Size now = root()->bounds().size();
    if (!before.IsEmpty() && before != now) {
      const int dw = now.width() - before.width();
      const int dh = now.height() - before.height();
      for (aura::Window* page : pages->children()) {
        if (page == page_background) {
          continue;
        }
        gfx::Rect r = page->bounds();
        r.set_width(std::max(0, r.width() + dw));
        r.set_height(std::max(0, r.height() + dh));
        page->SetBounds(r);
      }
    }
    const gfx::Rect all(now);
    background->SetBounds(all);
    pages->SetBounds(all);
    page_background->SetBounds(all);
    shell_holder->SetBounds(all);
    if (aura::Window* view = Shell()) {
      view->SetBounds(all);
    }
  }

  bool Dark() const {
    if (theme == 1 || theme == 2) {
      return theme == 2;
    }
    return ui::NativeTheme::GetInstanceForNativeUi()->preferred_color_scheme() ==
           ui::NativeTheme::PreferredColorScheme::kDark;
  }

  // The frame's colour wherever nothing else draws, and our pages' background under them.
  void Paint() {
    const bool dark = Dark();
    if (frame_rgb >= 0) {
      background->layer()->AsSolidColor()->SetColor(
          Hex(static_cast<uint32_t>(dark ? frame_rgb_dark : frame_rgb)));
    }
    if (page_rgb >= 0) {
      page_background->layer()->AsSolidColor()->SetColor(
          Hex(static_cast<uint32_t>(dark ? page_rgb_dark : page_rgb)));
    }
  }

  ui::PlatformWindowState PlatformState() const {
    return host->platform() ? host->platform()->GetPlatformWindowState()
                            : ui::PlatformWindowState::kNormal;
  }

  // The window's frame on screen in DIPs, y down from the top of the screen. Wayland gives an
  // app no place of its own, so there the origin is 0.
  gfx::Rect Bounds() const {
    return gfx::ScaleToEnclosingRect(host->GetBoundsInPixels(),
                                     1.0f / host->device_scale_factor());
  }

  void ReportState() {
    if (!on_state) {
      return;
    }
    const ui::PlatformWindowState platform = PlatformState();
    const State state = {
        .size = gfx::SizeF(root()->bounds().size()),
        // The shell draws the window buttons itself, at the right of its top row.
        .controls = gfx::RectF(),
        .corner_radius = 0,
        .full_screen = platform == ui::PlatformWindowState::kFullScreen,
        .bounds = Bounds(),
        .minimised = platform == ui::PlatformWindowState::kMinimized,
        .zoomed = platform == ui::PlatformWindowState::kMaximized,
    };
    if (state.size == last_state.size && state.full_screen == last_state.full_screen &&
        state.bounds == last_state.bounds && state.minimised == last_state.minimised &&
        state.zoomed == last_state.zoomed && last_state.corner_radius == 0) {
      return;
    }
    last_state = state;
    on_state.Run(state);
  }

  // Runs on_close once, a turn later, so whoever closed the window returns first: Go's
  // close deletes this window.
  void Closed() {
    if (!on_close) {
      return;
    }
    host->Hide();
    base::SequencedTaskRunner::GetCurrentDefault()->PostTask(
        FROM_HERE, base::BindOnce(
                       [](base::WeakPtr<Impl> impl) {
                         if (impl && impl->on_close) {
                           std::move(impl->on_close).Run();
                         }
                       },
                       weak.GetWeakPtr()));
  }

  std::unique_ptr<Host> host;
  std::unique_ptr<wm::FocusController> focus;
  std::unique_ptr<aura::client::DefaultCaptureClient> capture;
  std::unique_ptr<ParentingClient> parenting;
  // The holders, owned by the tree: the window's colour, the pages and the shell.
  raw_ptr<aura::Window> background = nullptr;
  raw_ptr<aura::Window> pages = nullptr;
  raw_ptr<aura::Window> page_background = nullptr;
  raw_ptr<aura::Window> shell_holder = nullptr;
  // The shell's view and an extension's popup are content's: a tracker drops one content
  // destroyed first, as it does a closing window's shell.
  aura::WindowTracker shell;
  aura::WindowTracker popup;
  aura::Window* Shell() const { return shell.windows().empty() ? nullptr : shell.windows().front(); }
  aura::Window* Popup() const { return popup.windows().empty() ? nullptr : popup.windows().front(); }
  std::vector<Region> regions;

  base::OnceClosure on_close;
  base::RepeatingClosure on_key;
  base::RepeatingCallback<void(bool)> on_minimised;
  base::RepeatingCallback<void(const State&)> on_state;
  State last_state;

  int64_t frame_rgb = -1;
  int64_t frame_rgb_dark = -1;
  int64_t page_rgb = -1;
  int64_t page_rgb_dark = -1;
  double frame_width = -1;
  int theme = -1;
  size_t ignored_stale_bounds = 0;
  uint32_t shown_rgb = 0;
  bool shown = false;
  gfx::Rect place = kFirstBounds;

  base::WeakPtrFactory<Impl> weak{this};
};

namespace {

// A holder: an Aura window of ours that draws nothing (or one colour) and holds others. Its
// parent owns it.
aura::Window* AddHolder(aura::Window* parent, ui::LayerType type) {
  aura::Window* window = new aura::Window(nullptr);
  window->Init(type);
  window->Show();
  parent->AddChild(window);
  return window;
}

// The view under view that takes a page's keys, content's RenderWidgetHostViewAura, or
// null: the view a WebContents gives is the one around it, which drops them.
aura::Window* RenderView(aura::Window* view) {
  if (!view) {
    return nullptr;
  }
  if (view->GetName() == "RenderWidgetHostViewAura") {
    return view;
  }
  for (aura::Window* child : view->children()) {
    if (aura::Window* found = RenderView(child)) {
      return found;
    }
  }
  return nullptr;
}

}  // namespace

Window::Window(base::OnceClosure on_close,
               base::RepeatingClosure on_key,
               base::RepeatingCallback<void(bool)> on_minimised)
    : impl_(std::make_unique<Impl>(std::move(on_close), std::move(on_key),
                                   std::move(on_minimised))) {
  ui::PlatformWindowInitProperties properties;
  properties.bounds = kFirstBounds;
  // The shell draws the top row, the window buttons and the frame (docs/research/linux.md, 2).
  properties.remove_standard_frame = true;
  properties.wm_class_name = "io.github.sudiptadeb.project-b";
  properties.wm_class_class = "io.github.sudiptadeb.project-b";
  properties.wayland_app_id = "io.github.sudiptadeb.project-b";
  impl_->host = std::make_unique<Impl::Host>(std::move(properties), impl_.get());
  impl_->host->InitHost();

  aura::Window* root = impl_->root();
  impl_->focus = std::make_unique<wm::FocusController>(new FocusRules());
  aura::client::SetFocusClient(root, impl_->focus.get());
  wm::SetActivationClient(root, impl_->focus.get());
  root->AddPreTargetHandler(impl_->focus.get());
  impl_->capture = std::make_unique<aura::client::DefaultCaptureClient>(root);
  impl_->parenting = std::make_unique<ParentingClient>(root);

  impl_->background = AddHolder(root, ui::LAYER_SOLID_COLOR);
  impl_->background->SetEventTargetingPolicy(aura::EventTargetingPolicy::kNone);
  impl_->pages = AddHolder(root, ui::LAYER_NOT_DRAWN);
  impl_->pages->SetEventTargeter(std::make_unique<Impl::Targeter>(impl_.get(), Area::kPage));
  impl_->page_background = AddHolder(impl_->pages, ui::LAYER_SOLID_COLOR);
  impl_->page_background->SetEventTargetingPolicy(aura::EventTargetingPolicy::kNone);
  // Unseen until ShowPages. Transparent rather than hidden: content takes a page in a hidden
  // window as hidden, and it would only begin to draw once shown.
  impl_->pages->layer()->SetOpacity(0);
  impl_->shell_holder = AddHolder(root, ui::LAYER_NOT_DRAWN);
  impl_->shell_holder->SetEventTargeter(
      std::make_unique<Impl::Targeter>(impl_.get(), Area::kShell));
  impl_->Relayout(gfx::Size());
}

Window::~Window() {
  impl_->on_state.Reset();
  aura::Window* root = impl_->root();
  root->RemovePreTargetHandler(impl_->focus.get());
  // Content's views are content's: those still here leave the tree before it goes.
  if (aura::Window* shell = impl_->Shell()) {
    impl_->shell_holder->RemoveChild(shell);
  }
  while (impl_->pages->children().size() > 1) {
    impl_->pages->RemoveChild(impl_->pages->children().back());
  }
  if (aura::Window* popup = impl_->Popup()) {
    root->RemoveChild(popup);
  }
  aura::client::SetFocusClient(root, nullptr);
  wm::SetActivationClient(root, nullptr);
  impl_->parenting.reset();
  impl_->capture.reset();
  impl_->host.reset();
  impl_->focus.reset();
}

// Where the other window is, a step down and right of it; Wayland ignores the place.
void Window::PlaceNear(Window* other) {
  gfx::Rect near = other->impl_->host->GetBoundsInPixels();
  near.Offset(24, 24);
  impl_->host->SetBoundsInPixels(near);
}

void Window::Show() {
  impl_->Paint();
  if (impl_->frame_rgb >= 0) {
    impl_->shown_rgb =
        static_cast<uint32_t>(impl_->Dark() ? impl_->frame_rgb_dark : impl_->frame_rgb);
  }
  impl_->host->Show();
  impl_->shown = true;
  BringToFront();
}

void Window::Close() {
  impl_->Closed();
}

void Window::SetTitle(const std::u16string& title) {
  impl_->root()->SetTitle(title);
  if (impl_->host->platform()) {
    impl_->host->platform()->SetTitle(title);
  }
}

// Sharing comes with the portals (step 4).
void Window::ShareFile(const std::string& path, const gfx::RectF& anchor) {}
void Window::ShareText(const std::string& text) {}
void CopyImageFile(const std::string& path) {}

void Window::SetShellView(gfx::NativeView view) {
  aura::Window* shell = view;
  if (impl_->Shell() == shell) {
    return;
  }
  if (aura::Window* old = impl_->Shell()) {
    impl_->shell_holder->RemoveChild(old);
    impl_->shell.Remove(old);
  }
  impl_->shell.Add(shell);
  impl_->shell_holder->AddChild(shell);
  shell->SetBounds(gfx::Rect(impl_->root()->bounds().size()));
  shell->Show();
}

// A page's view joins the holder the first time the window shows it, and moves here from
// another window's holder the same way. Its place is the layout's, rounded to whole DIPs.
void Window::SetPages(const std::vector<Page>& pages) {
  aura::Window* holder = impl_->pages;
  std::vector<aura::Window*> shown;
  for (const Page& p : pages) {
    aura::Window* page = p.contents->GetNativeView();
    if (page->parent() != holder) {
      if (page->parent()) {
        page->parent()->RemoveChild(page);
      }
      holder->AddChild(page);
    }
    page->SetBounds(gfx::ToRoundedRect(p.rect));
    page->Show();
    shown.push_back(page);
  }
  for (aura::Window* other : holder->children()) {
    if (other != impl_->page_background && std::ranges::find(shown, other) == shown.end()) {
      other->Hide();
    }
  }
}

void Window::ShowPages() {
  impl_->pages->layer()->SetOpacity(1);
  timeline::Mark("window.pages_shown");
}

void Window::HidePages() {
  impl_->pages->layer()->SetOpacity(0);
}

void Window::RemovePageView(gfx::NativeView view) {
  aura::Window* page = view;
  if (page->parent()) {
    page->parent()->RemoveChild(page);
  }
}

#if BUILDFLAG(PB_EXTENSIONS)
// The popup goes over everything, under the anchor with a 4 point gap, right edges together,
// inside the window. Presses inside it are its own.
void Window::SetPopupView(gfx::NativeView view,
                          const gfx::RectF& anchor,
                          const gfx::Size& size) {
  aura::Window* popup = view;
  aura::Window* root = impl_->root();
  if (impl_->Popup() != popup) {
    if (aura::Window* old = impl_->Popup()) {
      root->RemoveChild(old);
      impl_->popup.Remove(old);
    }
    impl_->popup.Add(popup);
    root->AddChild(popup);
  }
  root->StackChildAtTop(popup);
  const gfx::Size bounds = root->bounds().size();
  const int x = std::clamp<int>(std::lround(anchor.right()) - size.width(), 0,
                                std::max(0, bounds.width() - size.width()));
  popup->SetBounds(gfx::Rect(x, std::lround(anchor.bottom()) + 4, size.width(), size.height()));
  popup->Show();
}

void Window::RemovePopupView(gfx::NativeView view) {
  aura::Window* popup = view;
  if (popup->parent()) {
    popup->parent()->RemoveChild(popup);
  }
  impl_->popup.Remove(popup);
}

gfx::RectF Window::popup_bounds() const {
  aura::Window* popup = impl_->Popup();
  return popup ? gfx::RectF(popup->bounds()) : gfx::RectF();
}
#endif

void Window::NoteClick(const gfx::PointF& in_window, base::TimeTicks made) {
  if (impl_->AreaAt(in_window) == Area::kShell) {
    timeline::Input("input.click", made);
  }
}

bool Window::SetLayout(Layout layout) {
  impl_->regions = std::move(layout.regions);
  if (layout.frame_rgb != impl_->frame_rgb || layout.frame_rgb_dark != impl_->frame_rgb_dark ||
      layout.frame_width != impl_->frame_width || layout.page_rgb != impl_->page_rgb ||
      layout.page_rgb_dark != impl_->page_rgb_dark || layout.theme != impl_->theme) {
    impl_->frame_rgb = layout.frame_rgb;
    impl_->frame_rgb_dark = layout.frame_rgb_dark;
    impl_->frame_width = layout.frame_width;
    impl_->page_rgb = layout.page_rgb;
    impl_->page_rgb_dark = layout.page_rgb_dark;
    impl_->theme = layout.theme;
    impl_->Paint();
  }
  const gfx::Size size = impl_->root()->bounds().size();
  if (std::round(layout.viewport.width()) != size.width() ||
      std::round(layout.viewport.height()) != size.height()) {
    impl_->ignored_stale_bounds++;
    return false;
  }
  return true;
}

void Window::ListenForState(base::RepeatingCallback<void(const State&)> on_change) {
  impl_->on_state = std::move(on_change);
  impl_->last_state = {.corner_radius = -1};  // So the first report always goes.
  impl_->ReportState();
}

void Window::Minimize() {
  if (impl_->host->platform()) {
    impl_->host->platform()->Minimize();
  }
}

void Window::Zoom() {
  if (IsMaximized()) {
    Restore();
  } else {
    Maximize();
  }
}

void Window::ToggleFullScreen() {
  SetFullscreen(!IsFullscreen());
}

void Window::SetSystemBarsHidden(bool hidden) {}

void Window::BringToFront() {
  if (!impl_->host->platform()) {
    return;
  }
  if (IsMinimized()) {
    impl_->host->platform()->Restore();
  }
  impl_->host->platform()->Activate();
}

// No menu bar of the app's on Linux; the menu strip, in step 3, is the shell's.
// static
bool Window::HandleUnhandledKey(const input::NativeWebKeyboardEvent& event) {
  return false;
}

gfx::Rect Window::GetBounds() const {
  return impl_->Bounds();
}

void Window::SetBounds(const gfx::Rect& bounds) {
  impl_->host->SetBoundsInPixels(
      gfx::ScaleToEnclosingRect(bounds, impl_->host->device_scale_factor()));
}

bool Window::IsMinimized() const {
  return impl_->PlatformState() == ui::PlatformWindowState::kMinimized;
}

bool Window::IsMaximized() const {
  return impl_->PlatformState() == ui::PlatformWindowState::kMaximized;
}

bool Window::IsFullscreen() const {
  return impl_->PlatformState() == ui::PlatformWindowState::kFullScreen;
}

void Window::Maximize() {
  if (impl_->host->platform() && !IsMaximized()) {
    impl_->host->platform()->Maximize();
  }
}

void Window::SetFullscreen(bool fullscreen) {
  if (impl_->host->platform() && IsFullscreen() != fullscreen) {
    impl_->host->platform()->SetFullscreen(fullscreen, display::kInvalidDisplayId);
  }
}

void Window::Restore() {
  if (impl_->host->platform()) {
    impl_->host->platform()->Restore();
  }
}

void Window::SetMode(int mode) {
  switch (mode) {
    case PB_WINDOW_NORMAL:
      Restore();
      break;
    case PB_WINDOW_MINIMISED:
      Minimize();
      break;
    case PB_WINDOW_ZOOMED:
      SetFullscreen(false);
      Maximize();
      break;
    case PB_WINDOW_IN_FULL_SCREEN:
      SetFullscreen(true);
      break;
  }
}

// The driver activates the window itself, as a person's click would (driver_linux.cc).
void Window::ForceKeyForDriver(bool key) {
  if (key) {
    // The keyboard goes back to the view of this window that last had it, else the shell's,
    // as a click on the window gives it.
    aura::Window* focused = impl_->focus->GetFocusedWindow();
    const bool ours = focused == impl_->root() || focused == impl_->shell_holder ||
                      focused == impl_->pages;
    aura::Window* to = focused && !ours && impl_->root()->Contains(focused)
                           ? focused
                           : RenderView(impl_->Shell());
    if (to) {
      impl_->focus->FocusWindow(to);
    } else {
      impl_->focus->ActivateWindow(impl_->shell_holder);
    }
    impl_->on_key.Run();
  } else if (const aura::Window* active = impl_->focus->GetActiveWindow()) {
    impl_->focus->DeactivateWindow(const_cast<aura::Window*>(active));
  }
}

gfx::NativeWindow Window::native_window() const {
  return impl_->root();
}

// The app moves the window from a drag area itself on Linux, from step 3.
bool Window::MovesFromApp(const gfx::PointF& in_window) const {
  return impl_->AreaAt(in_window) == Area::kDrag;
}

size_t Window::ignored_stale_bounds() const {
  return impl_->ignored_stale_bounds;
}

uint32_t Window::shown_rgb() const {
  return impl_->shown_rgb;
}

}  // namespace project_b
