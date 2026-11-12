#include "project_b/engine/extensions/browser/popup.h"

#include <utility>

#include "components/input/native_web_keyboard_event.h"
#include "content/public/browser/browser_task_traits.h"
#include "content/public/browser/browser_thread.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/render_widget_host_view.h"
#include "content/public/browser/web_contents.h"
#include "extensions/common/mojom/view_type.mojom.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/shell_page.h"
#include "project_b/engine/browser/window.h"
#include "ui/events/keycodes/keyboard_codes.h"

namespace project_b {

namespace {

// The smallest and largest a popup's page may make it, as in Chrome.
constexpr gfx::Size kMinSize(25, 25);
constexpr gfx::Size kMaxSize(800, 600);

}  // namespace

ExtensionPopup::ExtensionPopup(const extensions::Extension* extension,
                               content::BrowserContext* browser_context,
                               int64_t window,
                               const GURL& url,
                               const gfx::RectF& anchor,
                               base::OnceClosure on_close)
    : extensions::ExtensionHost(
          extension,
          browser_context,
          url,
          extensions::mojom::ViewType::kExtensionPopup),
      window_(window),
      anchor_(anchor),
      size_(kMinSize),
      on_close_(std::move(on_close)) {
  // The page closing itself comes through here.
  SetCloseHandler(base::BindOnce(
      [](base::WeakPtr<ExtensionPopup> self, extensions::ExtensionHost* host) {
        if (self) {
          self->AskToClose();
        }
      },
      weak_factory_.GetWeakPtr()));
  CreateRendererSoon();
}

ExtensionPopup::~ExtensionPopup() {
  if (shown_ && window()) {
    window()->window->RemovePopupView(host_contents()->GetNativeView());
  }
}

Engine::AppWindow* ExtensionPopup::window() const {
  return Engine::Get() ? Engine::Get()->window(window_) : nullptr;
}

// The popup may be asking from inside one of its own events, so it goes on a later task.
void ExtensionPopup::AskToClose() {
  if (on_close_) {
    content::GetUIThreadTaskRunner({})->PostTask(FROM_HERE,
                                                 std::move(on_close_));
  }
}

// The view goes up once the page has loaded and laid out, so it is never seen empty.
void ExtensionPopup::OnDidStopFirstLoad() {
  if (!window()) {
    AskToClose();
    return;
  }
  shown_ = true;
  window()->window->SetPopupView(host_contents()->GetNativeView(), anchor_,
                                 size_);
  host_contents()->Focus();
}

bool ExtensionPopup::IsBackgroundPage() const {
  return false;
}

void ExtensionPopup::RenderFrameCreated(content::RenderFrameHost* frame_host) {
  extensions::ExtensionHost::RenderFrameCreated(frame_host);
  // The page's own frame, which may be made before the page it is for is shown.
  if (!frame_host->GetParentOrOuterDocument()) {
    frame_host->GetView()->EnableAutoResize(kMinSize, kMaxSize);
  }
}

void ExtensionPopup::ResizeDueToAutoResize(content::WebContents* web_contents,
                                           const gfx::Size& new_size) {
  size_ = new_size;
  if (shown_ && window()) {
    window()->window->SetPopupView(host_contents()->GetNativeView(), anchor_,
                                   size_);
  }
}

bool ExtensionPopup::HandleKeyboardEvent(
    content::WebContents* source,
    const input::NativeWebKeyboardEvent& event) {
  if (event.GetType() == blink::WebInputEvent::Type::kRawKeyDown &&
      event.windows_key_code == ui::VKEY_ESCAPE) {
    AskToClose();
    return true;
  }
  return extensions::ExtensionHost::HandleKeyboardEvent(source, event);
}

// Who has the keyboard now is known once the press that moved it is done, a task later.
void ExtensionPopup::OnWebContentsLostFocus(
    content::RenderWidgetHost* render_widget_host) {
  if (shown_) {
    content::GetUIThreadTaskRunner({})->PostTask(
        FROM_HERE, base::BindOnce(&ExtensionPopup::CloseUnlessShellHasFocus,
                                  weak_factory_.GetWeakPtr()));
  }
}

void ExtensionPopup::CloseUnlessShellHasFocus() {
  if (!window() ||
      !window()->shell->contents()->GetRenderWidgetHostView()->HasFocus()) {
    AskToClose();
  }
}

}  // namespace project_b
