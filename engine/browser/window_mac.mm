#include "project_b/engine/browser/window.h"

#import <Cocoa/Cocoa.h>

#include <cmath>
#include <utility>

#include "base/functional/callback.h"
#include "base/strings/sys_string_conversions.h"
#include "components/input/native_web_keyboard_event.h"

// Answers the OS hit test. Content's own views ask this same question, so a view that
// loses the answer ignores the event (see shouldIgnoreMouseEvent: in
// content/app_shim_remote_cocoa/render_widget_host_view_cocoa.mm).
@interface PBContainerView : NSView
@property(nonatomic, weak) NSView* shellView;
@property(nonatomic, weak) NSView* pageView;
- (void)setRegions:(std::vector<gfx::RectF>)regions;
@end

@implementation PBContainerView {
  std::vector<gfx::RectF> _regions;
}

@synthesize shellView = _shellView;
@synthesize pageView = _pageView;

- (void)setRegions:(std::vector<gfx::RectF>)regions {
  _regions = std::move(regions);
}

- (NSView*)hitTest:(NSPoint)point {
  // The point is in our superview's coordinates, and our subviews want ours.
  NSPoint p = [self convertPoint:point fromView:self.superview];
  if (!NSPointInRect(p, self.bounds)) {
    return nil;
  }
  // Shell rectangles measure y from the top; Cocoa measures it from the bottom.
  const float y = self.bounds.size.height - p.y;
  for (const gfx::RectF& r : _regions) {
    if (r.Contains(p.x, y)) {
      return [self.shellView hitTest:p];
    }
  }
  NSView* page = [self.pageView hitTest:p];
  return page ? page : self;
}

@end

// In a session with no real GUI the OS never makes us the key window. The driver can
// say we are, so focus behaves as it does on a real desktop. Never set otherwise.
@interface PBNSWindow : NSWindow
@property(nonatomic) BOOL forceKeyForDriver;
@end

@implementation PBNSWindow

@synthesize forceKeyForDriver = _forceKeyForDriver;

- (BOOL)isKeyWindow {
  return _forceKeyForDriver || [super isKeyWindow];
}

- (BOOL)isMainWindow {
  return _forceKeyForDriver || [super isMainWindow];
}

@end

@interface PBWindowDelegate : NSObject <NSWindowDelegate>
@property(nonatomic) base::RepeatingClosure onControlsMayHaveMoved;
- (instancetype)initWithOnClose:(base::OnceClosure)onClose;
@end

@implementation PBWindowDelegate {
  base::OnceClosure _onClose;
}

@synthesize onControlsMayHaveMoved = _onControlsMayHaveMoved;

- (instancetype)initWithOnClose:(base::OnceClosure)onClose {
  if ((self = [super init])) {
    _onClose = std::move(onClose);
  }
  return self;
}

- (void)windowWillClose:(NSNotification*)notification {
  std::move(_onClose).Run();
}

- (void)windowDidResize:(NSNotification*)notification {
  if (_onControlsMayHaveMoved) {
    _onControlsMayHaveMoved.Run();
  }
}

- (void)windowDidEnterFullScreen:(NSNotification*)notification {
  if (_onControlsMayHaveMoved) {
    _onControlsMayHaveMoved.Run();
  }
}

- (void)windowDidExitFullScreen:(NSNotification*)notification {
  if (_onControlsMayHaveMoved) {
    _onControlsMayHaveMoved.Run();
  }
}

@end

namespace project_b {

struct Window::Impl {
  PBNSWindow* __strong window;
  PBContainerView* __strong container;
  PBWindowDelegate* __strong delegate;
  gfx::RectF page_bounds;
  size_t ignored_stale_bounds = 0;
  base::RepeatingCallback<void(const gfx::RectF&)> on_controls;
  gfx::RectF last_controls;

  // The close and zoom buttons bound the controls. In full screen macOS hides them.
  gfx::RectF Controls() const {
    if (window.styleMask & NSWindowStyleMaskFullScreen) {
      return gfx::RectF();
    }
    NSButton* close_button = [window standardWindowButton:NSWindowCloseButton];
    NSButton* zoom_button = [window standardWindowButton:NSWindowZoomButton];
    if (!close_button || !zoom_button || close_button.hidden) {
      return gfx::RectF();
    }
    const NSRect close_frame = [close_button convertRect:close_button.bounds
                                                  toView:container];
    const NSRect zoom_frame = [zoom_button convertRect:zoom_button.bounds
                                                toView:container];
    // Shell coordinates measure y from the top.
    return gfx::RectF(NSMinX(close_frame),
                      container.bounds.size.height - NSMaxY(close_frame),
                      NSMaxX(zoom_frame) - NSMinX(close_frame),
                      NSHeight(close_frame));
  }

