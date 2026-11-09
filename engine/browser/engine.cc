#include "project_b/engine/browser/engine.h"

#include <string>
#include <utility>
#include <vector>

#include "base/check.h"
#include "base/functional/bind.h"
#include "base/location.h"
#include "base/task/single_thread_task_runner.h"
#include "components/input/native_web_keyboard_event.h"
#include "content/public/browser/browser_task_traits.h"
#include "content/public/browser/browser_thread.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/abi/pb_exports.h"
#include "project_b/engine/browser/page_tab.h"
#include "project_b/engine/browser/shell_page.h"
#include "project_b/engine/browser/window.h"
#include "third_party/blink/public/common/input/web_input_event.h"
#include "ui/events/keycodes/keyboard_codes.h"
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

void CallLoadURL(int64_t tab, const char* url) {
  CheckMainThread();
  g_engine->LoadURL(tab, url);
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

void CallSetShellRegions(const double* rects, size_t count) {
  CheckMainThread();
  std::vector<gfx::RectF> regions;
  regions.reserve(count);
  for (size_t i = 0; i < count; i++) {
    const double* r = UNSAFE_BUFFERS(rects + i * 4);
    UNSAFE_BUFFERS(regions.emplace_back(r[0], r[1], r[2], r[3]));
  }
  g_engine->window()->SetShellRegions(std::move(regions));
}

void CallFocusPage() {
  CheckMainThread();
  g_engine->FocusPage();
}

void CallSendToShell(const char* json, size_t length) {
  CheckMainThread();
  g_engine->SendToShell(json, length);
}

// The one call that any thread may make.
void CallPostWakeup() {
  content::GetUIThreadTaskRunner({})->PostTask(FROM_HERE,
                                               base::BindOnce(&PBRunPosted));
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
bool Engine::IsFocusAddressBarKey(const input::NativeWebKeyboardEvent& event) {
  return event.GetType() == blink::WebInputEvent::Type::kRawKeyDown &&
         event.windows_key_code == ui::VKEY_L &&
         (event.GetModifiers() & blink::WebInputEvent::kMetaKey);
}

// static
const PBEngineCalls* Engine::Calls() {
  static const PBEngineCalls calls = {
      .create_tab = CallCreateTab,
      .load_url = CallLoadURL,
      .set_page_bounds = CallSetPageBounds,
      .set_shell_regions = CallSetShellRegions,
      .focus_page = CallFocusPage,
      .send_to_shell = CallSendToShell,
      .post_wakeup = CallPostWakeup,
  };
  return &calls;
}

PageTab* Engine::active_tab() const {
  auto it = tabs_.find(active_tab_);
  return it == tabs_.end() ? nullptr : it->second.get();
}

PageTab* Engine::TabFor(content::WebContents* contents) const {
  for (const auto& [id, tab] : tabs_) {
    if (tab->contents() == contents) {
      return tab.get();
    }
  }
  return nullptr;
}

void Engine::FocusAddressBar() {
  shell_->contents()->Focus();
  PBFocusAddressBarRequested();
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
  const int64_t id = next_tab_++;
  auto tab = std::make_unique<PageTab>(
      id, content::WebContents::Create(
              content::WebContents::CreateParams(browser_context_)));
  window_->SetPageView(tab->contents()->GetNativeView());
  tabs_[id] = std::move(tab);
  active_tab_ = id;
  return id;
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

void Engine::FocusPage() {
  if (PageTab* tab = active_tab()) {
    tab->contents()->Focus();
  }
}

void Engine::SendToShell(const char* json, size_t length) {
  shell_->Send(std::string(json, length));
}

}  // namespace project_b
