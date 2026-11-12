#include "project_b/engine/browser/page_tab.h"

#include <cstdlib>
#include <string>

#include "base/base64.h"
#include "base/functional/bind.h"
#include "base/strings/string_number_conversions.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/navigation_handle.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser/window.h"
#include "third_party/blink/public/mojom/favicon/favicon_url.mojom.h"
#include "ui/base/window_open_disposition.h"
#include "ui/gfx/codec/png_codec.h"

namespace project_b {

PageTab::PageTab(int64_t id, std::unique_ptr<content::WebContents> contents)
    : WebContentsObserverHooks(contents.get()),
      id_(id),
      contents_(std::move(contents)) {
  hooks::TagTab(contents_.get(), id_);
  contents_->SetDelegate(this);
}

PageTab::~PageTab() {
  // Tearing down the WebContents finishes its pending navigations, which would
  // otherwise arrive here mid-destruction.
  contents_->SetDelegate(nullptr);
  Observe(nullptr);
  contents_.reset();
}

// A page coming back from history already has its icon and may not announce it again.
void PageTab::DidFinishNavigation(content::NavigationHandle* handle) {
  if (handle->IsInPrimaryMainFrame() && handle->HasCommitted() &&
      !handle->IsSameDocument()) {
    timeline::Mark("tab.committed", "tab=" + base::NumberToString(id_));
  }
  if (handle->IsInPrimaryMainFrame() && handle->HasCommitted() &&
      !handle->IsErrorPage() && !contents_->GetFaviconURLs().empty()) {
    FetchFavicon(contents_->GetFaviconURLs());
  }
  WebContentsObserverHooks::DidFinishNavigation(handle);
}

void PageTab::DidFirstVisuallyNonEmptyPaint() {
  timeline::Mark("tab.first_paint", "tab=" + base::NumberToString(id_));
  WebContentsObserverHooks::DidFirstVisuallyNonEmptyPaint();
}

void PageTab::TitleWasSet(content::NavigationEntry* entry) {
  Engine::Get()->TabTitleChanged(this);
  WebContentsObserverHooks::TitleWasSet(entry);
}

void PageTab::OnWebContentsFocused(
    content::RenderWidgetHost* render_widget_host) {
  Engine::Get()->TabFocused(this);
  WebContentsObserverHooks::OnWebContentsFocused(render_widget_host);
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
    hooks::Args args;
    args.Int(id_);
    args.Str("");
    hooks::Fire(PB_EVENT_FAVICON_CHANGED, args);
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
  hooks::Args args;
  args.Int(id_);
  args.Str(png ? "data:image/png;base64," + base::Base64Encode(*png) : "");
  hooks::Fire(PB_EVENT_FAVICON_CHANGED, args);
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
    Engine::TabOpened(id_, tab->id(), params.url,
                      params.disposition != WindowOpenDisposition::NEW_BACKGROUND_TAB,
                      /*new_window=*/false);
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
  Engine::TabOpened(id_, tab->id(), target_url,
                    disposition != WindowOpenDisposition::NEW_BACKGROUND_TAB,
                    /*new_window=*/false);
  return tab->contents();
}

// Go gets the key first; one it doesn't use goes to the menu bar.
bool PageTab::HandleKeyboardEvent(content::WebContents* source,
                                  const input::NativeWebKeyboardEvent& event) {
  return WebContentsDelegateHooks::HandleKeyboardEvent(source, event) ||
         Window::HandleUnhandledKey(event);
}

bool PageTab::TakeFocus(content::WebContents* source, bool reverse) {
  Engine::Get()->FocusShellForTraversal(id_, reverse);
  return true;
}

content::WebContents* PageTab::hook_contents() const {
  return contents_.get();
}

}  // namespace project_b
