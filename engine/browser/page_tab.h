#ifndef PROJECT_B_ENGINE_BROWSER_PAGE_TAB_H_
#define PROJECT_B_ENGINE_BROWSER_PAGE_TAB_H_

#include <cstdint>
#include <memory>

#include "content/public/browser/web_contents_delegate.h"
#include "content/public/browser/web_contents_observer.h"

namespace project_b {

// One tab's connection to content. Everything content says goes straight to Go.
class PageTab : public content::WebContentsObserver,
                public content::WebContentsDelegate {
 public:
  PageTab(int64_t id, std::unique_ptr<content::WebContents> contents);
  PageTab(const PageTab&) = delete;
  PageTab& operator=(const PageTab&) = delete;
  ~PageTab() override;

  int64_t id() const { return id_; }
  content::WebContents* contents() const { return contents_.get(); }

 private:
  // content::WebContentsObserver:
  void DidFinishNavigation(content::NavigationHandle* handle) override;
  void DidStartLoading() override;
  void DidStopLoading() override;
  void TitleWasSet(content::NavigationEntry* entry) override;
  void PrimaryMainFrameRenderProcessGone(
      base::TerminationStatus status) override;

  // content::WebContentsDelegate:
  content::WebContents* OpenURLFromTab(
      content::WebContents* source,
      const content::OpenURLParams& params,
      base::OnceCallback<void(content::NavigationHandle&)> callback) override;
  content::KeyboardEventProcessingResult PreHandleKeyboardEvent(
      content::WebContents* source,
      const input::NativeWebKeyboardEvent& event) override;
  bool HandleKeyboardEvent(content::WebContents* source,
                           const input::NativeWebKeyboardEvent& event) override;
  bool TakeFocus(content::WebContents* source, bool reverse) override;

  const int64_t id_;
  std::unique_ptr<content::WebContents> contents_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_PAGE_TAB_H_
