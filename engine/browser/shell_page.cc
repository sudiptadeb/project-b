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

constexpr char kHost[] = "shell";

// Forwards chrome.send('shell', [json]) to Go.
class ShellMessageHandler : public content::WebUIMessageHandler {
 public:
  void RegisterMessages() override {
    web_ui()->RegisterMessageCallback(
        "shell", base::BindRepeating(&ShellMessageHandler::OnMessage,
                                     base::Unretained(this)));
  }

 private:
  void OnMessage(const base::ListValue& args) {
    if (args.size() != 1 || !args[0].is_string()) {
      LOG(WARNING) << "dropped a malformed message from the shell";
      return;
    }
    hooks::Args hook_args;
    hook_args.Str(args[0].GetString());
    hooks::Fire(PB_EVENT_SHELL_MESSAGE, hook_args);
  }
};

// The bridge to Go belongs to the shell's own WebContents. The factory sees only the URL,
// so content makes a controller for chrome://shell/ wherever it loads; anywhere else the
// controller takes away the page's bindings, so it has no chrome.send, and drops
// whatever arrives anyway.
class ShellController : public content::WebUIController {
 public:
  explicit ShellController(content::WebUI* web_ui)
      : content::WebUIController(web_ui),
        is_shell_(web_ui->GetWebContents() ==
                  Engine::Get()->shell()->contents()) {
    if (!is_shell_) {
      LOG(WARNING) << "chrome://shell/ outside the shell: no bindings";
      web_ui->SetBindings(content::BindingsPolicySet());
      return;
    }
    web_ui->AddMessageHandler(std::make_unique<ShellMessageHandler>());
  }

  bool OverrideHandleWebUIMessage(const GURL& source_url,
                                  const std::string& message,
                                  const base::ListValue& args) override {
    if (is_shell_) {
      return false;
    }
    LOG(WARNING) << "dropped \"" << message
                 << "\" from chrome://shell/ outside the shell";
    return true;
  }

  WEB_UI_CONTROLLER_TYPE_DECL();

 private:
  const bool is_shell_;
};

WEB_UI_CONTROLLER_TYPE_IMPL(ShellController)

class ShellControllerFactory : public content::WebUIControllerFactory {
 public:
  std::unique_ptr<content::WebUIController> CreateWebUIControllerForURL(
      content::WebUI* web_ui,
      const GURL& url) override {
    return IsShell(url) ? std::make_unique<ShellController>(web_ui) : nullptr;
  }

  content::WebUI::TypeID GetWebUIType(content::BrowserContext* browser_context,
                                      const GURL& url) override {
    return IsShell(url) ? reinterpret_cast<content::WebUI::TypeID>(0x1)
                        : content::WebUI::kNoWebUI;
  }

  bool UseWebUIForURL(content::BrowserContext* browser_context,
                      const GURL& url) override {
    return IsShell(url);
  }

 private:
  static bool IsShell(const GURL& url) {
    return url.SchemeIs("chrome") && url.host() == kHost;
  }
};

// Serves the built shell from disk. No remote content ever reaches this page.
class ShellDataSource : public content::URLDataSource {
 public:
  std::string GetSource() override { return kHost; }

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
      // The inline <style> in index.html paints before any JS has run.
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
  static std::string Name(const GURL& url) {
    std::string path(url.path());
    if (path == "/shell.js") {
      return "shell.js";
    }
    if (path == "/shell.css") {
      return "shell.css";
    }
    return "index.html";
  }
};

}  // namespace

void RegisterShellWebUI(content::BrowserContext* browser_context) {
  static base::NoDestructor<ShellControllerFactory> factory;
  content::WebUIControllerFactory::RegisterFactory(factory.get());
  content::URLDataSource::Add(browser_context,
                              std::make_unique<ShellDataSource>());
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
  if (content::WebUI* web_ui = contents_->GetWebUI()) {
    std::vector<base::ValueView> args = {json};
    web_ui->CallJavascriptFunctionUnsafe("__shellReceive", args);
  }
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

void ShellPage::DidFirstVisuallyNonEmptyPaint() {
  if (first_paint_.is_zero()) {
    first_paint_ = base::Time::Now() - base::Process::Current().CreationTime();
    LOG(INFO) << "shell first paint " << first_paint_.InMilliseconds()
              << "ms after process start";
  }
}

// Go's window manager takes back what a dead shell claimed, so the page keeps working
// until the new shell reports its layout again.
void ShellPage::PrimaryMainFrameRenderProcessGone(
    base::TerminationStatus status) {
  hooks::Fire(PB_EVENT_SHELL_CRASHED, hooks::Args());
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
