#include "project_b/engine/browser/shell_page.h"

#include <algorithm>
#include <string_view>
#include <utility>
#include <vector>

#include "base/check.h"
#include "base/containers/to_vector.h"
#include "base/files/file_path.h"
#include "base/files/file_util.h"
#include "base/functional/bind.h"
#include "base/location.h"
#include "base/memory/ref_counted_memory.h"
#include "base/memory/weak_ptr.h"
#include "base/no_destructor.h"
#include "base/path_service.h"
#include "base/strings/strcat.h"
#include "base/strings/string_util.h"
#include "base/task/thread_pool.h"
#include "base/values.h"
#include "build/build_config.h"
#include "content/public/browser/browser_context.h"
#include "content/public/browser/browser_task_traits.h"
#include "content/public/browser/browser_thread.h"
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
#include "project_b/engine/browser/settings_page.h"
#include "project_b/engine/browser/timeline.h"
#include "project_b/engine/browser/window.h"
#include "services/network/public/mojom/content_security_policy.mojom.h"
#include "third_party/skia/include/core/SkColor.h"
#include "url/gurl.h"

#if BUILDFLAG(IS_ANDROID)
#include "base/android/apk_assets.h"
#include "base/files/file.h"
#include "base/files/memory_mapped_file.h"
#include "base/strings/string_view_util.h"
#endif

namespace project_b {

namespace {

// Our pages, each an app of its own (shell/src/apps/<app>) at chrome://<app>/, served from
// shell/<app>/ beside the binary. Every shell is an app of "shell": shell <id> is
// chrome://shell/<id>/, served from shell/shell/<id>/. Each window has the one WebContents
// each belongs in, and their messages cross to Go as one event, with the window.
struct App {
  const char* host;
  PBEvent event;
};
constexpr App kApps[] = {
    {"shell", PB_EVENT_SHELL_MESSAGE},
    {"settings", PB_EVENT_SETTINGS_MESSAGE},
    {"picker", PB_EVENT_PICKER_MESSAGE},
};

const App* AppFor(const GURL& url) {
  if (!url.SchemeIs("chrome")) {
    return nullptr;
  }
  for (const App& app : kApps) {
    if (url.host() == app.host) {
      return &app;
    }
  }
  return nullptr;
}

// The window whose page for app this WebContents is: the page covering it when that is
// app's, or its settings page while that is open. 0 when it is no window's.
int64_t HomeWindow(const App& app, content::WebContents* contents) {
  for (const auto& [id, w] : Engine::Get()->windows()) {
    content::WebContents* home =
        app.event == PB_EVENT_SETTINGS_MESSAGE
            ? (w->settings ? w->settings->contents() : nullptr)
        : w->shell->app() == app.host ? w->shell->contents()
                                      : nullptr;
    if (home == contents) {
      return id;
    }
  }
  return 0;
}

// Forwards chrome.send(name, [json]) to Go as one event, with the window: 'shell' from a
// shell, 'settings' from a settings page.
class ShellMessageHandler : public content::WebUIMessageHandler {
 public:
  ShellMessageHandler(const char* name, PBEvent event, int64_t window)
      : name_(name), event_(event), window_(window) {}

  void RegisterMessages() override {
    web_ui()->RegisterMessageCallback(
        name_, base::BindRepeating(&ShellMessageHandler::OnMessage,
                                   base::Unretained(this)));
  }

 private:
  void OnMessage(const base::ListValue& args) {
    if (args.size() != 1 || !args[0].is_string()) {
      LOG(WARNING) << "dropped a malformed message from " << name_;
      return;
    }
    // One sent just before its window closed arrives with the window gone.
    if (!Engine::Get()->window(window_)) {
      return;
    }
    hooks::Args hook_args;
    hook_args.Int(window_);
    hook_args.Str(args[0].GetString());
    hooks::Fire(event_, hook_args);
  }

  const char* const name_;
  const PBEvent event_;
  const int64_t window_;
};

// The bridge to Go belongs to each app's own WebContents in a window. The factory sees only the URL,
// so content makes a controller for chrome://shell/ or chrome://settings/ wherever it
// loads; anywhere else the controller takes away the page's bindings, so it has no
// chrome.send, and drops whatever arrives anyway.
class AppController : public content::WebUIController {
 public:
  AppController(content::WebUI* web_ui, const App& app)
      : content::WebUIController(web_ui),
        app_(app),
        window_(HomeWindow(app, web_ui->GetWebContents())) {
    if (!window_) {
      LOG(WARNING) << "chrome://" << app_.host << "/ outside its page: no bindings";
      web_ui->SetBindings(content::BindingsPolicySet());
      return;
    }
    web_ui->AddMessageHandler(
        std::make_unique<ShellMessageHandler>(app_.host, app_.event, window_));
  }

