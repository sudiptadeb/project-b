#include "project_b/engine/browser/driver.h"

#import <Cocoa/Cocoa.h>
#include <stdio.h>

#include <string>
#include <vector>

#include "base/files/file_util.h"
#include "base/functional/bind.h"
#include "base/functional/callback.h"
#include "base/json/json_writer.h"
#include "base/memory/ref_counted.h"
#include "base/process/process.h"
#include "base/strings/string_number_conversions.h"
#include "base/strings/string_split.h"
#include "base/strings/string_util.h"
#include "base/strings/stringprintf.h"
#include "base/strings/sys_string_conversions.h"
#include "base/strings/utf_string_conversions.h"
#include "base/threading/thread_restrictions.h"
#include "base/time/time.h"
#include "base/values.h"
#include "components/viz/common/frame_sinks/copy_output_result.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/render_process_host.h"
#include "content/public/browser/render_widget_host_view.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/hooks_gen.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/browser/shell_page.h"
#include "project_b/engine/browser/window.h"
#include "third_party/skia/include/core/SkBitmap.h"
#include "third_party/skia/include/core/SkCanvas.h"
#include "third_party/skia/include/core/SkImage.h"
#include "third_party/skia/include/core/SkRect.h"
#include "ui/gfx/codec/png_codec.h"

