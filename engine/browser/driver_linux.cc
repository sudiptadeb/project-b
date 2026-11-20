// Linux's part of the driver (driver_platform.h): real ui::Events through the window's Aura
// tree, the way the platform window hands them in, so each goes through the same targeting
// (Go's regions) and focus as a real mouse and keyboard.

#include "project_b/engine/browser/driver_platform.h"

#include <fcntl.h>
#include <poll.h>
#include <stdio.h>
#include <unistd.h>

#include <string>
#include <tuple>
#include <utility>

#include "base/functional/bind.h"
#include "base/functional/callback.h"
#include "base/json/json_reader.h"
#include "base/json/json_writer.h"
#include "base/no_destructor.h"
#include "base/strings/string_split.h"
#include "base/values.h"
#include "base/strings/string_number_conversions.h"
#include "base/strings/stringprintf.h"
#include "base/strings/utf_string_conversions.h"
#include "base/task/single_thread_task_runner.h"
#include "base/threading/platform_thread.h"
#include "base/time/time.h"
#include "content/public/browser/browser_task_traits.h"
#include "content/public/browser/browser_thread.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/browser/shell_page.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser/window.h"
#include "project_b/engine/buildflags/buildflags.h"
#include "third_party/skia/include/core/SkColor.h"
#include "ui/aura/client/focus_client.h"
#include "ui/aura/window.h"
#include "ui/aura/window_targeter.h"
#include "ui/aura/window_tree_host.h"
#include "ui/base/ime/input_method.h"
#include "ui/compositor/layer.h"
#include "ui/compositor/layer_solid_color.h"
#include "ui/events/event.h"
#include "ui/events/event_sink.h"
#include "ui/events/keycodes/dom/dom_code.h"
#include "ui/events/keycodes/dom/dom_key.h"
#include "ui/events/keycodes/keyboard_code_conversion.h"
#include "ui/gfx/geometry/point_conversions.h"
#include "ui/gfx/geometry/rect_conversions.h"
#include "ui/wm/public/activation_client.h"

#if BUILDFLAG(PB_EXTENSIONS)
#include "project_b/engine/extensions/browser/extensions.h"
#endif

