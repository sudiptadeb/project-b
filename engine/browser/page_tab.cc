#include "project_b/engine/browser/page_tab.h"

#include <cstdlib>
#include <string>

#include "base/base64.h"
#include "base/functional/bind.h"
#include "base/process/kill.h"
#include "base/strings/utf_string_conversions.h"
#include "components/input/native_web_keyboard_event.h"
#include "content/public/browser/keyboard_event_processing_result.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/navigation_entry.h"
#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/web_contents.h"
#include "net/base/net_errors.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/go.h"
#include "project_b/engine/browser/window.h"
#include "third_party/blink/public/common/input/web_input_event.h"
#include "third_party/blink/public/mojom/favicon/favicon_url.mojom.h"
#include "ui/base/window_open_disposition.h"
#include "ui/events/keycodes/keyboard_codes.h"
#include "ui/gfx/codec/png_codec.h"

namespace project_b {

PageTab::PageTab(int64_t id, std::unique_ptr<content::WebContents> contents)
    : content::WebContentsObserver(contents.get()),
      id_(id),
      contents_(std::move(contents)) {
  contents_->SetDelegate(this);
}

PageTab::~PageTab() {
  // Tearing down the WebContents finishes its pending navigations, which would
  // otherwise arrive here mid-destruction.
  contents_->SetDelegate(nullptr);
  Observe(nullptr);
  contents_.reset();
}

void PageTab::DidFinishNavigation(content::NavigationHandle* handle) {
  if (!handle->IsInPrimaryMainFrame()) {
    return;
  }
  content::NavigationController& controller = contents_->GetController();
  Go().history_changed(id_, controller.CanGoBack(), controller.CanGoForward());

  const std::string& url = handle->GetURL().spec();
  if (handle->HasCommitted() && !handle->IsErrorPage()) {
    Go().navigation_finished(id_, url.c_str());
    // A page coming back from history already has its title and icon, and may not
    // announce them again.
    content::NavigationEntry* entry = controller.GetLastCommittedEntry();
    if (entry && !entry->GetTitle().empty()) {
      TitleWasSet(entry);
    }
    if (!contents_->GetFaviconURLs().empty()) {
      FetchFavicon(contents_->GetFaviconURLs());
    }
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
  if (Engine::Get()->active_tab() == this) {
    Engine::Get()->window()->SetTitle(title);
  }
  Go().title_changed(id_, base::UTF16ToUTF8(title).c_str());
}

void PageTab::PrimaryMainFrameRenderProcessGone(
    base::TerminationStatus status) {
  Go().page_crashed(id_);
}

void PageTab::DidUpdateFaviconURL(
    content::RenderFrameHost* render_frame_host,
    const std::vector<blink::mojom::FaviconURLPtr>& candidates,
    blink::mojom::FaviconUpdateReason reason) {
  FetchFavicon(candidates);
}

// The page's plain favicon, preferring one declared at a small size. Touch icons are
// for home screens and much bigger than a tab row wants.
void PageTab::FetchFavicon(
    const std::vector<blink::mojom::FaviconURLPtr>& candidates) {
  const blink::mojom::FaviconURL* best = nullptr;
  for (const auto& candidate : candidates) {
    if (candidate->icon_type != blink::mojom::FaviconIconType::kFavicon ||
        !candidate->icon_url.is_valid()) {
      continue;
    }
    bool small = false;
    for (const gfx::Size& size : candidate->icon_sizes) {
      small = small || size.width() <= 32;
    }
    if (!best || small) {
      best = candidate.get();
    }
  }
  const int request = ++favicon_request_;
  if (!best) {
    Go().favicon_changed(id_, "");
    return;
  }
  contents_->DownloadImage(
      best->icon_url, /*is_favicon=*/true, gfx::Size(32, 32),
      /*max_bitmap_size=*/128, /*bypass_cache=*/false,
      base::BindOnce(&PageTab::OnFaviconDownloaded, weak_factory_.GetWeakPtr(),
                     request));
}

void PageTab::OnFaviconDownloaded(int request,
                                  int id,
                                  int http_status_code,
                                  const GURL& image_url,
                                  const std::vector<SkBitmap>& bitmaps,
                                  const std::vector<gfx::Size>& sizes) {
  if (request != favicon_request_) {
    return;
  }
  // The one nearest 32px, which the row shows at 16px on a 2x screen.
  const SkBitmap* best = nullptr;
  for (const SkBitmap& bitmap : bitmaps) {
    if (bitmap.empty()) {
      continue;
    }
    if (!best || std::abs(bitmap.width() - 32) < std::abs(best->width() - 32)) {
      best = &bitmap;
    }
  }
  std::optional<std::vector<uint8_t>> png;
  if (best) {
    png = gfx::PNGCodec::EncodeBGRASkBitmap(*best, /*discard_transparency=*/false);
  }
  if (!png) {
    Go().favicon_changed(id_, "");
    return;
  }
  const std::string data_url = "data:image/png;base64," + base::Base64Encode(*png);
  Go().favicon_changed(id_, data_url.c_str());
}

content::WebContents* PageTab::OpenURLFromTab(
    content::WebContents* source,
    const content::OpenURLParams& params,
    base::OnceCallback<void(content::NavigationHandle&)> callback) {
  content::WebContents* target = source;
  if (params.disposition != WindowOpenDisposition::CURRENT_TAB) {
    // A link asked for another tab. Go hears about the tab before its first
    // navigation, so the gate finds it.
    PageTab* tab = Engine::Get()->AdoptTab(content::WebContents::Create(
        content::WebContents::CreateParams(source->GetBrowserContext())));
    Go().tab_opened(id_, tab->id(), params.url.spec().c_str(),
                    params.disposition != WindowOpenDisposition::NEW_BACKGROUND_TAB);
    target = tab->contents();
  }
  base::WeakPtr<content::NavigationHandle> handle =
      target->GetController().LoadURLWithParams(
          content::NavigationController::LoadURLParams(params));
  if (handle && callback) {
    std::move(callback).Run(*handle);
  }
  return target;
}

// target=_blank and window.open arrive here with a WebContents content already made.
// Content loads it after this returns.
content::WebContents* PageTab::AddNewContents(
    content::WebContents* source,
    std::unique_ptr<content::WebContents> new_contents,
    const GURL& target_url,
    WindowOpenDisposition disposition,
    const blink::mojom::WindowFeatures& window_features,
    bool user_gesture,
    bool* was_blocked) {
  PageTab* tab = Engine::Get()->AdoptTab(std::move(new_contents));
  Go().tab_opened(id_, tab->id(), target_url.spec().c_str(),
                  disposition != WindowOpenDisposition::NEW_BACKGROUND_TAB);
  return tab->contents();
}

content::KeyboardEventProcessingResult PageTab::PreHandleKeyboardEvent(
    content::WebContents* source,
    const input::NativeWebKeyboardEvent& event) {
  if (PBShortcut shortcut = Engine::ShortcutFor(event)) {
    Go().shortcut(shortcut);
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
