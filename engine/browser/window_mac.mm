#include "project_b/engine/browser/window.h"

#import <Cocoa/Cocoa.h>

#include <algorithm>
#include <cmath>
#include <string_view>
#include <utility>

#include "base/functional/callback.h"
#include "base/strings/sys_string_conversions.h"
#include "base/time/time.h"
#include "components/input/native_web_keyboard_event.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/timeline.h"
#include "third_party/skia/include/core/SkRegion.h"
#include "ui/gfx/geometry/rect_conversions.h"
#include "ui/gfx/mac/coordinate_conversion.h"

// A part of the title bar that is the app's. AppKit asks every view, privately, which
// part of the title bar it needs, and gives the rest to the window server, which moves
// the window from there by itself, without asking the app. Our views are transparent
// and need none, so one of these covers each part that is not a drag area. It draws
// nothing, and the container's hitTest: never returns it.
@interface PBAppAreaView : NSView
@end

@implementation PBAppAreaView

- (NSRect)_opaqueRectForWindowMoveWhenInTitlebar {
  return self.bounds;
}

@end

// Answers the OS hit test by walking the layout's regions. Content's own views ask this
// same question, so a view that loses the answer ignores the event (see
// shouldIgnoreMouseEvent: in content/app_shim_remote_cocoa/render_widget_host_view_cocoa.mm).
//
// In a drag area the answer is nil, so the event falls through to the window's own
// frame, which zooms on a double-click. The window server moves the window from a drag
// area by itself, because the app areas leave it to the server.
@interface PBContainerView : NSView
@property(nonatomic, weak) NSView* shellView;
// Over everything while it is there, and it takes every press.
@property(nonatomic, weak) NSView* overlayView;
#if BUILDFLAG(PB_EXTENSIONS)
// Floats over the shell and the pages, and takes the presses inside it: an
// extension's popup.
@property(nonatomic, strong) NSView* popupView;
#endif
// Holds every page view the window has shown; those it shows now are the only ones not
// hidden.
@property(nonatomic, strong) NSView* pages;
- (void)setRegions:(std::vector<project_b::Window::Region>)regions;
@end

@implementation PBContainerView {
  std::vector<project_b::Window::Region> _regions;
  NSMutableArray<PBAppAreaView*>* _appAreas;
}

@synthesize shellView = _shellView;
@synthesize overlayView = _overlayView;
#if BUILDFLAG(PB_EXTENSIONS)
@synthesize popupView = _popupView;
#endif
@synthesize pages = _pages;

- (void)setRegions:(std::vector<project_b::Window::Region>)regions {
  _regions = std::move(regions);
  [self updateAppAreas];
}

- (void)setOverlayView:(NSView*)overlayView {
  _overlayView = overlayView;
  [self updateAppAreas];
}

// The app's areas are everything but the drag areas. The overlay takes every press, so
// while it is there the whole window is the app's.
- (void)updateAppAreas {
  const NSSize size = self.bounds.size;
  SkRegion app(SkIRect::MakeWH(std::ceil(size.width), std::ceil(size.height)));
  if (!self.overlayView) {
    for (const project_b::Window::Region& r : _regions) {
      if (r.area == project_b::Window::Area::kDrag) {
        const gfx::Rect drag = gfx::ToEnclosedRect(r.rect);
        app.op(SkIRect::MakeXYWH(drag.x(), drag.y(), drag.width(), drag.height()),
               SkRegion::kDifference_Op);
      }
    }
  }
  for (PBAppAreaView* view in _appAreas) {
    [view removeFromSuperview];
  }
  _appAreas = [NSMutableArray array];
  for (SkRegion::Iterator it(app); !it.done(); it.next()) {
    const SkIRect& r = it.rect();
    // Shell rectangles measure y from the top; Cocoa measures it from the bottom.
    PBAppAreaView* view = [[PBAppAreaView alloc]
        initWithFrame:NSMakeRect(r.x(), size.height - r.bottom(), r.width(),
                                 r.height())];
    [_appAreas addObject:view];
    [self addSubview:view];
  }
}