namespace project_b::driver {

namespace {

aura::Window* RootOf(Window* window) {
  return window->native_window();
}

Engine::AppWindow* AppWindowOf(Window* window) {
  for (const auto& [id, w] : Engine::Get()->windows()) {
    if (w->window.get() == window) {
      return w.get();
    }
  }
  return nullptr;
}

// The window an event at this point goes to, found as the event dispatcher finds it.
aura::Window* TargetAt(Window* window, double x, double y) {
  aura::Window* root = RootOf(window);
  const gfx::PointF p(x, y);
  ui::MouseEvent event(ui::EventType::kMouseMoved, p, p, base::TimeTicks::Now(), 0, 0);
  aura::WindowTargeter targeter;
  return static_cast<aura::Window*>(targeter.FindTargetForEvent(root, &event));
}

// What a target is, as the driver reports it, in the Mac's words: a press in a drag area
// reaches the root, as the Mac's reaches its frame.
std::string Classify(Window* window, aura::Window* hit) {
  if (!hit) {
    return "nil";
  }
  Engine::AppWindow* app = AppWindowOf(window);
  if (app->shell && app->shell->contents()->GetNativeView()->Contains(hit)) {
    return "shell";
  }
  for (const Engine::VisibleTab& v : app->visible) {
    if (Engine::Get()->tab(v.tab)->contents()->GetNativeView()->Contains(hit)) {
      return "page";
    }
  }
#if BUILDFLAG(PB_EXTENSIONS)
  content::WebContents* popup =
      Extensions::Get() ? Extensions::Get()->popup_contents() : nullptr;
  if (popup && popup->GetNativeView()->Contains(hit)) {
    return "popup";
  }
#endif
  if (hit == RootOf(window)) {
    return "frame";
  }
  return "container";
}

// Hands an event in as the platform window does: in the host's pixels.
void Dispatch(Window* window, ui::Event* event) {
  aura::WindowTreeHost* host = RootOf(window)->GetHost();
  if (event->IsLocatedEvent()) {
    ui::LocatedEvent* located = event->AsLocatedEvent();
    const float scale = host->device_scale_factor();
    gfx::PointF p = located->location_f();
    p.Scale(scale);
    located->set_location_f(p);
    located->set_root_location_f(p);
  }
  // What became of it is content's to say, as for a real one.
  std::ignore = host->GetEventSink()->OnEventFromSource(event);
}

std::string Hex(uint32_t rgb) {
  return base::StringPrintf("#%06x", rgb);
}

}  // namespace

// Reads the FIFO on a thread of its own and runs each line on the main thread. Every half
// second it also looks at its parent: once the process that started the app is gone, the app
// quits.
void StartReading(const std::string& path,
                  base::RepeatingCallback<void(const std::string&)> run,
                  base::OnceClosure quit) {
  class Reader : public base::PlatformThread::Delegate {
   public:
    Reader(std::string path,
           base::RepeatingCallback<void(const std::string&)> run,
           base::OnceClosure quit)
        : path_(std::move(path)),
          run_(std::move(run)),
          quit_(std::move(quit)),
          main_(content::GetUIThreadTaskRunner({})),
          parent_(getppid()) {}

    void ThreadMain() override {
      // Opened read-write so it doesn't see end-of-file between writers.
      const int fd = open(path_.c_str(), O_RDWR);
      if (fd < 0) {
        fprintf(stderr, "driver: cannot open %s\n", path_.c_str());
        return;
      }
      std::string pending;
      char buffer[4096];
      while (true) {
        pollfd p = {.fd = fd, .events = POLLIN};
        const int ready = poll(&p, 1, 500);
        if (getppid() != parent_) {
          main_->PostTask(FROM_HERE, std::move(quit_));
          return;
        }
        if (ready <= 0) {
          continue;
        }
        const ssize_t n = read(fd, buffer, sizeof(buffer));
        if (n <= 0) {
          continue;
        }
        pending.append(buffer, n);
        for (size_t end = pending.find('\n'); end != std::string::npos;
             end = pending.find('\n')) {
          std::string line = pending.substr(0, end);
          pending.erase(0, end + 1);
          if (!line.empty() && line.back() == '\r') {
            line.pop_back();
          }
          main_->PostTask(FROM_HERE, base::BindOnce(run_, std::move(line)));
        }
      }
    }

   private:
    const std::string path_;
    base::RepeatingCallback<void(const std::string&)> run_;
    base::OnceClosure quit_;
    scoped_refptr<base::SingleThreadTaskRunner> main_;
    const pid_t parent_;
  };
  // It reads for as long as the app runs.
  if (!base::PlatformThread::CreateNonJoinable(
          0, new Reader(path, std::move(run), std::move(quit)))) {
    fprintf(stderr, "driver: cannot start its reader\n");
  }
}

std::string Hit(Window* window, double x, double y) {
  return Classify(window, TargetAt(window, x, y));
}

// As on the Mac, only our own views get events: a drag area or the window's own parts is
// reported and left alone.
std::string SendMouse(Window* window, Mouse kind_of_event, double x, double y, int clicks) {
  const std::string kind = Classify(window, TargetAt(window, x, y));
  if (kind != "shell" && kind != "page" && kind != "popup") {
    return kind;
  }
  const gfx::PointF p(x, y);
  const base::TimeTicks now = base::TimeTicks::Now();
  const int button = kind_of_event == Mouse::kMove ? 0 : ui::EF_LEFT_MOUSE_BUTTON;
  ui::MouseEvent event(kind_of_event == Mouse::kMove   ? ui::EventType::kMouseMoved
                       : kind_of_event == Mouse::kDown ? ui::EventType::kMousePressed
                                                       : ui::EventType::kMouseReleased,
                       p, p, now, button, button);
  if (clicks > 1) {
    event.SetClickCount(clicks);
  }
  if (kind_of_event == Mouse::kUp && kind == "shell") {
    timeline::Input("input.click", now);
  }
  Dispatch(window, &event);
  return kind;
}

// The app moves the window from a drag area itself on Linux (step 3).
std::string Press(Window* window, double x, double y) {
  if (window->MovesFromApp(gfx::PointF(x, y))) {
    return "moves";
  }
  return Hit(window, x, y);
}

std::string DragRegion(Window* window) {
  return "unknown";
}

// A key at the focused window, pressed and let go. A character's key is the US layout's;
// cmd is Ctrl, the app's own modifier on Linux.
void SendKey(Window* window, const Key& key) {
  int flags = 0;
  if (key.cmd || key.ctrl) {
    flags |= ui::EF_CONTROL_DOWN;
  }
  if (key.shift) {
    flags |= ui::EF_SHIFT_DOWN;
  }
  if (key.opt) {
    flags |= ui::EF_ALT_DOWN;
  }
  ui::DomKey dom_key;
  ui::DomCode code;
  if (key.name == "enter") {
    dom_key = ui::DomKey::ENTER;
    code = ui::DomCode::ENTER;
  } else if (key.name == "escape") {
    dom_key = ui::DomKey::ESCAPE;
    code = ui::DomCode::ESCAPE;
  } else if (key.name == "tab") {
    dom_key = ui::DomKey::TAB;
    code = ui::DomCode::TAB;
  } else if (key.name == "up") {
    dom_key = ui::DomKey::ARROW_UP;
    code = ui::DomCode::ARROW_UP;
  } else if (key.name == "down") {
    dom_key = ui::DomKey::ARROW_DOWN;
    code = ui::DomCode::ARROW_DOWN;
  } else if (!key.character.empty()) {
    const char16_t c = key.character[0];
    dom_key = ui::DomKey::FromCharacter(c);
    code = ui::UsLayoutDomKeyToDomCode(dom_key);
    // An upper-case letter or a shifted symbol is Shift and its key.
    if (code != ui::DomCode::NONE && ui::DomCodeToUsLayoutCharacter(code, 0) != c) {
      flags |= ui::EF_SHIFT_DOWN;
    }
  } else {
    return;
  }
  const ui::KeyboardCode key_code = code != ui::DomCode::NONE
                                        ? ui::DomCodeToUsLayoutKeyboardCode(code)
                                        : ui::VKEY_UNKNOWN;
  const base::TimeTicks now = base::TimeTicks::Now();
  timeline::Input("input.key", now);
  ui::KeyEvent down(ui::EventType::kKeyPressed, key_code, code, flags, dom_key, now);
  Dispatch(window, &down);
  ui::KeyEvent up(ui::EventType::kKeyReleased, key_code, code, flags, dom_key, now);
  Dispatch(window, &up);
}

std::string Focused(Window* window) {
  aura::client::FocusClient* focus = aura::client::GetFocusClient(RootOf(window));
  aura::Window* focused = focus ? focus->GetFocusedWindow() : nullptr;
  // Which view, by the name content or we gave it, and whether the input method has a text
  // field to put characters in: a key with no client there gives none.
  ui::InputMethod* ime = RootOf(window)->GetHost()->GetInputMethod();
  return Classify(window, focused) + ":" + (focused ? focused->GetName() : "") +
         (ime && ime->GetTextInputClient() ? " ime:client" : " ime:none");
}

namespace {

base::DictValue TreeOf(aura::Window* w) {
  base::DictValue d;
  const gfx::Rect b = w->bounds();
  d.Set("name", w->GetName());
  d.Set("bounds", base::ListValue().Append(b.x()).Append(b.y()).Append(b.width()).Append(b.height()));
  d.Set("visible", w->IsVisible());
  d.Set("shown", w->TargetVisibility());
  if (ui::Layer* layer = w->layer()) {
    d.Set("layer", layer->name());
    d.Set("type", static_cast<int>(layer->type()));
    d.Set("opacity", layer->opacity());
    d.Set("layerVisible", layer->visible());
  }
  base::ListValue children;
  for (aura::Window* child : w->children()) {
    children.Append(TreeOf(child));
  }
  if (!children.empty()) {
    d.Set("children", std::move(children));
  }
  return d;
}

}  // namespace

std::string Tree(Window* window) {
  return base::WriteJson(TreeOf(RootOf(window))).value_or("{}");
}

std::string Title(Window* window) {
  return base::UTF16ToUTF8(RootOf(window)->GetTitle());
}

bool IsKey(Window* window) {
  wm::ActivationClient* activation = wm::GetActivationClient(RootOf(window));
  return activation && activation->GetActiveWindow();
}

bool IsVisible(Window* window) {
  return RootOf(window)->IsVisible();
}

std::string Flags(Window* window) {
  const bool key = IsKey(window);
  const bool visible = IsVisible(window);
  return base::StringPrintf("\"key\":%d,\"main\":%d,\"visible\":%d,\"occlusionVisible\":%d",
                            key, key, visible, visible);
}

void ResizeContent(Window* window, double width, double height) {
  aura::WindowTreeHost* host = RootOf(window)->GetHost();
  gfx::Rect bounds = host->GetBoundsInPixels();
  bounds.set_size(gfx::ToRoundedSize(
      gfx::ScaleSize(gfx::SizeF(width, height), host->device_scale_factor())));
  host->SetBoundsInPixels(bounds);
}

double ContentWidth(Window* window) {
  return RootOf(window)->bounds().width();
}

// The window's first child is its colour (window_linux.cc).
uint32_t BackgroundColor(Window* window) {
  const SkColor c =
      RootOf(window)->children().front()->layer()->AsSolidColor()->GetTargetColor().toSkColor();
  return SkColorGetR(c) << 16 | SkColorGetG(c) << 8 | SkColorGetB(c);
}

// No band is drawn round the window yet (step 3).
std::string Frame(Window* window) {
  return base::StringPrintf("{\"radius\":0,\"width\":0,\"color\":\"%s\"}",
                            Hex(BackgroundColor(window)).c_str());
}

gfx::RectF ViewFrame(Window* window, gfx::NativeView view) {
  gfx::Rect f = view->bounds();
  if (view->parent()) {
    aura::Window::ConvertRectToTarget(view->parent(), RootOf(window), &f);
  }
  return gfx::RectF(f);
}

bool ViewHidden(gfx::NativeView view) {
  return !view->TargetVisibility();
}

void SetViewHidden(gfx::NativeView view, bool hidden) {
  if (hidden) {
    view->Hide();
  } else {
    view->Show();
  }
}

// The shell draws the window buttons on Linux, so the window has none of its own.
std::string Bounds(Window* window, const gfx::RectF& b, content::WebContents* page) {
  // The frame as the Mac's driver gives it, y up from the bottom, which the tests read.
  gfx::RectF f = ViewFrame(window, page->GetNativeView());
  const gfx::Size c = RootOf(window)->bounds().size();
  f.set_y(c.height() - f.bottom());
  return base::StringPrintf(
      "{\"reported\":[%g,%g,%g,%g],\"frame\":[%g,%g,%g,%g],"
      "\"content\":[%d,%d],\"titlebar\":0,\"buttonsStart\":0,"
      "\"buttonsEnd\":0,\"buttonsMiddleY\":0,\"buttonSize\":0}",
      b.x(), b.y(), b.width(), b.height(), f.x(), f.y(), f.width(), f.height(), c.width(),
      c.height());
}

void PressClose(Window* window) {
  window->Close();
}

void SendModifiers(uint64_t flags) {
  int mods = 0;
  if (flags & (1u << 18)) {
    mods |= PB_MOD_CTRL;
  }
  if (flags & (1u << 19)) {
    mods |= PB_MOD_ALT;
  }
  if (flags & (1u << 17)) {
    mods |= PB_MOD_SHIFT;
  }
  if (flags & (1u << 20)) {
    mods |= PB_MOD_CMD;
  }
  hooks::Args args;
  args.Int(mods);
  hooks::Fire(PB_EVENT_MODS_CHANGED, args);
}

namespace {

std::string& Menu() {
  static base::NoDestructor<std::string> menu;
  return *menu;
}

// The item titled title among items, or null.
const base::DictValue* Titled(const base::ListValue* items, const std::string& title) {
  if (!items) {
    return nullptr;
  }
  for (const base::Value& item : *items) {
    if (item.is_dict() && item.GetDict().FindString("title") &&
        *item.GetDict().FindString("title") == title) {
      return &item.GetDict();
    }
  }
  return nullptr;
}

}  // namespace

void KeepMenu(const std::string& json) {
  Menu() = json;
}

namespace {

// A key as the Mac's menu writes it, which the tests read: "^~$@" for Ctrl, Option, Shift and
// Cmd, then the key's character, from Go's {code, mods}.
// A function key is its name, F1 to F24.
std::string KeyText(const base::DictValue& key) {
  const int mods = key.FindInt("mods").value_or(0);
  const auto code = static_cast<ui::KeyboardCode>(key.FindInt("code").value_or(0));
  std::string name;
  if (code >= ui::VKEY_F1 && code <= ui::VKEY_F24) {
    name = "F" + base::NumberToString(code - ui::VKEY_F1 + 1);
  } else if (const char16_t c =
                 ui::DomCodeToUsLayoutCharacter(ui::UsLayoutKeyboardCodeToDomCode(code), 0)) {
    name = base::UTF16ToUTF8(std::u16string(1, c));
  } else {
    return "";
  }
  return std::string(mods & PB_MOD_CTRL ? "^" : "") + (mods & PB_MOD_ALT ? "~" : "") +
         (mods & PB_MOD_SHIFT ? "$" : "") + (mods & PB_MOD_CMD ? "@" : "") + name;
}

// A menu's items as the Mac's driver reports them: each one's title, key, whether it is
// enabled and checked, and its submenu's items; "-" for a separator; a list's items, asked
// of Go as the menu opens, in its place.
base::ListValue Items(const base::ListValue& from) {
  base::ListValue items;
  for (const base::Value& value : from) {
    if (!value.is_dict()) {
      continue;
    }
    const base::DictValue& item = value.GetDict();
    if (item.FindBool("separator").value_or(false)) {
      items.Append("-");
      continue;
    }
    if (const std::string* list = item.FindString("list")) {
      hooks::Args args;
      args.Str(*list);
      PBValue result;
      if (hooks::Ask(PB_EVENT_MENU_LIST, args, &result)) {
        std::optional<base::Value> listed = base::JSONReader::Read(hooks::TakeString(result), 0);
        if (listed && listed->is_list()) {
          for (base::Value& one : Items(listed->GetList())) {
            items.Append(std::move(one));
          }
        }
      }
      continue;
    }
    const std::string* title = item.FindString("title");
    const base::DictValue* key = item.FindDict("key");
    base::DictValue d;
    d.Set("title", title ? *title : "");
    d.Set("key", key ? KeyText(*key) : "");
    d.Set("enabled", item.FindBool("enabled").value_or(false));
    d.Set("checked", item.FindBool("checked").value_or(false));
    if (const base::ListValue* sub = item.FindList("items")) {
      d.Set("items", Items(*sub));
    }
    items.Append(std::move(d));
  }
  return items;
}

}  // namespace

// Go's menu, in the Mac's driver's terms; Linux has no menu bar of its own.
std::string MenuJson() {
  std::optional<base::Value> bar = base::JSONReader::Read(Menu(), 0);
  if (!bar || !bar->is_list()) {
    return "[]";
  }
  return base::WriteJson(Items(bar->GetList())).value_or("[]");
}

// Finds the item by its titles in Go's menu and tells Go it was chosen, as a click on the
// Mac's menu bar does.
bool ChooseMenu(const std::string& menu, const std::string& path) {
  std::optional<base::Value> bar = base::JSONReader::Read(Menu(), 0);
  if (!bar || !bar->is_list()) {
    return false;
  }
  const base::DictValue* item = Titled(&bar->GetList(), menu);
  for (const std::string& title :
       base::SplitStringUsingSubstr(path, " > ", base::TRIM_WHITESPACE, base::SPLIT_WANT_ALL)) {
    item = item ? Titled(item->FindList("items"), title) : nullptr;
  }
  const std::string* action = item ? item->FindString("action") : nullptr;
  if (!action || !item->FindBool("enabled").value_or(false)) {
    return false;
  }
  hooks::Args args;
  args.Str(*action);
  hooks::Fire(PB_EVENT_MENU_CHOSEN, args);
  return true;
}

std::string DockDot() {
  return "none";
}

}  // namespace project_b::driver