  bool OverrideHandleWebUIMessage(const GURL& source_url,
                                  const std::string& message,
                                  const base::ListValue& args) override {
    if (window_) {
      return false;
    }
    LOG(WARNING) << "dropped \"" << message << "\" from chrome://" << app_.host
                 << "/ outside its page";
    return true;
  }

  WEB_UI_CONTROLLER_TYPE_DECL();

 private:
  const App app_;
  // The window this is the page of, or 0 when it is outside its page.
  const int64_t window_;
};

WEB_UI_CONTROLLER_TYPE_IMPL(AppController)

// Our pages are WebUI only in our pages' context. Anywhere else, as in a tab, their
// address is no page at all. The factory lives as long as the process, past the context.
class AppControllerFactory : public content::WebUIControllerFactory {
 public:
  explicit AppControllerFactory(content::BrowserContext* pages_context)
      : pages_context_(pages_context->GetWeakPtr()) {}

  std::unique_ptr<content::WebUIController> CreateWebUIControllerForURL(
      content::WebUI* web_ui,
      const GURL& url) override {
    const App* app = AppFor(url);
    return app ? std::make_unique<AppController>(web_ui, *app) : nullptr;
  }

  content::WebUI::TypeID GetWebUIType(content::BrowserContext* browser_context,
                                      const GURL& url) override {
    const App* app = AppIn(browser_context, url);
    return app ? reinterpret_cast<content::WebUI::TypeID>(app)
               : content::WebUI::kNoWebUI;
  }

  bool UseWebUIForURL(content::BrowserContext* browser_context,
                      const GURL& url) override {
    return AppIn(browser_context, url) != nullptr;
  }

 private:
  const App* AppIn(content::BrowserContext* browser_context,
                   const GURL& url) const {
    return browser_context == pages_context_.get() ? AppFor(url) : nullptr;
  }

  const base::WeakPtr<content::BrowserContext> pages_context_;
};

// Serves one app's built files from disk. No remote content ever reaches its page.
class AppDataSource : public content::URLDataSource {
 public:
  explicit AppDataSource(const char* host) : host_(host) {}

  std::string GetSource() override { return host_; }

  void StartDataRequest(const GURL& url,
                        const content::WebContents::Getter& wc_getter,
                        GotDataCallback callback) override {
    std::string name = Name(url);
    if (name.empty()) {
      std::move(callback).Run(nullptr);
      return;
    }
#if BUILDFLAG(IS_ANDROID)
    // On Android the apps are in the APK, not in a folder.
    base::MemoryMappedFile::Region region;
    base::File asset(
        base::android::OpenApkAsset("assets/shell/" + name, &region));
    base::MemoryMappedFile mapped;
    CHECK(mapped.Initialize(std::move(asset), region));
    std::string data(base::as_string_view(mapped.bytes()));
    std::move(callback).Run(
        base::MakeRefCounted<base::RefCountedString>(std::move(data)));
    return;
#else
    base::FilePath dir;
    CHECK(base::PathService::Get(base::DIR_ASSETS, &dir));
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
#endif
  }

  std::string GetMimeType(const GURL& url) override {
    std::string name = Name(url);
    if (base::EndsWith(name, ".js")) {
      return "text/javascript";
    }
    if (base::EndsWith(name, ".css")) {
      return "text/css";
    }
    if (base::EndsWith(name, ".woff2")) {
      return "font/woff2";
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
      case Name::FontSrc:
        return "font-src 'self';";
      case Name::RequireTrustedTypesFor:
        return "require-trusted-types-for 'script';";
      // The policies the frameworks our pages are built with make for their templates.
      // The mock engine serves our pages with the same policy (core/cmd/dev, pageCSP).
      case Name::TrustedTypes:
        return "trusted-types vue svelte-trusted-html;";
      default:
        return content::URLDataSource::GetContentSecurityPolicy(directive);
    }
  }

