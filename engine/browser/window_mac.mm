#include "project_b/engine/browser/window.h"

#import <Cocoa/Cocoa.h>

#include <cmath>
#include <string_view>
#include <utility>

#include "base/functional/callback.h"
#include "base/strings/sys_string_conversions.h"
#include "components/input/native_web_keyboard_event.h"
#include "ui/gfx/mac/coordinate_conversion.h"

// Answers the OS hit test by walking the layout's regions. Content's own views ask this
// same question, so a view that loses the answer ignores the event (see
// shouldIgnoreMouseEvent: in content/app_shim_remote_cocoa/render_widget_host_view_cocoa.mm).
//
// In a drag area the answer is nil, so the event falls through to the window's own
// frame, which moves the window and, inside the title bar, zooms on a double-click.
// That is how Chrome does it (components/remote_cocoa/app_shim/bridged_content_view.mm).
@interface PBContainerView : NSView
@property(nonatomic, weak) NSView* shellView;
// Holds every page view; the shown one is the only one not hidden.
@property(nonatomic, strong) NSView* pages;
- (void)setRegions:(std::vector<project_b::Window::Region>)regions;
@end

@implementation PBContainerView {
  std::vector<project_b::Window::Region> _regions;
}

@synthesize shellView = _shellView;
@synthesize pages = _pages;

- (void)setRegions:(std::vector<project_b::Window::Region>)regions {
  _regions = std::move(regions);
}

// AppKit asks this, privately, which part of the title bar the window server must leave
// to the app. A transparent view leaves none, so the server moved the window from any
// drag that started in the title bar strip, the address field included. The container
// leaves all of itself: every press there goes through hitTest:, and only a drag area
// falls through to the frame, which moves the window. Chrome's content view answers the
// same (bridged_content_view.mm).
- (NSRect)_opaqueRectForWindowMoveWhenInTitlebar {
  return self.bounds;
}