  void ReportControls() {
    const gfx::RectF controls = Controls();
    if (controls == last_controls) {
      return;
    }
    last_controls = controls;
    on_controls.Run(controls);
  }
};

Window::Window(base::OnceClosure on_close) : impl_(std::make_unique<Impl>()) {
  impl_->window = [[PBNSWindow alloc]
      initWithContentRect:NSMakeRect(120, 120, 1100, 700)
                styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
                          NSWindowStyleMaskMiniaturizable |
                          NSWindowStyleMaskResizable |
                          NSWindowStyleMaskFullSizeContentView
                  backing:NSBackingStoreBuffered
                    defer:NO];
  impl_->window.releasedWhenClosed = NO;
  impl_->window.title = @"Project B";
  // The shell draws all the way to the top of the window, with the window buttons on it.
  // The title bar still drags the window, so the shell keeps its controls below it.
  impl_->window.titlebarAppearsTransparent = YES;
  impl_->window.titleVisibility = NSWindowTitleHidden;
  impl_->window.acceptsMouseMovedEvents = YES;
  impl_->delegate = [[PBWindowDelegate alloc] initWithOnClose:std::move(on_close)];
  impl_->window.delegate = impl_->delegate;

  impl_->container = [[PBContainerView alloc]
      initWithFrame:NSMakeRect(0, 0, 1100, 700)];
  impl_->container.wantsLayer = YES;
  impl_->window.contentView = impl_->container;
}

Window::~Window() {
  impl_->window.delegate = nil;
}

void Window::Show() {
  [impl_->window makeKeyAndOrderFront:nil];
  [NSApp activateIgnoringOtherApps:YES];
}

void Window::SetTitle(const std::u16string& title) {
  impl_->window.title = base::SysUTF16ToNSString(title);
}

void Window::SetShellView(gfx::NativeView view) {
  NSView* shell = view.GetNativeNSView();
  shell.frame = impl_->container.bounds;
  shell.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
  impl_->container.shellView = shell;
  [impl_->container addSubview:shell];
}

void Window::SetPageView(gfx::NativeView view) {
  NSView* page = view.GetNativeNSView();
  // Keeping its margins means it follows a window resize in the same frame, before
  // the shell has had a chance to say anything.
  page.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
  page.frame = impl_->container.bounds;
  [impl_->container.pageView removeFromSuperview];
  impl_->container.pageView = page;
  [impl_->container addSubview:page
                    positioned:NSWindowBelow
                    relativeTo:impl_->container.shellView];
}

void Window::SetPageBounds(const gfx::RectF& bounds,
                           const gfx::SizeF& viewport) {
  const NSSize size = impl_->container.bounds.size;
  if (std::round(viewport.width()) != std::round(size.width) ||
      std::round(viewport.height()) != std::round(size.height)) {
    impl_->ignored_stale_bounds++;
    return;
  }
  impl_->page_bounds = bounds;
  impl_->container.pageView.frame =
      NSMakeRect(std::round(bounds.x()),
                 std::round(size.height - bounds.y() - bounds.height()),
                 std::round(bounds.width()), std::round(bounds.height()));
}

void Window::SetShellRegions(std::vector<gfx::RectF> regions) {
  [impl_->container setRegions:std::move(regions)];
}

void Window::ListenForControls(
    base::RepeatingCallback<void(const gfx::RectF&)> on_change) {
  impl_->on_controls = std::move(on_change);
  impl_->last_controls = gfx::RectF(-1, -1, 0, 0);  // So the first report always goes.
  impl_->delegate.onControlsMayHaveMoved = base::BindRepeating(
      [](Impl* impl) { impl->ReportControls(); }, impl_.get());
  impl_->ReportControls();
}

bool Window::HandleUnhandledKey(const input::NativeWebKeyboardEvent& event) {
  NSEvent* ns_event = event.os_event.Get();
  if (ns_event.type != NSEventTypeKeyDown) {
    return false;
  }
  return [NSApp.mainMenu performKeyEquivalent:ns_event];
}

void Window::ForceKeyForDriver() {
  if (impl_->window.isKeyWindow) {
    return;
  }
  impl_->window.forceKeyForDriver = YES;
  NSNotificationCenter* center = NSNotificationCenter.defaultCenter;
  [center postNotificationName:NSWindowDidBecomeMainNotification
                        object:impl_->window];
  [center postNotificationName:NSWindowDidBecomeKeyNotification
                        object:impl_->window];
}

gfx::NativeWindow Window::native_window() const {
  return gfx::NativeWindow(impl_->window);
}

gfx::RectF Window::page_bounds() const {
  return impl_->page_bounds;
}

size_t Window::ignored_stale_bounds() const {
  return impl_->ignored_stale_bounds;
}

}  // namespace project_b
