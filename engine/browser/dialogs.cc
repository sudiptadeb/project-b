#include "project_b/engine/browser/dialogs.h"

#include <utility>
#include <vector>

#include "base/functional/bind.h"
#include "base/memory/weak_ptr.h"
#include "base/no_destructor.h"
#include "base/task/single_thread_task_runner.h"
#include "net/base/auth.h"
#include "base/strings/utf_string_conversions.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"
#include "url/origin.h"

namespace project_b {

namespace {

int32_t KindOf(content::JavaScriptDialogType type) {
  switch (type) {
    case content::JAVASCRIPT_DIALOG_TYPE_CONFIRM:
      return PB_DIALOG_CONFIRM;
    case content::JAVASCRIPT_DIALOG_TYPE_PROMPT:
      return PB_DIALOG_PROMPT;
    default:
      return PB_DIALOG_ALERT;
  }
}

}  // namespace

Dialogs::Dialogs() = default;

Dialogs::~Dialogs() = default;

// static
Dialogs* Dialogs::Get() {
  static base::NoDestructor<Dialogs> dialogs;
  return dialogs.get();
}

void Dialogs::RunJavaScriptDialog(content::WebContents* web_contents,
                                  content::RenderFrameHost* render_frame_host,
                                  content::JavaScriptDialogType dialog_type,
                                  const std::u16string& message_text,
                                  const std::u16string& default_prompt_text,
                                  DialogClosedCallback callback,
                                  bool* did_suppress_message) {
  Open(web_contents, render_frame_host, KindOf(dialog_type), message_text,
       default_prompt_text, false, std::move(callback));
}

void Dialogs::RunBeforeUnloadDialog(content::WebContents* web_contents,
                                    content::RenderFrameHost* render_frame_host,
                                    bool is_reload,
                                    DialogClosedCallback callback) {
  Open(web_contents, render_frame_host, PB_DIALOG_BEFOREUNLOAD, std::u16string(),
       std::u16string(), is_reload, std::move(callback));
}

void Dialogs::Open(content::WebContents* contents,
                   content::RenderFrameHost* frame,
                   int32_t kind,
                   const std::u16string& message,
                   const std::u16string& default_text,
                   bool is_reload,
                   DialogClosedCallback callback) {
  int64_t tab = hooks::TabOf(contents);
  if (tab <= 0) {
    // Our own pages ask nothing.
    std::move(callback).Run(false, std::u16string());
    return;
  }
  int64_t id = next_id_++;
  pending_[id] = Pending{contents, std::move(callback)};
  hooks::Args args;
  args.Int(id);
  args.Int(tab);
  args.Int(kind);
  args.Str(base::UTF16ToUTF8(message));
  args.Str(base::UTF16ToUTF8(default_text));
  args.Str(frame ? frame->GetLastCommittedOrigin().Serialize() : std::string());
  args.Bool(is_reload);
  hooks::Fire(PB_EVENT_DIALOG_OPENED, args);
}

// One sign-in while content waits for it. Content drops it when the request ends another
// way, as when its tab navigates; Go then hears the dialog went.
class Dialogs::SignInRequest : public content::LoginDelegate {
 public:
  SignInRequest(Dialogs* dialogs, int64_t id, LoginAuthRequiredCallback callback)
      : dialogs_(dialogs), id_(id), callback_(std::move(callback)) {}
  ~SignInRequest() override {
    if (dialogs_->sign_ins_.erase(id_)) {
      hooks::Args args;
      args.Int(id_);
      hooks::Fire(PB_EVENT_DIALOG_CLOSED, args);
    }
  }

  // Answers content, not from inside its own call, as it asks.
  void Answer(std::optional<net::AuthCredentials> credentials) {
    dialogs_->sign_ins_.erase(id_);
    base::SingleThreadTaskRunner::GetCurrentDefault()->PostTask(
        FROM_HERE, base::BindOnce(&SignInRequest::Run, weak_factory_.GetWeakPtr(),
                                  std::move(credentials)));
  }

 private:
  void Run(std::optional<net::AuthCredentials> credentials) {
    std::move(callback_).Run(credentials);
  }

  raw_ptr<Dialogs> dialogs_;
  const int64_t id_;
  LoginAuthRequiredCallback callback_;
  base::WeakPtrFactory<SignInRequest> weak_factory_{this};
};

std::unique_ptr<content::LoginDelegate> Dialogs::SignIn(
    content::WebContents* contents,
    const net::AuthChallengeInfo& auth_info,
    content::LoginDelegate::LoginAuthRequiredCallback callback) {
  int64_t tab = contents ? hooks::TabOf(contents) : 0;
  if (tab <= 0) {
    return nullptr;
  }
  int64_t id = next_id_++;
  auto request = std::make_unique<SignInRequest>(this, id, std::move(callback));
  sign_ins_[id] = request.get();
  hooks::Args args;
  args.Int(id);
  args.Int(tab);
  args.Int(PB_DIALOG_SIGN_IN);
  args.Str(auth_info.realm);
  args.Str(std::string());
  args.Str(auth_info.challenger.Serialize());
  args.Bool(auth_info.is_proxy);
  hooks::Fire(PB_EVENT_DIALOG_OPENED, args);
  return request;
}

void Dialogs::AnswerSignIn(int64_t id,
                           bool ok,
                           const std::string& username,
                           const std::string& password) {
  auto it = sign_ins_.find(id);
  if (it == sign_ins_.end()) {
    return;
  }
  SignInRequest* request = it->second;
  if (ok) {
    request->Answer(net::AuthCredentials(base::UTF8ToUTF16(username), base::UTF8ToUTF16(password)));
  } else {
    request->Answer(std::nullopt);
  }
}

void Dialogs::Answer(int64_t id, bool ok, const std::string& text) {
  auto it = pending_.find(id);
  if (it == pending_.end()) {
    return;
  }
  DialogClosedCallback callback = std::move(it->second.callback);
  pending_.erase(it);
  std::move(callback).Run(ok, base::UTF8ToUTF16(text));
}

bool Dialogs::HandleJavaScriptDialog(content::WebContents* web_contents,
                                     bool accept,
                                     const std::u16string* prompt_override) {
  for (auto& [id, pending] : pending_) {
    if (pending.contents == web_contents) {
      int64_t answered = id;
      // Go hears it went, as when content cancels it, then the page has its answer.
      hooks::Args args;
      args.Int(answered);
      hooks::Fire(PB_EVENT_DIALOG_CLOSED, args);
      Answer(answered, accept, prompt_override ? base::UTF16ToUTF8(*prompt_override) : "");
      return true;
    }
  }
  return false;
}

void Dialogs::CancelDialogs(content::WebContents* web_contents, bool reset_state) {
  std::vector<int64_t> gone;
  for (auto& [id, pending] : pending_) {
    if (pending.contents == web_contents) {
      gone.push_back(id);
    }
  }
  for (int64_t id : gone) {
    pending_.erase(id);
    hooks::Args args;
    args.Int(id);
    hooks::Fire(PB_EVENT_DIALOG_CLOSED, args);
  }
}

}  // namespace project_b
