// The driver's commands, the same on every desktop: what each needs of the OS's windows and
// input is driver_platform.h's, driver_mac.mm on the Mac and driver_linux.cc on Linux.

#include "project_b/engine/browser/driver.h"

#include <stdio.h>

#include <memory>
#include <set>
#include <string>
#include <vector>

#include "base/files/file_util.h"
#include "base/functional/bind.h"
#include "base/functional/callback.h"
#include "base/json/json_writer.h"
#include "base/memory/ref_counted.h"
#include "base/memory/weak_ptr.h"
#include "base/no_destructor.h"
#include "base/process/process.h"
#include "base/strings/string_number_conversions.h"
#include "base/strings/string_split.h"
#include "base/strings/string_util.h"
#include "base/strings/stringprintf.h"
#include "base/strings/utf_string_conversions.h"
#include "base/task/sequenced_task_runner.h"
#include "base/threading/thread_restrictions.h"
#include "base/time/time.h"
#include "base/values.h"
#include "components/viz/common/frame_sinks/copy_output_result.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/render_process_host.h"
#include "content/public/browser/render_widget_host_view.h"
#include "content/public/browser/web_contents.h"
#include "content/public/browser/web_contents_observer.h"
#include "project_b/engine/browser/choose_path.h"
#include "project_b/engine/browser/driver_platform.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser/hooks_gen.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/browser/shell_page.h"
#include "project_b/engine/browser/window.h"
#include "project_b/engine/buildflags/buildflags.h"
#include "third_party/skia/include/core/SkBitmap.h"
#include "third_party/skia/include/core/SkCanvas.h"
#include "third_party/skia/include/core/SkColor.h"
#include "third_party/skia/include/core/SkImage.h"
#include "third_party/skia/include/core/SkRect.h"
#include "ui/gfx/codec/png_codec.h"
#include "url/gurl.h"

#if BUILDFLAG(PB_EXTENSIONS)
#include "project_b/engine/extensions/browser/extensions.h"
#endif

namespace project_b {

namespace {

void Reply(const std::string& id, const std::string& text) {
  printf("PBDRIVER %s %s\n", id.c_str(), text.c_str());
  fflush(stdout);
}

// Go's id for the window the driver acts on: the first, until "window" picks another.
int64_t g_window = 1;

Engine::AppWindow* Current() {
  return Engine::Get()->window(g_window);
}

// Makes a window the one the driver acts on, and the key window, as a click on it would.
// A session with no display has no key window of its own.
void UseWindow(int64_t id) {
  for (const auto& [other, w] : Engine::Get()->windows()) {
    if (other != id) {
      w->window->ForceKeyForDriver(false);
    }
  }
  g_window = id;
  Current()->window->ForceKeyForDriver(true);
}

Window* TheWindow() {
  return Current()->window.get();
}

// The tab the driver's window has the keyboard in, which "page" commands act on.
PageTab* Focused() {
  return Engine::Get()->FocusedTab(Current());
}

// The driver's window's settings tab: a tab of ours there showing chrome://settings/, or
// null.
PageTab* SettingsTab() {
  for (const auto& [id, tab] : Engine::Get()->tabs()) {
    if (tab->internal() && tab->window() == g_window &&
        tab->contents()->GetVisibleURL().host() == "settings") {
      return tab.get();
    }
  }
  return nullptr;
}

// Where the layout put the focused tab's page, in shell coordinates, or nothing.
gfx::RectF FocusedBounds() {
  for (const Engine::VisibleTab& v : Current()->visible) {
    if (v.tab == Current()->focused_tab) {
      return v.rect;
    }
  }
  return gfx::RectF();
}

void Capture(const std::string& id, const std::string& path);

// The reply to one eval. Content drops the result's callback, unrun, when the page the
// script went to is destroyed first, as a tab's is when it closes. So the reply
// watches the page, and answers "ERR gone" if it goes before the result comes. It
// answers once either way, and goes with its answer.
class EvalReply : public content::WebContentsObserver {
 public:
  EvalReply(const std::string& id, content::WebContents* contents)
      : content::WebContentsObserver(contents), id_(id) {}

