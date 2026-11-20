// The Mac's part of the driver (driver_platform.h): real NSEvents through the window.

#include "project_b/engine/browser/driver_platform.h"

#import <Cocoa/Cocoa.h>
#include <stdio.h>
#include <unistd.h>

#include <string>

#include "base/functional/callback.h"
#include "base/json/json_writer.h"
#include "base/strings/string_split.h"
#include "base/strings/string_util.h"
#include "base/strings/stringprintf.h"
#include "base/strings/sys_string_conversions.h"
#include "base/values.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/mac_app.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/browser/shell_page.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser/window.h"
#include "project_b/engine/buildflags/buildflags.h"

#if BUILDFLAG(PB_EXTENSIONS)
#include "project_b/engine/extensions/browser/extensions.h"
#endif

// Private, for the driver only: AppKit's description of the window's drag region.
@interface NSWindow (PBDriverPrivate)
- (id)_lastDragRegionDataDescription;
@end

namespace project_b::driver {

namespace {

NSWindow* NativeOf(Window* window) {
  return window->native_window().GetNativeNSWindow();
}

// The app window of ours that is window, for its shell and its tabs.
Engine::AppWindow* AppWindowOf(Window* window) {
  for (const auto& [id, w] : Engine::Get()->windows()) {
    if (w->window.get() == window) {
      return w.get();
    }
  }
  return nullptr;
}

NSTimeInterval Now() {
  return NSProcessInfo.processInfo.systemUptime;
}

std::string Hex(NSColor* color) {
  NSColor* c = [color colorUsingColorSpace:NSColorSpace.sRGBColorSpace];
  return base::StringPrintf("#%02x%02x%02x", int(c.redComponent * 255 + 0.5),
                            int(c.greenComponent * 255 + 0.5),
                            int(c.blueComponent * 255 + 0.5));
}

// The view AppKit would send a mouse event at this point to, asked the way AppKit asks:
// from the window's frame view, which holds the title bar and its buttons above our
// content view. x and y are shell coordinates, y from the top.
NSView* HitFromFrame(Window* w, double x, double y) {
  NSWindow* window = NativeOf(w);
  NSView* content = window.contentView;
  NSView* frame = content.superview;
  NSPoint in_window = [content convertPoint:NSMakePoint(x, content.bounds.size.height - y)
                                     toView:nil];
  return [frame hitTest:in_window];
}

// What a hit view is, as the driver reports it.
std::string Classify(Window* w, NSView* hit) {
  if (!hit) {
    return "nil";
  }
  Engine::AppWindow* app = AppWindowOf(w);
  NSView* shell = app->shell->contents()->GetNativeView().GetNativeNSView();
  if ([hit isDescendantOf:shell]) {
    return "shell";
  }
  for (const Engine::VisibleTab& v : app->visible) {
    if ([hit isDescendantOf:Engine::Get()
                                ->tab(v.tab)
                                ->contents()
                                ->GetNativeView()
                                .GetNativeNSView()]) {
      return "page";
    }
  }
#if BUILDFLAG(PB_EXTENSIONS)
  content::WebContents* popup =
      Extensions::Get() ? Extensions::Get()->popup_contents() : nullptr;
  if (popup && [hit isDescendantOf:popup->GetNativeView().GetNativeNSView()]) {
    return "popup";
  }
#endif
  if (hit == NativeOf(w).contentView) {
    return "container";
  }
  if (hit == NativeOf(w).contentView.superview) {
    return "frame";
  }
  return std::string("native:") + hit.className.UTF8String;
}

// Virtual key codes for what tests type. Blink takes the text from the characters.
unsigned short CodeForChar(unichar c) {
  static const struct {
    const char* chars;
    unsigned short code;
  } kCodes[] = {
      {"a", 0},  {"s", 1},  {"d", 2},  {"f", 3},  {"h", 4},  {"g", 5},
      {"z", 6},  {"x", 7},  {"c", 8},  {"v", 9},  {"b", 11}, {"q", 12},
      {"w", 13}, {"e", 14}, {"r", 15}, {"y", 16}, {"t", 17}, {"1", 18},
      {"2", 19}, {"3", 20}, {"4", 21}, {"6", 22}, {"5", 23}, {"9", 25},
      {"7", 26}, {"8", 28}, {"0", 29}, {"o", 31}, {"u", 32}, {"i", 34},
      {"p", 35}, {"l", 37}, {"j", 38}, {"k", 40}, {"n", 45}, {"m", 46},
      {".", 47}, {"/", 44}, {":", 41}, {"-", 27}, {",", 43},
  };
  for (const auto& entry : kCodes) {
    if (entry.chars[0] == c) {
      return entry.code;
    }
  }
  return 49;
}

void SendNSKey(NSWindow* window,
               NSString* chars,
               unsigned short code,
               NSEventModifierFlags flags) {
  for (NSEventType type : {NSEventTypeKeyDown, NSEventTypeKeyUp}) {
    NSEvent* event = [NSEvent keyEventWithType:type
                                      location:NSZeroPoint
                                 modifierFlags:flags
                                     timestamp:Now()
                                  windowNumber:window.windowNumber
                                       context:nil
                                    characters:chars
                   charactersIgnoringModifiers:chars
                                     isARepeat:NO
                                       keyCode:code];
    NSResponder* responder = window.firstResponder;
    if (type == NSEventTypeKeyDown) {
      timeline::Input("input.key",
                      base::TimeTicks() + base::Seconds(event.timestamp));
      if ((flags & NSEventModifierFlagCommand) &&
          [responder performKeyEquivalent:event]) {
        continue;
      }
      [responder keyDown:event];
    } else {
      [responder keyUp:event];
    }
  }
}

// A menu's items as it shows them when it opens: its lists read then, and nothing hidden.
base::ListValue MenuItems(NSMenu* menu) {
  if ([menu.delegate respondsToSelector:@selector(menuNeedsUpdate:)]) {
    [menu.delegate menuNeedsUpdate:menu];
  }
  base::ListValue items;
  for (NSMenuItem* item in menu.itemArray) {
    if (item.hidden) {
      continue;
    }
    if (item.separatorItem) {
      items.Append("-");
      continue;
    }
    std::string key;
    NSEventModifierFlags m = item.keyEquivalentModifierMask;
    if (item.keyEquivalent.length) {
      key = std::string(m & NSEventModifierFlagControl ? "^" : "") +
            (m & NSEventModifierFlagOption ? "~" : "") +
            (m & NSEventModifierFlagShift ? "$" : "") +
            (m & NSEventModifierFlagCommand ? "@" : "") +
            base::SysNSStringToUTF8(item.keyEquivalent);
    }
    base::DictValue d;
    d.Set("title", base::SysNSStringToUTF8(item.title));
    d.Set("key", key);
    d.Set("enabled", static_cast<bool>(item.enabled));
    d.Set("checked", item.state == NSControlStateValueOn);
    if (item.submenu) {
      d.Set("items", MenuItems(item.submenu));
    }
    items.Append(std::move(d));
  }
  return items;
}

}  // namespace

void StartReading(const std::string& path,
                  base::RepeatingCallback<void(const std::string&)> run,
                  base::OnceClosure quit) {
  static dispatch_source_t driver_process = nil;
  static base::NoDestructor<base::OnceClosure> on_gone;
  *on_gone = std::move(quit);
  const pid_t driver = getppid();
  driver_process = dispatch_source_create(DISPATCH_SOURCE_TYPE_PROC, driver,
                                          DISPATCH_PROC_EXIT, dispatch_get_main_queue());
  dispatch_source_set_event_handler(driver_process, ^{
    if (*on_gone) {
      std::move(*on_gone).Run();
    }
  });
  dispatch_resume(driver_process);
  // A copy for the block, which runs after this returns: a block holds a C++ reference as a
  // reference, and path's string is the caller's.
  const std::string fifo = path;
  dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
    // Opened read-write so it doesn't see end-of-file between writers.
    FILE* file = fopen(fifo.c_str(), "r+");
    if (!file) {
      fprintf(stderr, "driver: cannot open %s\n", fifo.c_str());
      return;
    }
    char* buffer = nullptr;
    size_t capacity = 0;
    while (getline(&buffer, &capacity, file) >= 0) {
      std::string line(buffer);
      base::TrimString(line, "\r\n", &line);
      dispatch_async(dispatch_get_main_queue(), ^{
        run.Run(line);
      });
    }
  });
}

