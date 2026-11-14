#ifndef PROJECT_B_ENGINE_BROWSER_DIALOGS_H_
#define PROJECT_B_ENGINE_BROWSER_DIALOGS_H_

#include <cstdint>
#include <map>
#include <string>

#include "base/functional/callback.h"
#include "base/memory/raw_ptr.h"
#include "content/public/browser/javascript_dialog_manager.h"

namespace project_b {

// A page's alert, confirm, prompt and leave-page question (docs/daily-driver.md). Go owns
// each: the engine tells it of one and waits, with the page held, until Go answers by
// the dialog's id; the shell draws it. When content gives up on one, as when its tab
// navigates or closes, Go hears that it went.
class Dialogs : public content::JavaScriptDialogManager {
 public:
  Dialogs();
  Dialogs(const Dialogs&) = delete;
  Dialogs& operator=(const Dialogs&) = delete;
  ~Dialogs() override;

  // The one for every tab, made when first asked for.
  static Dialogs* Get();

  // Go's answer to dialog id: OK or not, and a prompt's text.
  void Answer(int64_t id, bool ok, const std::string& text);

  // content::JavaScriptDialogManager:
  void RunJavaScriptDialog(content::WebContents* web_contents,
                           content::RenderFrameHost* render_frame_host,
                           content::JavaScriptDialogType dialog_type,
                           const std::u16string& message_text,
                           const std::u16string& default_prompt_text,
                           DialogClosedCallback callback,
                           bool* did_suppress_message) override;
  void RunBeforeUnloadDialog(content::WebContents* web_contents,
                             content::RenderFrameHost* render_frame_host,
                             bool is_reload,
                             DialogClosedCallback callback) override;
  bool HandleJavaScriptDialog(content::WebContents* web_contents,
                              bool accept,
                              const std::u16string* prompt_override) override;
  void CancelDialogs(content::WebContents* web_contents, bool reset_state) override;

 private:
  struct Pending {
    raw_ptr<content::WebContents> contents;
    DialogClosedCallback callback;
  };

  void Open(content::WebContents* contents,
            content::RenderFrameHost* frame,
            int32_t kind,
            const std::u16string& message,
            const std::u16string& default_text,
            bool is_reload,
            DialogClosedCallback callback);

  int64_t next_id_ = 1;
  std::map<int64_t, Pending> pending_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_DIALOGS_H_
