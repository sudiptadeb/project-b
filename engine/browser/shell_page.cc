#include "project_b/engine/browser/shell_page.h"

#include <utility>
#include <vector>

#include "base/check.h"
#include "base/containers/to_vector.h"
#include "base/files/file_path.h"
#include "base/files/file_util.h"
#include "base/functional/bind.h"
#include "base/memory/ref_counted_memory.h"
#include "base/path_service.h"
#include "base/process/process.h"
#include "base/strings/string_util.h"
#include "base/task/thread_pool.h"
#include "base/values.h"
#include "content/public/browser/navigation_controller.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/render_widget_host_view.h"
#include "content/public/browser/url_data_source.h"
#include "content/public/browser/web_contents.h"
#include "content/public/browser/web_ui.h"
#include "content/public/browser/web_ui_controller.h"
#include "content/public/browser/web_ui_controller_factory.h"
#include "content/public/browser/web_ui_message_handler.h"
#include "content/public/common/bindings_policy.h"
#include "project_b/engine/browser/engine.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/window.h"
#include "services/network/public/mojom/content_security_policy.mojom.h"
#include "third_party/skia/include/core/SkColor.h"
#include "url/gurl.h"

namespace project_b {

namespace {

// Our two internal pages. Each binds only in its own kind of WebContents: the shell's page
// in the shell's, the settings page in a tab's.
constexpr char kShellHost[] = "shell";
constexpr char kSettingsHost[] = "settings";

bool IsOurs(const GURL& url) {
  return url.SchemeIs("chrome") &&
         (url.host() == kShellHost || url.host() == kSettingsHost);
}

// Forwards chrome.send('shell', [json]) to Go, tagged with who sent it: 0 for the shell,
// or the id of the tab the page is in. Go decides what each sender may do.
class ShellMessageHandler : public content::WebUIMessageHandler {
 public:
  explicit ShellMessageHandler(int64_t sender) : sender_(sender) {}

  void RegisterMessages() override {
    web_ui()->RegisterMessageCallback(
        "shell", base::BindRepeating(&ShellMessageHandler::OnMessage,
                                     base::Unretained(this)));
  }

 private:
  void OnMessage(const base::ListValue& args) {
    if (args.size() != 1 || !args[0].is_string()) {
      LOG(WARNING) << "dropped a malformed message from sender " << sender_;
      return;
    }
    hooks::Args hook_args;
    hook_args.Int(sender_);
    hook_args.Str(args[0].GetString());
    hooks::Fire(PB_EVENT_SHELL_MESSAGE, hook_args);
  }

  const int64_t sender_;
};

// The factory sees only the URL, so content makes a controller for our page wherever it
// loads. The controller gives the page its bridge only in the page's own WebContents: the
// shell's page in the shell's, tagged 0; the settings page in a tab's, tagged with the
// tab's id. Anywhere else it takes away the page's bindings, so it has no chrome.send,
// and drops whatever arrives anyway.
class PageController : public content::WebUIController {
 public:
  PageController(content::WebUI* web_ui, const GURL& url)
      : content::WebUIController(web_ui) {
    content::WebContents* contents = web_ui->GetWebContents();
    if (url.host() == kShellHost) {
      bound_ = contents == Engine::Get()->shell()->contents();
    } else {
      sender_ = hooks::TabOf(contents);
      bound_ = sender_ != 0;
    }
    if (!bound_) {
      LOG(WARNING) << url.spec() << " outside its own WebContents: no bindings";
      web_ui->SetBindings(content::BindingsPolicySet());
      return;
    }
    web_ui->AddMessageHandler(std::make_unique<ShellMessageHandler>(sender_));
  }

  bool bound() const { return bound_; }

  bool OverrideHandleWebUIMessage(const GURL& source_url,
                                  const std::string& message,
                                  const base::ListValue& args) override {
    if (bound_) {
      return false;
    }
    LOG(WARNING) << "dropped \"" << message << "\" from " << source_url.spec()
                 << " outside its own WebContents";
    return true;
  }

  WEB_UI_CONTROLLER_TYPE_DECL();

 private:
  bool bound_ = false;
  int64_t sender_ = 0;
};

WEB_UI_CONTROLLER_TYPE_IMPL(PageController)

class PageControllerFactory : public content::WebUIControllerFactory {
 public:
  std::unique_ptr<content::WebUIController> CreateWebUIControllerForURL(
      content::WebUI* web_ui,
      const GURL& url) override {
    return IsOurs(url) ? std::make_unique<PageController>(web_ui, url) : nullptr;
  }

  content::WebUI::TypeID GetWebUIType(content::BrowserContext* browser_context,
                                      const GURL& url) override {
    if (!IsOurs(url)) {
      return content::WebUI::kNoWebUI;
    }
    return url.host() == kShellHost
               ? reinterpret_cast<content::WebUI::TypeID>(0x1)
               : reinterpret_cast<content::WebUI::TypeID>(0x2);
  }

  bool UseWebUIForURL(content::BrowserContext* browser_context,
                      const GURL& url) override {
    return IsOurs(url);
  }
};

// Serves one of our pages from the folder the Vue repo builds into, beside the binary: the
// shell from index.html, shell.js and shell.css; the settings page from settings.html,
// settings.js and settings.css. No remote content ever reaches either.
class PageDataSource : public content::URLDataSource {
 public:
  explicit PageDataSource(std::string host) : host_(std::move(host)) {}

  std::string GetSource() override { return host_; }

