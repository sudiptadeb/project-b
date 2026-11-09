#include "project_b/engine/browser/page_tab.h"

#include <string>

#include "base/process/kill.h"
#include "base/strings/utf_string_conversions.h"
#include "components/input/native_web_keyboard_event.h"
#include "content/public/browser/keyboard_event_processing_result.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/web_contents.h"
#include "net/base/net_errors.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser/window.h"
#include "third_party/blink/public/common/input/web_input_event.h"
#include "ui/events/keycodes/keyboard_codes.h"

namespace project_b {

PageTab::PageTab(int64_t id, std::unique_ptr<content::WebContents> contents)
    : content::WebContentsObserver(contents.get()),
      id_(id),
      contents_(std::move(contents)) {
  contents_->SetDelegate(this);
}

PageTab::~PageTab() {
  contents_->SetDelegate(nullptr);
}

void PageTab::DidFinishNavigation(content::NavigationHandle* handle) {
  if (!handle->IsInPrimaryMainFrame()) {
    return;
  }
  const std::string& url = handle->GetURL().spec();
  if (handle->HasCommitted() && !handle->IsErrorPage()) {
    Go().navigation_finished(id_, url.c_str());
    return;
  }
  // ERR_ABORTED is a navigation replaced by a newer one, which isn't a failure.
  if (handle->GetNetErrorCode() != net::OK &&
      handle->GetNetErrorCode() != net::ERR_ABORTED) {
    Go().navigation_failed(id_, url.c_str(),
                       net::ErrorToString(handle->GetNetErrorCode()).c_str());
  }
}

void PageTab::DidStartLoading() {
  Go().loading_changed(id_, 1);
}

void PageTab::DidStopLoading() {
  Go().loading_changed(id_, 0);
}

void PageTab::TitleWasSet(content::NavigationEntry* entry) {
  const std::u16string& title = contents_->GetTitle();
  Engine::Get()->window()->SetTitle(title);
  Go().title_changed(id_, base::UTF16ToUTF8(title).c_str());
}

void PageTab::PrimaryMainFrameRenderProcessGone(
    base::TerminationStatus status) {
  Go().page_crashed(id_);
}

content::WebContents* PageTab::OpenURLFromTab(
    content::WebContents* source,
    const content::OpenURLParams& params,
    base::OnceCallback<void(content::NavigationHandle&)> callback) {
  // One tab so far, so everything opens here.
  base::WeakPtr<content::NavigationHandle> handle =
      source->GetController().LoadURLWithParams(
          content::NavigationController::LoadURLParams(params));
  if (handle && callback) {
    std::move(callback).Run(*handle);
  }
  return source;
}

content::KeyboardEventProcessingResult PageTab::PreHandleKeyboardEvent(
    content::WebContents* source,
    const input::NativeWebKeyboardEvent& event) {
  if (Engine::IsFocusAddressBarKey(event)) {
    Engine::Get()->FocusAddressBar();
    return content::KeyboardEventProcessingResult::HANDLED;
  }
  return content::KeyboardEventProcessingResult::NOT_HANDLED;
}

bool PageTab::HandleKeyboardEvent(content::WebContents* source,
                                  const input::NativeWebKeyboardEvent& event) {
  return Engine::Get()->window()->HandleUnhandledKey(event);
}

bool PageTab::TakeFocus(content::WebContents* source, bool reverse) {
  Engine::Get()->FocusShellForTraversal(reverse);
  return true;
}

}  // namespace project_b
