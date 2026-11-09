#include "project_b/engine/browser/window.h"

#import <Cocoa/Cocoa.h>

#include <cmath>
#include <string_view>
#include <utility>

#include "base/functional/callback.h"
#include "base/strings/sys_string_conversions.h"
#include "components/input/native_web_keyboard_event.h"

// Answers the OS hit test. Content's own views ask this same question, so a view that
// loses the answer ignores the event (see shouldIgnoreMouseEvent: in
// content/app_shim_remote_cocoa/render_widget_host_view_cocoa.mm).
//
// In a drag region the answer is nil, so the event falls through to the window's own
// frame, which moves the window and, inside the title bar, zooms on a double-click.
// That is how Chrome does it (components/remote_cocoa/app_shim/bridged_content_view.mm).
@interface PBContainerView : NSView
@property(nonatomic, weak) NSView* shellView;
// Holds every page view; the shown one is the only one not hidden.
@property(nonatomic, strong) NSView* pages;
- (void)setRegions:(std::vector<gfx::RectF>)regions;
- (void)setDragRegions:(std::vector<gfx::RectF>)regions;
@end

@implementation PBContainerView {
  std::vector<gfx::RectF> _regions;
  std::vector<gfx::RectF> _dragRegions;
}

@synthesize shellView = _shellView;
@synthesize pages = _pages;

- (void)setRegions:(std::vector<gfx::RectF>)regions {
  _regions = std::move(regions);
}

- (void)setDragRegions:(std::vector<gfx::RectF>)regions {
  _dragRegions = std::move(regions);
}

- (NSView*)hitTest:(NSPoint)point {
  // The point is in our superview's coordinates, and our subviews want ours.
  NSPoint p = [self convertPoint:point fromView:self.superview];
  if (!NSPointInRect(p, self.bounds)) {
    return nil;
  }
  // Shell rectangles measure y from the top; Cocoa measures it from the bottom.
  const float y = self.bounds.size.height - p.y;
  for (const gfx::RectF& r : _dragRegions) {
    if (r.Contains(p.x, y)) {
      return nil;
    }
  }
  for (const gfx::RectF& r : _regions) {
    if (r.Contains(p.x, y)) {
      return [self.shellView hitTest:p];
    }
  }
  // hitTest: skips hidden views, so this finds the shown page or nothing.
  NSView* page = [self.pages hitTest:p];
  return page && page != self.pages ? page : self;
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

// AppKit zooms on a double-click in the title bar by itself. A drag region below the
// title bar gets the same, following the user's "double-click a window's title bar to"
// setting, as Chrome does (native_widget_mac_nswindow.mm).
- (void)sendEvent:(NSEvent*)event {
  if (event.type == NSEventTypeLeftMouseUp && event.clickCount == 2 &&
      NSPointInRect(event.locationInWindow, self.contentLayoutRect) &&
      [self.contentView hitTest:event.locationInWindow] == nil) {
    NSString* action = [NSUserDefaults.standardUserDefaults
        stringForKey:@"AppleActionOnDoubleClick"];
    if (!action || [action isEqualToString:@"Maximize"] ||
        [action isEqualToString:@"Fill"]) {
      [self performZoom:nil];
    } else if ([action isEqualToString:@"Minimize"]) {
      [self performMiniaturize:nil];
    }
    return;
  }
  [super sendEvent:event];
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
  base::RepeatingCallback<void(const gfx::RectF&, double)> on_controls;
  gfx::RectF last_controls;
  double last_radius = -1;

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

  // The radius macOS rounds this window's corners with. AppKit has no public getter
  // (macOS 26 SDK), so this asks the window itself through its private
  // -_cornerRadius, and reports 0 if a future AppKit drops it. The value depends on
  // the toolbar style, so it is read rather than assumed. Full screen has no corners.
  double CornerRadius() const {
    if (window.styleMask & NSWindowStyleMaskFullScreen) {
      return 0;
    }
    SEL selector = NSSelectorFromString(@"_cornerRadius");
    if (![window respondsToSelector:selector]) {
      return 0;
    }
    NSMethodSignature* signature = [window methodSignatureForSelector:selector];
    if (std::string_view(signature.methodReturnType) != @encode(double)) {
      return 0;
    }
    NSInvocation* invocation =
        [NSInvocation invocationWithMethodSignature:signature];
    invocation.selector = selector;
    [invocation invokeWithTarget:window];
    double radius = 0;
    [invocation getReturnValue:&radius];
    return radius;
  }

  void ReportControls() {
    const gfx::RectF controls = Controls();
    const double radius = CornerRadius();
    if (controls == last_controls && radius == last_radius) {
      return;
    }
    last_controls = controls;
    last_radius = radius;
    on_controls.Run(controls, radius);
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
  // An empty toolbar makes the title bar tall enough that the buttons centre on the shell's
  // top row, inside its frame.
  impl_->window.titlebarAppearsTransparent = YES;
  impl_->window.titleVisibility = NSWindowTitleHidden;
  impl_->window.toolbar = [[NSToolbar alloc] initWithIdentifier:@"project-b"];
  impl_->window.toolbarStyle = NSWindowToolbarStyleUnifiedCompact;
  impl_->window.acceptsMouseMovedEvents = YES;
  impl_->delegate = [[PBWindowDelegate alloc] initWithOnClose:std::move(on_close)];
  impl_->window.delegate = impl_->delegate;

  impl_->container = [[PBContainerView alloc]
      initWithFrame:NSMakeRect(0, 0, 1100, 700)];
  impl_->container.wantsLayer = YES;
  impl_->window.contentView = impl_->container;

  // Keeping its margins means the pages follow a window resize in the same frame,
  // before the shell has had a chance to say anything.
  impl_->container.pages = [[NSView alloc] initWithFrame:impl_->container.bounds];
  impl_->container.pages.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
  [impl_->container addSubview:impl_->container.pages];
}

Window::~Window() {
  impl_->window.delegate = nil;
}

void Window::Show() {
  [impl_->window makeKeyAndOrderFront:nil];
  [NSApp activateIgnoringOtherApps:YES];
}

void Window::Close() {
  [impl_->window close];
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

void Window::AddPageView(gfx::NativeView view) {
  NSView* page = view.GetNativeNSView();
  NSView* pages = impl_->container.pages;
  page.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
  page.frame = pages.bounds;
  page.hidden = YES;
  [pages addSubview:page];
}

void Window::ShowPageView(gfx::NativeView view) {
  NSView* page = view.GetNativeNSView();
  // The new one first, so the window is never empty between the two.
  page.hidden = NO;
  for (NSView* other in impl_->container.pages.subviews) {
    if (other != page) {
      other.hidden = YES;
    }
  }
}

void Window::RemovePageView(gfx::NativeView view) {
  [view.GetNativeNSView() removeFromSuperview];
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
  impl_->container.pages.frame =
      NSMakeRect(std::round(bounds.x()),
                 std::round(size.height - bounds.y() - bounds.height()),
                 std::round(bounds.width()), std::round(bounds.height()));
}

void Window::SetShellRegions(std::vector<gfx::RectF> regions) {
  [impl_->container setRegions:std::move(regions)];
}

void Window::SetDragRegions(std::vector<gfx::RectF> regions) {
  [impl_->container setDragRegions:std::move(regions)];
}

void Window::ListenForControls(
    base::RepeatingCallback<void(const gfx::RectF&, double)> on_change) {
  impl_->on_controls = std::move(on_change);
  impl_->last_controls = gfx::RectF(-1, -1, 0, 0);  // So the first report always goes.
  impl_->last_radius = -1;
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