  void StartDataRequest(const GURL& url,
                        const content::WebContents::Getter& wc_getter,
                        GotDataCallback callback) override {
    base::FilePath dir;
    CHECK(base::PathService::Get(base::DIR_ASSETS, &dir));
    std::string name = Name(url);
    base::FilePath file = dir.Append("shell").AppendASCII(name);
    base::ThreadPool::PostTaskAndReplyWithResult(
        FROM_HERE, {base::MayBlock(), base::TaskPriority::USER_BLOCKING},
        base::BindOnce(
            [](const base::FilePath& file) -> scoped_refptr<base::RefCountedMemory> {
              std::string data;
              if (!base::ReadFileToString(file, &data)) {
                return nullptr;
              }
              return base::MakeRefCounted<base::RefCountedString>(std::move(data));
            },
            file),
        std::move(callback));
  }

  std::string GetMimeType(const GURL& url) override {
    std::string name = Name(url);
    if (base::EndsWith(name, ".js")) {
      return "text/javascript";
    }
    if (base::EndsWith(name, ".css")) {
      return "text/css";
    }
    return "text/html";
  }

  bool ShouldServeMimeTypeAsContentTypeHeader() override { return true; }

  std::string GetContentSecurityPolicy(
      network::mojom::CSPDirectiveName directive) override {
    using Name = network::mojom::CSPDirectiveName;
    switch (directive) {
      case Name::DefaultSrc:
        return "default-src 'none';";
      case Name::ScriptSrc:
        return "script-src 'self';";
      // The inline <style> in each page paints before any JS has run.
      case Name::StyleSrc:
        return "style-src 'self' 'unsafe-inline';";
      case Name::ImgSrc:
        return "img-src 'self' data:;";
      case Name::RequireTrustedTypesFor:
        return "require-trusted-types-for 'script';";
      case Name::TrustedTypes:
        return "trusted-types vue;";
      default:
        return content::URLDataSource::GetContentSecurityPolicy(directive);
    }
  }

 private:
  // Only these files exist, so a path can't reach outside the folder.
  std::string Name(const GURL& url) const {
    const std::string path(url.path());
    if (path == "/" + host_ + ".js") {
      return host_ + ".js";
    }
    if (path == "/" + host_ + ".css") {
      return host_ + ".css";
    }
    return host_ == kShellHost ? "index.html" : host_ + ".html";
  }

  const std::string host_;
};

}  // namespace

void RegisterShellWebUI(content::BrowserContext* browser_context) {
  static base::NoDestructor<PageControllerFactory> factory;
  content::WebUIControllerFactory::RegisterFactory(factory.get());
  content::URLDataSource::Add(browser_context,
                              std::make_unique<PageDataSource>(kShellHost));
  content::URLDataSource::Add(browser_context,
                              std::make_unique<PageDataSource>(kSettingsHost));
}

void SendToOurPage(content::WebContents* contents, const std::string& json) {
  content::WebUI* web_ui = contents->GetWebUI();
  if (!web_ui) {
    return;
  }
  auto* page = web_ui->GetController()->GetAs<PageController>();
  if (!page || !page->bound()) {
    return;
  }
  std::vector<base::ValueView> args = {json};
  web_ui->CallJavascriptFunctionUnsafe("__shellReceive", args);
}

ShellPage::ShellPage(content::BrowserContext* browser_context)
    : contents_(content::WebContents::Create(
          content::WebContents::CreateParams(browser_context))) {
  Observe(contents_.get());
  contents_->SetDelegate(this);
  contents_->SetPageBaseBackgroundColor(SK_ColorTRANSPARENT);
}

ShellPage::~ShellPage() {
  contents_->SetDelegate(nullptr);
}

void ShellPage::Load() {
  contents_->GetController().LoadURLWithParams(
      content::NavigationController::LoadURLParams(GURL("chrome://shell/")));
}

void ShellPage::Send(const std::string& json) {
  SendToOurPage(contents_.get(), json);
}

// The shell's view is transparent wherever the shell paints nothing, so the page under
// it shows. Content treats a view it was never told about as opaque, and the page area
// then stays white until the shell next paints. The view belongs to the main frame, so
// it is set as each main frame comes alive: the first one, one after a renderer crash,
// and any later swap. A frame change is not enough: the first frame is made inside
// WebContents::Create, before this observes, and the shell loads into that same frame.
void ShellPage::RenderFrameCreated(content::RenderFrameHost* frame) {
  if (frame->GetParent()) {
    return;
  }
  if (content::RenderWidgetHostView* view = frame->GetView()) {
    view->SetBackgroundColor(SK_ColorTRANSPARENT);
  }
}

// Content reports this once the frame has been presented, so the window can show it.
void ShellPage::DidFirstVisuallyNonEmptyPaint() {
  if (first_paint_.is_zero()) {
    first_paint_ = base::Time::Now() - base::Process::Current().CreationTime();
    LOG(INFO) << "shell first paint " << first_paint_.InMilliseconds()
              << "ms after process start";
  }
  Engine::Get()->window()->ShellPainted();
}

// A dead shell claims nothing, so the page keeps working until the new shell asks
// for a fresh snapshot.
void ShellPage::PrimaryMainFrameRenderProcessGone(
    base::TerminationStatus status) {
  Engine::Get()->window()->SetShellRegions({});
  contents_->GetController().Reload(content::ReloadType::NORMAL, false);
}

bool ShellPage::HandleKeyboardEvent(
    content::WebContents* source,
    const input::NativeWebKeyboardEvent& event) {
  return Engine::Get()->window()->HandleUnhandledKey(event);
}

bool ShellPage::TakeFocus(content::WebContents* source, bool reverse) {
  Engine::Get()->FocusPageForTraversal(reverse);
  return true;
}

content::WebContents* ShellPage::hook_contents() const {
  return contents_.get();
}

}  // namespace project_b
