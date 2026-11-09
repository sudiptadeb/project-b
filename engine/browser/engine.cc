#include "project_b/engine/browser/engine.h"

#include <string>
#include <utility>
#include <vector>

#include "base/check.h"
#include "base/functional/bind.h"
#include "base/location.h"
#include "base/task/single_thread_task_runner.h"
#include "content/public/browser/browser_task_traits.h"
#include "content/public/browser/browser_thread.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/reload_type.h"
#include "content/public/browser/render_widget_host_view.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/browser/shell_page.h"
#include "project_b/engine/browser/window.h"
#include "ui/gfx/geometry/rect_f.h"
#include "ui/gfx/geometry/size_f.h"
#include "url/gurl.h"

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

void CallShowTab(int64_t tab) {
  CheckMainThread();
  g_engine->ShowTab(tab);
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

void CallCloseWindow() {
  CheckMainThread();
  g_engine->CloseWindow();
}

void CallSetPageBounds(double x,
                       double y,
                       double w,
                       double h,
                       double view_w,
                       double view_h) {
  CheckMainThread();
  g_engine->window()->SetPageBounds(gfx::RectF(x, y, w, h),
                                    gfx::SizeF(view_w, view_h));
}

std::vector<gfx::RectF> ToRects(const double* rects, size_t count) {
  std::vector<gfx::RectF> out;
  out.reserve(count);
  for (size_t i = 0; i < count; i++) {
    const double* r = UNSAFE_BUFFERS(rects + i * 4);
    UNSAFE_BUFFERS(out.emplace_back(r[0], r[1], r[2], r[3]));
  }
  return out;
}

void CallSetShellRegions(const double* rects, size_t count) {
  CheckMainThread();
  g_engine->window()->SetShellRegions(ToRects(rects, count));
}

void CallSetWindowFrame(int32_t rgb, double width) {
  CheckMainThread();
  g_engine->window()->SetFrame(static_cast<uint32_t>(rgb), width);
}

void CallSetDragRegions(const double* rects, size_t count) {
  CheckMainThread();
  g_engine->window()->SetDragRegions(ToRects(rects, count));
}

void CallFocusPage() {
  CheckMainThread();
  g_engine->FocusPage();
}

void CallFocusShell() {
  CheckMainThread();
  g_engine->FocusShell();
}

void CallSendToShell(const char* json, size_t length) {
  CheckMainThread();
  g_engine->SendToShell(json, length);
}

void CallSendToTab(int64_t tab, const char* json, size_t length) {
  CheckMainThread();
  g_engine->SendToTab(tab, json, length);
}

void CallSetHookSubscribed(int32_t hook, int on) {
  CheckMainThread();
  hooks::SetSubscribed(hook, on != 0);
}

// The one call that any thread may make.
void CallPostWakeup() {
  content::GetUIThreadTaskRunner({})->PostTask(FROM_HERE,
                                               base::BindOnce(Go().run_posted));
}

}  // namespace

Engine::Engine(content::BrowserContext* browser_context,
               Window* window,
               ShellPage* shell)
    : browser_context_(browser_context), window_(window), shell_(shell) {
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
      .show_tab = CallShowTab,
      .load_url = CallLoadURL,
      .go_back = CallGoBack,
      .go_forward = CallGoForward,
      .reload = CallReload,
      .close_window = CallCloseWindow,
      .set_page_bounds = CallSetPageBounds,
      .set_shell_regions = CallSetShellRegions,
      .set_drag_regions = CallSetDragRegions,
      .set_window_frame = CallSetWindowFrame,
      .focus_page = CallFocusPage,
      .focus_shell = CallFocusShell,
      .send_to_shell = CallSendToShell,
      .send_to_tab = CallSendToTab,
      .set_hook_subscribed = CallSetHookSubscribed,
      .post_wakeup = CallPostWakeup,
  };
  return &calls;
}

PageTab* Engine::active_tab() const {
  auto it = tabs_.find(active_tab_);
  return it == tabs_.end() ? nullptr : it->second.get();
}

std::vector<content::RenderWidgetHost*> Engine::DrawnWidgets() const {
  std::vector<content::RenderWidgetHost*> widgets;
  PageTab* tab = active_tab();
  for (content::WebContents* contents :
       {shell_->contents(), tab ? tab->contents() : nullptr}) {
    content::RenderWidgetHostView* view =
        contents ? contents->GetRenderWidgetHostView() : nullptr;
    if (view) {
      widgets.push_back(view->GetRenderWidgetHost());
    }
  }
  return widgets;
}

void Engine::FocusShellForTraversal(bool reverse) {
  shell_->contents()->Focus();
  shell_->contents()->FocusThroughTabTraversal(reverse);
}

void Engine::FocusPageForTraversal(bool reverse) {
  if (PageTab* tab = active_tab()) {
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

// A new tab starts hidden, whoever made it. ShowTab is the only way to the screen.
PageTab* Engine::Add(std::unique_ptr<content::WebContents> contents) {
  const int64_t id = next_tab_++;
  auto tab = std::make_unique<PageTab>(id, std::move(contents));
  PageTab* raw = tab.get();
  tab->contents()->WasHidden();
  window_->AddPageView(tab->contents()->GetNativeView());
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
  window_->RemovePageView(it->second->contents()->GetNativeView());
  if (active_tab_ == tab) {
    active_tab_ = 0;
  }
  content::GetUIThreadTaskRunner({})->DeleteSoon(FROM_HERE,
                                                 std::move(it->second));
  tabs_.erase(it);
}

void Engine::ShowTab(int64_t tab) {
  auto it = tabs_.find(tab);
  if (it == tabs_.end()) {
    return;
  }
  active_tab_ = tab;
  window_->ShowPageView(it->second->contents()->GetNativeView());
  window_->SetTitle(it->second->contents()->GetTitle());
  it->second->contents()->WasShown();
  for (const auto& [id, other] : tabs_) {
    if (id != tab) {
      other->contents()->WasHidden();
    }
  }
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

// Same reason as CloseTab: the request can arrive inside a view's own event.
void Engine::CloseWindow() {
  content::GetUIThreadTaskRunner({})->PostTask(
      FROM_HERE, base::BindOnce(&Window::Close, base::Unretained(window_)));
}

void Engine::FocusPage() {
  if (PageTab* tab = active_tab()) {
    tab->contents()->Focus();
  }
}

void Engine::FocusShell() {
  shell_->contents()->Focus();
}

void Engine::SendToShell(const char* json, size_t length) {
  shell_->Send(std::string(json, length));
}

void Engine::SendToTab(int64_t tab, const char* json, size_t length) {
  auto it = tabs_.find(tab);
  if (it != tabs_.end()) {
    SendToOurPage(it->second->contents(), std::string(json, length));
  }
}

}  // namespace project_b