std::string Hit(Window* window, double x, double y) {
  return Classify(window, HitFromFrame(window, x, y));
}

// In a session with no real GUI, [window sendEvent:] doesn't deliver anything, so this
// does what AppKit's routing does: ask the frame's hitTest:, make the answer first
// responder on mouse down, and hand it the event. Only our own views get events here;
// a native area (the frame, a window button) is reported and left alone, because
// AppKit's own handling of it would start a tracking loop that never ends without a
// display.
std::string SendMouse(Window* w, Mouse kind_of_event, double x, double y, int clicks) {
  const NSEventType type = kind_of_event == Mouse::kMove   ? NSEventTypeMouseMoved
                           : kind_of_event == Mouse::kDown ? NSEventTypeLeftMouseDown
                                                           : NSEventTypeLeftMouseUp;
  NSWindow* window = NativeOf(w);
  static NSInteger number = 0;
  NSView* content = window.contentView;
  NSPoint p = NSMakePoint(x, content.bounds.size.height - y);
  NSEvent* event = [NSEvent mouseEventWithType:type
                                      location:p
                                 modifierFlags:0
                                     timestamp:Now()
                                  windowNumber:window.windowNumber
                                       context:nil
                                   eventNumber:++number
                                    clickCount:clicks
                                      pressure:clicks ? 1.0 : 0.0];
  NSView* hit = HitFromFrame(w, x, y);
  const std::string kind = Classify(w, hit);
  if (kind != "shell" && kind != "page" && kind != "popup") {
    return kind;
  }
  switch (type) {
    case NSEventTypeMouseMoved:
      [hit mouseMoved:event];
      break;
    case NSEventTypeLeftMouseDown:
      if (hit.acceptsFirstResponder) {
        [window makeFirstResponder:hit];
      }
      [hit mouseDown:event];
      break;
    case NSEventTypeLeftMouseUp:
      // What the window's sendEvent: does, which nothing calls here.
      if (kind == "shell") {
        timeline::Input("input.click",
                        base::TimeTicks() + base::Seconds(event.timestamp));
      }
      [hit mouseUp:event];
      break;
    default:
      break;
  }
  return kind;
}