namespace project_b {

namespace {

void Reply(const std::string& id, const std::string& text) {
  printf("PBDRIVER %s %s\n", id.c_str(), text.c_str());
  fflush(stdout);
}

NSWindow* TheWindow() {
  return Engine::Get()->window()->native_window().GetNativeNSWindow();
}

NSTimeInterval Now() {
  return NSProcessInfo.processInfo.systemUptime;
}

// The view AppKit would send a mouse event at this point to, asked the way AppKit asks:
// from the window's frame view, which holds the title bar and its buttons above our
// content view. x and y are shell coordinates, y from the top.
NSView* HitFromFrame(double x, double y) {
  NSWindow* window = TheWindow();
  NSView* content = window.contentView;
  NSView* frame = content.superview;
  NSPoint in_window = [content convertPoint:NSMakePoint(x, content.bounds.size.height - y)
                                     toView:nil];
  return [frame hitTest:in_window];
}

// What a hit view is, as the driver reports it.
std::string Classify(NSView* hit) {
  if (!hit) {
    return "nil";
  }
  NSView* shell =
      Engine::Get()->shell()->contents()->GetNativeView().GetNativeNSView();
  if ([hit isDescendantOf:shell]) {
    return "shell";
  }
  PageTab* tab = Engine::Get()->active_tab();
  if (tab && [hit isDescendantOf:tab->contents()->GetNativeView().GetNativeNSView()]) {
    return "page";
  }
  if (hit == TheWindow().contentView) {
    return "container";
  }
  if (hit == TheWindow().contentView.superview) {
    return "frame";
  }
  return std::string("native:") + hit.className.UTF8String;
}

// In a session with no real GUI, [window sendEvent:] doesn't deliver anything, so this
// does what AppKit's routing does: ask the frame's hitTest:, make the answer first
// responder on mouse down, and hand it the event. Only our own views get events here;
// a native area (the frame, a window button) is reported and left alone, because
// AppKit's own handling of it would start a tracking loop that never ends without a
// display.
std::string SendMouse(NSEventType type, double x, double y, int clicks) {
  NSWindow* window = TheWindow();
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
  NSView* hit = HitFromFrame(x, y);
  const std::string kind = Classify(hit);
  if (kind != "shell" && kind != "page") {
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
      [hit mouseUp:event];
      break;
    default:
      break;
  }
  return kind;
}

void SendKey(NSString* chars, unsigned short code, NSEventModifierFlags flags) {
  NSWindow* window = TheWindow();
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
      {".", 47}, {"/", 44}, {":", 41}, {"-", 27},
  };
  for (const auto& entry : kCodes) {
    if (entry.chars[0] == c) {
      return entry.code;
    }
  }
  return 49;
}

void Capture(const std::string& id, const std::string& path);

void Eval(const std::string& id, content::WebContents* contents, const std::string& js) {
  contents->GetPrimaryMainFrame()->ExecuteJavaScriptForTests(
      base::UTF8ToUTF16(js),
      base::BindOnce(
          [](const std::string& id, base::Value value) {
            std::string json;
            base::JSONWriter::Write(value, &json);
            Reply(id, json);
          },
          id),
      /*world_id=*/0);
}

// Nanoseconds per call of fn, over a million calls.
template <typename Fn>
double PerCall(Fn fn) {
  const int kCalls = 1000000;
  const base::TimeTicks start = base::TimeTicks::Now();
  for (int i = 0; i < kCalls; i++) {
    fn();
  }
  return (base::TimeTicks::Now() - start).InMicrosecondsF() * 1000 / kCalls;
}

void Stats(const std::string& id) {
  // The bare crossing: a hook with no values.
  const double ns = PerCall([] {
    PBValue unused;
    Go().hook(PB_EVENT_PING, nullptr, 0, &unused);
  });
  // A generated hook nobody subscribed to: the check and content's own call.
  PageTab* tab = Engine::Get()->active_tab();
  WebContentsObserverHooks* observer = tab;
  const double hook_off_ns = PerCall([observer] {
    observer->DidChangeVisibleSecurityState();
  });
  // The same hook subscribed: the tab's state copied and crossing to Go, which decodes
  // it and finds no handler.
  hooks::SetSubscribed(hooks::kWebContentsObserver_DidChangeVisibleSecurityState, true);
  const double hook_on_ns = PerCall([observer] {
    observer->DidChangeVisibleSecurityState();
  });
  hooks::SetSubscribed(hooks::kWebContentsObserver_DidChangeVisibleSecurityState, false);

  char* go = Go().go_stats();
  std::string go_stats(go);
  free(go);

  NSWindow* window = TheWindow();
  Reply(id, base::StringPrintf(
                "{\"cgoCallNs\":%.1f,\"hookOffNs\":%.1f,\"hookOnNs\":%.1f,"
                "\"firstPaintMs\":%lld,\"go\":%s,"
                "\"key\":%d,\"main\":%d,\"visible\":%d,\"occlusionVisible\":%d,"
                "\"ignoredStaleBounds\":%zu}",
                ns, hook_off_ns, hook_on_ns,
                Engine::Get()->shell()->first_paint().InMilliseconds(),
                go_stats.c_str(), window.isKeyWindow, window.isMainWindow,
                window.isVisible,
                (window.occlusionState & NSWindowOcclusionStateVisible) != 0,
                Engine::Get()->window()->ignored_stale_bounds()));
}

void Run(const std::string& line) {
  std::vector<std::string> parts = base::SplitString(
      line, " ", base::KEEP_WHITESPACE, base::SPLIT_WANT_ALL);
  if (parts.size() < 2) {
    return;
  }
  const std::string id = parts[0];
  const std::string cmd = parts[1];
  auto number = [&](size_t i) {
    double value = 0;
    base::StringToDouble(parts[i], &value);
    return value;
  };
  auto rest = [&](size_t i) {
    return base::JoinString(
        std::vector<std::string>(parts.begin() + i, parts.end()), " ");
  };

  if (cmd == "move") {
    Reply(id, SendMouse(NSEventTypeMouseMoved, number(2), number(3), 0));
  } else if (cmd == "click") {
    SendMouse(NSEventTypeMouseMoved, number(2), number(3), 0);
    SendMouse(NSEventTypeLeftMouseDown, number(2), number(3), 1);
    Reply(id, SendMouse(NSEventTypeLeftMouseUp, number(2), number(3), 1));
  } else if (cmd == "hit") {
    // Who would get a mouse event here: shell, page, container, frame (AppKit drags the
    // window), or a native view such as a window button.
    Reply(id, Classify(HitFromFrame(number(2), number(3))));
  } else if (cmd == "type") {
    NSString* text = base::SysUTF8ToNSString(rest(2));
    for (NSUInteger i = 0; i < text.length; i++) {
      SendKey([text substringWithRange:NSMakeRange(i, 1)],
              CodeForChar([text characterAtIndex:i]), 0);
    }
    Reply(id, "ok");
  } else if (cmd == "key") {
    const std::string name = parts[2];
    if (name == "enter") {
      SendKey(@"\r", 36, 0);
    } else if (name == "escape") {
      SendKey(@"\x1b", 53, 0);
    } else if (name == "tab") {
      SendKey(@"\t", 48, 0);
    } else if (name == "up") {
      unichar c = NSUpArrowFunctionKey;
      SendKey([NSString stringWithCharacters:&c length:1], 126,
              NSEventModifierFlagFunction | NSEventModifierFlagNumericPad);
    } else if (name == "down") {
      unichar c = NSDownArrowFunctionKey;
      SendKey([NSString stringWithCharacters:&c length:1], 125,
              NSEventModifierFlagFunction | NSEventModifierFlagNumericPad);
    } else {
      // cmd+<char>, ctrl+tab, ctrl+shift+tab.
      NSEventModifierFlags flags = 0;
      std::string key = name;
      for (size_t plus = key.find('+'); plus != std::string::npos;
           plus = key.find('+')) {
        const std::string mod = key.substr(0, plus);
        key = key.substr(plus + 1);
        if (mod == "cmd") {
          flags |= NSEventModifierFlagCommand;
        } else if (mod == "ctrl") {
          flags |= NSEventModifierFlagControl;
        } else if (mod == "shift") {
          flags |= NSEventModifierFlagShift;
        }
      }
      if (key == "tab") {
        SendKey(@"\t", 48, flags);
      } else if (key.size() == 1) {
        SendKey(base::SysUTF8ToNSString(key), CodeForChar(key[0]), flags);
      }
    }
    Reply(id, "ok");
  } else if (cmd == "resize") {
    [TheWindow() setContentSize:NSMakeSize(number(2), number(3))];
    Reply(id, "ok");
  } else if (cmd == "fullscreen") {
    [TheWindow() toggleFullScreen:nil];
    Reply(id, "ok");
  } else if (cmd == "eval") {
    Eval(id, Engine::Get()->shell()->contents(), rest(2));
  } else if (cmd == "evalpage") {
    Eval(id, Engine::Get()->active_tab()->contents(), rest(2));
  } else if (cmd == "capture") {
    Capture(id, parts[2]);
  } else if (cmd == "stats") {
    Stats(id);
  } else if (cmd == "title") {
    Reply(id, base::SysNSStringToUTF8(TheWindow().title));
  } else if (cmd == "bounds") {
    gfx::RectF b = Engine::Get()->window()->page_bounds();
    // The shown page's frame in the window's content view, where SetPageBounds put it.
    NSView* page_view =
        Engine::Get()->active_tab()->contents()->GetNativeView().GetNativeNSView();
    NSRect f = [page_view.superview convertRect:page_view.frame
                                         toView:TheWindow().contentView];
    NSSize c = TheWindow().contentView.bounds.size;
    NSButton* close_button = [TheWindow() standardWindowButton:NSWindowCloseButton];
    NSRect zoom = [TheWindow() standardWindowButton:NSWindowZoomButton].frame;
    NSRect close_frame = close_button.frame;
    const double from_top =
        close_button.superview.bounds.size.height - NSMidY(close_frame);
    Reply(id, base::StringPrintf(
                  "{\"reported\":[%g,%g,%g,%g],\"frame\":[%g,%g,%g,%g],"
                  "\"content\":[%g,%g],\"titlebar\":%g,\"buttonsStart\":%g,"
                  "\"buttonsEnd\":%g,\"buttonsMiddleY\":%g,\"buttonSize\":%g}",
                  b.x(), b.y(), b.width(), b.height(), f.origin.x, f.origin.y,
                  f.size.width, f.size.height, c.width, c.height,
                  c.height - TheWindow().contentLayoutRect.size.height,
                  NSMinX(close_frame), NSMaxX(zoom), from_top,
                  close_frame.size.width));
  } else if (cmd == "shellhidden") {
    Engine::Get()->shell()->contents()->GetNativeView().GetNativeNSView().hidden =
        parts[2] == "1";
    Reply(id, "ok");
  } else if (cmd == "pids") {
    Reply(id, base::StringPrintf(
                  "{\"shell\":%d,\"page\":%d}",
                  Engine::Get()->shell()->contents()->GetPrimaryMainFrame()->GetProcess()->GetProcess().Pid(),
                  Engine::Get()->active_tab()->contents()->GetPrimaryMainFrame()->GetProcess()->GetProcess().Pid()));
  } else if (cmd == "gc") {
    const base::TimeTicks start = base::TimeTicks::Now();
    Go().force_gc();
    Reply(id, base::StringPrintf(
                  "%.1f", (base::TimeTicks::Now() - start).InMicrosecondsF()));
  } else if (cmd == "gopanic") {
    Reply(id, Go().recover_nil_deref() ? "recovered" : "not recovered");
  } else if (cmd == "segv") {
    Reply(id, "ok");
    volatile int* nothing = nullptr;
    *nothing = 1;
  } else if (cmd == "crashpage") {
    Engine::Get()->active_tab()->contents()->GetPrimaryMainFrame()->GetProcess()->Shutdown(1);
    Reply(id, "ok");
  } else if (cmd == "crashshell") {
    Engine::Get()->shell()->contents()->GetPrimaryMainFrame()->GetProcess()->Shutdown(1);
    Reply(id, "ok");
  } else if (cmd == "close") {
    Reply(id, "ok");
    [TheWindow() performClose:nil];
  } else {
    Reply(id, "unknown");
  }
}

// Holds the two surfaces until both have arrived, then draws the shell over the page.
// Each layer is what content really rendered, but the result is drawn here, not read
// back from the window.
struct CaptureState : public base::RefCounted<CaptureState> {
  std::string id;
  std::string path;
  SkBitmap shell;
  SkBitmap page;
  bool shell_done = false;
  bool page_done = false;
  gfx::RectF bounds;
  std::string error;