  base::OnceCallback<void(base::Value)> Result() {
    return base::BindOnce(&EvalReply::Answer, weak_factory_.GetWeakPtr());
  }

 private:
  void Answer(base::Value value) {
    std::string json;
    base::JSONWriter::Write(value, &json);
    Done(json);
  }

  // content::WebContentsObserver:
  void WebContentsDestroyed() override { Done("ERR gone"); }

  void Done(const std::string& text) {
    Reply(id_, text);
    delete this;
  }

  const std::string id_;
  base::WeakPtrFactory<EvalReply> weak_factory_{this};
};

void Eval(const std::string& id, content::WebContents* contents, const std::string& js) {
  contents->GetPrimaryMainFrame()->ExecuteJavaScriptForTests(
      base::UTF8ToUTF16(js), (new EvalReply(id, contents))->Result(),
      /*world_id=*/0);
}

// A WebContents that is neither the shell nor a tab, asked for the standard shell: a copy of
// the shell that got past the gate, which only sees tabs. It is in our pages' context, or
// in the profile's, where the tabs are.
std::unique_ptr<content::WebContents>& Stray() {
  static base::NoDestructor<std::unique_ptr<content::WebContents>> stray;
  return *stray;
}

// Once the stray copy has loaded, replies with its URL and what its page can call.
void AskStrayShell(std::string id, int tries) {
  content::WebContents* contents = Stray().get();
  if (tries > 0 && (contents->IsLoading() ||
                    contents->GetLastCommittedURL() != GURL("chrome://shell/standard/"))) {
    base::SequencedTaskRunner::GetCurrentDefault()->PostDelayedTask(
        FROM_HERE, base::BindOnce(&AskStrayShell, id, tries - 1), base::Milliseconds(50));
    return;
  }
  contents->GetPrimaryMainFrame()->ExecuteJavaScriptForTests(
      u"location.href + ' ' + typeof (window.chrome && chrome.send)",
      base::BindOnce(
          [](const std::string& id, base::Value value) {
            Reply(id, value.is_string() ? value.GetString() : "no answer");
            base::SequencedTaskRunner::GetCurrentDefault()->PostTask(
                FROM_HERE, base::BindOnce([] { Stray().reset(); }));
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
  WebContentsObserverHooks* observer = Focused();
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

  Reply(id, base::StringPrintf(
                "{\"cgoCallNs\":%.1f,\"hookOffNs\":%.1f,\"hookOnNs\":%.1f,"
                "\"go\":%s,%s,"
                "\"ignoredStaleBounds\":%zu}",
                ns, hook_off_ns, hook_on_ns, go_stats.c_str(),
                driver::Flags(TheWindow()).c_str(),
                Current()->window->ignored_stale_bounds()));
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

  // The commands that act on the focused tab's page, which a window with no tabs hasn't.
  static const base::NoDestructor<std::set<std::string>> kNeedTab(
      std::set<std::string>{"evalpage", "committed", "bounds", "pids", "pagecolor",
                            "crashpage", "stats"});
  if (kNeedTab->contains(cmd) && !Focused()) {
    Reply(id, "ERR no tab");
    return;
  }

  if (cmd == "window") {
    // "window <id>" acts on that window from now on, and makes it the key window.
    if (Engine::Get()->window(number(2))) {
      UseWindow(number(2));
      Reply(id, "ok");
    } else {
      Reply(id, "ERR no window");
    }
  } else if (cmd == "windows") {
    // Every window: Go's id, its frame on screen, y down from the top of the main
    // screen, its title, whether it is the key window, and the tabs it shows.
    base::ListValue out;
    for (const auto& [wid, w] : Engine::Get()->windows()) {
      Window* window = w->window.get();
      const gfx::Rect frame = w->window->GetBounds();
      base::ListValue tabs;
      for (const Engine::VisibleTab& v : w->visible) {
        tabs.Append(static_cast<int>(v.tab));
      }
      out.Append(base::DictValue()
                     .Set("id", static_cast<int>(wid))
                     .Set("frame", base::ListValue()
                                       .Append(frame.x())
                                       .Append(frame.y())
                                       .Append(frame.width())
                                       .Append(frame.height()))
                     .Set("title", driver::Title(window))
                     .Set("key", driver::IsKey(window))
                     .Set("visible", driver::IsVisible(window))
                     .Set("tabs", std::move(tabs)));
    }
    Reply(id, base::WriteJson(out).value_or("[]"));
  } else if (cmd == "move") {
    Reply(id, driver::SendMouse(TheWindow(), driver::Mouse::kMove, number(2), number(3), 0));
  } else if (cmd == "click") {
    driver::SendMouse(TheWindow(), driver::Mouse::kMove, number(2), number(3), 0);
    driver::SendMouse(TheWindow(), driver::Mouse::kDown, number(2), number(3), 1);
    Reply(id, driver::SendMouse(TheWindow(), driver::Mouse::kUp, number(2), number(3), 1));
  } else if (cmd == "hit") {
    // Who would get a mouse event here: shell, page, container, frame (the OS drags the
    // window), or a native view such as a window button.
    Reply(id, driver::Hit(TheWindow(), number(2), number(3)));
  } else if (cmd == "press") {
    // What a press here does to the window: "moves" when the window hands it to the
    // window server to move, below the title bar; "titlebar" when it falls through to
    // the frame inside the title bar, where the window server moves the window by itself
    // from what "dragregion" lists; otherwise who gets it, as "hit" says.
    Reply(id, driver::Press(TheWindow(), number(2), number(3)));
  } else if (cmd == "dragregion") {
    // What AppKit last told the window server it may drag by itself, as AppKit
    // describes it: rectangles in window points, y from the bottom.
    Reply(id, driver::DragRegion(TheWindow()));
  } else if (cmd == "type") {
    for (char16_t c : base::UTF8ToUTF16(rest(2))) {
      driver::SendKey(TheWindow(), {.character = std::u16string(1, c)});
    }
    Reply(id, "ok");
  } else if (cmd == "key") {
    // enter, escape, tab, up, down, or modifiers and a key: cmd+<char>, ctrl+tab,
    // ctrl+shift+tab.
    driver::Key key;
    std::string name = parts[2];
    for (size_t plus = name.find('+'); plus != std::string::npos; plus = name.find('+')) {
      const std::string mod = name.substr(0, plus);
      name = name.substr(plus + 1);
      key.cmd |= mod == "cmd";
      key.ctrl |= mod == "ctrl";
      key.shift |= mod == "shift";
      key.opt |= mod == "opt";
    }
    if (name == "enter" || name == "escape" || name == "tab" || name == "up" ||
        name == "down") {
      key.name = name;
      driver::SendKey(TheWindow(), key);
    } else if (name.size() == 1) {
      key.character = base::UTF8ToUTF16(name);
      driver::SendKey(TheWindow(), key);
    }
    Reply(id, "ok");
  } else if (cmd == "regions") {
    // The layout's regions the window holds, in order: [x, y, w, h, area], area 0 the page,
    // 1 the shell, 2 a drag area.
    base::ListValue out;
    for (const Window::Region& r : TheWindow()->regions()) {
      out.Append(base::ListValue()
                     .Append(r.rect.x())
                     .Append(r.rect.y())
                     .Append(r.rect.width())
                     .Append(r.rect.height())
                     .Append(static_cast<int>(r.area)));
    }
    Reply(id, base::WriteJson(out).value_or("[]"));
  } else if (cmd == "focused") {
    // Which view the OS sends keys to now.
    Reply(id, driver::Focused(TheWindow()));
  } else if (cmd == "resize") {
    driver::ResizeContent(TheWindow(), number(2), number(3));
    Reply(id, "ok");
  } else if (cmd == "state") {
    // What the native window is now, as the OS says.
    Window* window = TheWindow();
    Reply(id, base::StringPrintf(
                  "{\"fullScreen\":%s,\"zoomed\":%s,\"miniaturized\":%s}",
                  window->IsFullscreen() ? "true" : "false",
                  window->IsMaximized() ? "true" : "false",
                  window->IsMinimized() ? "true" : "false"));
  } else if (cmd == "fullscreen") {
    TheWindow()->ToggleFullScreen();
    Reply(id, "ok");
  } else if (cmd == "eval") {
    Eval(id, Current()->shell->contents(), rest(2));
  } else if (cmd == "evalpage") {
    Eval(id, Focused()->contents(), rest(2));
  } else if (cmd == "evaltab") {
    // In one tab's page, by Go's id for it.
    if (PageTab* tab = Engine::Get()->tab(number(2))) {
      Eval(id, tab->contents(), rest(3));
    } else {
      Reply(id, "ERR no tab");
    }
  } else if (cmd == "opentab") {
    // "opentab <workspace> <url>" opens a tab in a workspace, by Go's id for it, as a page
    // opens one: Go hears of it and shows it, then it loads. Replies with the tab's id.
    PageTab* tab = Engine::Get()->CreateTab(parts[2]);
    Engine::TabOpened(/*opener=*/0, tab->id(), GURL(rest(3)), /*foreground=*/true,
                      /*new_window=*/false);
    Engine::Get()->LoadURL(tab->id(), rest(3).c_str());
    Reply(id, base::NumberToString(tab->id()));
  } else if (cmd == "showtabs") {
    // Shows these tabs in the window as a layout from Go would, without Go: "showtabs
    // <tab> <x> <y> <w> <h> ..." in shell coordinates. Go's next layout replaces it.
    std::vector<Engine::VisibleTab> tabs;
    for (size_t i = 2; i + 4 < parts.size(); i += 5) {
      tabs.push_back({static_cast<int64_t>(number(i)),
                      gfx::RectF(number(i + 1), number(i + 2), number(i + 3),
                                 number(i + 4))});
    }
    Engine::Get()->ShowTabs(Current(), std::move(tabs));
    Reply(id, "ok");
  } else if (cmd == "visible") {
    // The tabs the window shows: each one's id, where its view is in shell
    // coordinates, and whether content was told it is visible.
    base::ListValue out;
    for (const Engine::VisibleTab& v : Current()->visible) {
      content::WebContents* contents = Engine::Get()->tab(v.tab)->contents();
      const gfx::RectF f = driver::ViewFrame(TheWindow(), contents->GetNativeView());
      out.Append(base::DictValue()
                     .Set("tab", static_cast<int>(v.tab))
                     .Set("frame", base::ListValue()
                                       .Append(f.x())
                                       .Append(f.y())
                                       .Append(f.width())
                                       .Append(f.height()))
                     .Set("hidden", driver::ViewHidden(contents->GetNativeView()))
                     .Set("shown", contents->GetVisibility() ==
                                       content::Visibility::VISIBLE));
    }
    Reply(id, base::WriteJson(out).value_or("[]"));
  } else if (cmd == "committed") {
    // The focused tab's page as content has it: the URL it last committed, and whether it
    // is still loading. The shell can show a typed address with no progress a moment before
    // its navigation begins, and an eval sent to a page about to be replaced is never
    // answered, so a test waits on this before it talks to the page it asked for.
    content::WebContents* contents = Focused()->contents();
    Reply(id, base::WriteJson(base::DictValue()
                                  .Set("url", contents->GetLastCommittedURL().spec())
                                  .Set("loading", contents->IsLoading()))
                  .value_or("{}"));
  } else if (cmd == "evalsettings") {
    // In the settings page of the driver's window: its tab of ours whose page settings is.
    if (PageTab* settings = SettingsTab()) {
      Eval(id, settings->contents(), rest(2));
    } else {
      Reply(id, "ERR no settings page");
    }
#if BUILDFLAG(PB_EXTENSIONS)
  } else if (cmd == "evalpopup") {
    // In the open extension popup's page.
    if (content::WebContents* popup =
            Extensions::Get() ? Extensions::Get()->popup_contents() : nullptr) {
      Eval(id, popup, rest(2));
    } else {
      Reply(id, "ERR no popup");
    }
  } else if (cmd == "popup") {
    // Where the popup is, in shell coordinates: x y w h, all zero for none.
    const gfx::RectF b = Current()->window->popup_bounds();
    Reply(id, base::StringPrintf("%.0f %.0f %.0f %.0f", b.x(), b.y(), b.width(),
                                 b.height()));
#endif
  } else if (cmd == "choose") {
    // What the next folder or package chooser answers, in place of the system's panel.
    ChooseNextForDriver(base::FilePath(rest(2)));
    Reply(id, "ok");
  } else if (cmd == "mods") {
    // What the app reports when the modifiers change; "mods 0" lets go of them all.
    driver::SendModifiers(static_cast<uint64_t>(number(2)));
    Reply(id, "ok");
  } else if (cmd == "menu") {
    Reply(id, driver::MenuJson());
  } else if (cmd == "choosemenu") {
    // "choosemenu <menu> <item>": chooses the item, as a click on it does, each menu read
    // as it opens. An item in a submenu is "<submenu> > <item>".
    Reply(id, driver::ChooseMenu(parts[2], rest(3)) ? "ok" : "no such item");
  } else if (cmd == "dockdot") {
    // The colour at the middle of the Dock icon's dot, or of where it goes: a tenth in from
    // the right and the bottom, and half the dot's quarter across. The bitmap counts y
    // down from the top. Its pixels are sRGB already, so they are read as they are.
    Reply(id, driver::DockDot());
  } else if (cmd == "capture") {
    Capture(id, parts[2]);
  } else if (cmd == "stats") {
    Stats(id);
  } else if (cmd == "timeline") {
    // The timeline kept in memory after entry <since>, as ProjectB.getTimeline answers.
    hooks::Args args;
    args.Str("ProjectB.getTimeline");
    args.Str("{\"since\":" + (parts.size() > 2 ? parts[2] : "0") + "}");
    PBValue result{};
    hooks::Ask(PB_EVENT_DEVTOOLS_COMMAND, args, &result);
    Reply(id, hooks::TakeString(result));
  } else if (cmd == "projectb") {
    // One of our own DevTools commands, "projectb <method> <params json>", as a client of
    // the debugging port sends ProjectB.<method>: Go's answer is the reply.
    hooks::Args args;
    args.Str("ProjectB." + parts[2]);
    args.Str(rest(3));
    PBValue result{};
    hooks::Ask(PB_EVENT_DEVTOOLS_COMMAND, args, &result);
    Reply(id, hooks::TakeString(result));
  } else if (cmd == "color") {
    Reply(id, base::StringPrintf("#%06x", driver::BackgroundColor(TheWindow())));
  } else if (cmd == "showncolor") {
    // What the window painted when it first went on screen.
    Reply(id, base::StringPrintf("#%06x", Current()->window->shown_rgb()));
  } else if (cmd == "frame") {
    // The band the window's content view draws over everything, and its curve.
    Reply(id, driver::Frame(TheWindow()));
  } else if (cmd == "title") {
    Reply(id, driver::Title(TheWindow()));
  } else if (cmd == "bounds") {
    // The focused page's frame in the window, where the layout put it, and the window's
    // buttons.
    Reply(id, driver::Bounds(TheWindow(), FocusedBounds(), Focused()->contents()));
  } else if (cmd == "shellhidden") {
    driver::SetViewHidden(Current()->shell->contents()->GetNativeView(), parts[2] == "1");
    Reply(id, "ok");
  } else if (cmd == "pids") {
    Reply(id, base::StringPrintf(
                  "{\"shell\":%d,\"page\":%d}",
                  Current()->shell->contents()->GetPrimaryMainFrame()->GetProcess()->GetProcess().Pid(),
                  Focused()->contents()->GetPrimaryMainFrame()->GetProcess()->GetProcess().Pid()));
  } else if (cmd == "gc") {
    const base::TimeTicks start = base::TimeTicks::Now();
    Go().force_gc();
    Reply(id, base::StringPrintf(
                  "%.1f", (base::TimeTicks::Now() - start).InMicrosecondsF()));
  } else if (cmd == "gopanic") {
    Reply(id, Go().recover_nil_deref() ? "recovered" : "not recovered");
  } else if (cmd == "gocrash") {
    Reply(id, "ok");
    int kind = 0;
    base::StringToInt(parts[2], &kind);
    Go().crash(kind);
  } else if (cmd == "segv") {
    Reply(id, "ok");
    volatile int* nothing = nullptr;
    *nothing = 1;
  } else if (cmd == "pagecolor") {
    // The colour in the middle of what the shown tab's page view draws, or "none"
    // when it draws nothing.
    content::RenderWidgetHostView* view =
        Focused()->contents()->GetRenderWidgetHostView();
    if (!view) {
      Reply(id, "none");
      return;
    }
    view->CopyFromSurface(
        gfx::Rect(), gfx::Size(), base::Seconds(5),
        base::BindOnce(
            [](const std::string& id, const content::CopyFromSurfaceResult& result) {
              if (!result.has_value() || result->bitmap.empty()) {
                Reply(id, "none");
                return;
              }
              const SkBitmap& bitmap = result->bitmap;
              const SkColor c = bitmap.getColor(bitmap.width() / 2, bitmap.height() / 2);
              Reply(id, base::StringPrintf("#%02x%02x%02x", SkColorGetR(c),
                                           SkColorGetG(c), SkColorGetB(c)));
            },
            id));
  } else if (cmd == "strayshell") {
    // "strayshell pages" or "strayshell tabs": which context it is in.
    Engine* engine = Engine::Get();
    Stray() = content::WebContents::Create(content::WebContents::CreateParams(
        parts.size() > 2 && parts[2] == "tabs" ? engine->browser_context()
                                               : engine->pages_context()));
    Stray()->GetController().LoadURLWithParams(
        content::NavigationController::LoadURLParams(GURL("chrome://shell/standard/")));
    AskStrayShell(id, 100);
  } else if (cmd == "crashpage") {
    Focused()->contents()->GetPrimaryMainFrame()->GetProcess()->Shutdown(1);
    Reply(id, "ok");
  } else if (cmd == "crashshell") {
    Current()->shell->contents()->GetPrimaryMainFrame()->GetProcess()->Shutdown(1);
    Reply(id, "ok");
  } else if (cmd == "close") {
    Reply(id, "ok");
    driver::PressClose(TheWindow());
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
      state->shell.width() / std::max(1.0f, float(driver::ContentWidth(TheWindow())));
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
  state->bounds = FocusedBounds();

  auto copy = [](content::WebContents* contents,
                 base::OnceCallback<void(const content::CopyFromSurfaceResult&)> done) {
    content::RenderWidgetHostView* view = contents->GetRenderWidgetHostView();
    if (!view) {
      std::move(done).Run(base::unexpected(content::CopyFromSurfaceError::kUnknown));
      return;
    }
    view->CopyFromSurface(gfx::Rect(), gfx::Size(), base::Seconds(5), std::move(done));
  };

  // A window that shows no tab has no page under its shell: the shell is the picture.
  copy(Current()->shell->contents(),
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
  PageTab* page = Focused();
  if (!page) {
    state->page_done = true;
    return;
  }
  copy(page->contents(),
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
  UseWindow(g_window);
  // A driver that exits or dies without closing the app would leave it running, its
  // window on the screen, with nothing to drive it. So the app quits, as Cmd+Q does, once
  // the process that started it is gone.
  driver::StartReading(fifo.value(), base::BindRepeating(&Run), base::BindOnce([] {
                         fprintf(stderr,
                                 "driver: the process that started the app is gone; quitting\n");
                         if (Engine* engine = Engine::Get()) {
                           engine->Quit();
                         }
                       }));
  Reply("start", "ready");
}

}  // namespace project_b