std::string Press(Window* w, double x, double y) {
  NSWindow* window = NativeOf(w);
  NSView* content = window.contentView;
  const NSPoint p =
      [content convertPoint:NSMakePoint(x, content.bounds.size.height - y) toView:nil];
  const std::string kind = Classify(w, HitFromFrame(w, x, y));
  if (w->MovesFromApp(gfx::PointF(p.x, p.y))) {
    return "moves";
  }
  if (kind == "frame" && !NSPointInRect(p, window.contentLayoutRect)) {
    return "titlebar";
  }
  return kind;
}

std::string DragRegion(Window* w) {
  NSWindow* window = NativeOf(w);
  NSString* text = [window respondsToSelector:@selector(_lastDragRegionDataDescription)]
                       ? [[window _lastDragRegionDataDescription] description]
                       : @"unknown";
  return base::SysNSStringToUTF8(
      [text stringByReplacingOccurrencesOfString:@"\n" withString:@" | "]);
}

void SendKey(Window* w, const Key& key) {
  NSWindow* window = NativeOf(w);
  NSEventModifierFlags flags = 0;
  if (key.cmd) {
    flags |= NSEventModifierFlagCommand;
  }
  if (key.ctrl) {
    flags |= NSEventModifierFlagControl;
  }
  if (key.shift) {
    flags |= NSEventModifierFlagShift;
  }
  if (key.opt) {
    flags |= NSEventModifierFlagOption;
  }
  if (key.name == "enter") {
    SendNSKey(window, @"\r", 36, flags);
  } else if (key.name == "escape") {
    SendNSKey(window, @"\x1b", 53, flags);
  } else if (key.name == "tab") {
    SendNSKey(window, @"\t", 48, flags);
  } else if (key.name == "up") {
    unichar c = NSUpArrowFunctionKey;
    SendNSKey(window, [NSString stringWithCharacters:&c length:1], 126,
              flags | NSEventModifierFlagFunction | NSEventModifierFlagNumericPad);
  } else if (key.name == "down") {
    unichar c = NSDownArrowFunctionKey;
    SendNSKey(window, [NSString stringWithCharacters:&c length:1], 125,
              flags | NSEventModifierFlagFunction | NSEventModifierFlagNumericPad);
  } else if (!key.character.empty()) {
    SendNSKey(window, base::SysUTF16ToNSString(key.character),
              CodeForChar(key.character[0]), flags);
  }
}

std::string Focused(Window* w) {
  NSResponder* responder = NativeOf(w).firstResponder;
  if ([responder isKindOfClass:[NSView class]]) {
    return Classify(w, static_cast<NSView*>(responder));
  }
  return responder ? std::string("native:") + responder.className.UTF8String : "nil";
}

std::string Title(Window* window) {
  return base::SysNSStringToUTF8(NativeOf(window).title);
}

bool IsKey(Window* window) {
  return NativeOf(window).isKeyWindow;
}

bool IsVisible(Window* window) {
  return NativeOf(window).visible;
}

std::string Flags(Window* w) {
  NSWindow* window = NativeOf(w);
  return base::StringPrintf(
      "\"key\":%d,\"main\":%d,\"visible\":%d,\"occlusionVisible\":%d", window.isKeyWindow,
      window.isMainWindow, window.isVisible,
      (window.occlusionState & NSWindowOcclusionStateVisible) != 0);
}

void ResizeContent(Window* window, double width, double height) {
  [NativeOf(window) setContentSize:NSMakeSize(width, height)];
}

