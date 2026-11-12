#ifndef PROJECT_B_ENGINE_EXTENSIONS_BROWSER_POPUP_H_
#define PROJECT_B_ENGINE_EXTENSIONS_BROWSER_POPUP_H_

#include "base/functional/callback.h"
#include "base/memory/raw_ptr.h"
#include "base/memory/weak_ptr.h"
#include "extensions/browser/extension_host.h"
#include "ui/gfx/geometry/rect_f.h"
#include "ui/gfx/geometry/size.h"

namespace project_b {

// An extension's popup: a page of the extension in a WebContents of its own, shown in a
// view under the toolbar button that opened it, over the shell and the page. It takes
// the size its page lays out to, as Chrome's does, and asks to be closed when the page
// closes itself, the user presses Escape, or the keyboard goes to a page. When the
// keyboard goes to the shell it stays: the shell knows which press was on the popup's
// own button, which closes it, and closes it for any other.
class ExtensionPopup : public extensions::ExtensionHost {
 public:
  // on_close runs, on a later task, when the popup should go; the owner destroys it.
  ExtensionPopup(const extensions::Extension* extension,
                 content::BrowserContext* browser_context,
                 int64_t window,
                 const GURL& url,
                 const gfx::RectF& anchor,
                 base::OnceClosure on_close);
  ExtensionPopup(const ExtensionPopup&) = delete;
  ExtensionPopup& operator=(const ExtensionPopup&) = delete;
  ~ExtensionPopup() override;

 private:
  void AskToClose();
  void CloseUnlessShellHasFocus();

  // extensions::ExtensionHost:
  void OnDidStopFirstLoad() override;
  bool IsBackgroundPage() const override;
  void RenderFrameCreated(content::RenderFrameHost* frame_host) override;
  void ResizeDueToAutoResize(content::WebContents* web_contents,
                             const gfx::Size& new_size) override;
  bool HandleKeyboardEvent(content::WebContents* source,
                           const input::NativeWebKeyboardEvent& event) override;
  void OnWebContentsLostFocus(
      content::RenderWidgetHost* render_widget_host) override;

  // Go's id for the window the popup is over. The windows close before the extensions
  // do at shutdown, so it may be gone.
  const int64_t window_;
  const gfx::RectF anchor_;
  gfx::Size size_;
  bool shown_ = false;
  base::OnceClosure on_close_;
  base::WeakPtrFactory<ExtensionPopup> weak_factory_{this};
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_EXTENSIONS_BROWSER_POPUP_H_