- (NSView*)hitTest:(NSPoint)point {
  // The point is in our superview's coordinates, and our subviews want ours.
  NSPoint p = [self convertPoint:point fromView:self.superview];
  if (!NSPointInRect(p, self.bounds)) {
    return nil;
  }
  if (self.overlayView) {
    return [self.overlayView hitTest:p];
  }
#if BUILDFLAG(PB_EXTENSIONS)
  if (self.popupView && NSPointInRect(p, self.popupView.frame)) {
    return [self.popupView hitTest:p];
  }
#endif
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
  // hitTest: skips hidden views, so this finds the shown page there or nothing.
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
- (instancetype)initWithOnClose:(base::OnceClosure)onClose
                          onKey:(base::RepeatingClosure)onKey
                    onMinimised:(base::RepeatingCallback<void(bool)>)onMinimised;
@end

@implementation PBWindowDelegate {
  base::OnceClosure _onClose;
  base::RepeatingClosure _onKey;
  base::RepeatingCallback<void(bool)> _onMinimised;
}

@synthesize onStateMayHaveChanged = _onStateMayHaveChanged;

- (instancetype)initWithOnClose:(base::OnceClosure)onClose
                          onKey:(base::RepeatingClosure)onKey
                    onMinimised:(base::RepeatingCallback<void(bool)>)onMinimised {
  if ((self = [super init])) {
    _onClose = std::move(onClose);
    _onKey = std::move(onKey);
    _onMinimised = std::move(onMinimised);
  }
  return self;
}

- (void)windowWillClose:(NSNotification*)notification {
  std::move(_onClose).Run();
}

- (void)windowDidBecomeKey:(NSNotification*)notification {
  _onKey.Run();
}

- (void)windowDidMiniaturize:(NSNotification*)notification {
  _onMinimised.Run(true);
}

- (void)windowDidDeminiaturize:(NSNotification*)notification {
  _onMinimised.Run(false);
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
  size_t ignored_stale_bounds = 0;
  uint32_t shown_rgb = 0;
  // The frame last painted; none yet.
  int64_t frame_rgb = -1;
  int theme = -1;
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

Window::Window(base::OnceClosure on_close,
               base::RepeatingClosure on_key,
               base::RepeatingCallback<void(bool)> on_minimised)
    : impl_(std::make_unique<Impl>()) {
  impl_->window = [[PBNSWindow alloc]
      initWithContentRect:NSMakeRect(80, 120, 1280, 720)
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
  impl_->delegate =
      [[PBWindowDelegate alloc] initWithOnClose:std::move(on_close)
                                          onKey:std::move(on_key)
                                    onMinimised:std::move(on_minimised)];
  impl_->window.delegate = impl_->delegate;

  impl_->container = [[PBContainerView alloc]
      initWithFrame:NSMakeRect(0, 0, 1280, 720)];
  impl_->container.wantsLayer = YES;
  impl_->window.contentView = impl_->container;

  // The holder covers the window, and each page in it keeps its margins, so a page
  // follows a window resize in the same frame, before the shell has had a chance to say
  // anything.
  impl_->container.pages = [[NSView alloc] initWithFrame:impl_->container.bounds];
  impl_->container.pages.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
  [impl_->container addSubview:impl_->container.pages];
}

Window::~Window() {
  impl_->window.delegate = nil;
}

// AppKit's cascade: the top left of the other window, then one step down and right of it.
void Window::PlaceNear(Window* other) {
  NSWindow* near = other->impl_->window;
  [impl_->window setFrame:near.frame display:NO];
  [impl_->window
      cascadeTopLeftFromPoint:[impl_->window
                                  cascadeTopLeftFromPoint:NSMakePoint(
                                                              NSMinX(near.frame),
                                                              NSMaxY(near.frame))]];
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

// A page's view joins the holder the first time the window shows it, and moves here
// from another window's holder by the same addSubview:. Each keeps its four margins
// through autoresizing, which is right for pages that each touch the window's edges;
// pages split side by side would need their inner edges to be the flexible ones.
void Window::SetPages(const std::vector<Page>& pages) {
  NSView* holder = impl_->container.pages;
  const CGFloat height = holder.bounds.size.height;
  NSMutableSet<NSView*>* shown = [NSMutableSet set];
  // The new ones first, so the window is never empty between the two.
  for (const Page& p : pages) {
    NSView* page = p.contents->GetNativeView().GetNativeNSView();
    if (page.superview != holder) {
      page.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
      [holder addSubview:page];
    }
    page.frame = NSMakeRect(std::round(p.rect.x()),
                            std::round(height - p.rect.y() - p.rect.height()),
                            std::round(p.rect.width()), std::round(p.rect.height()));
    page.hidden = NO;
    [shown addObject:page];
  }
  for (NSView* other in holder.subviews) {
    if (![shown containsObject:other]) {
      other.hidden = YES;
    }
  }
}

void Window::RemovePageView(gfx::NativeView view) {
  [view.GetNativeNSView() removeFromSuperview];
}

void Window::SetOverlayView(gfx::NativeView view) {
  NSView* overlay = view.GetNativeNSView();
  overlay.frame = impl_->container.bounds;
  overlay.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
  impl_->container.overlayView = overlay;
  [impl_->container addSubview:overlay];
}

#if BUILDFLAG(PB_EXTENSIONS)
// The popup's view sits in a holder that rounds its corners and draws a hairline
// around it, as the system's own popovers have. The holder goes over everything.
void Window::SetPopupView(gfx::NativeView view,
                          const gfx::RectF& anchor,
                          const gfx::Size& size) {
  PBContainerView* container = impl_->container;
  NSView* contents = view.GetNativeNSView();
  if (!container.popupView) {
    NSView* holder = [[NSView alloc] initWithFrame:NSZeroRect];
    holder.wantsLayer = YES;
    holder.layer.cornerRadius = 8;
    holder.layer.masksToBounds = YES;
    holder.layer.borderWidth = 1;
    holder.layer.borderColor = NSColor.separatorColor.CGColor;
    contents.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    [holder addSubview:contents];
    container.popupView = holder;
    [container addSubview:holder];
  }
  // Under the anchor with a 4 point gap, right edges together, inside the window.
  const NSRect bounds = container.bounds;
  const CGFloat x =
      std::clamp<CGFloat>(anchor.right() - size.width(), 0,
                          std::max<CGFloat>(0, bounds.size.width - size.width()));
  const CGFloat top = anchor.bottom() + 4;
  container.popupView.frame = NSMakeRect(
      x, bounds.size.height - top - size.height(), size.width(), size.height());
  contents.frame = container.popupView.bounds;
}

void Window::RemovePopupView(gfx::NativeView view) {
  [view.GetNativeNSView() removeFromSuperview];
  [impl_->container.popupView removeFromSuperview];
  impl_->container.popupView = nil;
}

gfx::RectF Window::popup_bounds() const {
  NSView* popup = impl_->container.popupView;
  if (!popup) {
    return gfx::RectF();
  }
  const NSRect f = popup.frame;
  return gfx::RectF(f.origin.x,
                    impl_->container.bounds.size.height - NSMaxY(f),
                    f.size.width, f.size.height);
}
#endif

void Window::NoteClick(const gfx::PointF& in_window, base::TimeTicks made) {
  PBContainerView* container = impl_->container;
  NSView* hit = [container hitTest:NSMakePoint(in_window.x(), in_window.y())];
  if (hit && ([hit isDescendantOf:container.shellView] ||
              (container.overlayView &&
               [hit isDescendantOf:container.overlayView]))) {
    timeline::Input("input.click", made);
  }
}

void Window::RemoveOverlayView(gfx::NativeView view) {
  NSView* overlay = view.GetNativeNSView();
  if (impl_->container.overlayView == overlay) {
    impl_->container.overlayView = nil;
  }
  [overlay removeFromSuperview];
}

bool Window::SetLayout(Layout layout) {
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

  // The window's own appearance only: NSApp's would change what pages see as the
  // system's light or dark too.
  if (layout.theme != impl_->theme) {
    impl_->theme = layout.theme;
    impl_->window.appearance =
        layout.theme == 1   ? [NSAppearance appearanceNamed:NSAppearanceNameAqua]
        : layout.theme == 2 ? [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua]
                            : nil;
  }

  const NSSize size = impl_->container.bounds.size;
  if (std::round(layout.viewport.width()) != std::round(size.width) ||
      std::round(layout.viewport.height()) != std::round(size.height)) {
    impl_->ignored_stale_bounds++;
    return false;
  }
  return true;
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

// static
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

void Window::BringToFront() {
  [impl_->window deminiaturize:nil];
  [impl_->window makeKeyAndOrderFront:nil];
  [NSApp activateIgnoringOtherApps:YES];
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

void Window::ForceKeyForDriver(bool key) {
  impl_->window.forceKeyForDriver = key;
  if (!key) {
    return;
  }
  NSNotificationCenter* center = NSNotificationCenter.defaultCenter;
  [center postNotificationName:NSWindowDidBecomeMainNotification
                        object:impl_->window];
  [center postNotificationName:NSWindowDidBecomeKeyNotification
                        object:impl_->window];
}

gfx::NativeWindow Window::native_window() const {
  return gfx::NativeWindow(impl_->window);
}

size_t Window::ignored_stale_bounds() const {
  return impl_->ignored_stale_bounds;
}

uint32_t Window::shown_rgb() const {
  return impl_->shown_rgb;
}

}  // namespace project_b