- (NSView*)hitTest:(NSPoint)point {
  // The point is in our superview's coordinates, and our subviews want ours.
  NSPoint p = [self convertPoint:point fromView:self.superview];
  if (!NSPointInRect(p, self.bounds)) {
    return nil;
  }
  // Shell rectangles measure y from the top; Cocoa measures it from the bottom.
  const float y = self.bounds.size.height - p.y;
  for (const project_b::Window::Region& r : _regions) {
    if (!r.rect.Contains(p.x, y)) {
      continue;
    }
    if (r.area == project_b::Window::Area::kDrag) {
      return nil;
    }
    if (r.area == project_b::Window::Area::kShell) {
      return [self.shellView hitTest:p];
    }
    break;
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

// AppKit zooms on a double-click in the title bar by itself. A drag area below the
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
@property(nonatomic) base::RepeatingClosure onStateMayHaveChanged;
- (instancetype)initWithOnClose:(base::OnceClosure)onClose;
@end

@implementation PBWindowDelegate {
  base::OnceClosure _onClose;
}

@synthesize onStateMayHaveChanged = _onStateMayHaveChanged;

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
  if (_onStateMayHaveChanged) {
    _onStateMayHaveChanged.Run();
  }
}

// The toolbar exists only to size the title bar. In full screen AppKit would show it as
// an opaque strip over the content, so it goes away for the duration. Chrome has no
// NSToolbar at all and draws its own, which is why it never shows this strip.
- (void)windowWillEnterFullScreen:(NSNotification*)notification {
  [notification.object toolbar].visible = NO;
}

- (void)windowDidEnterFullScreen:(NSNotification*)notification {
  if (_onStateMayHaveChanged) {
    _onStateMayHaveChanged.Run();
  }
}

// The toolbar comes back once the window is windowed again; set during the transition
// it stays hidden. The buttons take their place beside it a turn later, so the report
// goes out now and once more then.
- (void)windowDidExitFullScreen:(NSNotification*)notification {
  [notification.object toolbar].visible = YES;
  if (_onStateMayHaveChanged) {
    _onStateMayHaveChanged.Run();
    base::RepeatingClosure again = _onStateMayHaveChanged;
    dispatch_async(dispatch_get_main_queue(), ^{
      again.Run();
    });
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
  uint32_t shown_rgb = 0;
  // The frame last painted; none yet.
  int64_t frame_rgb = -1;
  double frame_width = -1;
  base::RepeatingCallback<void(const State&)> on_state;
  State last_state;

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

  void ReportState() {
    const State state = {
        .size = gfx::SizeF(container.bounds.size.width,
                           container.bounds.size.height),
        .controls = Controls(),
        .corner_radius = CornerRadius(),
        .full_screen = (window.styleMask & NSWindowStyleMaskFullScreen) != 0,
    };
    if (state.size == last_state.size && state.controls == last_state.controls &&
        state.corner_radius == last_state.corner_radius &&
        state.full_screen == last_state.full_screen) {
      return;
    }
    last_state = state;
    // The frame band the container draws follows the window's curve.
    container.layer.cornerRadius = state.corner_radius;
    on_state.Run(state);
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
  // An empty unified toolbar puts the buttons 26px down, the centre of the shell's 44px
  // top row inside its 4px frame.
  impl_->window.titlebarAppearsTransparent = YES;
  impl_->window.titleVisibility = NSWindowTitleHidden;
  impl_->window.toolbar = [[NSToolbar alloc] initWithIdentifier:@"project-b"];
  impl_->window.toolbarStyle = NSWindowToolbarStyleUnified;
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
  NSColor* color = [impl_->window.backgroundColor
      colorUsingColorSpace:NSColorSpace.sRGBColorSpace];
  impl_->shown_rgb = uint32_t(std::lround(color.redComponent * 255)) << 16 |
                     uint32_t(std::lround(color.greenComponent * 255)) << 8 |
                     uint32_t(std::lround(color.blueComponent * 255));
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

void Window::SetLayout(Layout layout) {
  [impl_->container setRegions:std::move(layout.regions)];

  if (layout.frame_rgb != impl_->frame_rgb ||
      layout.frame_width != impl_->frame_width) {
    impl_->frame_rgb = layout.frame_rgb;
    impl_->frame_width = layout.frame_width;
    const uint32_t rgb = layout.frame_rgb;
    NSColor* color = [NSColor colorWithSRGBRed:((rgb >> 16) & 0xff) / 255.0
                                         green:((rgb >> 8) & 0xff) / 255.0
                                          blue:(rgb & 0xff) / 255.0
                                         alpha:1];
    impl_->window.backgroundColor = color;
    // A layer's border is drawn above its sublayers, so this band sits over the page
    // and the shell alike; its corners take the window's radius in ReportState.
    impl_->container.layer.backgroundColor = color.CGColor;
    impl_->container.layer.borderColor = color.CGColor;
    impl_->container.layer.borderWidth = layout.frame_width;
  }

  const NSSize size = impl_->container.bounds.size;
  if (std::round(layout.viewport.width()) != std::round(size.width) ||
      std::round(layout.viewport.height()) != std::round(size.height)) {
    impl_->ignored_stale_bounds++;
    return;
  }
  const gfx::RectF& page = layout.page;
  impl_->page_bounds = page;
  impl_->container.pages.frame =
      NSMakeRect(std::round(page.x()),
                 std::round(size.height - page.y() - page.height()),
                 std::round(page.width()), std::round(page.height()));
}

void Window::ListenForState(
    base::RepeatingCallback<void(const State&)> on_change) {
  impl_->on_state = std::move(on_change);
  impl_->last_state = {.corner_radius = -1};  // So the first report always goes.
  impl_->delegate.onStateMayHaveChanged = base::BindRepeating(
      [](Impl* impl) { impl->ReportState(); }, impl_.get());
  impl_->ReportState();
}

void Window::Zoom() {
  [impl_->window performZoom:nil];
}

void Window::ToggleFullScreen() {
  [impl_->window toggleFullScreen:nil];
}

bool Window::HandleUnhandledKey(const input::NativeWebKeyboardEvent& event) {
  NSEvent* ns_event = event.os_event.Get();
  if (ns_event.type != NSEventTypeKeyDown) {
    return false;
  }
  return [NSApp.mainMenu performKeyEquivalent:ns_event];
}

gfx::Rect Window::GetBounds() const {
  return gfx::ScreenRectFromNSRect(impl_->window.frame);
}

void Window::SetBounds(const gfx::Rect& bounds) {
  [impl_->window setFrame:gfx::ScreenRectToNSRect(bounds) display:YES];
}

bool Window::IsMinimized() const {
  return impl_->window.miniaturized;
}

bool Window::IsMaximized() const {
  return impl_->window.zoomed;
}

bool Window::IsFullscreen() const {
  return (impl_->window.styleMask & NSWindowStyleMaskFullScreen) != 0;
}

void Window::Minimize() {
  [impl_->window performMiniaturize:nil];
}

void Window::Maximize() {
  if (!impl_->window.zoomed) {
    [impl_->window performZoom:nil];
  }
}

void Window::SetFullscreen(bool fullscreen) {
  if (IsFullscreen() != fullscreen) {
    [impl_->window toggleFullScreen:nil];
  }
}

void Window::Restore() {
  SetFullscreen(false);
  [impl_->window deminiaturize:nil];
  if (impl_->window.zoomed) {
    [impl_->window performZoom:nil];
  }
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

uint32_t Window::shown_rgb() const {
  return impl_->shown_rgb;
}

}  // namespace project_b
