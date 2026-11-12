#include "project_b/engine/browser/engine.h"

#include <algorithm>
#include <string>
#include <utility>
#include <vector>

#include "base/check.h"
#include "base/functional/bind.h"
#include "base/location.h"
#include "base/task/single_thread_task_runner.h"
#include "build/build_config.h"
#include "content/public/browser/browser_task_traits.h"
#include "content/public/browser/browser_thread.h"
#include "content/public/browser/host_zoom_map.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/reload_type.h"
#include "content/public/browser/render_widget_host_view.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/browser/settings_page.h"
#include "project_b/engine/buildflags/buildflags.h"
#if !BUILDFLAG(IS_ANDROID)
#include "project_b/engine/browser/mac_app.h"
#endif
#include "project_b/engine/browser/shell_page.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser/window.h"
#include "third_party/blink/public/common/page/page_zoom.h"
#include "ui/gfx/geometry/rect_f.h"
#include "ui/gfx/geometry/size_f.h"
#include "url/gurl.h"

#if BUILDFLAG(PB_EXTENSIONS)
#include "extensions/browser/extensions_browser_client.h"
#include "project_b/engine/extensions/browser/extensions.h"
#endif

namespace project_b {

namespace {

Engine* g_engine = nullptr;

// Every call from Go except post_wakeup must already be on the main thread.
void CheckMainThread() {
  CHECK(content::BrowserThread::CurrentlyOn(content::BrowserThread::UI));
}

int64_t CallCreateTab() {
  CheckMainThread();
  return g_engine->CreateTab();
}

void CallCloseTab(int64_t tab) {
  CheckMainThread();
  g_engine->CloseTab(tab);
}

void CallLoadURL(int64_t tab, const char* url) {
  CheckMainThread();
  g_engine->LoadURL(tab, url);
}

void CallGoBack(int64_t tab) {
  CheckMainThread();
  g_engine->GoBack(tab);
}

void CallGoForward(int64_t tab) {
  CheckMainThread();
  g_engine->GoForward(tab);
}

void CallReload(int64_t tab) {
  CheckMainThread();
  g_engine->Reload(tab);
}

void CallReloadBypassingCache(int64_t tab) {
  CheckMainThread();
  g_engine->ReloadBypassingCache(tab);
}

void CallStop(int64_t tab) {
  CheckMainThread();
  g_engine->Stop(tab);
}

void CallSetZoom(int64_t tab, double factor) {
  CheckMainThread();
  g_engine->SetZoom(tab, factor);
}

double CallGetZoom(int64_t tab) {
  CheckMainThread();
  return g_engine->GetZoom(tab);
}

int64_t CallDuplicateTab(int64_t tab) {
  CheckMainThread();
  return g_engine->DuplicateTab(tab);
}

void CallEdit(int32_t command) {
  CheckMainThread();
  g_engine->Edit(command);
}

void CallOpenSettings(int64_t window) {
  CheckMainThread();
  g_engine->OpenSettings(window);
}

void CallCloseSettings(int64_t window) {
  CheckMainThread();
  g_engine->CloseSettings(window);
}

void CallSendToSettings(int64_t window, const char* json, size_t length) {
  CheckMainThread();
  g_engine->SendToSettings(window, json, length);
}

void CallSetMenu(const char* json, size_t length) {
  CheckMainThread();
#if !BUILDFLAG(IS_ANDROID)  // A phone has no menu bar.
  SetMainMenu(std::string(json, length));
#endif
}

void CallAppCommand(int32_t command) {
  CheckMainThread();
#if !BUILDFLAG(IS_ANDROID)
  RunAppCommand(command);
#endif
}

// Go only names windows it opened and the engine hasn't closed.
Window* NativeWindow(int64_t window) {
  Engine::AppWindow* w = g_engine->window(window);
  CHECK(w) << "no window " << window;
  return w->window.get();
}

void CallRequestWindow(int64_t window, int32_t request) {
  CheckMainThread();
  switch (request) {
    case PB_WINDOW_CLOSE:
      g_engine->CloseWindow(window);
      break;
    case PB_WINDOW_MINIMISE:
      NativeWindow(window)->Minimize();
      break;
    case PB_WINDOW_ZOOM:
      NativeWindow(window)->Zoom();
      break;
    case PB_WINDOW_FULL_SCREEN:
      NativeWindow(window)->ToggleFullScreen();
      break;
    case PB_WINDOW_FRONT:
      NativeWindow(window)->BringToFront();
      break;
  }
}

void CallSetWindowLayout(int64_t window,
                         const PBWindowLayout* l,
                         const PBRegion* regions,
                         size_t region_count,
                         const PBVisibleTab* tabs,
                         size_t tab_count) {
  CheckMainThread();
  Window::Layout layout = {
      .viewport = gfx::SizeF(l->view_w, l->view_h),
      .frame_rgb = static_cast<uint32_t>(l->frame_rgb),
      .frame_width = l->frame_width,
      .theme = l->theme,
  };
  layout.regions.reserve(region_count);
  for (size_t i = 0; i < region_count; i++) {
    const PBRegion& r = UNSAFE_BUFFERS(regions[i]);
    layout.regions.push_back({gfx::RectF(r.x, r.y, r.w, r.h),
                              static_cast<Window::Area>(r.area)});
  }
  if (!NativeWindow(window)->SetLayout(std::move(layout))) {
    return;
  }
  std::vector<Engine::VisibleTab> visible;
  visible.reserve(tab_count);
  for (size_t i = 0; i < tab_count; i++) {
    const PBVisibleTab& t = UNSAFE_BUFFERS(tabs[i]);
    visible.push_back({t.tab, gfx::RectF(t.x, t.y, t.w, t.h)});
  }
  g_engine->ShowTabs(g_engine->window(window), std::move(visible));
}

void CallFocusTab(int64_t tab) {
  CheckMainThread();
  g_engine->FocusTab(tab);
}

void CallFocusShell(int64_t window) {
  CheckMainThread();
  g_engine->FocusShell(window);
}

void CallSendToShell(int64_t window, const char* json, size_t length) {
  CheckMainThread();
  g_engine->SendToShell(window, json, length);
}

void CallSetHookSubscribed(int32_t hook, int on) {
  CheckMainThread();
  hooks::SetSubscribed(hook, on != 0);
}

#if BUILDFLAG(IS_ANDROID)
void CallInstallUpdate(const char* path) {
  CheckMainThread();
  // Android has one window, and the installer belongs to the app, not to it.
  g_engine->windows().begin()->second->window->InstallPackage(path);
}
#endif

// The one call that any thread may make.
void CallPostWakeup() {
  content::GetUIThreadTaskRunner({})->PostTask(FROM_HERE,
                                               base::BindOnce(Go().run_posted));
}

}  // namespace

Engine::AppWindow::AppWindow(int64_t id) : id(id) {}
Engine::AppWindow::~AppWindow() = default;

Engine::Engine(content::BrowserContext* browser_context)
    : browser_context_(browser_context) {
  CHECK(!g_engine);
  g_engine = this;
}

Engine::~Engine() {
  g_engine = nullptr;
}

// static
Engine* Engine::Get() {
  return g_engine;
}

// static
void Engine::TabOpened(int64_t opener, int64_t tab, const GURL& url, bool foreground) {
  hooks::Args args;
  args.Int(opener);
  args.Int(tab);
  args.Str(url.spec());
  args.Bool(foreground);
  hooks::Fire(PB_EVENT_TAB_OPENED, args);
}

// static
const PBEngineCalls* Engine::Calls() {
  static const PBEngineCalls calls = {
      .create_tab = CallCreateTab,
      .close_tab = CallCloseTab,
      .load_url = CallLoadURL,
      .go_back = CallGoBack,
      .go_forward = CallGoForward,
      .reload = CallReload,
      .reload_bypassing_cache = CallReloadBypassingCache,
      .stop = CallStop,
      .set_zoom = CallSetZoom,
      .get_zoom = CallGetZoom,
      .duplicate_tab = CallDuplicateTab,
      .edit = CallEdit,
      .request_window = CallRequestWindow,
      .set_window_layout = CallSetWindowLayout,
      .focus_tab = CallFocusTab,
      .focus_shell = CallFocusShell,
      .send_to_shell = CallSendToShell,
      .open_settings = CallOpenSettings,
      .close_settings = CallCloseSettings,
      .send_to_settings = CallSendToSettings,
      .set_menu = CallSetMenu,
      .app_command = CallAppCommand,
      .set_hook_subscribed = CallSetHookSubscribed,
      .post_wakeup = CallPostWakeup,
#if BUILDFLAG(IS_ANDROID)
      .install_update = CallInstallUpdate,
#endif
#if BUILDFLAG(PB_EXTENSIONS)
      .extensions = Extensions::Calls,
#endif
  };
  return &calls;
}

PageTab* Engine::tab(int64_t id) const {
  auto it = tabs_.find(id);
  return it == tabs_.end() ? nullptr : it->second.get();
}

Engine::AppWindow* Engine::window(int64_t id) const {
  auto it = windows_.find(id);
  return it == windows_.end() ? nullptr : it->second.get();
}

Engine::AppWindow* Engine::WindowOf(int64_t tab) const {
  PageTab* t = this->tab(tab);
  return t ? window(t->window()) : nullptr;
}

PageTab* Engine::FocusedTab(const AppWindow* window) const {
  return tab(window->focused_tab);
}

namespace {

bool Lists(const std::vector<Engine::VisibleTab>& tabs, int64_t tab) {
  return std::ranges::any_of(
      tabs, [tab](const Engine::VisibleTab& v) { return v.tab == tab; });
}

}  // namespace

// A tab that closed after Go sent the layout is left out. A tab another window shows now
// has moved there, and is shown there.
void Engine::ShowTabs(AppWindow* window, std::vector<VisibleTab> tabs) {
  std::erase_if(tabs, [this](const VisibleTab& v) { return !tab(v.tab); });
  std::vector<Window::Page> pages;
  for (const VisibleTab& v : tabs) {
    pages.push_back({tab(v.tab)->contents(), v.rect});
  }
  window->window->SetPages(pages);
  for (const VisibleTab& v : window->visible) {
    PageTab* t = tab(v.tab);
    if (t && !Lists(tabs, v.tab) && t->window() == window->id) {
      t->contents()->WasHidden();
    }
  }
  for (const VisibleTab& v : tabs) {
    if (!Lists(window->visible, v.tab)) {
      tab(v.tab)->set_window(window->id);
      tab(v.tab)->contents()->WasShown();
    }
  }
  window->visible = std::move(tabs);
  if (!Lists(window->visible, window->focused_tab)) {
    window->focused_tab =
        window->visible.empty() ? 0 : window->visible.front().tab;
  }
  UpdateTitle(window);
}

void Engine::TabFocused(PageTab* tab) {
  if (AppWindow* w = WindowOf(tab->id())) {
    w->focused_tab = tab->id();
    UpdateTitle(w);
  }
}

void Engine::TabTitleChanged(PageTab* tab) {
  if (AppWindow* w = WindowOf(tab->id()); w && w->focused_tab == tab->id()) {
    UpdateTitle(w);
  }
}

void Engine::UpdateTitle(AppWindow* window) {
  if (PageTab* t = FocusedTab(window)) {
    window->window->SetTitle(t->contents()->GetTitle());
  }
}

void Engine::OpenWindow(int64_t id, base::OnceClosure on_close) {
  auto w = std::make_unique<AppWindow>(id);
  w->window = std::make_unique<Window>(std::move(on_close));
  timeline::Mark("window.created");
  w->shell = std::make_unique<ShellPage>(browser_context_, id);
  timeline::Mark("shell.created");
#if BUILDFLAG(IS_ANDROID)
  w->window->SetShellContents(w->shell->contents());
#else
  w->window->SetShellView(w->shell->contents()->GetNativeView());
#endif
  ShellPage* shell = w->shell.get();
  windows_[id] = std::move(w);
  shell->Load();
}

void Engine::ShowWindow(int64_t id) {
  Window* native = window(id)->window.get();
  native->Show();
  timeline::Mark("window.shown");
  native->ListenForState(base::BindRepeating(
      [](int64_t id, const Window::State& s) {
        hooks::Args args;
        args.Int(id);
        args.Double(s.size.width());
        args.Double(s.size.height());
        args.Double(s.controls.x());
        args.Double(s.controls.y());
        args.Double(s.controls.width());
        args.Double(s.controls.height());
        args.Double(s.corner_radius);
        args.Bool(s.full_screen);
        hooks::Fire(PB_EVENT_WINDOW_CHANGED, args);
      },
      id));
}

void Engine::FocusShellForTraversal(int64_t from_tab, bool reverse) {
  content::WebContents* shell = WindowOf(from_tab)->shell->contents();
  shell->Focus();
  shell->FocusThroughTabTraversal(reverse);
}

void Engine::FocusPageForTraversal(int64_t window, bool reverse) {
  if (PageTab* tab = FocusedTab(this->window(window))) {
    tab->contents()->Focus();
    tab->contents()->FocusThroughTabTraversal(reverse);
  }
}

int64_t Engine::CreateTab() {
  content::WebContents::CreateParams params(browser_context_);
  params.initially_hidden = true;
  return Add(content::WebContents::Create(params))->id();
}

PageTab* Engine::AdoptTab(std::unique_ptr<content::WebContents> contents) {
  return Add(std::move(contents));
}

// A new tab starts hidden, whoever made it. A window's layout is the only way to the
// screen.
PageTab* Engine::Add(std::unique_ptr<content::WebContents> contents) {
  const int64_t id = next_tab_++;
  auto tab = std::make_unique<PageTab>(id, std::move(contents));
  PageTab* raw = tab.get();
  tab->contents()->WasHidden();
#if BUILDFLAG(PB_EXTENSIONS)
  // Extensions run in every tab: content scripts, messages, chrome.scripting.
  if (Extensions::Get()) {
    extensions::ExtensionsBrowserClient::Get()
        ->CreateExtensionWebContentsObserver(tab->contents());
  }
#endif
#if BUILDFLAG(IS_ANDROID)
  // Android has one window, and every tab's view is in it from the start.
  windows_.begin()->second->window->AddPageContents(raw->contents());
#endif
  tabs_[id] = std::move(tab);
  return raw;
}

// Go may ask for this from inside one of the tab's own events (Cmd+W arrives through
// the tab's keyboard handling), so the tab leaves the map now and dies on a later task.
void Engine::CloseTab(int64_t tab) {
  auto it = tabs_.find(tab);
  if (it == tabs_.end()) {
    return;
  }
#if BUILDFLAG(IS_ANDROID)
  windows_.begin()->second->window->RemovePageContents(it->second->contents());
#endif
  if (AppWindow* w = WindowOf(tab)) {
#if !BUILDFLAG(IS_ANDROID)
    w->window->RemovePageView(it->second->contents()->GetNativeView());
#endif
    std::erase_if(w->visible, [tab](const VisibleTab& v) { return v.tab == tab; });
    if (w->focused_tab == tab) {
      w->focused_tab = w->visible.empty() ? 0 : w->visible.front().tab;
    }
  }
  content::GetUIThreadTaskRunner({})->DeleteSoon(FROM_HERE,
                                                 std::move(it->second));
  tabs_.erase(it);
}

void Engine::LoadURL(int64_t tab, const char* url) {
  auto it = tabs_.find(tab);
  if (it == tabs_.end()) {
    return;
  }
  content::NavigationController::LoadURLParams params((GURL(url)));
  params.transition_type = ui::PageTransitionFromInt(
      ui::PAGE_TRANSITION_TYPED | ui::PAGE_TRANSITION_FROM_ADDRESS_BAR);
  it->second->contents()->GetController().LoadURLWithParams(params);
}

void Engine::GoBack(int64_t tab) {
  auto it = tabs_.find(tab);
  if (it != tabs_.end() && it->second->contents()->GetController().CanGoBack()) {
    it->second->contents()->GetController().GoBack();
  }
}

void Engine::GoForward(int64_t tab) {
  auto it = tabs_.find(tab);
  if (it != tabs_.end() &&
      it->second->contents()->GetController().CanGoForward()) {
    it->second->contents()->GetController().GoForward();
  }
}

void Engine::Reload(int64_t tab) {
  auto it = tabs_.find(tab);
  if (it != tabs_.end()) {
    it->second->contents()->GetController().Reload(content::ReloadType::NORMAL,
                                                   /*check_for_repost=*/false);
  }
}

content::WebContents* Engine::Contents(int64_t tab) const {
  auto it = tabs_.find(tab);
  return it == tabs_.end() ? nullptr : it->second->contents();
}

void Engine::ReloadBypassingCache(int64_t tab) {
  if (content::WebContents* contents = Contents(tab)) {
    contents->GetController().Reload(content::ReloadType::BYPASSING_CACHE,
                                     /*check_for_repost=*/false);
  }
}

void Engine::Stop(int64_t tab) {
  if (content::WebContents* contents = Contents(tab)) {
    contents->Stop();
  }
}

void Engine::SetZoom(int64_t tab, double factor) {
  if (content::WebContents* contents = Contents(tab)) {
    content::HostZoomMap::SetZoomLevel(contents,
                                       blink::ZoomFactorToZoomLevel(factor));
  }
}

double Engine::GetZoom(int64_t tab) {
  content::WebContents* contents = Contents(tab);
  return contents ? blink::ZoomLevelToZoomFactor(
                        content::HostZoomMap::GetZoomLevel(contents))
                  : 1;
}

// A clone has its history but no page until it is told to load.
int64_t Engine::DuplicateTab(int64_t tab) {
  content::WebContents* contents = Contents(tab);
  if (!contents) {
    return 0;
  }
  PageTab* copy = Add(contents->Clone());
  copy->contents()->GetController().LoadIfNecessary();
  return copy->id();
}

content::WebContents* Engine::FocusedContents() const {
  for (const auto& [id, w] : windows_) {
    std::vector<content::WebContents*> pages = {
        w->settings ? w->settings->contents() : nullptr, w->shell->contents()};
    for (const VisibleTab& v : w->visible) {
      pages.push_back(tab(v.tab)->contents());
    }
    for (content::WebContents* contents : pages) {
      if (contents && contents->GetRenderWidgetHostView() &&
          contents->GetRenderWidgetHostView()->HasFocus()) {
        return contents;
      }
    }
  }
  return nullptr;
}

void Engine::Edit(int32_t command) {
  content::WebContents* contents = FocusedContents();
  if (!contents) {
    return;
  }
  switch (command) {
    case PB_EDIT_UNDO:
      contents->Undo();
      break;
    case PB_EDIT_REDO:
      contents->Redo();
      break;
    case PB_EDIT_CUT:
      contents->Cut();
      break;
    case PB_EDIT_COPY:
      contents->Copy();
      break;
    case PB_EDIT_PASTE:
      contents->Paste();
      break;
    case PB_EDIT_PASTE_AND_MATCH_STYLE:
      contents->PasteAndMatchStyle();
      break;
    case PB_EDIT_DELETE:
      contents->Delete();
      break;
    case PB_EDIT_SELECT_ALL:
      contents->SelectAll();
      break;
  }
}

void Engine::OpenSettings(int64_t window) {
  AppWindow* w = this->window(window);
  if (w->settings) {
    return;
  }
  w->settings = std::make_unique<SettingsPage>(browser_context_);
  timeline::Mark("settings.open.created");
#if BUILDFLAG(IS_ANDROID)
  w->window->SetOverlayContents(w->settings->contents());
#else
  w->window->SetOverlayView(w->settings->contents()->GetNativeView());
#endif
  timeline::Mark("settings.open.shown");
  w->settings->Load();
  w->settings->contents()->Focus();
}

// Like CloseTab, the request can come from the page's own key event, so the page dies
// on a later task.
void Engine::CloseSettings(int64_t window) {
  AppWindow* w = this->window(window);
  if (!w->settings) {
    return;
  }
#if BUILDFLAG(IS_ANDROID)
  w->window->RemoveOverlayContents(w->settings->contents());
#else
  w->window->RemoveOverlayView(w->settings->contents()->GetNativeView());
#endif
  content::GetUIThreadTaskRunner({})->DeleteSoon(FROM_HERE,
                                                 std::move(w->settings));
  if (PageTab* tab = FocusedTab(w)) {
    tab->contents()->Focus();
  }
}

void Engine::SendToSettings(int64_t window, const char* json, size_t length) {
  AppWindow* w = this->window(window);
  if (w->settings) {
    w->settings->Send(std::string(json, length));
  }
}

// Same reason as CloseTab: the request can arrive inside a view's own event.
void Engine::CloseWindow(int64_t window) {
  content::GetUIThreadTaskRunner({})->PostTask(
      FROM_HERE, base::BindOnce(&Window::Close,
                                base::Unretained(this->window(window)->window.get())));
}

// While a window's settings page is open it keeps the keyboard: a focus request from
// the shell or Go, such as one still on its way from before it opened, waits until it
// closes.
void Engine::FocusTab(int64_t tab) {
  PageTab* t = this->tab(tab);
  AppWindow* w = WindowOf(tab);
  if (t && w && !w->settings) {
    t->contents()->Focus();
  }
}

void Engine::FocusShell(int64_t window) {
  AppWindow* w = this->window(window);
  if (w->settings) {
    return;
  }
  w->shell->contents()->Focus();
}

void Engine::SendToShell(int64_t window, const char* json, size_t length) {
  this->window(window)->shell->Send(std::string(json, length));
}

}  // namespace project_b
