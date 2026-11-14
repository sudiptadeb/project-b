#ifndef PROJECT_B_ENGINE_BROWSER_PAGE_TAB_H_
#define PROJECT_B_ENGINE_BROWSER_PAGE_TAB_H_

#include <cstdint>
#include <memory>
#include <string>
#include <vector>

#include "base/memory/weak_ptr.h"
#include "build/build_config.h"
#include "project_b/engine/browser/hooks_gen.h"
#include "third_party/skia/include/core/SkBitmap.h"
#include "ui/gfx/geometry/size.h"

namespace project_b {

// One tab's connection to content. The generated bases carry every hook to Go; what is
// overridden here needs content's objects first, and still calls the base after.
class PageTab : public WebContentsObserverHooks, public WebContentsDelegateHooks {
 public:
  PageTab(int64_t id,
          std::unique_ptr<content::WebContents> contents,
          const std::string& workspace);
  PageTab(const PageTab&) = delete;
  PageTab& operator=(const PageTab&) = delete;
  ~PageTab() override;

  int64_t id() const { return id_; }
  content::WebContents* contents() const { return contents_.get(); }
  // Go's id for the workspace the tab is in, "" for the profile's own storage. It never
  // changes.
  const std::string& workspace() const { return workspace_; }
  // Go's id for the window whose layout last showed this tab, 0 before any has.
  int64_t window() const { return window_; }
  void set_window(int64_t window) { window_ = window; }

 private:
  // content::WebContentsObserver:
  void DidFinishNavigation(content::NavigationHandle* handle) override;
  void DidFirstVisuallyNonEmptyPaint() override;
  void TitleWasSet(content::NavigationEntry* entry) override;
  void OnWebContentsFocused(
      content::RenderWidgetHost* render_widget_host) override;
  void DidUpdateFaviconURL(
      content::RenderFrameHost* render_frame_host,
      const std::vector<blink::mojom::FaviconURLPtr>& candidates,
      blink::mojom::FaviconUpdateReason reason) override;

  // Fetches the best of the page's icon candidates and hands it to Go as a PNG data
  // URL, or "" when there is none to show.
  void FetchFavicon(const std::vector<blink::mojom::FaviconURLPtr>& candidates);
  void OnFaviconDownloaded(int request,
                           int id,
                           int http_status_code,
                           const GURL& image_url,
                           const std::vector<SkBitmap>& bitmaps,
                           const std::vector<gfx::Size>& sizes);

  // content::WebContentsDelegate: these hand over content's objects, so they stay here.
  content::WebContents* hook_contents() const override;
  content::WebContents* OpenURLFromTab(
      content::WebContents* source,
      const content::OpenURLParams& params,
      base::OnceCallback<void(content::NavigationHandle&)> callback) override;
  content::WebContents* AddNewContents(
      content::WebContents* source,
      std::unique_ptr<content::WebContents> new_contents,
      const GURL& target_url,
      WindowOpenDisposition disposition,
      const blink::mojom::WindowFeatures& window_features,
      bool user_gesture,
      bool* was_blocked) override;
  bool HandleKeyboardEvent(content::WebContents* source,
                           const input::NativeWebKeyboardEvent& event) override;
  bool TakeFocus(content::WebContents* source, bool reverse) override;
#if BUILDFLAG(IS_MAC)
  content::JavaScriptDialogManager* GetJavaScriptDialogManager(
      content::WebContents* source) override;
  void RunFileChooser(content::RenderFrameHost* render_frame_host,
                      scoped_refptr<content::FileSelectListener> listener,
                      const blink::mojom::FileChooserParams& params) override;
#endif

  const int64_t id_;
  const std::string workspace_;
  int64_t window_ = 0;
  std::unique_ptr<content::WebContents> contents_;
  // Counts favicon fetches, so a slow one for an earlier page is dropped.
  int favicon_request_ = 0;
  base::WeakPtrFactory<PageTab> weak_factory_{this};
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_PAGE_TAB_H_