 private:
  // The file a URL names, as a path in the folder of our pages, or "" for none. Settings
  // and the picker have their index.html, their one script and their one stylesheet; any
  // other path of theirs is their index.html. A shell's folder is its id, the path's first
  // part, and a shell has any file its build put in that folder, such as its fonts, by a
  // plain name: lower-case letters, digits, dashes and dots, not starting with a dot. So
  // no path reaches outside its folder.
  std::string Name(const GURL& url) const {
    std::string_view path = url.path().substr(1);
    if (host_ != "shell") {
      return host_ + "/" + (IsOwnFile(host_, path) ? std::string(path) : "index.html");
    }
    size_t slash = path.find('/');
    if (slash == std::string_view::npos) {
      return "";
    }
    std::string_view id = path.substr(0, slash);
    std::string_view file = path.substr(slash + 1);
    if (file.empty()) {
      file = "index.html";
    }
    if (id.empty() || !std::ranges::all_of(id, [](char c) {
          return base::IsAsciiLower(c) || base::IsAsciiDigit(c) || c == '-';
        }) ||
        file.front() == '.' || !std::ranges::all_of(file, [](char c) {
          return base::IsAsciiLower(c) || base::IsAsciiDigit(c) || c == '-' ||
                 c == '.';
        })) {
      return "";
    }
    return base::StrCat({"shell/", id, "/", file});
  }

  // Whether file is the script or the stylesheet of the app whose files are called name.
  static bool IsOwnFile(std::string_view name, std::string_view file) {
    return file == base::StrCat({name, ".js"}) ||
           file == base::StrCat({name, ".css"});
  }

  const std::string host_;
};

}  // namespace

void RegisterShellWebUI(content::BrowserContext* pages_context) {
  static base::NoDestructor<AppControllerFactory> factory(pages_context);
  content::WebUIControllerFactory::RegisterFactory(factory.get());
  for (const App& app : kApps) {
    content::URLDataSource::Add(pages_context,
                                std::make_unique<AppDataSource>(app.host));
  }
}

ShellPage::ShellPage(content::BrowserContext* browser_context,
                     int64_t window,
                     const std::string& page)
    : window_(window),
      page_(page),
      contents_(content::WebContents::Create(
          content::WebContents::CreateParams(browser_context))) {
  Observe(contents_.get());
  contents_->SetDelegate(this);
  contents_->SetPageBaseBackgroundColor(SK_ColorTRANSPARENT);
}

ShellPage::~ShellPage() {
  contents_->SetDelegate(nullptr);
}

std::string_view ShellPage::app() const {
  return std::string_view(page_).substr(0, page_.find('/'));
}

void ShellPage::Load() {
  contents_->GetController().LoadURLWithParams(
      content::NavigationController::LoadURLParams(
          GURL("chrome://" + page_ + "/")));
}

void ShellPage::SetPage(const std::string& page) {
  page_ = page;
  Load();
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

void ShellPage::DidFinishNavigation(content::NavigationHandle* handle) {
  timeline::PageCommitted(app(), handle);
}

// Go's window manager takes back what a dead shell claimed, so the page keeps working
// until the new shell reports its layout again.
void ShellPage::PrimaryMainFrameRenderProcessGone(
    base::TerminationStatus status) {
  hooks::Args args;
  args.Int(window_);
  hooks::Fire(PB_EVENT_SHELL_CRASHED, args);
  // The reload waits for a later task. Content is still telling its observers the
  // process is gone, and a renderer started before they have all heard is one that
  // those who track renderers lose.
  content::GetUIThreadTaskRunner({})->PostTask(
      FROM_HERE, base::BindOnce(
                     [](base::WeakPtr<ShellPage> self) {
                       if (self) {
                         self->contents_->GetController().Reload(
                             content::ReloadType::NORMAL, false);
                       }
                     },
                     weak_factory_.GetWeakPtr()));
}

// Go gets the key first; one it doesn't use goes to the menu bar.
bool ShellPage::HandleKeyboardEvent(
    content::WebContents* source,
    const input::NativeWebKeyboardEvent& event) {
  return WebContentsDelegateHooks::HandleKeyboardEvent(source, event) ||
         Window::HandleUnhandledKey(event);
}

bool ShellPage::TakeFocus(content::WebContents* source, bool reverse) {
  Engine::Get()->FocusPageForTraversal(window_, reverse);
  return true;
}

content::WebContents* ShellPage::hook_contents() const {
  return contents_.get();
}

}  // namespace project_b