 private:
  friend class base::RefCounted<CaptureState>;
  ~CaptureState() = default;
};

void FinishCapture(scoped_refptr<CaptureState> state) {
  if (!state->shell_done || !state->page_done) {
    return;
  }
  if (state->shell.empty()) {
    Reply(state->id, "error no shell surface " + state->error);
    return;
  }
  SkBitmap out;
  out.allocN32Pixels(state->shell.width(), state->shell.height());
  SkCanvas canvas(out);
  canvas.clear(SK_ColorWHITE);
  const float scale =
      state->shell.width() / std::max(1.0f, float(TheWindow().contentView.bounds.size.width));
  if (!state->page.empty()) {
    canvas.drawImage(state->page.asImage(), state->bounds.x() * scale,
                     state->bounds.y() * scale);
  }
  canvas.drawImage(state->shell.asImage(), 0, 0);

  auto write = [](const std::string& path, const SkBitmap& bitmap) {
    base::ScopedAllowBlockingForTesting allow_blocking;
    std::optional<std::vector<uint8_t>> png =
        gfx::PNGCodec::EncodeBGRASkBitmap(bitmap, false);
    return png && base::WriteFile(base::FilePath(path), *png);
  };
  bool ok = write(state->path, out);
  write(state->path + ".shell.png", state->shell);
  if (!state->page.empty()) {
    write(state->path + ".page.png", state->page);
  }
  Reply(state->id, ok ? "ok" : "error writing file");
}

void Capture(const std::string& id, const std::string& path) {
  auto state = base::MakeRefCounted<CaptureState>();
  state->id = id;
  state->path = path;
  state->bounds = Engine::Get()->window()->page_bounds();

  auto copy = [](content::WebContents* contents,
                 base::OnceCallback<void(const content::CopyFromSurfaceResult&)> done) {
    content::RenderWidgetHostView* view = contents->GetRenderWidgetHostView();
    if (!view) {
      std::move(done).Run(base::unexpected(content::CopyFromSurfaceError::kUnknown));
      return;
    }
    view->CopyFromSurface(gfx::Rect(), gfx::Size(), base::Seconds(5), std::move(done));
  };

  copy(Engine::Get()->shell()->contents(),
       base::BindOnce(
           [](scoped_refptr<CaptureState> state,
              const content::CopyFromSurfaceResult& result) {
             if (result.has_value()) {
               state->shell = result->bitmap;
             } else {
               state->error += " shell:" + base::NumberToString(static_cast<int>(result.error()));
             }
             state->shell_done = true;
             FinishCapture(state);
           },
           state));
  copy(Engine::Get()->active_tab()->contents(),
       base::BindOnce(
           [](scoped_refptr<CaptureState> state,
              const content::CopyFromSurfaceResult& result) {
             if (result.has_value()) {
               state->page = result->bitmap;
             } else {
               state->error += " page:" + base::NumberToString(static_cast<int>(result.error()));
             }
             state->page_done = true;
             FinishCapture(state);
           },
           state));
}

}  // namespace

void StartDriver(const base::FilePath& fifo) {
  Engine::Get()->window()->ForceKeyForDriver();
  const std::string path = fifo.value();
  dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
    // Opened read-write so it doesn't see end-of-file between writers.
    FILE* file = fopen(path.c_str(), "r+");
    if (!file) {
      fprintf(stderr, "driver: cannot open %s\n", path.c_str());
      return;
    }
    char* buffer = nullptr;
    size_t capacity = 0;
    while (getline(&buffer, &capacity, file) >= 0) {
      std::string line(buffer);
      base::TrimString(line, "\r\n", &line);
      dispatch_async(dispatch_get_main_queue(), ^{
        Run(line);
      });
    }
  });
  Reply("start", "ready");
}

}  // namespace project_b