double ContentWidth(Window* window) {
  return NativeOf(window).contentView.bounds.size.width;
}

uint32_t BackgroundColor(Window* window) {
  NSColor* c =
      [NativeOf(window).backgroundColor colorUsingColorSpace:NSColorSpace.sRGBColorSpace];
  return uint32_t(c.redComponent * 255 + 0.5) << 16 |
         uint32_t(c.greenComponent * 255 + 0.5) << 8 | uint32_t(c.blueComponent * 255 + 0.5);
}

std::string Frame(Window* window) {
  CALayer* layer = NativeOf(window).contentView.layer;
  return base::StringPrintf("{\"radius\":%g,\"width\":%g,\"color\":\"%s\"}",
                            layer.cornerRadius, layer.borderWidth,
                            Hex([NSColor colorWithCGColor:layer.borderColor]).c_str());
}

gfx::RectF ViewFrame(Window* window, gfx::NativeView native) {
  NSView* view = native.GetNativeNSView();
  NSView* content = NativeOf(window).contentView;
  const NSRect f = [view.superview convertRect:view.frame toView:content];
  return gfx::RectF(f.origin.x, content.bounds.size.height - NSMaxY(f), f.size.width,
                    f.size.height);
}

bool ViewHidden(gfx::NativeView view) {
  return view.GetNativeNSView().hidden;
}

void SetViewHidden(gfx::NativeView view, bool hidden) {
  view.GetNativeNSView().hidden = hidden;
}

std::string Bounds(Window* w, const gfx::RectF& b, content::WebContents* page) {
  NSWindow* window = NativeOf(w);
  NSView* page_view = page->GetNativeView().GetNativeNSView();
  NSRect f = [page_view.superview convertRect:page_view.frame toView:window.contentView];
  NSSize c = window.contentView.bounds.size;
  NSButton* close_button = [window standardWindowButton:NSWindowCloseButton];
  NSRect zoom = [window standardWindowButton:NSWindowZoomButton].frame;
  NSRect close_frame = close_button.frame;
  const double from_top = close_button.superview.bounds.size.height - NSMidY(close_frame);
  return base::StringPrintf(
      "{\"reported\":[%g,%g,%g,%g],\"frame\":[%g,%g,%g,%g],"
      "\"content\":[%g,%g],\"titlebar\":%g,\"buttonsStart\":%g,"
      "\"buttonsEnd\":%g,\"buttonsMiddleY\":%g,\"buttonSize\":%g}",
      b.x(), b.y(), b.width(), b.height(), f.origin.x, f.origin.y, f.size.width,
      f.size.height, c.width, c.height, c.height - window.contentLayoutRect.size.height,
      NSMinX(close_frame), NSMaxX(zoom), from_top, close_frame.size.width);
}

void PressClose(Window* window) {
  [NativeOf(window) performClose:nil];
}

void SendModifiers(uint64_t flags) {
  ReportModifiers(flags);
}

std::string MenuJson() {
  return base::WriteJson(MenuItems(NSApp.mainMenu)).value_or("[]");
}

bool ChooseMenu(const std::string& title, const std::string& path) {
  NSMenu* menu = [NSApp.mainMenu itemWithTitle:base::SysUTF8ToNSString(title)].submenu;
  NSMenuItem* item = nil;
  for (const std::string& part : base::SplitStringUsingSubstr(
           path, " > ", base::TRIM_WHITESPACE, base::SPLIT_WANT_ALL)) {
    [menu.delegate menuNeedsUpdate:menu];
    item = [menu itemWithTitle:base::SysUTF8ToNSString(part)];
    menu = item.submenu;
  }
  [NSApp sendAction:item.action to:item.target from:item];
  return item != nil;
}

// The colour at the middle of the Dock icon's dot, or of where it goes: a tenth in from
// the right and the bottom, and half the dot's quarter across. The bitmap counts y down
// from the top. Its pixels are sRGB already, so they are read as they are.
std::string DockDot() {
  NSRect rect = NSMakeRect(0, 0, 200, 200);
  NSBitmapImageRep* rep = [[NSBitmapImageRep alloc]
      initWithCGImage:[NSApp.applicationIconImage CGImageForProposedRect:&rect
                                                                 context:nil
                                                                   hints:nil]];
  NSColor* c = [rep colorAtX:rep.pixelsWide * 155 / 200 y:rep.pixelsHigh * 155 / 200];
  return base::StringPrintf("#%02x%02x%02x", int(c.redComponent * 255 + 0.5),
                            int(c.greenComponent * 255 + 0.5),
                            int(c.blueComponent * 255 + 0.5));
}

}  // namespace project_b::driver
